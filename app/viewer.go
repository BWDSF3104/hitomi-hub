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
	infoCache   map[int]*GalleryInfo
}

func newViewer(db *sql.DB, cfg Config, root string) *viewer {
	return &viewer{
		db:          db,
		cfg:         cfg,
		root:        root,
		galleryRoot: filepath.Join(root, orDefault(cfg.GalleryDir, "Galleries")),
		client:      &Client{http: &http.Client{Timeout: 60 * time.Second}, cfg: cfg.Hitomi},
		infoCache:   map[int]*GalleryInfo{},
	}
}

func (v *viewer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", v.index)
	mux.HandleFunc("GET /api/lists", v.lists)
	mux.HandleFunc("GET /api/search", v.search)
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
	if _, err := os.Stat(full); err != nil {
		http.Error(w, "not downloaded: "+r.PathValue("name"), http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, full)
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
	results := []map[string]any{}
	for _, id := range ids {
		results = append(results, v.searchResult(id))
	}
	jsonOut(w, map[string]any{"query": q, "lang": r.URL.Query().Get("lang"), "page": page,
		"total": total, "ids": ids, "results": results})
}

func (v *viewer) searchResult(id int) map[string]any {
	info, err := v.info(id)
	if err != nil {
		return map[string]any{"id": id, "error": err.Error(), "tags": []string{}}
	}
	thumbnail := ""
	if len(info.Files) > 0 {
		format := imageFormat(v.cfg)
		thumbnail = "/api/thumb/" + info.Files[0].Hash + "?format=" + format + "&gallery=" + strconv.Itoa(id)
	}
	return map[string]any{"id": id, "title": info.Title, "language": info.Language,
		"type": info.Type, "url": info.URL, "tags": info.Tags, "artists": info.Artists,
		"count": len(info.Files), "local": v.localCount(id), "thumbnail": thumbnail}
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
	format := imageFormat(v.cfg)
	files := []map[string]any{}
	for _, entry := range info.Files {
		files = append(files, map[string]any{
			"hash": entry.Hash, "name": entry.Name, "width": entry.Width, "height": entry.Height,
			"local": v.fileLocal(id, entry.Hash, format),
		})
	}
	jsonOut(w, map[string]any{"gallery": map[string]any{
		"id": info.ID, "title": info.Title, "japanese_title": info.JapaneseTitle,
		"language": info.Language, "type": info.Type, "tags": info.Tags, "artists": info.Artists,
		"groups": info.Groups, "characters": info.Characters, "url": info.URL, "files": files,
	}, "read_state": state, "local": v.localCount(id), "format": format})
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
	format := imageFormat(v.cfg)
	entry := info.Files[page-1]
	local, err := safeGalleryPath(v.galleryRoot, strconv.Itoa(id)+"/"+entry.Hash+"."+format)
	if err == nil {
		if _, err := os.Stat(local); err == nil {
			jsonOut(w, map[string]any{"page": page, "src": "/files/" + strconv.Itoa(id) + "/" + entry.Hash + "." + format,
				"local": true, "width": entry.Width, "height": entry.Height})
			return
		}
	}
	jsonOut(w, map[string]any{"page": page, "src": "/api/image/" + entry.Hash + "?format=" + format, "local": false,
		"width": entry.Width, "height": entry.Height})
}

func (v *viewer) thumbProxy(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	if !validHash(hash) {
		http.Error(w, "hash must be 64 lowercase hex chars", http.StatusBadRequest)
		return
	}
	format := imageFormat(v.cfg)
	if q := r.URL.Query().Get("format"); q != "" {
		format = q
	}
	if v.serveLocalImage(w, r, hash, format) {
		return
	}
	body, err := v.client.get(r.Context(), ThumbnailURL(hash, format, v.client.GG()), [2]int64{-1, -1})
	if err != nil {
		v.imageProxy(w, r)
		return
	}
	setImageHeaders(w, format)
	w.Write(body)
}

func (v *viewer) imageProxy(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	if !validHash(hash) {
		http.Error(w, "hash must be 64 lowercase hex chars", http.StatusBadRequest)
		return
	}
	format := imageFormat(v.cfg)
	if q := r.URL.Query().Get("format"); q != "" {
		format = q
	}
	if v.serveLocalImage(w, r, hash, format) {
		return
	}
	body, err := v.client.get(r.Context(), StorageURL(hash, format, v.client.GG()), [2]int64{-1, -1})
	if err != nil {
		http.Error(w, "image: "+err.Error(), http.StatusBadGateway)
		return
	}
	setImageHeaders(w, format)
	w.Write(body)
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
	if _, err := os.Stat(full); err != nil {
		return false
	}
	setImageHeaders(w, format)
	http.ServeFile(w, r, full)
	return true
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
	return orDefault(cfg.Hitomi.ImageFormat, "webp")
}

func jsonOut(w http.ResponseWriter, value any) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}
