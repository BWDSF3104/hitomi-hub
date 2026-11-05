package main

import (
	"database/sql"
	"errors"
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
	return setReadStateWithClear(db, galleryID, page, complete, false)
}

func setReadStateWithClear(db *sql.DB, galleryID int, page int, complete bool, clearCompletion bool) error {
	flag := 0
	if complete && !clearCompletion {
		flag = 1
	}
	if clearCompletion {
		_, err := db.Exec(`INSERT INTO read_state (gallery_id, last_page, fully_read, opened_at, favorite)
			VALUES (?, ?, ?, ?, 0)
			ON CONFLICT(gallery_id) DO UPDATE SET last_page = excluded.last_page,
			fully_read = excluded.fully_read, opened_at = excluded.opened_at`,
			galleryID, page, flag, nowISO())
		return err
	}
	_, err := db.Exec(`INSERT INTO read_state (gallery_id, last_page, fully_read, opened_at, favorite)
		VALUES (?, ?, ?, ?, 0)
		ON CONFLICT(gallery_id) DO UPDATE SET last_page = excluded.last_page,
		fully_read = CASE WHEN excluded.fully_read = 1 OR fully_read = 1 THEN 1 ELSE fully_read END,
		opened_at = excluded.opened_at`,
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

func markQueryRun(db dbExec, id int) error {
	_, err := db.Exec("UPDATE bookmark_queries SET last_run_at = ? WHERE id = ?", nowISO(), id)
	return err
}

func readStateRow(db *sql.DB, galleryID int) (map[string]any, error) {
	var lastPage, fully, fav, viewed int
	var viewedAt sql.NullString
	err := db.QueryRow("SELECT last_page, fully_read, favorite, viewed, viewed_at FROM read_state WHERE gallery_id = ?", galleryID).
		Scan(&lastPage, &fully, &fav, &viewed, &viewedAt)
	if err == sql.ErrNoRows {
		return map[string]any{"gallery_id": galleryID, "last_page": 0, "fully_read": 0, "favorite": 0, "viewed": 0, "viewed_at": nil}, nil
	}
	if err != nil {
		return nil, err
	}
	var at any
	if viewedAt.Valid {
		at = viewedAt.String
	}
	return map[string]any{"gallery_id": galleryID, "last_page": lastPage, "fully_read": fully, "favorite": fav, "viewed": viewed, "viewed_at": at}, nil
}

// setViewed は viewer document が gallery を開いた事件を記録する（route 入力時点）。
// viewed_at は初回のみ写入（reopen は idempotent・既存 timestamp を上書きしない）。
// setFavorite が作った last_page=0 行とは別経路（setFavorite は viewed を立てない）。
func setViewed(db *sql.DB, galleryID int) error {
	_, err := db.Exec(`INSERT INTO read_state (gallery_id, viewed, viewed_at) VALUES (?, 1, ?)
		ON CONFLICT(gallery_id) DO UPDATE SET viewed = 1,
		viewed_at = COALESCE(viewed_at, excluded.viewed_at)`, galleryID, nowISO())
	return err
}

// resetReadState は手動リセット。cascade（承認済み設計）:
// target "viewed" → viewed=0・fully_read=0・last_page=0（fully_read は viewed を含意するため）。
// target "read" → fully_read=0 のみ（viewed・last_page 保持）。
// target "all" → last_page・fully_read・viewed・favorite をまとめて 0（ファイル由来の saved は削除しない）。
func resetReadState(db *sql.DB, galleryID int, target string) error {
	switch target {
	case "viewed":
		_, err := db.Exec("UPDATE read_state SET viewed = 0, fully_read = 0, last_page = 0 WHERE gallery_id = ?", galleryID)
		return err
	case "read":
		_, err := db.Exec("UPDATE read_state SET fully_read = 0 WHERE gallery_id = ?", galleryID)
		return err
	case "all":
		_, err := db.Exec("UPDATE read_state SET last_page = 0, fully_read = 0, viewed = 0, favorite = 0 WHERE gallery_id = ?", galleryID)
		return err
	}
	return errors.New("reset target must be viewed or read")
}

func recordPage(db *sql.DB, galleryID, pageIndex int, hash string) error {
	_, err := db.Exec(`INSERT INTO pageorder (gallery_id, page_index, hash) VALUES (?, ?, ?)
		ON CONFLICT(gallery_id, page_index) DO UPDATE SET hash = excluded.hash`,
		galleryID, pageIndex, hash)
	return err
}

func galleryDir(root string, galleryID int) string {
	return filepath.Join("Galleries", strconv.Itoa(galleryID))
}
