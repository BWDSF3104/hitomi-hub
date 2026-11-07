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

	// R-16: 正典 core atomic write（writeAtomically）の shared test seam。atomicWriteSeam を embed
	// することで既存テストの v.cacheWriteMu / hooks / counts 参照は promotion で不変。
	// downloadFilesParallel は nil または own seam を受け取る（R-16 失敗注入テスト）。
	atomicWriteSeam
	cacheWriteGroup           map[string]*cacheWriteCall
	cacheSingleflightJoinHook func(key string)

	// R-13: TN / info fetch dedup（案 A・既存流儀: key 単位・mutex・network 中に mutex を保持しない）。
	// TN は search-card cover 専用・reader 表示源禁止・TN display fallback 禁止・TN バイトを full
	// cache に保存しない正典不変条件を変えない。joiner は leader result を共有し、x-cache は source
	// を保持し、per-request の write helper（TN write）・marker record を skip しない。
	tnFetchMu             sync.Mutex
	tnFetchGroup          map[string]*tnFetchCall
	tnFetchJoinObserver   func(key string)
	infoFetchMu           sync.Mutex
	infoFetchGroup        map[int]*infoFetchCall
	infoFetchJoinObserver func(int)

	// R-15（decision 2026-11-06 確定・2026-11-08 supersede）: disk cache の件数・作成順・status を
	// DB の 1 表 cache_entries だけで持つ。filesystem scan で数えない。書き込み時 1 行 INSERT、
	// prune 時にファイル削除 + 1 行 DELETE。cap 超過は作成順（古い順）で捨てる。現在閲覧中の
	// gallery（書き込みリクエストの gallery）の項目は捨ない。
	cacheEntriesMu      sync.Mutex
	cacheDiskMaxEntries int
}

type tnFetchCall struct {
	body []byte
	err  error
	done chan struct{}
}

// infoFetchCall の joiner は shared pointer を返す（既存 infoCache が pointer を共有する意味と同一）。
// info は ctx を持たない（既存 guard test の signature を変えないため join は ctx guard なし）。
type infoFetchCall struct {
	info *GalleryInfo
	err  error
	done chan struct{}
}

type cacheWriteCall struct {
	done   chan struct{}
	result error
}

