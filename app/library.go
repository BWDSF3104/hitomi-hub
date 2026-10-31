package main

import (
	"database/sql"
	"path/filepath"
	"strconv"
)

func upsertGallery(db *sql.DB, info *GalleryInfo, dir string) error {
	_, err := db.Exec(`INSERT INTO galleries (gallery_id, title, author, lang, dir, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(gallery_id) DO UPDATE SET title = excluded.title, author = excluded.author,
		lang = excluded.lang, dir = excluded.dir, last_seen = excluded.last_seen`,
		info.ID, info.Title, firstOf(info.Artists), info.Language, dir, nowISO(), nowISO())
	return err
}

func firstOf(list []string) string {
	if len(list) > 0 {
		return list[0]
	}
	return ""
}

func setReadState(db *sql.DB, galleryID int, page int, complete bool) error {
	flag := 0
	if complete {
		flag = 1
	}
	_, err := db.Exec(`INSERT INTO read_state (gallery_id, last_page, fully_read, opened_at, favorite)
		VALUES (?, ?, ?, ?, 0)
		ON CONFLICT(gallery_id) DO UPDATE SET last_page = excluded.last_page,
		fully_read = excluded.fully_read, opened_at = excluded.opened_at`,
		galleryID, page, flag, nowISO())
	return err
}

func setFavorite(db *sql.DB, galleryID int, on bool) error {
	flag := 0
	if on {
		flag = 1
	}
	_, err := db.Exec(`INSERT INTO read_state (gallery_id, last_page, fully_read, opened_at, favorite)
		VALUES (?, 0, 0, ?, ?)
		ON CONFLICT(gallery_id) DO UPDATE SET favorite = excluded.favorite`,
		galleryID, nowISO(), flag)
	return err
}

func addQuery(db *sql.DB, name, query string, notify int) error {
	_, err := db.Exec("INSERT INTO bookmark_queries (name, query, created_at, notify) VALUES (?, ?, ?, ?)",
		name, query, nowISO(), notify)
	return err
}

func markQueryRun(db *sql.DB, id int) error {
	_, err := db.Exec("UPDATE bookmark_queries SET last_run_at = ? WHERE id = ?", nowISO(), id)
	return err
}

func galleryDir(root string, galleryID int) string {
	return filepath.Join("Galleries", strconv.Itoa(galleryID))
}
