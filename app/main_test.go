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

func TestMigrateSchemaCollapsesFindingsDuplicatesAndSetsVersion(t *testing.T) {
	dataDir = dataRoot()
	db, err := openDB(filepath.Join(t.TempDir(), "hitomi.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// 移行前の findings 形状（unique index なし）を再現する
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
	if _, err := db.Exec("INSERT INTO findings (path, sha256, rule, detail, found_at) VALUES (?, ?, ?, ?, ?)",
		"Galleries/1/file.cbz", "sha1", "structural", "path_traversal:x", nowISO()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version = 2"); err != nil {
		t.Fatal(err)
	}
	if err := ensureSchema(db); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("duplicates must be collapsed to first occurrence: got %d", count)
	}
	var userVersion int
	if err := db.QueryRow("PRAGMA user_version").Scan(&userVersion); err != nil {
		t.Fatal(err)
	}
	if userVersion != schemaVersion {
		t.Errorf("user_version: got %d want %d", userVersion, schemaVersion)
	}
	_, err = db.Exec("INSERT INTO findings (path, sha256, rule, detail, found_at) VALUES (?, ?, ?, ?, ?)",
		"Galleries/2/file.cbz", "sha1", "structural", "bad_zip", nowISO())
	if err == nil {
		t.Errorf("duplicate insert must be rejected after migration")
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
