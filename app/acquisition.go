package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func safeAcquisitionPath(acquisitionRoot, rel string) (string, error) {
	if rel == "" {
		return "", errors.New("empty path")
	}
	clean := filepath.Clean("/" + filepath.FromSlash(rel))
	full := filepath.Join(acquisitionRoot, clean)
	if full == acquisitionRoot || !strings.HasPrefix(full, acquisitionRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path outside acquisition dir: %s", rel)
	}
	return full, nil
}

// R-05（decision 2026-11-06・案 A）: folder 名は「作品名 + 作品番号」。変換順は
// sanitize（禁止文字 `_` 置換・既存挙動維持）→ 末尾の `.`・末尾空白除去 → Windows
// 予約名回避 → title 部長さ上限截断 → gallery id 接尾。check・acquire はこの同一関数
// を共有する（B-04 統一は維持）。
const acquisitionTitleMaxRunes = 200

func acquisitionTitleName(title, galleryID string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return galleryID
	}
	var out strings.Builder
	for _, r := range title {
		if r < 32 || r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			out.WriteRune('_')
		} else {
			out.WriteRune(r)
		}
	}
	sanitized := strings.TrimRight(out.String(), ". ")
	if sanitized == "" {
		return galleryID
	}
	if isWindowsReservedName(sanitized) {
		sanitized = "_" + sanitized
	}
	if r := []rune(sanitized); len(r) > acquisitionTitleMaxRunes {
		sanitized = string(r[:acquisitionTitleMaxRunes])
	}
	return sanitized + "_" + galleryID
}

func isWindowsReservedName(name string) bool {
	upper := strings.ToUpper(name)
	switch upper {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(upper) == 4 && (strings.HasPrefix(upper, "COM") || strings.HasPrefix(upper, "LPT")) && upper[3] >= '1' && upper[3] <= '9' {
		return true
	}
	return false
}

func acquisitionPageName(page int, format string) string {
	return fmt.Sprintf("%03d.%s", page, normalizeFormat(format))
}

func acquisitionRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

func acquisitionTitleFromDB(db *sql.DB, galleryID int) string {
	var title sql.NullString
	err := db.QueryRow("SELECT title FROM galleries WHERE gallery_id = ?", galleryID).Scan(&title)
	if err != nil || title.String == "" {
		return ""
	}
	return title.String
}

func acquisitionStatus(w http.ResponseWriter, status string, fields map[string]any) {
	out := map[string]any{"status": status}
	for key, value := range fields {
		out[key] = value
	}
	jsonOut(w, out)
}

func (v *viewer) acquisitionCheck(w http.ResponseWriter, r *http.Request) {
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

	// B-04（decision 2026-11-03）: check は acquire と同一の共有関数で folder を指す。
	// 「命名は変更しない」の点は R-05（decision 2026-11-06・案 A）で supersede（gallery id
	// 接尾・予約名・末尾点・末尾空白・長さ上限）。
	titleName := acquisitionTitleName(acquisitionTitleFromDB(v.db, body.GalleryID), strconv.Itoa(body.GalleryID))
	titlePath, err := safeAcquisitionPath(v.acquisitionRoot, titleName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if _, err := os.Stat(titlePath); err == nil {
		acquisitionStatus(w, "confirm", map[string]any{
			"gallery_id": body.GalleryID,
			"title":      titleName,
			"path":       acquisitionRel(v.root, titlePath),
		})
		return
	}
	acquisitionStatus(w, "not_exists", map[string]any{
		"gallery_id": body.GalleryID,
		"title":      titleName,
		"path":       acquisitionRel(v.root, titlePath),
	})
}

func (v *viewer) acquire(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GalleryID int    `json:"gallery_id"`
		Page      int    `json:"page"`
		Hash      string `json:"hash"`
		Format    string `json:"format"`
		Overwrite bool   `json:"overwrite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.GalleryID <= 0 || body.Page < 1 || body.Hash == "" {
		http.Error(w, "gallery_id, page >= 1, hash, format, and overwrite are required", http.StatusBadRequest)
		return
	}

	format := normalizeFormat(body.Format)
	if format == "" {
		format = imageFormat(v.cfg)
	}
	if !validHash(body.Hash) {
		http.Error(w, "invalid hash", http.StatusBadRequest)
		return
	}

	cachePath, err := safeCachePath(v.cacheRoot, body.Hash, format)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		acquisitionStatus(w, "missing cache", map[string]any{
			"gallery_id": body.GalleryID,
			"page":       body.Page,
			"hash":       body.Hash,
			"format":     format,
			"path":       acquisitionRel(v.root, cachePath),
			"error":      err.Error(),
		})
		return
	}

	if err := recordDiscovered(v.db, body.GalleryID, galleryDir(v.cfg, body.GalleryID)); err != nil {
		http.Error(w, "gallery: "+err.Error(), http.StatusInternalServerError)
		return
	}

	titleName := acquisitionTitleName(acquisitionTitleFromDB(v.db, body.GalleryID), strconv.Itoa(body.GalleryID))
	titlePath, err := safeAcquisitionPath(v.acquisitionRoot, titleName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if !body.Overwrite {
		if stat, err := os.Stat(titlePath); err == nil && stat.IsDir() {
			acquisitionStatus(w, "confirm", map[string]any{
				"gallery_id": body.GalleryID,
				"page":       body.Page,
				"hash":       body.Hash,
				"format":     format,
				"title":      titleName,
				"path":       acquisitionRel(v.root, titlePath),
			})
			return
		}
	}

	pageName := acquisitionPageName(body.Page, format)
	dest, err := safeAcquisitionPath(v.acquisitionRoot, filepath.Join(titleName, pageName))
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if !body.Overwrite {
		if _, err := os.Stat(dest); err == nil {
			acquisitionStatus(w, "confirm", map[string]any{
				"gallery_id": body.GalleryID,
				"page":       body.Page,
				"hash":       body.Hash,
				"format":     format,
				"title":      titleName,
				"path":       acquisitionRel(v.root, dest),
			})
			return
		}
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		http.Error(w, "acquisition: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		http.Error(w, "acquisition: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ext, magic, _, scanStatus := ScanFile(dest, v.cfg)
	sha, err := Sha256File(dest)
	if err != nil {
		http.Error(w, "acquisition: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rel, err := filepath.Rel(v.root, dest)
	if err != nil {
		rel = dest
	}
	if err := recordFile(v.db, rel, sql.NullInt64{Int64: int64(body.GalleryID), Valid: true}, sha, fileSize(dest), ext, magic, scanStatus, "acquisition"); err != nil {
		acquisitionStatus(w, "record_error", map[string]any{
			"gallery_id": body.GalleryID,
			"page":       body.Page,
			"hash":       body.Hash,
			"format":     format,
			"title":      titleName,
			"path":       rel,
			"scan_status": scanStatus,
			"error":      err.Error(),
		})
		return
	}

	acquisitionStatus(w, "copied", map[string]any{
		"gallery_id": body.GalleryID,
		"page":       body.Page,
		"hash":       body.Hash,
		"format":     format,
		"title":      titleName,
		"path":       rel,
		"scan_status": scanStatus,
	})
}
