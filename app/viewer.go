package main

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed viewer.html search.html
var uiPage embed.FS

type viewer struct {
	db              *sql.DB
	cfg             Config
	client          *Client
	root            string
	galleryRoot     string
	cacheRoot       string
	acquisitionRoot string
	infoCache       map[int]*infoCacheEntry
	infoCacheMu     sync.Mutex
	infoCacheBytes  int
	infoCacheMaxEntries int
	infoCacheMaxBytes   int

	// Phase 3（decision 2026-11-05）: gallery 情報の永続キャッシュ層（memory → disk → remote）。
	// 画像バイト経路（imageProxy・TN・full cache）とは分離。置き場は完成フォルダ内
	// cache/galleryinfo/<id>.json。
	infoCacheRoot   string
	infoDiskTTL     time.Duration
	infoDiskMaxEntries int
	infoDiskMaxBytes   int
	infoDiskMu      sync.Mutex
	infoDiskCount   int
	infoDiskBytes   int64
	infoDiskSynced  bool
	infoDiskStaleHits int

	imageFetchMu          sync.Mutex
	imageFetchGroup       map[string]*imageFetchCall
	imageFetchJoinObserver func(hashFormatKey string)

	cacheWriteMu              sync.Mutex
	cacheWriteGroup           map[string]*cacheWriteCall
	cacheWriteAttemptCount    int
	cacheRenameSuccessCount   int
	cacheWriteFailureHook     func(path string) error
	cacheCloseFailureHook     func(path string) error
	cacheRenameFailureHook    func(path string) error
	cacheSingleflightJoinHook func(key string)
}

type cacheWriteCall struct {
	done   chan struct{}
	result error
}

type imageFetchCall struct {
	body []byte
	err  error
	done chan struct{}
}

func newViewer(db *sql.DB, cfg Config, root string) *viewer {
	return &viewer{
		db:              db,
		cfg:             cfg,
		root:            root,
		galleryRoot:     filepath.Join(root, orDefault(cfg.GalleryDir, "Galleries")),
		cacheRoot:       filepath.Join(root, orDefault(cfg.CacheDir, "cache/images")),
		acquisitionRoot: filepath.Join(root, orDefault(cfg.AcquisitionDir, "acquisition")),
		client:          &Client{http: &http.Client{Timeout: 60 * time.Second}, cfg: cfg.Hitomi},
		infoCache:       map[int]*infoCacheEntry{},
		infoCacheRoot:   filepath.Join(root, orDefault(cfg.CacheDir, "cache"), "galleryinfo"),
		infoDiskTTL:     galleryInfoCacheTTL,
	}
}

const (
	defaultInfoCacheMaxEntries = 256
	defaultInfoCacheMaxBytes   = 4 << 20
	// Phase 3 disk 層の上限（decision 2026-11-05）。C-05/A-14 の bounded 流儀（cap 超過は
	// eviction）と同じ考え方。disk 層は access order を持たず、evict は取得時刻古い順。
	defaultInfoDiskMaxEntries = 5000
	defaultInfoDiskMaxBytes   = 256 << 20
)

type infoCacheEntry struct {
	info       *GalleryInfo
	lastAccess time.Time
}

func (e *infoCacheEntry) entrySize() int {
	n := 8
	for _, s := range []string{e.info.Title, e.info.JapaneseTitle, e.info.Language, e.info.Type, e.info.URL} {
		n += len(s)
	}
	for _, group := range [][]string{e.info.Tags, e.info.Artists, e.info.Groups, e.info.Characters} {
		for _, s := range group {
			n += len(s)
		}
	}
	for _, f := range e.info.Files {
		n += len(f.Hash) + len(f.Name) + 24
	}
	n += 8 * len(e.info.SceneIndexes)
	return n
}

func (e *infoCacheEntry) lastAccessed() time.Time { return e.lastAccess }

// putInfoCacheLocked must be called with infoCacheMu held.
func (v *viewer) putInfoCacheLocked(id int, entry *infoCacheEntry) {
	maxEntries, maxBytes := boundedMax(v.infoCacheMaxEntries, v.infoCacheMaxBytes, defaultInfoCacheMaxEntries, defaultInfoCacheMaxBytes)
	boundedCachePut(v.infoCache, &v.infoCacheBytes, maxEntries, maxBytes, id, entry)
}

