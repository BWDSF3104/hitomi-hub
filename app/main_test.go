package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestResolveRootUsesCwdWithoutGitParent(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cwd := filepath.Join(root, "copied-project")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)

	got, err := filepath.Abs(resolveRoot(""))
	if err != nil {
		t.Fatal(err)
	}
	if got != cwd {
		t.Errorf("resolveRoot: got %q want %q", got, cwd)
	}

	explicit := filepath.Join(root, "scratch/live")
	got, err = filepath.Abs(resolveRoot(explicit))
	if err != nil {
		t.Fatal(err)
	}
	if got != explicit {
		t.Errorf("explicit resolveRoot: got %q want %q", got, explicit)
	}
}

func TestOpenDBAndEnsureSchemaSetSQLitePRAGMAs(t *testing.T) {
	dataDir = dataRoot()
	db, err := openDB(filepath.Join(t.TempDir(), "hitomi.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Errorf("journal_mode: got %q", journalMode)
	}
	var busyTimeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if busyTimeout <= 0 {
		t.Errorf("busy_timeout: got %d", busyTimeout)
	}
	if err := ensureSchema(db); err != nil {
		t.Fatal(err)
	}
	var userVersion int
	if err := db.QueryRow("PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatal(err)
	}
	if userVersion != schemaVersion {
		t.Errorf("user_version: got %d want %d", userVersion, schemaVersion)
	}

	var seenCount int
	if err := db.QueryRow("SELECT COUNT(1) FROM bookmark_seen").Scan(&seenCount); err != nil {
		t.Fatal(err)
	}
	var vtCount int
	if err := db.QueryRow("SELECT COUNT(1) FROM vt_rate_state").Scan(&vtCount); err != nil {
		t.Fatal(err)
	}
	if seenCount != 0 || vtCount != 0 {
		t.Errorf("schema tables missing or unusable: seen %d vt %d", seenCount, vtCount)
	}
}

func TestSetReadStatePreservesFullyReadAndClearIsExplicit(t *testing.T) {
	db := openTestDB(t)
	if err := recordDiscovered(db, 100, "Galleries/100"); err != nil {
		t.Fatal(err)
	}
	if err := setViewed(db, 100); err != nil {
		t.Fatal(err)
	}
	if err := setReadState(db, 100, 1, true); err != nil {
		t.Fatal(err)
	}
	if err := setReadState(db, 100, 2, false); err != nil {
		t.Fatal(err)
	}
	row, err := readStateRow(db, 100)
	if err != nil {
		t.Fatal(err)
	}
	if row["fully_read"] != 1 || row["last_page"] != 2 {
		t.Errorf("normal page move must preserve fully_read: got %v", row)
	}
	if row["viewed"] != 1 {
		t.Errorf("setReadState must preserve viewed: got %v", row)
	}
	if err := setReadStateWithClear(db, 100, 3, false, true); err != nil {
		t.Fatal(err)
	}
	row, err = readStateRow(db, 100)
	if err != nil {
		t.Fatal(err)
	}
	if row["fully_read"] != 0 || row["last_page"] != 3 {
		t.Errorf("explicit clear: got %v", row)
	}
	if row["viewed"] != 1 {
		t.Errorf("explicit clear of fully_read must not clear viewed: got %v", row)
	}
}

func TestSchemaMigration6AddsViewedColumnsAndPreservesReadStateRows(t *testing.T) {
	dataDir = dataRoot()
	db, err := openDB(filepath.Join(t.TempDir(), "hitomi.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// pre-migration shape (schemaVersion 5: read_state has no viewed / viewed_at)
	if _, err := db.Exec(`CREATE TABLE galleries (
		gallery_id INTEGER PRIMARY KEY, title TEXT, author TEXT, lang TEXT, dir TEXT,
		first_seen TEXT NOT NULL, last_seen TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE read_state (
		gallery_id INTEGER PRIMARY KEY REFERENCES galleries(gallery_id),
		last_page INTEGER NOT NULL DEFAULT 0, fully_read INTEGER NOT NULL DEFAULT 0,
		opened_at TEXT, favorite INTEGER NOT NULL DEFAULT 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO galleries (gallery_id, first_seen) VALUES (7, ?)", nowISO()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO read_state (gallery_id, last_page, fully_read) VALUES (7, 4, 1)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version = 5"); err != nil {
		t.Fatal(err)
	}

	if err := ensureSchema(db); err != nil {
		t.Fatal(err)
	}
	var userVersion int
	if err := db.QueryRow("PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatal(err)
	}
	if userVersion != schemaVersion {
		t.Errorf("user_version: got %d want %d", userVersion, schemaVersion)
	}

	have := map[string]bool{}
	rows, err := db.Query("PRAGMA table_info(read_state)")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			continue
		}
		have[name] = true
	}
	rows.Close()
	if !have["viewed"] || !have["viewed_at"] {
		t.Fatalf("migration must add viewed / viewed_at columns: got %v", have)
	}

	var lastPage, fully, viewed int
	var viewedAt sql.NullString
	if err := db.QueryRow("SELECT last_page, fully_read, viewed, viewed_at FROM read_state WHERE gallery_id = 7").
		Scan(&lastPage, &fully, &viewed, &viewedAt); err != nil {
		t.Fatal(err)
	}
	if lastPage != 4 || fully != 1 {
		t.Errorf("existing read_state row must survive the non-destructive migration: got last_page=%d fully_read=%d", lastPage, fully)
	}
	if viewed != 0 || viewedAt.Valid {
		t.Errorf("added viewed column must default to 0 / NULL for existing rows: got %d %v", viewed, viewedAt)
	}
}

func TestSetViewedRecordsEventOnceAndKeepsFirstTimestamp(t *testing.T) {
	db := openTestDB(t)
	if err := recordDiscovered(db, 101, "Galleries/101"); err != nil {
		t.Fatal(err)
	}
	if err := setViewed(db, 101); err != nil {
		t.Fatal(err)
	}
	row, err := readStateRow(db, 101)
	if err != nil {
		t.Fatal(err)
	}
	if row["viewed"] != 1 {
		t.Fatalf("setViewed must set viewed: got %v", row)
	}
	first, ok := row["viewed_at"].(string)
	if !ok || first == "" {
		t.Fatalf("first setViewed must write viewed_at: got %v", row)
	}
	if _, err := db.Exec("UPDATE read_state SET viewed_at = 'first-marker' WHERE gallery_id = 101"); err != nil {
		t.Fatal(err)
	}
	if err := setViewed(db, 101); err != nil {
		t.Fatal(err)
	}
	row, err = readStateRow(db, 101)
	if err != nil {
		t.Fatal(err)
	}
	if row["viewed_at"] != "first-marker" {
		t.Errorf("reopen must be idempotent and keep the first viewed_at: got %v", row)
	}
	if row["last_page"] != 0 || row["fully_read"] != 0 || row["favorite"] != 0 {
		t.Errorf("setViewed must not touch other flags: got %v", row)
	}
}

func TestSetFavoriteRowDoesNotBecomeViewed(t *testing.T) {
	db := openTestDB(t)
	if err := recordDiscovered(db, 102, "Galleries/102"); err != nil {
		t.Fatal(err)
	}
	if err := setFavorite(db, 102, true); err != nil {
		t.Fatal(err)
	}
	row, err := readStateRow(db, 102)
	if err != nil {
		t.Fatal(err)
	}
	if row["favorite"] != 1 {
		t.Fatalf("setFavorite must set favorite: got %v", row)
	}
	if row["viewed"] != 0 || row["viewed_at"] != nil || row["last_page"] != 0 {
		t.Errorf("setFavorite must not set viewed (last_page=0 row must stay unviewed): got %v", row)
	}
}

func TestResetReadStateCascadeAndExplicitTargets(t *testing.T) {
	db := openTestDB(t)
	if err := recordDiscovered(db, 103, "Galleries/103"); err != nil {
		t.Fatal(err)
	}
	if err := setViewed(db, 103); err != nil {
		t.Fatal(err)
	}
	if err := setReadState(db, 103, 5, true); err != nil {
		t.Fatal(err)
	}
	row, err := readStateRow(db, 103)
	if err != nil || row["viewed"] != 1 || row["fully_read"] != 1 || row["last_page"] != 5 {
		t.Fatalf("precondition: got %v err %v", row, err)
	}

	if err := resetReadState(db, 103, "read"); err != nil {
		t.Fatal(err)
	}
	row, err = readStateRow(db, 103)
	if err != nil {
		t.Fatal(err)
	}
	if row["fully_read"] != 0 || row["viewed"] != 1 || row["last_page"] != 5 {
		t.Errorf("reset-read must clear fully_read only (keep viewed and last_page): got %v", row)
	}

	if err := resetReadState(db, 103, "viewed"); err != nil {
		t.Fatal(err)
	}
	row, err = readStateRow(db, 103)
	if err != nil {
		t.Fatal(err)
	}
	if row["viewed"] != 0 || row["fully_read"] != 0 || row["last_page"] != 0 {
		t.Errorf("reset-viewed must cascade viewed, fully_read and last_page: got %v", row)
	}

	if err := resetReadState(db, 103, "bogus"); err == nil {
		t.Error("unknown reset target must fail")
	}

	if err := setFavorite(db, 103, true); err != nil {
		t.Fatal(err)
	}
	if err := resetReadState(db, 103, "all"); err != nil {
		t.Fatal(err)
	}
	row, err = readStateRow(db, 103)
	if err != nil {
		t.Fatal(err)
	}
	if row["last_page"] != 0 || row["fully_read"] != 0 || row["viewed"] != 0 || row["favorite"] != 0 {
		t.Errorf("reset-all must clear read, viewed and favorite together: got %v", row)
	}
}

func TestCLIReadResetFlagsUseResetReadState(t *testing.T) {
	text, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(text)
	if !strings.Contains(src, `fsFlags.BoolVar(&f.resetViewed, "reset-viewed"`) || !strings.Contains(src, `fsFlags.BoolVar(&f.resetRead, "reset-read"`) {
		t.Errorf("read command must register --reset-viewed and --reset-read flags")
	}
	readCase := src[strings.Index(src, `case "read":`):strings.Index(src, `case "fav":`)]
	if !strings.Contains(readCase, `resetReadState(db, id, "viewed")`) || !strings.Contains(readCase, `resetReadState(db, id, "read")`) {
		t.Errorf("read command must apply the approved reset cascade through resetReadState: got %q", readCase)
	}
}

func TestGalleryIDForPathExportFileIsNotGalleryID(t *testing.T) {
	got := galleryIDForPath("Galleries/123.cbz", "Galleries")
	if got.Valid {
		t.Errorf("export file must be NULL, got %+v", got)
	}
	got = galleryIDForPath(filepath.Join("Galleries", "123", "file.cbz"), "Galleries")
	if !got.Valid || got.Int64 != 123 {
		t.Errorf("gallery dir file: got %+v", got)
	}
}

func TestRecordScanFileCreatesKnownGalleryParent(t *testing.T) {
	db := openTestDB(t)
	root := t.TempDir()
	galleryID := sql.NullInt64{Int64: 123, Valid: true}
	if err := recordScanFile(db, root, galleryID, filepath.Join("Galleries", "123", "file.cbz"), "0", 1, "cbz", "zip", "pass", "structural"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM galleries WHERE gallery_id = 123").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("known gallery parent: got %d", count)
	}
	var filePath string
	if err := db.QueryRow("SELECT path FROM files WHERE path = ?", filepath.Join("Galleries", "123", "file.cbz")).Scan(&filePath); err != nil {
		t.Fatal(err)
	}
}

func TestProcessFileDoesNotCountSummaryWhenDBWriteFails(t *testing.T) {
	db := openTestDB(t)
	root := t.TempDir()
	path := filepath.Join(root, "Galleries", "100", "bad.cbz")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("PK\x03\x04garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	summary := map[string]int{}
	status := processFile(db, cfg(), &RateState{}, root, path, summary, nil)
	if status != "scan_error" {
		t.Fatalf("status: got %s", status)
	}
	if len(summary) != 0 {
		t.Errorf("summary must not increment on failed DB write: got %v", summary)
	}
}

func TestProcessFileRollsBackFindingsWhenFileRecordFails(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	path := makeZip(t, dir, map[string]string{"../evil.exe": "x"})
	if _, err := db.Exec("DROP TABLE files"); err != nil {
		t.Fatal(err)
	}
	summary := map[string]int{}
	status := processFile(db, cfg(), &RateState{}, dir, path, summary, nil)
	if status != "hit" {
		t.Fatalf("status: got %s", status)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("findings must roll back together with the failed file record: got %d", count)
	}
	if len(summary) != 0 {
		t.Errorf("summary must not increment on failed commit: got %v", summary)
	}
}

func TestProcessFileCommitsFindingsAndFileTogether(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	path := makeZip(t, dir, map[string]string{"../evil.exe": "x"})
	summary := map[string]int{}
	if status := processFile(db, cfg(), &RateState{}, dir, path, summary, nil); status != "hit" {
		t.Fatalf("status: got %s", status)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("committed findings: got %d", count)
	}
	var foundPath string
	if err := db.QueryRow("SELECT path FROM files").Scan(&foundPath); err != nil {
		t.Fatal(err)
	}
	if summary["hit"] != 1 {
		t.Errorf("summary: got %v", summary)
	}
}

func TestAddFindingIsIdempotentForSameFinding(t *testing.T) {
	db := openTestDB(t)
	for i := 0; i < 2; i++ {
		if err := addFinding(db, "Galleries/1/file.cbz", "sha1", "structural", "bad_zip"); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("duplicate finding must be ignored: got %d", count)
	}
	if err := addFinding(db, "Galleries/1/file.cbz", "sha1", "yara", "Zip_Path_Traversal"); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("different rule must still be recorded: got %d", count)
	}
}

func indexDef(t *testing.T, db *sql.DB, name string) string {
	t.Helper()
	var def sql.NullString
	if err := db.QueryRow("SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?", name).Scan(&def); err != nil {
		if err == sql.ErrNoRows {
			return ""
		}
		t.Fatal(err)
	}
	return def.String
}

func TestNewDBFindingsUniqueIndexKeyIncludesPath(t *testing.T) {
	db := openTestDB(t)
	def := indexDef(t, db, "idx_findings_unique")
	if def == "" {
		t.Fatal("new DB must have idx_findings_unique")
	}
	if normalizeIndexSQL(def) != normalizeIndexSQL(findingsUniqueIndexSQL) {
		t.Errorf("index definition: got %q want %q", def, findingsUniqueIndexSQL)
	}
	if err := addFinding(db, "Galleries/1/file.cbz", "sha1", "structural", "bad_zip"); err != nil {
		t.Fatal(err)
	}
	if err := addFinding(db, "Galleries/1/file.cbz", "sha1", "structural", "bad_zip"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("INSERT OR IGNORE must skip duplicate: got %d", count)
	}
}

func TestScanTwiceDoesNotChangeFindingsCount(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	path := makeZip(t, dir, map[string]string{"../evil.exe": "x"})
	summary := map[string]int{}
	if status := processFile(db, cfg(), &RateState{}, dir, path, summary, nil); status != "hit" {
		t.Fatalf("first scan: status %s", status)
	}
	var first int
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&first); err != nil {
		t.Fatal(err)
	}
	if first == 0 {
		t.Fatal("first scan must record findings")
	}
	if status := processFile(db, cfg(), &RateState{}, dir, path, summary, nil); status != "hit" {
		t.Fatalf("second scan: status %s", status)
	}
	var second int
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&second); err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Errorf("findings count after second scan: got %d want %d", second, first)
	}
}

func TestMigrateSchemaAlignsUniqueIndexWithoutDeletingRows(t *testing.T) {
	dataDir = dataRoot()
	db, err := openDB(filepath.Join(t.TempDir(), "hitomi.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// 移行済み（v3）だが old key（path を含まない）unique index を持つ既存 DB を再現する
	if _, err := db.Exec(`CREATE TABLE findings (
		id INTEGER PRIMARY KEY, path TEXT NOT NULL, sha256 TEXT NOT NULL,
		rule TEXT NOT NULL, detail TEXT, found_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE UNIQUE INDEX idx_findings_unique ON findings(sha256, rule, detail)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO findings (path, sha256, rule, detail, found_at) VALUES (?, ?, ?, ?, ?)",
		"Galleries/1/file.cbz", "sha1", "structural", "bad_zip", nowISO()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version = 3"); err != nil {
		t.Fatal(err)
	}
	if err := ensureSchema(db); err != nil {
		t.Fatal(err)
	}
	def := indexDef(t, db, "idx_findings_unique")
	if normalizeIndexSQL(def) != normalizeIndexSQL(findingsUniqueIndexSQL) {
		t.Errorf("index definition after migration: got %q want %q", def, findingsUniqueIndexSQL)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("rows must not be deleted by migration: got %d", count)
	}
	// new key では別 path の同一 content が別 finding として記録される（D-03 の意図）
	if err := addFinding(db, "Galleries/2/file.cbz", "sha1", "structural", "bad_zip"); err != nil {
		t.Fatal(err)
	}
	if err := addFinding(db, "Galleries/1/file.cbz", "sha1", "structural", "bad_zip"); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("same content different path must be recorded, duplicate must be skipped: got %d", count)
	}
	var userVersion int
	if err := db.QueryRow("PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatal(err)
	}
	if userVersion != schemaVersion {
		t.Errorf("user_version: got %d want %d", userVersion, schemaVersion)
	}
}

func TestMigrateSchemaRefusesDuplicatesWithoutDeletingRows(t *testing.T) {
	dataDir = dataRoot()
	db, err := openDB(filepath.Join(t.TempDir(), "hitomi.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// 移行前の findings 形状（unique index なし・重複行あり）を再現する
	if _, err := db.Exec(`CREATE TABLE findings (
		id INTEGER PRIMARY KEY, path TEXT NOT NULL, sha256 TEXT NOT NULL,
		rule TEXT NOT NULL, detail TEXT, found_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := db.Exec("INSERT INTO findings (path, sha256, rule, detail, found_at) VALUES (?, ?, ?, ?, ?)",
			"Galleries/1/file.cbz", "sha1", "structural", "bad_zip", nowISO()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("PRAGMA user_version = 2"); err != nil {
		t.Fatal(err)
	}
	if err := ensureSchema(db); err == nil {
		t.Fatal("duplicate rows must not be removed without operator decision; ensureSchema must fail")
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("duplicate rows must survive the failed migration: got %d", count)
	}
	if indexDef(t, db, "idx_findings_unique") != "" {
		t.Errorf("unique index must not be created while duplicates exist")
	}
}

// A-12: scan_cache への size / mtime 追加は列追加のみ（既存行の削除禁止・D-03 非破壊迁移規約）
func TestMigrateSchemaAddsScanCacheSizeMtimeWithoutDeletingRows(t *testing.T) {
	dataDir = dataRoot()
	db, err := openDB(filepath.Join(t.TempDir(), "hitomi.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sha := strings.Repeat("a", 64)
	// 移行前の scan_cache 形状（v4・size / mtime 列なし・VT verdict 既存行あり）を再現する
	if _, err := db.Exec(`CREATE TABLE scan_cache (
		sha256 TEXT PRIMARY KEY, verdict TEXT NOT NULL, engine TEXT NOT NULL,
		threat_name TEXT, detail TEXT, cached_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO scan_cache (sha256, verdict, engine, threat_name, detail, cached_at) VALUES (?, ?, ?, ?, ?, ?)",
		sha, "pass", "virustotal", "", "positives=0", nowISO()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version = 4"); err != nil {
		t.Fatal(err)
	}
	if err := ensureSchema(db); err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	rows, err := db.Query("PRAGMA table_info(scan_cache)")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			continue
		}
		have[name] = true
	}
	rows.Close()
	if !have["size"] || !have["mtime"] {
		t.Fatalf("scan_cache must have size / mtime columns after migration: got %v", have)
	}
	var verdict, detail string
	if err := db.QueryRow("SELECT verdict, detail FROM scan_cache WHERE sha256 = ?", sha).Scan(&verdict, &detail); err != nil {
		t.Fatal(err)
	}
	if verdict != "pass" || detail != "positives=0" {
		t.Errorf("existing VT verdict row must survive the migration: got verdict=%q detail=%q", verdict, detail)
	}
	var userVersion int
	if err := db.QueryRow("PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatal(err)
	}
	if userVersion != schemaVersion {
		t.Errorf("user_version: got %d want %d", userVersion, schemaVersion)
	}
}

func findingsSnapshot(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.Query("SELECT path, sha256, rule, detail FROM findings ORDER BY path, rule, detail")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var path, sha, rule string
		var detail sql.NullString
		if err := rows.Scan(&path, &sha, &rule, &detail); err != nil {
			t.Fatal(err)
		}
		out = append(out, path+"\t"+sha+"\t"+rule+"\t"+detail.String)
	}
	return strings.Join(out, "\n")
}

// A-12 合格条件固定: 2 回目 scan で unchanged ファイルの hash 計算回数は 0（skip）、
// findings / scan_status は 1 回目と同一。skip は VT gate を走らせない（回数を増やさない）。
func TestProcessFileSkipsUnchangedFileBySizeAndMtime(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	path := makeZip(t, dir, map[string]string{"ok.txt": "y"})
	c := cfg()
	c.Yr = ""
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 5, MaxDaily: 5, APIKey: "k"}

	hashCalls := 0
	origHash := hashFunc
	hashFunc = func(p string) (string, error) {
		hashCalls++
		return origHash(p)
	}
	defer func() { hashFunc = origHash }()

	fetchCalls := 0
	fetch := func(url, key string) (map[string]any, int, error) {
		fetchCalls++
		return nil, 404, nil
	}

	summary1 := map[string]int{}
	status1 := processFile(db, c, &RateState{}, dir, path, summary1, fetch)
	if status1 != "pass" {
		t.Fatalf("run 1 status: got %s", status1)
	}
	if hashCalls != 1 || fetchCalls != 1 {
		t.Fatalf("run 1 must hash and gate once: hashCalls %d fetchCalls %d", hashCalls, fetchCalls)
	}
	var statusRow1, scanTime1 string
	if err := db.QueryRow("SELECT scan_status, scan_time FROM files").Scan(&statusRow1, &scanTime1); err != nil {
		t.Fatal(err)
	}
	findings1 := findingsSnapshot(t, db)

	summary2 := map[string]int{}
	status2 := processFile(db, c, &RateState{}, dir, path, summary2, fetch)
	if status2 != status1 {
		t.Errorf("run 2 status: got %s want %s", status2, status1)
	}
	if hashCalls != 1 {
		t.Errorf("unchanged file must skip hash on second scan: hashCalls %d want 1", hashCalls)
	}
	if fetchCalls != 1 {
		t.Errorf("skip must not increase VT gate calls: fetchCalls %d", fetchCalls)
	}
	if summary2["skipped"] != 1 {
		t.Errorf("run 2 summary must count skipped: got %v", summary2)
	}
	var statusRow2, scanTime2 string
	if err := db.QueryRow("SELECT scan_status, scan_time FROM files").Scan(&statusRow2, &scanTime2); err != nil {
		t.Fatal(err)
	}
	if statusRow2 != statusRow1 {
		t.Errorf("scan_status must be kept: got %s want %s", statusRow2, statusRow1)
	}
	if findingsSnapshot(t, db) != findings1 {
		t.Errorf("findings must be identical to run 1")
	}

	// mtime 不一致は再ハッシュ（安全側）
	newTime := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(path, newTime, newTime); err != nil {
		t.Fatal(err)
	}
	summary3 := map[string]int{}
	if status := processFile(db, c, &RateState{}, dir, path, summary3, fetch); status != status1 {
		t.Errorf("mtime mismatch status: got %s", status)
	}
	if hashCalls != 2 {
		t.Errorf("mtime mismatch must rehash: hashCalls %d want 2", hashCalls)
	}
	if findingsSnapshot(t, db) != findings1 {
		t.Errorf("findings must stay identical after mtime mismatch rehash")
	}

	// scan_cache record なし（行削除）は再ハッシュ（安全側）
	sha, err := Sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM scan_cache WHERE sha256 = ?", sha); err != nil {
		t.Fatal(err)
	}
	summary4 := map[string]int{}
	if status := processFile(db, c, &RateState{}, dir, path, summary4, fetch); status != status1 {
		t.Errorf("missing record status: got %s", status)
	}
	if hashCalls != 3 {
		t.Errorf("missing scan_cache record must rehash: hashCalls %d want 3", hashCalls)
	}
	if findingsSnapshot(t, db) != findings1 {
		t.Errorf("findings must stay identical after missing record rehash")
	}
}

// A-12: size / mtime 一致で skip、external_removal 記録は安全側で再ハッシュ
func TestProcessFileRehashesOnSizeMismatchAndExternalRemoval(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	path := makeZip(t, dir, map[string]string{"ok.txt": "y"}) // path PK（rel）衝突回避で dir2 は別名を使う

	hashCalls := 0
	origHash := hashFunc
	hashFunc = func(p string) (string, error) {
		hashCalls++
		return origHash(p)
	}
	defer func() { hashFunc = origHash }()

	c := cfg()
	c.Yr = ""
	summary := map[string]int{}
	if status := processFile(db, c, &RateState{}, dir, path, summary, nil); status != "pass" {
		t.Fatalf("run 1 status: got %s", status)
	}
	if hashCalls != 1 {
		t.Fatalf("run 1 hash calls: got %d", hashCalls)
	}

	// size 不一致（内容は同じだがサイズ変更）→ 再ハッシュ
	data, err := os.ReadFile(path) // 有効な zip のまま複製を先に取る（path は overwritten される）
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("PK\x03\x04garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	summary2 := map[string]int{}
	if status := processFile(db, c, &RateState{}, dir, path, summary2, nil); status != "scan_error" {
		t.Fatalf("size mismatch status: got %s", status)
	}
	if hashCalls != 2 {
		t.Errorf("size mismatch must rehash: hashCalls %d want 2", hashCalls)
	}

	// 安定した pass ファイルで external_removal 記録のみ変えて skip しないこと（安全側）
	dir2 := t.TempDir()
	path2 := filepath.Join(dir2, "second.cbz") // path PK（rel）衝突を避ける
	if err := os.WriteFile(path2, data, 0o644); err != nil {
		t.Fatal(err)
	}
	summary3 := map[string]int{}
	if status := processFile(db, c, &RateState{}, dir2, path2, summary3, nil); status != "pass" {
		t.Fatalf("run 3 status: got %s", status)
	}
	if hashCalls != 3 {
		t.Errorf("new file must hash: hashCalls %d want 3", hashCalls)
	}
	rel2, err := filepath.Rel(dir2, path2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE files SET scan_status = 'external_removal' WHERE path = ?", rel2); err != nil {
		t.Fatal(err)
	}
	before := hashCalls
	summary4 := map[string]int{}
	if status := processFile(db, c, &RateState{}, dir2, path2, summary4, nil); status != "pass" {
		t.Fatalf("external_removal status: got %s", status)
	}
	if hashCalls != before+1 {
		t.Errorf("external_removal record must rehash (safe side): hashCalls %d want %d", hashCalls, before+1)
	}
	if summary4["skipped"] != 0 {
		t.Errorf("external_removal must not be counted as skipped: got %v", summary4)
	}
}

func TestResolveConfigPathPrefersNonTrackedConfigJSONAndFallsBackToExample(t *testing.T) {
	dir := t.TempDir()
	primary := filepath.Join(dir, "config.json")
	if err := os.WriteFile(primary, []byte(`{"api_key":"test"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := resolveConfigPath(dir, ""); got != primary {
		t.Fatalf("config.json should be preferred: got %q want %q", got, primary)
	}

	explicit := filepath.Join(dir, "explicit.json")
	if got := resolveConfigPath(dir, explicit); got != explicit {
		t.Fatalf("explicit path must survive: got %q want %q", got, explicit)
	}
}

func TestResolveConfigPathFallsBackToExampleWhenConfigJSONMissing(t *testing.T) {
	dir := t.TempDir()
	if got := resolveConfigPath(dir, ""); got != filepath.Join(dir, "config.example.json") {
		t.Fatalf("missing config.json should fall back to config.example.json: got %q", got)
	}
}

func TestLoadConfigPreservesUseLiveGGWhenOmitted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"hitomi":{"min_interval_ms":1000}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Hitomi.UseLiveGG {
		t.Errorf("omitted use_live_gg in loadConfig should remain true: got %v", cfg.Hitomi.UseLiveGG)
	}

	path = filepath.Join(t.TempDir(), "config-off.json")
	if err := os.WriteFile(path, []byte(`{"hitomi":{"min_interval_ms":1000,"use_live_gg":false}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Hitomi.UseLiveGG {
		t.Errorf("explicit use_live_gg false should remain false: got %v", cfg.Hitomi.UseLiveGG)
	}
}

func TestLoadConfigWithoutHitomiKeyUsesLiveGGViaMock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"viewer":{"port":8732}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Hitomi.UseLiveGG {
		t.Fatalf("config without hitomi key should default UseLiveGG true: got %v", cfg.Hitomi.UseLiveGG)
	}

	c := &Client{cfg: cfg.Hitomi}
	calls := 0
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		return []byte(ggFixtureA), nil
	}
	if source := c.LoadGG(context.Background()); source != "live" {
		t.Fatalf("mock live source: got %s", source)
	}
	if calls != 1 {
		t.Fatalf("mock fetch calls: got %d", calls)
	}
	metrics := c.metricsSnapshot()
	metric, ok := metrics["gg"].(map[string]int64)
	if !ok {
		t.Fatalf("metrics gg group missing: got %v", metrics)
	}
	if metric["calls"] != 1 {
		t.Errorf("metrics gg calls: got %v", metric)
	}

	path = filepath.Join(t.TempDir(), "config-false.json")
	if err := os.WriteFile(path, []byte(`{"hitomi":{"use_live_gg":false}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Hitomi.UseLiveGG {
		t.Fatalf("explicit use_live_gg false should remain false: got %v", cfg.Hitomi.UseLiveGG)
	}
	c = &Client{cfg: cfg.Hitomi}
	called := false
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		called = true
		return nil, errors.New("offline")
	}
	if source := c.LoadGG(context.Background()); source != "snapshot" {
		t.Fatalf("snapshot fallback source: got %s", source)
	}
	if called {
		t.Errorf("explicit false must not fetch live gg.js")
	}
	if c.GG().Bucket != snapshotGG().Bucket {
		t.Errorf("fallback bucket: got %s want %s", c.GG().Bucket, snapshotGG().Bucket)
	}
}

func TestLoadConfigUsesPerFieldHitomiDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"hitomi":{"min_interval_ms":1000,"user_agent":"custom UA","image_format":"avif"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	defs := defaultHitomiConfig()
	if cfg.Hitomi.MinIntervalMS != 1000 || cfg.Hitomi.UserAgent != "custom UA" || cfg.Hitomi.ImageFormat != "avif" {
		t.Errorf("explicit hitomi fields should survive defaults: got %+v", cfg.Hitomi)
	}
	if cfg.Hitomi.MaxRetries != defs.MaxRetries || cfg.Hitomi.RetryBackoffMS != defs.RetryBackoffMS {
		t.Errorf("omitted hitomi fields should use defaults: got %+v", cfg.Hitomi)
	}
	if !cfg.Hitomi.UseLiveGG {
		t.Errorf("omitted use_live_gg should use default true: got %+v", cfg.Hitomi)
	}
}

func TestLoadConfigLimitsPreservesExplicitZeroAndDefaultsOmittedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"limits":{"max_entries":0,"max_ratio":10}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	defs := defaultLimits()
	if cfg.Limits.MaxEntries != 0 || cfg.Limits.MaxRatio != 10 {
		t.Errorf("explicit limits fields should survive defaults: got %+v", cfg.Limits)
	}
	if cfg.Limits.MaxUncompressed != defs.MaxUncompressed || cfg.Limits.MaxScanBytes != defs.MaxScanBytes ||
		cfg.Limits.YrTimeoutSec != defs.YrTimeoutSec || cfg.Limits.YrCpuLimit != defs.YrCpuLimit ||
		cfg.Limits.YrSkipLarger != defs.YrSkipLarger || cfg.Limits.YrFastScan != defs.YrFastScan {
		t.Errorf("omitted limits fields should use defaults: got %+v", cfg.Limits)
	}
}

func TestOpenDBDSNCarriesPragmaSettings(t *testing.T) {
	dsn := dbDSN(filepath.Join(t.TempDir(), "hitomi.db"))
	for _, want := range []string{"_pragma=foreign_keys(1)", "_pragma=journal_mode(WAL)", "_pragma=busy_timeout(5000)"} {
		if !strings.Contains(dsn, want) {
			t.Errorf("DSN missing pragma setting %q: got %q", want, dsn)
		}
	}
	db, err := openDB(filepath.Join(t.TempDir(), "hitomi.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Errorf("DSN journal_mode: got %q", journalMode)
	}
	var busyTimeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if busyTimeout != 5000 {
		t.Errorf("DSN busy_timeout: got %d", busyTimeout)
	}
	var foreignKeys int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Errorf("DSN foreign_keys: got %d", foreignKeys)
	}
}

func TestFindCommandKeepsFlagValues(t *testing.T) {
	cmd, rest := findCommand([]string{"--root", "C:\\data", "--no-vt"})
	if cmd != "scan" || len(rest) != 3 {
		t.Errorf("flags only: cmd=%q rest=%v", cmd, rest)
	}
	cmd, rest = findCommand([]string{"search", "--term", "female:yuri", "--page", "2"})
	if cmd != "search" || len(rest) != 4 {
		t.Errorf("subcommand first: cmd=%q rest=%v", cmd, rest)
	}
	cmd, rest = findCommand([]string{"--root", "C:\\data", "list", "--list", "unread"})
	if cmd != "list" || len(rest) != 4 {
		t.Errorf("subcommand after flags: cmd=%q rest=%v", cmd, rest)
	}
	cmd, rest = findCommand([]string{"--root=C:\\data", "fav", "--id", "125"})
	if cmd != "fav" || len(rest) != 3 {
		t.Errorf("inline value: cmd=%q rest=%v", cmd, rest)
	}
	cmd, rest = findCommand([]string{"--config", "config.json", "scan"})
	if cmd != "scan" || len(rest) != 2 {
		t.Errorf("config value: cmd=%q rest=%v", cmd, rest)
	}
	if !valueFlags["config"] {
		t.Errorf("--config must be registered as a value flag")
	}
}

// A-02/C-03 合格条件テスト: commit 回数を dbTransactor fake で objectively 測る（mocked fetch/DB）。
// autocommit の 1 Exec = 1 implicit commit、tx 内 Exec は commit に数えず、tx Commit を 1 commit として計上する。

type testCommitCounter struct {
	mu     sync.Mutex
	events []string
	inTx   bool
}

func (c *testCommitCounter) add(event string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
}

func (c *testCommitCounter) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.events...)
}

func (c *testCommitCounter) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = nil
	c.inTx = false
}

func (c *testCommitCounter) commits() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, e := range c.events {
		if e == "commit" || e == "exec-autocommit" {
			n++
		}
	}
	return n
}

type fakeResult struct{}

func (fakeResult) LastInsertId() (int64, error) { return 1, nil }
func (fakeResult) RowsAffected() (int64, error) { return 1, nil }

type fakeExec struct {
	counter   *testCommitCounter
	galleries map[int]bool
}

func (f *fakeExec) Exec(query string, args ...any) (sql.Result, error) {
	if f.counter.inTx {
		f.counter.add("exec-in-tx")
	} else {
		f.counter.add("exec-autocommit")
	}
	if strings.Contains(query, "INSERT INTO galleries") {
		id, ok := args[0].(int)
		if !ok {
			return nil, errors.New("fakeExec: gallery id arg missing")
		}
		f.galleries[id] = true // ON CONFLICT DO UPDATE: 既存 id は行数を増やさない
	}
	return fakeResult{}, nil
}

func (f *fakeExec) Commit() error {
	if !f.counter.inTx {
		return errors.New("fakeExec: commit outside tx")
	}
	f.counter.add("commit")
	f.counter.inTx = false
	return nil
}

func (f *fakeExec) Rollback() error {
	if !f.counter.inTx {
		return errors.New("fakeExec: rollback outside tx")
	}
	f.counter.add("rollback")
	f.counter.inTx = false
	return nil
}

type fakeTransactor struct {
	fake    *fakeExec
	counter *testCommitCounter
}

func (ft *fakeTransactor) Begin() (txExec, error) {
	ft.counter.add("begin")
	ft.counter.inTx = true
	return ft.fake, nil
}

func TestRecordDiscoveredBatchCommitsOncePerRequestMockedFetch(t *testing.T) {
	counter := &testCommitCounter{}
	fake := &fakeExec{counter: counter, galleries: map[int]bool{}}
	tr := &fakeTransactor{fake: fake, counter: counter}
	root := t.TempDir()
	ids := make([]int, 25)
	for i := range ids {
		ids[i] = 1000 + i
	}

	// mocked fetch: external call は record へ入る前に完了する。event 順序で固定する
	mockedFetch := func() []int {
		counter.add("fetch")
		return ids
	}
	got := mockedFetch()
	if err := recordDiscoveredBatchTx(tr, got, root); err != nil {
		t.Fatal(err)
	}
	events := counter.snapshot()
	if events[0] != "fetch" {
		t.Fatalf("external call must complete before the tx: %v", events)
	}
	if events[1] != "begin" || events[len(events)-1] != "commit" {
		t.Fatalf("record phase must be one tx with 1 commit: %v", events)
	}
	tail := events[2 : len(events)-1]
	if len(tail) != 25 {
		t.Fatalf("want 25 exec-in-tx between begin and commit: %v", events)
	}
	for _, e := range tail {
		if e != "exec-in-tx" {
			t.Fatalf("no autocommit commit inside the tx: %v", events)
		}
	}
	if counter.commits() != 1 {
		t.Fatalf("commits per request: got %d events %v", counter.commits(), events)
	}
	if len(fake.galleries) != 25 {
		t.Fatalf("rows: got %d", len(fake.galleries))
	}

	// 修正前の基準（1 Exec = 1 autocommit commit）: mocked DB で request あたり commit 回数が 25 になることを示す
	counter.reset()
	for _, id := range ids {
		if _, err := fake.Exec(`INSERT INTO galleries (gallery_id, dir) VALUES (?, ?)`, id, galleryDir(root, id)); err != nil {
			t.Fatal(err)
		}
	}
	if counter.commits() != 25 {
		t.Fatalf("per-id autocommit baseline commits: got %d events %v", counter.commits(), counter.snapshot())
	}

	// 2 回目（同じ ids）も 1 commit。行数は 2 回実行で不変（unique key・INSERT OR IGNORE の意味を変えない）
	counter.reset()
	if err := recordDiscoveredBatchTx(tr, ids, root); err != nil {
		t.Fatal(err)
	}
	if counter.commits() != 1 {
		t.Fatalf("repeat request commits: got %d events %v", counter.commits(), counter.snapshot())
	}
	if len(fake.galleries) != 25 {
		t.Errorf("rows after second pass: got %d", len(fake.galleries))
	}

	// ids が空のときは tx を開かない（修正前と同じく commit 0・書き込みなし）
	counter.reset()
	if err := recordDiscoveredBatchTx(tr, nil, root); err != nil {
		t.Fatal(err)
	}
	if len(counter.snapshot()) != 0 {
		t.Errorf("empty ids must not open a tx: %v", counter.snapshot())
	}
}

// real DB 側で commit が実際に永続化する（commit 0 で終わらない）ことを openTestDB で確認する
func TestRecordDiscoveredBatchPersistsRowsOnRealDB(t *testing.T) {
	db := openTestDB(t)
	root := t.TempDir()
	ids := make([]int, 25)
	for i := range ids {
		ids[i] = 1000 + i
	}
	if err := recordDiscoveredBatch(db, ids, root); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := db.QueryRow("SELECT COUNT(1) FROM galleries").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 25 {
		t.Fatalf("galleries rows: got %d", rows)
	}
	if err := recordDiscoveredBatch(db, ids, root); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(1) FROM galleries").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 25 {
		t.Errorf("galleries rows after second pass: got %d", rows)
	}
}
