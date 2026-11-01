package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed ui.html
var uiPage embed.FS

type viewer struct {
	db          *sql.DB
	cfg         Config
	client      *Client
	root        string
	galleryRoot string
	cacheRoot   string
	infoCache   map[int]*GalleryInfo
}

func newViewer(db *sql.DB, cfg Config, root string) *viewer {
	return &viewer{
		db:          db,
		cfg:         cfg,
		root:        root,
		galleryRoot: filepath.Join(root, orDefault(cfg.GalleryDir, "Galleries")),
		cacheRoot:   filepath.Join(root, orDefault(cfg.CacheDir, "cache/images")),
		client:      &Client{http: &http.Client{Timeout: 60 * time.Second}, cfg: cfg.Hitomi},
		infoCache:   map[int]*GalleryInfo{},
	}
}

func (v *viewer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", v.index)
	mux.HandleFunc("GET /api/lists", v.lists)
	mux.HandleFunc("GET /api/search", v.search)
	mux.HandleFunc("GET /api/search-card/{id}", v.searchCard)
	mux.HandleFunc("GET /api/gallery/{id}", v.gallery)
	mux.HandleFunc("GET /api/gallery/{id}/page/{page}", v.pageSource)
	mux.HandleFunc("GET /api/thumb/{hash}", v.thumbProxy)
	mux.HandleFunc("GET /api/image/{hash}", v.imageProxy)
	mux.HandleFunc("POST /api/read", v.read)
	mux.HandleFunc("POST /api/fav", v.fav)
	mux.HandleFunc("POST /api/query", v.query)
	mux.HandleFunc("GET /files/{gallery}/{name}", v.serveFile)
	return mux
}

func (v *viewer) serve() error {
	bind := orDefault(v.cfg.Viewer.Bind, "127.0.0.1")
	port := v.cfg.Viewer.Port
	if port == 0 {
		port = 8731
	}
	fmt.Println("viewer: http://" + bind + ":" + strconv.Itoa(port))
	return http.ListenAndServe(bind+":"+strconv.Itoa(port), v.handler())
}

func (v *viewer) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	page, err := uiPage.ReadFile("ui.html")
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

func writeImage(w http.ResponseWriter, format string, data []byte, cacheState string) {
	setImageHeaders(w, format);
	if cacheState == "tn" {
		w.Header().Set("cache-control", "no-store");
	} else {
		w.Header().Set("cache-control", "private, max-age=604800");
	}
	w.Header().Set("x-cache", cacheState);
	w.Write(data);
}

func (v *viewer) serveStoredImage(w http.ResponseWriter, format string, path string, cacheState string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	writeImage(w, format, data, cacheState)
	return true
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
	if !v.serveStoredImage(w, format, full, "local") {
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
	ids, total, err := v.client.SearchHTML(q, orDefault(r.URL.Query().Get("lang"), "all"), page, limit)
	if err != nil {
		http.Error(w, "search: "+err.Error(), http.StatusBadGateway)
		return
	}
	for _, id := range ids {
		_ = recordDiscovered(v.db, id, galleryDir(v.root, id))
	}
	jsonOut(w, map[string]any{"query": q, "lang": r.URL.Query().Get("lang"), "page": page,
		"total": total, "ids": ids, "results": []map[string]any{}})
}

func (v *viewer) searchCard(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		http.Error(w, "gallery id must be a positive integer", http.StatusBadRequest)
		return
	}
	_ = recordDiscovered(v.db, id, galleryDir(v.root, id))
	jsonOut(w, v.searchResult(id))
}

func (v *viewer) searchResult(id int) map[string]any {
	info, err := v.info(id)
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
		readState = map[string]any{"gallery_id": id, "last_page": 0, "fully_read": 0, "favorite": 0}
	}
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
	return map[string]any{"id": id, "title": info.Title, "japanese_title": info.JapaneseTitle,
		"language": info.Language, "type": info.Type, "url": info.URL,
		"tags": info.Tags, "artists": info.Artists, "groups": info.Groups,
		"characters": info.Characters, "count": len(info.Files), "local": v.localCount(id),
		"thumbnail": thumbnail, "scene_indexes": info.SceneIndexes, "files": files,
		"format": baseFormat, "read_state": readState}
}

func (v *viewer) gallery(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		http.Error(w, "gallery id must be a positive integer", http.StatusBadRequest)
		return
	}
	info, err := v.info(id)
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
	info, err := v.info(id)
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
	hash := r.PathValue("hash")
	if !validHash(hash) {
		http.Error(w, "hash must be 64 lowercase hex chars", http.StatusBadRequest)
		return
	}
	format := resolveFormat(v.cfg, r.URL.Query().Get("format"))
	variant := TNVariantSuffix(r.URL.Query().Get("variant"))

	cachePath, cacheErr := safeCachePath(v.cacheRoot, hash, format)
	if cacheErr == nil && v.serveStoredImage(w, format, cachePath, "cache") {
		return
	}

	body, err := v.client.getImage(r.Context(), ThumbnailURLVariant(hash, format, variant, v.client.GG()), [2]int64{-1, -1})
	if err == nil {
		writeImage(w, format, body, "tn")
		return
	}
	if v.serveLocalImage(w, r, hash, format) {
		return
	}
	v.imageProxy(w, r)
}

func (v *viewer) imageProxy(w http.ResponseWriter, r *http.Request) {
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
	if cacheErr == nil && v.serveStoredImage(w, format, cachePath, "cache") {
		return
	}

	gg := v.client.GG()
	body, err := v.client.getImage(r.Context(), StorageURL(hash, format, gg), [2]int64{-1, -1})
	if err == nil {
		writeImage(w, format, body, "miss")
		if cacheErr == nil && len(body) > 0 {
			_ = os.MkdirAll(v.cacheRoot, 0o755)
			_ = os.WriteFile(cachePath, body, 0o644)
		}
		return
	}

	http.Error(w, "image: "+err.Error(), http.StatusBadGateway)
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
	return v.serveStoredImage(w, format, full, "local")
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

func (v *viewer) info(id int) (*GalleryInfo, error) {
	if cached, ok := v.infoCache[id]; ok {
		return cached, nil
	}
	info, err := v.client.Gallery(id)
	if err != nil {
		return nil, err
	}
	v.infoCache[id] = info
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