func (v *viewer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", v.index)
	mux.HandleFunc("GET /search", v.serveSearchPage)
	mux.HandleFunc("GET /api/lists", v.lists)
	mux.HandleFunc("GET /api/search", v.search)
	mux.HandleFunc("GET /api/search-card/{id}", v.searchCard)
	mux.HandleFunc("GET /api/gallery/{id}", v.gallery)
	mux.HandleFunc("GET /api/gallery/{id}/page/{page}", v.pageSource)
	mux.HandleFunc("GET /api/thumb/{hash}", v.thumbProxy)
	mux.HandleFunc("GET /api/image/{hash}", v.imageProxy)
	mux.HandleFunc("POST /api/read", v.read)
	mux.HandleFunc("POST /api/fav", v.fav)
	mux.HandleFunc("POST /api/viewed", v.markViewed)
	mux.HandleFunc("POST /api/reset", v.resetState)
	mux.HandleFunc("POST /api/query", v.query)
	mux.HandleFunc("POST /api/cache/acquire", v.acquire)
	mux.HandleFunc("POST /api/acquisition/check", v.acquisitionCheck)
	mux.HandleFunc("GET /files/{gallery}/{name}", v.serveFile)
	return v.withPOSTSecurity(mux)
}

func (v *viewer) withPOSTSecurity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		if !sameOriginPost(r) {
			writeJSONError(w, http.StatusForbidden, "cross-origin post is not allowed")
			return
		}
		ct := strings.TrimSpace(strings.Split(strings.ToLower(r.Header.Get("Content-Type")), ";")[0])
		if ct != "application/json" {
			writeJSONError(w, http.StatusUnsupportedMediaType, "content-type must be application/json")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sameOriginPost(r *http.Request) bool {
	host := strings.ToLower(r.Host)
	if host == "" {
		host = strings.ToLower(r.URL.Host)
	}
	if !isLoopbackHost(host) {
		return false
	}
	origin := strings.ToLower(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return strings.ToLower(u.Host) == host
}

func isLoopbackHost(raw string) bool {
	host := strings.ToLower(strings.TrimSpace(raw))
	if host == "" {
		return false
	}
	name, _, err := net.SplitHostPort(host)
	if err != nil {
		name = host
		if strings.HasPrefix(name, "[") {
			name = strings.TrimSuffix(name, "]")
			name = strings.TrimPrefix(name, "[")
		}
	}
	return name == "localhost" || name == "127.0.0.1" || name == "::1"
}

func startView(v *viewer, listener net.Listener) error {
	go func() {
		warnGG(loadGG(v.client))
	}()
	server := &http.Server{Handler: v.handler()}
	return server.Serve(listener)
}

func (v *viewer) serve() error {
	bind := orDefault(v.cfg.Viewer.Bind, "127.0.0.1")
	port := v.cfg.Viewer.Port
	if port == 0 {
		port = 8731
	}
	listener, err := net.Listen("tcp", bind+":"+strconv.Itoa(port))
	if err != nil {
		return err
	}
	fmt.Println("viewer: http://" + bind + ":" + strconv.Itoa(port))
	return startView(v, listener)
}

func (v *viewer) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	v.serveHTML(w, "viewer.html")
}

func (v *viewer) serveSearchPage(w http.ResponseWriter, r *http.Request) {
	v.serveHTML(w, "search.html")
}

func (v *viewer) serveHTML(w http.ResponseWriter, name string) {
	page, err := uiPage.ReadFile(name)
	if err != nil {
		http.Error(w, "ui: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("content-type", "text/html; charset=utf-8")
	w.Write(page)
}

// hitomi の galleryinfo.hash は 64 hex を前提にしているが、galleryblock の tn path で
// 46〜64 hex の hash を見た。proxy は len>=3 lowercase hex を許可する。
func validHash(hash string) bool {
	if len(hash) < 3 {
		return false
	}
	for _, c := range hash {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// imageContentType は format の mime を返す（TN の実測 content-type と同じ: webp -> image/webp、avif -> image/avif）
func imageContentType(format string) string {
	switch format {
	case "webp":
		return "image/webp"
	case "avif":
		return "image/avif"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	default:
		return "application/octet-stream"
	}
}

func normalizeFormat(format string) string {
	format = strings.ToLower(format)
	if format == "jpeg" {
		return "jpg"
	}
	format = strings.TrimPrefix(format, ".")
	if format == "" || len(format) > 12 {
		return ""
	}
	for _, c := range format {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			continue
		}
		return ""
	}
	return format
}

func resolveFormat(cfg Config, queryFormat string) string {
	if q := normalizeFormat(queryFormat); q != "" {
		return q
	}
	return orDefault(normalizeFormat(imageFormat(cfg)), "webp")
}

func safeCachePath(cacheRoot, hash, format string) (string, error) {
	if !validHash(hash) {
		return "", errors.New("invalid hash")
	}
	format = normalizeFormat(format)
	if format == "" {
		return "", errors.New("invalid format")
	}
	return filepath.Join(cacheRoot, hash+"."+format), nil
}

func safeThumbnailCachePath(cacheRoot, hash, format, variant string) (string, error) {
	if !validHash(hash) {
		return "", errors.New("invalid hash")
	}
	format = normalizeFormat(format)
	if format == "" {
		return "", errors.New("invalid format")
	}
	variant = TNVariantSuffix(variant)
	return filepath.Join(cacheRoot, "thumbs", hash+"."+format+"."+variant), nil
}

func writeImage(w http.ResponseWriter, format string, data []byte, cacheState string) {
	setImageHeaders(w, format)
	if cacheState == "tn" {
		w.Header().Set("cache-control", "no-store")
	} else {
		w.Header().Set("cache-control", "private, max-age=604800")
	}
	w.Header().Set("x-cache", cacheState)
	w.Write(data)
}

// serveStoredImage は保存済み画像を全readせず、stat + stream（http.ServeContent 相当）で返す。
// headers（content-type・cache-control・x-cache・cross-origin-resource-policy）は従来通り。
// ETag と Last-Modified は ServeContent が stat の modtime/size から生成し、If-None-Match/If-Modified-Since → 304、
// Range → 206・Content-Range・Accept-Ranges を返す。UI は cache: "no-store" fetch のため 304 を発火させない。
// cache 書き込み経路（temp+rename・magic 照合）には触れない。full actual pixels 判定基準にも影響しない。
func (v *viewer) serveStoredImage(w http.ResponseWriter, r *http.Request, format string, path string, cacheState string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	setImageHeaders(w, format)
	if cacheState == "tn" {
		w.Header().Set("cache-control", "no-store")
	} else {
		w.Header().Set("cache-control", "private, max-age=604800")
	}
	w.Header().Set("x-cache", cacheState)
	w.Header().Set("etag", fmt.Sprintf(`"%x-%x"`, info.ModTime().Unix(), info.Size()))
	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), f)
	return true
}

// writeImageCacheAtomically は full image cache を temp write + close + rename で書く。
// temp は cachePath と同一ディレクトリ（cacheRoot）内のみ。OS temp dir・scratch/・output/ は使わない。
// existing target は skip、close 完了前に rename しない。write / close / rename 失敗時は temp を cleanup する。
// singleflight の対象は write のみ。key は cachePath。waiting side は first actual attempt の result を共有する。
func (v *viewer) writeImageCacheAtomically(cachePath string, data []byte) error {
	if _, err := os.Stat(cachePath); err == nil {
		return nil
	}

	v.cacheWriteMu.Lock()
	if call := v.cacheWriteGroup[cachePath]; call != nil {
		v.cacheWriteMu.Unlock()
		if v.cacheSingleflightJoinHook != nil {
			v.cacheSingleflightJoinHook(cachePath)
		}
		<-call.done
		return call.result
	}
	if v.cacheWriteGroup == nil {
		v.cacheWriteGroup = map[string]*cacheWriteCall{}
	}
	call := &cacheWriteCall{done: make(chan struct{})}
	v.cacheWriteGroup[cachePath] = call
	v.cacheWriteMu.Unlock()

	err := v.writeCacheFileOnce(cachePath, data)

	v.cacheWriteMu.Lock()
	if v.cacheWriteGroup[cachePath] == call {
		delete(v.cacheWriteGroup, cachePath)
	}
	call.result = err
	v.cacheWriteMu.Unlock()
	close(call.done)
	return err
}

// matchesFormatMagic は full cache 書き込み直前の format magic 照合（A-07a）。
// webp = RIFF(0-3)+WEBP(8-11)、avif = ftyp(4-7)+avif brand(8-11)。webp/avif 以外は現行動作維持（照合なし）。
func matchesFormatMagic(format string, data []byte) bool {
	switch format {
	case "webp":
		return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	case "avif":
		return len(data) >= 12 && string(data[4:8]) == "ftyp" && string(data[8:12]) == "avif"
	default:
		return true
	}
}

func (v *viewer) writeCacheFileOnce(cachePath string, data []byte) error {
	format := normalizeFormat(filepath.Ext(cachePath))
	if !matchesFormatMagic(format, data) {
		return fmt.Errorf("cache magic mismatch for format %q", format)
	}

	dir := filepath.Dir(cachePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	v.cacheWriteMu.Lock()
	v.cacheWriteAttemptCount++
	v.cacheWriteMu.Unlock()

	tmp, err := os.CreateTemp(dir, ".cache-*.tmp")
	if err != nil {
		return err
	}
	cleanupTemp := func() {
		_ = os.Remove(tmp.Name())
	}

	if v.cacheWriteFailureHook != nil {
		if hookErr := v.cacheWriteFailureHook(tmp.Name()); hookErr != nil {
			_ = tmp.Close()
			cleanupTemp()
			return hookErr
		}
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanupTemp()
		return err
	}
	if v.cacheCloseFailureHook != nil {
		if hookErr := v.cacheCloseFailureHook(tmp.Name()); hookErr != nil {
			_ = tmp.Close()
			cleanupTemp()
			return hookErr
		}
	}
	if err := tmp.Close(); err != nil {
		cleanupTemp()
		return err
	}
	if v.cacheRenameFailureHook != nil {
		if hookErr := v.cacheRenameFailureHook(tmp.Name()); hookErr != nil {
			cleanupTemp()
			return hookErr
		}
	}
	if err := os.Rename(tmp.Name(), cachePath); err != nil {
		cleanupTemp()
		return err
	}

	v.cacheWriteMu.Lock()
	v.cacheRenameSuccessCount++
	v.cacheWriteMu.Unlock()
	return nil
}

// safeGalleryPath は完成フォルダ内の gallery dir に閉じた路径解決（`..` と絶対路径を拒否）
func safeGalleryPath(galleryRoot, rel string) (string, error) {
	if rel == "" {
		return "", errors.New("empty path")
	}
	clean := filepath.Clean("/" + filepath.FromSlash(rel))
	full := filepath.Join(galleryRoot, clean)
	if full == galleryRoot || !strings.HasPrefix(full, galleryRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path outside gallery dir: %s", rel)
	}
	return full, nil
}

func (v *viewer) serveFile(w http.ResponseWriter, r *http.Request) {
	full, err := safeGalleryPath(v.galleryRoot, r.PathValue("gallery")+"/"+r.PathValue("name"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	format := normalizeFormat(filepath.Ext(full))
	if format == "" {
		format = imageFormat(v.cfg)
	}
	if !v.serveStoredImage(w, r, format, full, "local") {
		http.Error(w, "not downloaded: "+r.PathValue("name"), http.StatusNotFound)
	}
}

func (v *viewer) lists(w http.ResponseWriter, r *http.Request) {
	unread, err := listRows(v.db, "unread")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	favorites, err := listRows(v.db, "favorites")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	queries, err := listRows(v.db, "queries")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOut(w, map[string]any{"unread": unread, "favorites": favorites, "queries": queries})
}

func (v *viewer) search(w http.ResponseWriter, r *http.Request) {
	priority := r.URL.Query().Get("priority")
	v.client.recordMarkerRequest(priority)
	background := priority == "background"
	q := r.URL.Query().Get("q")
	if q == "" {
		q = r.URL.Query().Get("query")
	}
	if q == "" {
		q = r.URL.Query().Get("term")
	}
	if q == "" {
		http.Error(w, "q or term is required", http.StatusBadRequest)
		return
	}
	page, err := strconv.Atoi(orDefault(r.URL.Query().Get("page"), "1"))
	if err != nil || page < 1 {
		http.Error(w, "page must be >= 1", http.StatusBadRequest)
		return
	}
	limit, err := strconv.Atoi(orDefault(r.URL.Query().Get("limit"), "0"))
	if err != nil || limit < 0 {
		http.Error(w, "limit must be 0 or positive", http.StatusBadRequest)
		return
	}
	searchStart := time.Now()
	ids, total, err := v.client.SearchHTML(q, orDefault(r.URL.Query().Get("lang"), "all"), page, limit, background)
	if err != nil {
		http.Error(w, "search: "+err.Error(), http.StatusBadGateway)
		return
	}
	searchMS := int64(time.Since(searchStart) / time.Millisecond)
	recordStart := time.Now()
	_ = recordDiscoveredBatch(v.db, ids, v.root)
	recordMS := int64(time.Since(recordStart) / time.Millisecond)
	jsonStart := time.Now()
	buf := &bytes.Buffer{}
	payload := map[string]any{
		"query": q,
		"lang":  r.URL.Query().Get("lang"),
		"page":  page,
		"total": total,
		"ids":   ids,
		"results": []map[string]any{},
		"timing": map[string]any{
			"search_ms": searchMS,
			"record_ms": recordMS,
			"json_ms":   0,
		},
		"metrics": v.client.metricsSnapshot(),
	}
	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonMS := int64(time.Since(jsonStart) / time.Millisecond)
	if jsonMS == 0 {
		jsonMS = 1
	}

	data := buf.Bytes()
	data = bytes.Replace(data, []byte(`"json_ms":0`), []byte(fmt.Sprintf(`"json_ms":%d`, jsonMS)), 1)

	w.Header().Set("cache-control", "no-store")
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.Header().Set("x-search-timing", fmt.Sprintf("search=%d;record=%d;json=%d", searchMS, recordMS, jsonMS))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (v *viewer) searchCard(w http.ResponseWriter, r *http.Request) {
	priority := r.URL.Query().Get("priority")
	v.client.recordMarkerRequest(priority)
	background := priority == "background"
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		http.Error(w, "gallery id must be a positive integer", http.StatusBadRequest)
		return
	}
	var cacheState string
	v.infoCacheMu.Lock()
	if _, ok := v.infoCache[id]; ok {
		cacheState = "cache"
	} else {
		cacheState = "miss"
	}
	v.infoCacheMu.Unlock()
	w.Header().Set("x-cache", cacheState)
	_ = recordDiscovered(v.db, id, galleryDir(v.root, id))
	jsonOut(w, v.searchResult(id, background))
}

func (v *viewer) searchResult(id int, background bool) map[string]any {
	info, err := v.info(id, background)
	if err != nil {
		return map[string]any{"id": id, "error": err.Error(), "tags": []string{}}
	}
	baseFormat := imageFormat(v.cfg)
	thumbnail := ""
	if len(info.Files) > 0 {
		thumbnail = "/api/thumb/" + info.Files[0].Hash + "?format=" + baseFormat + "&gallery=" + strconv.Itoa(id) + "&variant=bigtn"
	}
	readState, err := readStateRow(v.db, id)
	if err != nil {
		readState = map[string]any{"gallery_id": id, "last_page": 0, "fully_read": 0, "favorite": 0, "viewed": 0, "viewed_at": nil}
	}
	files := []map[string]any{}
	localEntries := 0
	for _, entry := range info.Files {
		eFormat := entryFormat(entry, baseFormat)
		localFormats := v.entryLocalFormats(id, entry.Hash, baseFormat, eFormat)
		if len(localFormats) > 0 {
			localEntries++
		}
		files = append(files, map[string]any{
			"hash": entry.Hash, "name": entry.Name, "width": entry.Width, "height": entry.Height,
			"hasavif": entry.HasAVIF, "format": eFormat, "local": len(localFormats) > 0,
			"local_formats": localFormats,
		})
	}
	// saved は指定フォルダ内に全 entry の local ファイルがあること（部分保存は saved=0。
	// meta の local N/M は card 応答の files[].local を分子へ使う）。追加 I/O なし・既存ループで計算。
	saved := boolToInt(len(info.Files) > 0 && localEntries == len(info.Files))
	return map[string]any{"id": id, "title": info.Title, "japanese_title": info.JapaneseTitle,
		"language": info.Language, "type": info.Type, "url": info.URL,
		"tags": info.Tags, "artists": info.Artists, "groups": info.Groups,
		"characters": info.Characters, "count": len(info.Files), "local": v.localCount(id),
		"thumbnail": thumbnail, "scene_indexes": info.SceneIndexes, "files": files,
		"format": baseFormat, "read_state": readState,
		"viewed": readState["viewed"], "saved": saved}
}

func (v *viewer) gallery(w http.ResponseWriter, r *http.Request) {
	background := r.URL.Query().Get("priority") == "background"
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		http.Error(w, "gallery id must be a positive integer", http.StatusBadRequest)
		return
	}
	info, err := v.info(id, background)
	if err != nil {
		http.Error(w, "gallery: "+err.Error(), http.StatusBadGateway)
		return
	}
	if err := upsertGallery(v.db, info, galleryDir(v.root, info.ID)); err != nil {
		http.Error(w, "upsert: "+err.Error(), http.StatusInternalServerError)
		return
	}
	state, err := readStateRow(v.db, id)
	if err != nil {
		http.Error(w, "read_state: "+err.Error(), http.StatusInternalServerError)
		return
	}
	baseFormat := imageFormat(v.cfg)
	files := []map[string]any{}
	for _, entry := range info.Files {
		eFormat := entryFormat(entry, baseFormat)
		localFormats := v.entryLocalFormats(id, entry.Hash, baseFormat, eFormat)
		files = append(files, map[string]any{
			"hash": entry.Hash, "name": entry.Name, "width": entry.Width, "height": entry.Height,
			"hasavif": entry.HasAVIF, "format": eFormat, "local": len(localFormats) > 0,
			"local_formats": localFormats,
		})
	}
	jsonOut(w, map[string]any{"gallery": map[string]any{
		"id": info.ID, "title": info.Title, "japanese_title": info.JapaneseTitle,
		"language": info.Language, "type": info.Type, "tags": info.Tags, "artists": info.Artists,
		"groups": info.Groups, "characters": info.Characters, "url": info.URL, "files": files,
		"scene_indexes": info.SceneIndexes,
	}, "read_state": state, "local": v.localCount(id), "format": baseFormat})
}

func (v *viewer) entryLocalFormats(id int, hash, baseFormat, eFormat string) []string {
	candidates := []string{baseFormat}
	if eFormat != baseFormat {
		candidates = append(candidates, eFormat)
	}
	localFormats := []string{}
	for _, format := range candidates {
		if v.fileLocal(id, hash, format) {
			localFormats = append(localFormats, format)
		}
	}
	return localFormats
}

func (v *viewer) pageSource(w http.ResponseWriter, r *http.Request) {
	background := r.URL.Query().Get("priority") == "background"
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		http.Error(w, "gallery id must be a positive integer", http.StatusBadRequest)
		return
	}
	page, err := strconv.Atoi(r.PathValue("page"))
	if err != nil || page < 1 {
		http.Error(w, "page must be >= 1", http.StatusBadRequest)
		return
	}
	info, err := v.info(id, background)
	if err != nil {
		http.Error(w, "gallery: "+err.Error(), http.StatusBadGateway)
		return
	}
	if page > len(info.Files) {
		http.Error(w, "page out of range", http.StatusBadRequest)
		return
	}
	baseFormat := imageFormat(v.cfg)
	entry := info.Files[page-1]
	format := entryFormat(entry, baseFormat)
	localFormats := v.entryLocalFormats(id, entry.Hash, baseFormat, format)
	for _, localFormat := range localFormats {
		local, err := safeGalleryPath(v.galleryRoot, strconv.Itoa(id)+"/"+entry.Hash+"."+localFormat)
		if err == nil {
			if _, err := os.Stat(local); err == nil {
				jsonOut(w, map[string]any{"page": page, "src": "/files/" + strconv.Itoa(id) + "/" + entry.Hash + "." + localFormat,
					"local": true, "format": localFormat, "width": entry.Width, "height": entry.Height})
				return
			}
		}
	}
	jsonOut(w, map[string]any{"page": page, "src": "/api/image/" + entry.Hash + "?format=" + format, "local": false,
		"format": format, "width": entry.Width, "height": entry.Height})
}

func (v *viewer) thumbProxy(w http.ResponseWriter, r *http.Request) {
	priority := r.URL.Query().Get("priority")
	v.client.recordMarkerRequest(priority)
	background := priority == "background"
	hash := r.PathValue("hash")
	if !validHash(hash) {
		http.Error(w, "hash must be 64 lowercase hex chars", http.StatusBadRequest)
		return
	}
	format := resolveFormat(v.cfg, r.URL.Query().Get("format"))
	variant := TNVariantSuffix(r.URL.Query().Get("variant"))

	thumbCachePath, thumbCacheErr := safeThumbnailCachePath(v.cacheRoot, hash, format, variant)
	if thumbCacheErr == nil && v.serveStoredImage(w, r, format, thumbCachePath, "tn-cache") {
		return
	}

	body, err := v.client.getImage(r.Context(), ThumbnailURLVariant(hash, format, variant, v.client.GG()), [2]int64{-1, -1}, background)
	if err == nil {
		writeImage(w, format, body, "tn")
		if thumbCacheErr == nil && len(body) > 0 {
			_ = os.MkdirAll(filepath.Join(v.cacheRoot, "thumbs"), 0o755)
			_ = os.WriteFile(thumbCachePath, body, 0o644)
		}
		return
	}
	writeNoTN(w, format)
}

func (v *viewer) fetchFullImageOnce(ctx context.Context, hash, format string, gg GG, background bool) ([]byte, error) {
	key := imageFetchKey(hash, format, gg)

	v.imageFetchMu.Lock()
	if v.imageFetchGroup == nil {
		v.imageFetchGroup = map[string]*imageFetchCall{}
	}
	if call := v.imageFetchGroup[key]; call != nil {
		if v.imageFetchJoinObserver != nil {
			v.imageFetchJoinObserver(key)
		}
		v.imageFetchMu.Unlock()
		select {
		case <-call.done:
			return call.body, call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	call := &imageFetchCall{done: make(chan struct{})}
	v.imageFetchGroup[key] = call
	v.imageFetchMu.Unlock()

	body, err := v.client.getImage(context.WithoutCancel(ctx), StorageURL(hash, format, gg), [2]int64{-1, -1}, background)
	call.body = body
	call.err = err

	v.imageFetchMu.Lock()
	if v.imageFetchGroup[key] == call {
		delete(v.imageFetchGroup, key)
	}
	v.imageFetchMu.Unlock()

	close(call.done)
	return body, err
}

func (v *viewer) imageProxy(w http.ResponseWriter, r *http.Request) {
	background := r.URL.Query().Get("priority") == "background"
	hash := r.PathValue("hash")
	if !validHash(hash) {
		http.Error(w, "hash must be 64 lowercase hex chars", http.StatusBadRequest)
		return
	}
	format := resolveFormat(v.cfg, r.URL.Query().Get("format"))
	if v.serveLocalImage(w, r, hash, format) {
		return
	}

	cachePath, cacheErr := safeCachePath(v.cacheRoot, hash, format)
	if cacheErr == nil && v.serveStoredImage(w, r, format, cachePath, "cache") {
		return
	}

	gg := v.client.GG()
	initialGG := gg
	force := r.URL.Query().Get("force") == "1"
	if force {
		v.client.clearNegativeNoFull(hash, format, gg)
	} else if v.client.canUseNegativeNoFull(hash, format, gg) {
		writeNoFull(w, format)
		return
	}

	body, err := v.fetchFullImageOnce(r.Context(), hash, format, gg, background)
	if err == nil {
		v.client.clearNegativeNoFull(hash, format, gg)
		writeImage(w, format, body, "miss")
		if cacheErr == nil && len(body) > 0 {
			_ = v.writeImageCacheAtomically(cachePath, body)
		}
		return
	}

	if strings.Contains(err.Error(), "status 404") {
		source := v.client.EnsureGG(r.Context(), true)
		gg = v.client.GG()

		if source == "live" && !sameGG(initialGG, gg) {
			body, err = v.fetchFullImageOnce(r.Context(), hash, format, gg, background)
			if err == nil {
				v.client.clearNegativeNoFull(hash, format, gg)
				writeImage(w, format, body, "miss")
				if cacheErr == nil && len(body) > 0 {
					_ = v.writeImageCacheAtomically(cachePath, body)
				}
				return
			}
			if strings.Contains(err.Error(), "status 404") {
				v.client.rememberNegativeNoFull(hash, format, gg)
				writeNoFull(w, format)
				return
			}
			http.Error(w, "image: "+err.Error(), http.StatusBadGateway)
			return
		}

		v.client.rememberNegativeNoFull(hash, format, gg)
		writeNoFull(w, format)
		return
	}
	http.Error(w, "image: "+err.Error(), http.StatusBadGateway)
}

func writeNoFull(w http.ResponseWriter, format string) {
	setImageHeaders(w, format)
	w.Header().Set("cache-control", "no-store")
	w.Header().Set("x-cache", "no_full")
	w.WriteHeader(http.StatusNotFound)
}

func writeNoTN(w http.ResponseWriter, format string) {
	setImageHeaders(w, format)
	w.Header().Set("cache-control", "no-store")
	w.Header().Set("x-cache", "no_tn")
	w.WriteHeader(http.StatusBadGateway)
}

func setImageHeaders(w http.ResponseWriter, format string) {
	w.Header().Set("content-type", imageContentType(format))
	w.Header().Set("cross-origin-resource-policy", "same-origin")
	w.Header().Set("cache-control", "private, max-age=604800")
}

func (v *viewer) fileLocal(id int, hash string, format string) bool {
	full, err := safeGalleryPath(v.galleryRoot, strconv.Itoa(id)+"/"+hash+"."+format)
	if err != nil {
		return false
	}
	_, err = os.Stat(full)
	return err == nil
}

func (v *viewer) serveLocalImage(w http.ResponseWriter, r *http.Request, hash string, format string) bool {
	gallery := r.URL.Query().Get("gallery")
	if gallery == "" {
		return false
	}
	id, err := strconv.Atoi(gallery)
	if err != nil || id <= 0 {
		return false
	}
	full, err := safeGalleryPath(v.galleryRoot, strconv.Itoa(id)+"/"+hash+"."+format)
	if err != nil {
		return false
	}
	return v.serveStoredImage(w, r, format, full, "local")
}

func (v *viewer) read(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GalleryID int  `json:"gallery_id"`
		Page      int  `json:"page"`
		Complete  bool `json:"complete"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.GalleryID <= 0 || body.Page < 1 {
		http.Error(w, "gallery_id and page >= 1 are required", http.StatusBadRequest)
		return
	}
	if err := recordDiscovered(v.db, body.GalleryID, galleryDir(v.root, body.GalleryID)); err != nil {
		http.Error(w, "gallery: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := setReadState(v.db, body.GalleryID, body.Page, body.Complete); err != nil {
		http.Error(w, "read: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOut(w, map[string]any{"gallery_id": body.GalleryID, "last_page": body.Page,
		"fully_read": boolToInt(body.Complete)})
}

func (v *viewer) markViewed(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GalleryID int `json:"gallery_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.GalleryID <= 0 {
		http.Error(w, "gallery_id is required", http.StatusBadRequest)
		return
	}
	if err := recordDiscovered(v.db, body.GalleryID, galleryDir(v.root, body.GalleryID)); err != nil {
		http.Error(w, "gallery: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := setViewed(v.db, body.GalleryID); err != nil {
		http.Error(w, "viewed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	row, err := readStateRow(v.db, body.GalleryID)
	if err != nil {
		http.Error(w, "read_state: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOut(w, row)
}

func (v *viewer) resetState(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GalleryID int    `json:"gallery_id"`
		Target    string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.GalleryID <= 0 {
		http.Error(w, "gallery_id is required", http.StatusBadRequest)
		return
	}
	if body.Target != "viewed" && body.Target != "read" {
		http.Error(w, "target must be viewed or read", http.StatusBadRequest)
		return
	}
	if err := recordDiscovered(v.db, body.GalleryID, galleryDir(v.root, body.GalleryID)); err != nil {
		http.Error(w, "gallery: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := resetReadState(v.db, body.GalleryID, body.Target); err != nil {
		http.Error(w, "reset: "+err.Error(), http.StatusInternalServerError)
		return
	}
	row, err := readStateRow(v.db, body.GalleryID)
	if err != nil {
		http.Error(w, "read_state: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOut(w, row)
}

func (v *viewer) fav(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GalleryID int  `json:"gallery_id"`
		On        bool `json:"on"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.GalleryID <= 0 {
		http.Error(w, "gallery_id is required", http.StatusBadRequest)
		return
	}
	if err := recordDiscovered(v.db, body.GalleryID, galleryDir(v.root, body.GalleryID)); err != nil {
		http.Error(w, "gallery: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := setFavorite(v.db, body.GalleryID, body.On); err != nil {
		http.Error(w, "fav: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOut(w, map[string]any{"gallery_id": body.GalleryID, "favorite": boolToInt(body.On)})
}

func (v *viewer) query(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string `json:"name"`
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.Name == "" || body.Query == "" {
		http.Error(w, "name and query are required", http.StatusBadRequest)
		return
	}
	if err := addQuery(v.db, body.Name, body.Query, 0); err != nil {
		http.Error(w, "query: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOut(w, map[string]any{"name": body.Name, "query": body.Query})
}

// info は 3 層参照（Phase 3・decision 2026-11-05）: memory `infoCache` → 永続 disk 層
// （cache/galleryinfo）→ remote `Client.Gallery`。disk hit は memory へ載せ、network（ゲート）を
// 消費しない。remote 成功は memory と永続の両方へ書く。TTL（既定 30 日）切れは remote 再取得、
// remote 失敗時のみ期限切れ永続 entry を stale 暫定表示に使う（counter+log で判別可）。
// 壊れた永続ファイルはミス扱い・除去して remote 再取得。Phase 2 の先読み成果もこの helper
// 経由で自動で永続化される（Phase 2 側追加作業不要）。
func (v *viewer) info(id int, background bool) (*GalleryInfo, error) {
	v.infoCacheMu.Lock()
	entry, ok := v.infoCache[id]
	if ok {
		entry.lastAccess = time.Now()
		v.infoCacheMu.Unlock()
		return entry.info, nil
	}
	v.infoCacheMu.Unlock()

	persisted := v.readInfoDisk(id)
	if persisted != nil && time.Since(persisted.FetchedAt) < v.infoDiskTTL {
		v.infoCacheMu.Lock()
		if v.infoCache == nil {
			v.infoCache = map[int]*infoCacheEntry{}
		}
		v.putInfoCacheLocked(id, &infoCacheEntry{info: &persisted.GalleryInfo, lastAccess: time.Now()})
		v.infoCacheMu.Unlock()
		return &persisted.GalleryInfo, nil
	}

	info, err := v.client.Gallery(id, background)
	if err != nil {
		if persisted != nil {
			v.recordInfoDiskStaleHit(id, persisted.FetchedAt)
			return &persisted.GalleryInfo, nil
		}
		return nil, err
	}
	if err := v.writeInfoDisk(id, info); err != nil {
		log.Printf("galleryinfo persist failed for %d: %s", id, err)
	}
	v.infoCacheMu.Lock()
	if v.infoCache == nil {
		v.infoCache = map[int]*infoCacheEntry{}
	}
	now := time.Now()
	v.putInfoCacheLocked(id, &infoCacheEntry{info: info, lastAccess: now})
	v.infoCacheMu.Unlock()
	return info, nil
}

func (v *viewer) localCount(id int) int {
	dir := filepath.Join(v.galleryRoot, strconv.Itoa(id))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			count++
		}
	}
	return count
}

func imageFormat(cfg Config) string {
	return orDefault(normalizeFormat(cfg.Hitomi.ImageFormat), "webp")
}

func entryFormat(entry FileEntry, base string) string {
	if entry.HasAVIF != 0 {
		return "avif"
	}
	return orDefault(base, "webp")
}

func jsonOut(w http.ResponseWriter, value any) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
