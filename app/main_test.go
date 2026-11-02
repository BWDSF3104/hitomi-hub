package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	status := processFile(db, cfg(), &RateState{}, root, path, summary)
	if status != "scan_error" {
		t.Fatalf("status: got %s", status)
	}
	if len(summary) != 0 {
		t.Errorf("summary must not increment on failed DB write: got %v", summary)
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
}
