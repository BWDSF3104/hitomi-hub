package main

import (
	"archive/zip"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type exportEntry struct {
	index  int
	hash   string
	path   string
	status string
}

// fileRows は gallery の files 行を rowid 順（download 時の page 順）で返す。files 行には hitomi の
// 内部 hash は無く sha256 しかないため、保存名 `<hash>.<format>` から hash を取り出して照合する
func fileRows(db *sql.DB, galleryID int) ([]exportEntry, error) {
	rows, err := db.Query("SELECT path, scan_status FROM files WHERE gallery_id = ? ORDER BY rowid", galleryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportEntry{}
	for rows.Next() {
		var path, status string
		if err := rows.Scan(&path, &status); err != nil {
			return nil, err
		}
		hash := hashOf(path)
		out = append(out, exportEntry{hash: hash, path: path, status: status})
	}
	return out, nil
}

// exportPages は pageorder を優先する。pageorder が無い gallery は files 行の挿入順
func exportPages(db *sql.DB, galleryID int, dirName, format string) ([]exportEntry, string, error) {
	rows, err := db.Query("SELECT page_index, hash FROM pageorder WHERE gallery_id = ? ORDER BY page_index", galleryID)
	if err != nil {
		return nil, "", err
	}
	type pageRow struct {
		index int
		hash  string
	}
	pages := []pageRow{}
	for rows.Next() {
		var index int
		var hash string
		if err := rows.Scan(&index, &hash); err != nil {
			rows.Close()
			return nil, "", err
		}
		pages = append(pages, pageRow{index, hash})
	}
	rows.Close()

	files, err := fileRows(db, galleryID)
	if err != nil {
		return nil, "", err
	}
	byHash := map[string]exportEntry{}
	for _, row := range files {
		if _, seen := byHash[row.hash]; !seen {
			byHash[row.hash] = row
		}
	}

	if len(pages) > 0 {
		out := []exportEntry{}
		for _, p := range pages {
			row, ok := byHash[p.hash]
			if !ok {
				out = append(out, exportEntry{index: p.index, hash: p.hash,
					path: filepath.Join(dirName, strconv.Itoa(galleryID), p.hash+"."+format)})
				continue
			}
			out = append(out, exportEntry{index: p.index, hash: p.hash, path: row.path, status: row.status})
		}
		return out, "pageorder", nil
	}
	for i := range files {
		files[i].index = i + 1
	}
	return files, "insertion", nil
}

// exportCBZ は保存済み画像を page 順で zip に詰める（Store・Deflate なし）。hit / scan_error は skip
func exportCBZ(db *sql.DB, cfg Config, root string, galleryID int, out string) (map[string]any, error) {
	dirName := orDefault(cfg.GalleryDir, "Galleries")
	entries, order, err := exportPages(db, galleryID, dirName, orDefault(cfg.Hitomi.ImageFormat, "webp"))
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, errors.New("no files recorded for gallery " + strconv.Itoa(galleryID))
	}
	target := out
	if target == "" {
		target = filepath.Join(root, dirName, strconv.Itoa(galleryID)+".cbz")
	}
	if !withinRoot(root, target) {
		return nil, fmt.Errorf("output must stay inside root: %s", target)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, err
	}
	file, err := os.Create(target)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	written := 0
	skipped := map[string]int{}
	for _, entry := range entries {
		if written >= cfg.Limits.MaxEntries {
			skipped["entry_count_exceeded"]++
			continue
		}
		if entry.status == "hit" || entry.status == "scan_error" {
			skipped[entry.status]++
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, entry.path))
		if err != nil {
			skipped["missing"]++
			continue
		}
		if int64(len(body)) > cfg.Limits.MaxUncompressed {
			skipped["entry_size_exceeded"]++
			continue
		}
		name := fmt.Sprintf("%03d_%s.%s", entry.index, entry.hash, extOf(entry.path))
		if containsTraversal(name) {
			skipped["path_traversal"]++
			continue
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(body); err != nil {
			return nil, err
		}
		written++
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return map[string]any{"gallery_id": galleryID, "output": target, "order": order,
		"entries": written, "skipped": skipped}, nil
}

func extOf(path string) string {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	if ext == "" {
		return "webp"
	}
	return ext
}

// hashOf は保存名 `<hash>.<format>` から hitomi の内部 hash を取り出す（files 行には sha256 しか無い）
func hashOf(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func containsTraversal(name string) bool {
	return strings.Contains(name, "..") || strings.Contains(name, "/")
}

func withinRoot(root, path string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