// atomicWriteSeam は core atomic write（writeAtomically）の shared test seam（A-07a 正典流儀）。
// counter と hooks は seam が非 nil のときだけ適用。existing target skip は actual attempt に数えない（正典）。
// viewer（full cache・TN）と downloadFilesParallel（R-16）で同一構造を使う。
type atomicWriteSeam struct {
	cacheWriteMu              sync.Mutex
	cacheWriteAttemptCount    int
	cacheRenameSuccessCount   int
	cacheWriteFailureHook     func(path string) error
	cacheCloseFailureHook     func(path string) error
	cacheRenameFailureHook    func(path string) error
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
		client:          &Client{http: &http.Client{Timeout: 60 * time.Second, Transport: newPooledTransport()}, cfg: cfg.Hitomi},
		infoCache:       map[int]*infoCacheEntry{},
		infoCacheRoot:   filepath.Join(root, orDefault(cfg.CacheDir, "cache"), "galleryinfo"),
		infoDiskTTL:     galleryInfoCacheTTL,
		cacheDiskMaxEntries: defaultCacheDiskMaxEntries,
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
	mux.HandleFunc("GET /api/viewer-config", v.uiConfig)
	mux.HandleFunc("GET /api/search", v.search)
	mux.HandleFunc("GET /api/search-first-status", v.searchFirstStatus)
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

// writeImageCacheAtomically は R-16 の shared singleflight wrapper。key は cachePath（full cache・TN で同一設計）。
// existing target は skip、close 完了前に rename しない。write / close / rename 失敗時は temp を cleanup する。
// singleflight の対象は write のみ。waiting side は first actual attempt の result を共有する。
// format は caller から明示引数（TN は request 側の format を渡す。TN cache path の filepath.Ext は
// variant 接尾なので format 導出に使えない）。実 write は shared core writeAtomically。
func (v *viewer) writeImageCacheAtomically(cachePath, format string, data []byte) error {
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

	err := v.writeCacheFileOnce(cachePath, format, data)

	v.cacheWriteMu.Lock()
	if v.cacheWriteGroup[cachePath] == call {
		delete(v.cacheWriteGroup, cachePath)
	}
	call.result = err
	v.cacheWriteMu.Unlock()
	close(call.done)
	return err
}

// writeAtomically は正典 atomic write helper（A-07a）の shared core（R-16 共通化）。
// viewer（full cache・TN）と downloadFilesParallel（CLI download）が同じ機構を使う。
// temp は targetPath と同一ディレクトリ内のみ（OS temp dir・scratch/・output/ は使わない）。
// close 完了前に rename しない。write / close / rename 失敗時は temp を cleanup する（壊れた target が残らない）。
// core では existing target skip を決めない（skip は wrapper 側の判断。download は existing へ rename 上書きし、
// size 不一致 self-repair を維持）。Windows semantics: 既存 target への rename は上書き（C1 実測）、
// open 中 target への rename は失敗し得る（C2 実測）— どちらも error result で吸収する。
// counter・hooks は seam（atomicWriteSeam）が非 nil のときだけ適用。existing target skip は actual attempt に数えない（正典）。
func writeAtomically(seam *atomicWriteSeam, targetPath, format string, data []byte) error {
	if !matchesFormatMagic(format, data) {
		return fmt.Errorf("cache magic mismatch for format %q", format)
	}

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	if seam != nil {
		seam.cacheWriteMu.Lock()
		seam.cacheWriteAttemptCount++
		seam.cacheWriteMu.Unlock()
	}

	tmp, err := os.CreateTemp(dir, ".cache-*.tmp")
	if err != nil {
		return err
	}
	cleanupTemp := func() {
		_ = os.Remove(tmp.Name())
	}

	if seam != nil && seam.cacheWriteFailureHook != nil {
		if hookErr := seam.cacheWriteFailureHook(tmp.Name()); hookErr != nil {
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
	if seam != nil && seam.cacheCloseFailureHook != nil {
		if hookErr := seam.cacheCloseFailureHook(tmp.Name()); hookErr != nil {
			_ = tmp.Close()
			cleanupTemp()
			return hookErr
		}
	}
	if err := tmp.Close(); err != nil {
		cleanupTemp()
		return err
	}
	if seam != nil && seam.cacheRenameFailureHook != nil {
		if hookErr := seam.cacheRenameFailureHook(tmp.Name()); hookErr != nil {
			cleanupTemp()
			return hookErr
		}
	}
	if err := os.Rename(tmp.Name(), targetPath); err != nil {
		cleanupTemp()
		return err
	}

	if seam != nil {
		seam.cacheWriteMu.Lock()
		seam.cacheRenameSuccessCount++
		seam.cacheWriteMu.Unlock()
	}
	return nil
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

// writeCacheFileOnce は shared core writeAtomically への viewer seam binding（A-07a 正典名維持）。
// format は明示引数（R-16）。full cache 呼び出し側は request 側 format を渡す（cache path の Ext と一致）。
func (v *viewer) writeCacheFileOnce(cachePath, format string, data []byte) error {
	return writeAtomically(&v.atomicWriteSeam, cachePath, format, data)
}

// effectiveMaxObjectURLs は R-15 の memory cap（client objectURL 件数）。loadConfig で
// 0 は既定 4000・下限 1000 が確定済み。テストの direct Config 構成でも既定へ揃う。
func (v *viewer) effectiveMaxObjectURLs() int {
	if v.cfg.Viewer.MaxObjectURLs > 0 {
		return v.cfg.Viewer.MaxObjectURLs
	}
	return defaultMaxObjectURLs
}

// uiConfig は page へ memory cap を渡す口（R-15）。設定名 viewer.max_object_urls。
// 通知 UI を置かない項目のため、この口は設定値の受け渡しのみ（画面表示しない）。
func (v *viewer) uiConfig(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, map[string]any{"max_object_urls": v.effectiveMaxObjectURLs()})
}

func requestGalleryID(r *http.Request) int {
	g := r.URL.Query().Get("gallery")
	if g == "" {
		return 0
	}
	id, err := strconv.Atoi(g)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

// recordCacheEntry は cache 書き込み時に cache_entries へ 1 行 INSERT する（R-15）。
// 件数・作成順・status の判定源は DB 行のみ（filesystem scan 禁止）。INSERT OR IGNORE:
// 同一 (hash, format, kind, variant) は最初の created_at を保持する。gallery_id は書き込み
// リクエストの gallery（なければ NULL）。source_status は書き込み時記録値（追加 lookup なし）。
func (v *viewer) recordCacheEntry(hash, format, kind, variant, sourceStatus string, galleryID int) error {
	v.cacheEntriesMu.Lock()
	defer v.cacheEntriesMu.Unlock()
	var gallery any
	if galleryID > 0 {
		gallery = galleryID
	}
	_, err := v.db.Exec(`INSERT OR IGNORE INTO cache_entries (hash, format, kind, variant, gallery_id, created_at, source_status) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7)`,
		hash, format, kind, variant, gallery, time.Now().UTC().Format(time.RFC3339), sourceStatus)
	return err
}

func cacheEntryPath(v *viewer, kind, hash, format, variant string) (string, error) {
	if kind == "full" {
		return safeCachePath(v.cacheRoot, hash, format)
	}
	return safeThumbnailCachePath(v.cacheRoot, hash, format, variant)
}

// pruneCacheEntries は kind 単位の disk cap（既定 8000 件・byte 計算なし）を DB 行だけで
// 判定し、作成順（古い順）で超ぶ分をファイル削除 + 1 行 DELETE で捨てる（R-15）。現在閲覧中の
// gallery の項目は捨ない。表に記録のない項目（表生成以前の file・backfill 禁止）は件数に
// 入らず prune 対象にならない。ファイル削除に失敗した行は保持（次の prune で再試行）。
func (v *viewer) pruneCacheEntries(kind string, currentGalleryID int) error {
	v.cacheEntriesMu.Lock()
	defer v.cacheEntriesMu.Unlock()
	var count int
	if err := v.db.QueryRow(`SELECT COUNT(1) FROM cache_entries WHERE kind = ?1`, kind).Scan(&count); err != nil {
		return err
	}
	if count <= v.cacheDiskMaxEntries {
		return nil
	}
	type cacheVictim struct {
		hash   string
		format string
		variant string
	}
	excess := count - v.cacheDiskMaxEntries
	rows, err := v.db.Query(`SELECT hash, format, variant FROM cache_entries WHERE kind = ?1 AND (gallery_id IS NULL OR gallery_id != ?2) ORDER BY created_at ASC, rowid ASC LIMIT ?3`,
		kind, currentGalleryID, excess)
	if err != nil {
		return err
	}
	victims := []cacheVictim{}
	for rows.Next() {
		var victim cacheVictim
		if err := rows.Scan(&victim.hash, &victim.format, &victim.variant); err != nil {
			rows.Close()
			return err
		}
		victims = append(victims, victim)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, victim := range victims {
		path, err := cacheEntryPath(v, kind, victim.hash, victim.format, victim.variant)
		if err != nil {
			continue // path を組めない行は保持（ファイル削除・行 DELETE しない）
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			continue // 削除失敗の行は保持（次の prune で再試行）
		}
		if _, err := v.db.Exec(`DELETE FROM cache_entries WHERE hash = ?1 AND format = ?2 AND kind = ?3 AND variant = ?4`,
			victim.hash, victim.format, kind, victim.variant); err != nil {
			return err
		}
	}
	return nil
}

func (v *viewer) recordAndPruneCacheEntry(hash, format, kind, variant, sourceStatus string, galleryID int) {
	if err := v.recordCacheEntry(hash, format, kind, variant, sourceStatus, galleryID); err != nil {
		log.Printf("cache_entries insert failed for %s %s %s: %s", kind, hash, format, err)
		return
	}
	if err := v.pruneCacheEntries(kind, galleryID); err != nil {
		log.Printf("cache_entries prune failed for kind %s: %s", kind, err)
	}
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
	_ = recordDiscoveredIfMissing(&sqlCardStore{v.db}, id, galleryDir(v.root, id))
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
	// R-08（案 A・1 gallery 1 回の ReadDir 集合判定）。response shape は現状のまま（guard test 不変）。
	localSet, localTotal := v.galleryLocalSet(id)
	files := []map[string]any{}
	for _, entry := range info.Files {
		eFormat := entryFormat(entry, baseFormat)
		localFormats := entryLocalFormatsFromSet(localSet, entry.Hash, baseFormat, eFormat)
		files = append(files, map[string]any{
			"hash": entry.Hash, "name": entry.Name, "width": entry.Width, "height": entry.Height,
			"hasavif": entry.HasAVIF, "format": eFormat, "local": len(localFormats) > 0,
			"local_formats": localFormats,
		})
	}
	// saved は event 記録（decision 2026-11-06-saved-as-event-record-approved）。完成後削除で
	// false へ戻さない（sticky）。meta の local N/M は disk 現况表示のまま（files[].local 分子）。
	saved := 0
	if ev, err := savedEvent(v.db, id); err == nil {
		saved = ev
	}
	return map[string]any{"id": id, "title": info.Title, "japanese_title": info.JapaneseTitle,
		"language": info.Language, "type": info.Type, "url": info.URL,
		"tags": info.Tags, "artists": info.Artists, "groups": info.Groups,
		"characters": info.Characters, "count": len(info.Files), "local": localTotal,
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

func entryLocalFormatsFromSet(localSet map[string]bool, hash, baseFormat, eFormat string) []string {
	candidates := []string{baseFormat}
	if eFormat != baseFormat {
		candidates = append(candidates, eFormat)
	}
	localFormats := []string{}
	for _, format := range candidates {
		if localSet[hash+"."+format] {
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

	galleryID := requestGalleryID(r)
	body, err := v.fetchTNOnce(r.Context(), hash, format, variant, v.client.GG(), background)
	if err == nil {
		writeImage(w, format, body, "tn")
		if thumbCacheErr == nil && len(body) > 0 {
			// R-16: TN 書き込みは shared atomic helper（writeImageCacheAtomically）経由。temp は
			// cacheRoot/thumbs 内・CreateTemp 一意名・close 前 rename なし・失敗時 temp cleanup。
			// 応答（x-cache tn）は write より前に届く・write 失敗は応答を変えない。magic 照合は
			// request 側の format（Ext の variant 接尾は使えない）。R-15 の record は write 成功時のみ。
			if writeErr := v.writeImageCacheAtomically(thumbCachePath, format, body); writeErr == nil {
				v.recordAndPruneCacheEntry(hash, format, "thumbs", variant, "tn", galleryID)
			}
		}
		return
	}
	writeNoTN(w, format)
}

// fetchTNOnce は TN fetch の singleflight（R-13・案 A）。key = TN identity + gg。network 中に mutex を
// 保持しない。joiner は leader result を共有し、x-cache は source（tn）を保持する。joiner は gate lane を
// 消費しない（leader のみ fetch）。waiter の cancellation は leader に伝播しない（join は ctx select、
// leader fetch は context.WithoutCancel）。TN write は per request 側で shared atomic helper（R-16・key = cachePath
// の writeImageCacheAtomically）で実施。
func (v *viewer) fetchTNOnce(ctx context.Context, hash, format, variant string, gg GG, background bool) ([]byte, error) {
	key := tnFetchKey(hash, format, variant, gg)

	v.tnFetchMu.Lock()
	if v.tnFetchGroup == nil {
		v.tnFetchGroup = map[string]*tnFetchCall{}
	}
	if call := v.tnFetchGroup[key]; call != nil {
		if v.tnFetchJoinObserver != nil {
			v.tnFetchJoinObserver(key)
		}
		v.tnFetchMu.Unlock()
		select {
		case <-call.done:
			return call.body, call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	call := &tnFetchCall{done: make(chan struct{})}
	v.tnFetchGroup[key] = call
	v.tnFetchMu.Unlock()

	body, err := v.client.getImage(context.WithoutCancel(ctx), ThumbnailURLVariant(hash, TNFormat(format), variant, gg), [2]int64{-1, -1}, background)
	call.body = body
	call.err = err

	v.tnFetchMu.Lock()
	if v.tnFetchGroup[key] == call {
		delete(v.tnFetchGroup, key)
	}
	v.tnFetchMu.Unlock()

	close(call.done)
	return body, err
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

	galleryID := requestGalleryID(r)
	body, err := v.fetchFullImageOnce(r.Context(), hash, format, gg, background)
	if err == nil {
		v.client.clearNegativeNoFull(hash, format, gg)
		writeImage(w, format, body, "miss")
		if cacheErr == nil && len(body) > 0 {
			if writeErr := v.writeImageCacheAtomically(cachePath, format, body); writeErr == nil {
				// R-15: 書き込み時 1 行 INSERT + kind 別 cap の prune（作成順・現在 gallery skip）。
				// x-cache の source（miss）と no_full 意味は変えない。
				v.recordAndPruneCacheEntry(hash, format, "full", "", "miss", galleryID)
			}
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
					if writeErr := v.writeImageCacheAtomically(cachePath, format, body); writeErr == nil {
						v.recordAndPruneCacheEntry(hash, format, "full", "", "miss", galleryID)
					}
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
	if body.Target != "viewed" && body.Target != "read" && body.Target != "all" {
		http.Error(w, "target must be viewed, read, or all", http.StatusBadRequest)
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

	return v.resolveInfoOnce(id, background, persisted)
}

// resolveInfoOnce は remote `Client.Gallery` の singleflight（R-13・案 A・key = id）。memory・disk を
// 通過した request だけがこの helper に入る（disk hit は network/gate を消費しない）。network 中に mutex を
// 保持しない。joiner は leader result を共有する（既存 infoCache が pointer を共有する意味と同一）。
// info は ctx を持たない（既存 guard test の signature を変えないため join は ctx guard なし・EnsureGG
// 流儀）。joiner は gate lane を消費しない。stale 暫定表示と disk persist・memory cache 書き込みは leader
// 側のみで 1 回（joiner は skip しない per-request の memory/disk hit は info 側）。
func (v *viewer) resolveInfoOnce(id int, background bool, persisted *persistedGalleryInfo) (*GalleryInfo, error) {
	v.infoFetchMu.Lock()
	if v.infoFetchGroup == nil {
		v.infoFetchGroup = map[int]*infoFetchCall{}
	}
	if call := v.infoFetchGroup[id]; call != nil {
		if v.infoFetchJoinObserver != nil {
			v.infoFetchJoinObserver(id)
		}
		v.infoFetchMu.Unlock()
		<-call.done
		return call.info, call.err
	}

	call := &infoFetchCall{done: make(chan struct{})}
	v.infoFetchGroup[id] = call
	v.infoFetchMu.Unlock()

	info, err := v.client.Gallery(id, background)
	if err != nil {
		if persisted != nil {
			v.recordInfoDiskStaleHit(id, persisted.FetchedAt)
			call.info = &persisted.GalleryInfo
			call.err = nil
		} else {
			call.err = err
		}
		v.infoFetchMu.Lock()
		if v.infoFetchGroup[id] == call {
			delete(v.infoFetchGroup, id)
		}
		v.infoFetchMu.Unlock()
		close(call.done)
		if call.err != nil {
			return nil, err
		}
		return call.info, nil
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

	call.info = info
	call.err = nil

	v.infoFetchMu.Lock()
	if v.infoFetchGroup[id] == call {
		delete(v.infoFetchGroup, id)
	}
	v.infoFetchMu.Unlock()

	close(call.done)
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

// galleryLocalSet は 1 回の ReadDir で file 名集合と non-dir 件数を返す（R-08・案 A）。
// 集合 membership は os.Stat 相当の local 判定と一致し、件数は localCount と一致する。
func (v *viewer) galleryLocalSet(id int) (map[string]bool, int) {
	dir := filepath.Join(v.galleryRoot, strconv.Itoa(id))
	entries, err := os.ReadDir(dir)
	localSet := map[string]bool{}
	if err != nil {
		return localSet, 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		count++
		localSet[entry.Name()] = true
	}
	return localSet, count
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
