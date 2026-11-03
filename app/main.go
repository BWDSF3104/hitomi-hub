package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var errBadZip = errors.New("bad zip")

var dataDir string

type Limits struct {
	MaxEntries       int   `json:"max_entries"`
	MaxUncompressed  int64 `json:"max_uncompressed_bytes"`
	MaxRatio         int64 `json:"max_ratio"`
	MaxScanBytes     int64 `json:"max_scan_bytes"`
	YrTimeoutSec     int   `json:"yr_timeout_sec"`
	YrCpuLimit       int   `json:"yr_cpu_limit"`
	YrSkipLarger     int64 `json:"yr_skip_larger"`
	YrFastScan       bool  `json:"yr_fast_scan"`
	MaxRequestsPerMinute int `json:"-"`
	MaxDailyRequests int   `json:"-"`
}

type VTConfig struct {
	Enabled       bool   `json:"enabled"`
	HashOnly      bool   `json:"hash_only"`
	UploadUnknown bool   `json:"upload_unknown"`
	APIKeyEnv     string `json:"api_key_env"`
	APIKey        string `json:"api_key"`
	MaxPerMinute  int    `json:"max_requests_per_minute"`
	MaxDaily      int    `json:"max_daily_requests"`
}

type UIConfig struct {
	Port int    `json:"port"`
	Bind string `json:"bind"`
}

type Config struct {
	DB             string        `json:"db"`
	GalleryDir     string        `json:"gallery_dir"`
	CacheDir       string        `json:"cache_dir"`
	AcquisitionDir string        `json:"acquisition_dir"`
	Yr             string        `json:"yr"`
	VirusTotal     VTConfig      `json:"virustotal"`
	Hitomi         HitomiConfig  `json:"hitomi"`
	Viewer         UIConfig      `json:"viewer"`
	Limits         Limits        `json:"limits"`
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func readHead(path string, n int) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	buf := make([]byte, n)
	read, _ := f.Read(buf)
	return buf[:read]
}

func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func defaultLimits() Limits {
	return Limits{
		MaxEntries:      2000,
		MaxUncompressed: 209715200,
		MaxRatio:        100,
		MaxScanBytes:    2097152,
		YrTimeoutSec:    10,
		YrCpuLimit:      50,
		YrSkipLarger:    67108864,
		YrFastScan:      true,
	}
}

func (l *Limits) UnmarshalJSON(data []byte) error {
	var raw struct {
		MaxEntries           *int   `json:"max_entries"`
		MaxUncompressed      *int64 `json:"max_uncompressed_bytes"`
		MaxRatio             *int64 `json:"max_ratio"`
		MaxScanBytes         *int64 `json:"max_scan_bytes"`
		YrTimeoutSec         *int   `json:"yr_timeout_sec"`
		YrCpuLimit           *int   `json:"yr_cpu_limit"`
		YrSkipLarger         *int64 `json:"yr_skip_larger"`
		YrFastScan           *bool  `json:"yr_fast_scan"`
		MaxRequestsPerMinute *int   `json:"-"`
		MaxDailyRequests     *int   `json:"-"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*l = defaultLimits()
	if raw.MaxEntries != nil {
		l.MaxEntries = *raw.MaxEntries
	}
	if raw.MaxUncompressed != nil {
		l.MaxUncompressed = *raw.MaxUncompressed
	}
	if raw.MaxRatio != nil {
		l.MaxRatio = *raw.MaxRatio
	}
	if raw.MaxScanBytes != nil {
		l.MaxScanBytes = *raw.MaxScanBytes
	}
	if raw.YrTimeoutSec != nil {
		l.YrTimeoutSec = *raw.YrTimeoutSec
	}
	if raw.YrCpuLimit != nil {
		l.YrCpuLimit = *raw.YrCpuLimit
	}
	if raw.YrSkipLarger != nil {
		l.YrSkipLarger = *raw.YrSkipLarger
	}
	if raw.YrFastScan != nil {
		l.YrFastScan = *raw.YrFastScan
	}
	if raw.MaxRequestsPerMinute != nil {
		l.MaxRequestsPerMinute = *raw.MaxRequestsPerMinute
	}
	if raw.MaxDailyRequests != nil {
		l.MaxDailyRequests = *raw.MaxDailyRequests
	}
	return nil
}

func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{Limits: defaultLimits(), Hitomi: defaultHitomiConfig()}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.VirusTotal.MaxPerMinute == 0 {
		cfg.VirusTotal.MaxPerMinute = 4
	}
	if cfg.VirusTotal.MaxDaily == 0 {
		cfg.VirusTotal.MaxDaily = 500
	}
	defs := defaultHitomiConfig()
	if cfg.Hitomi.MinIntervalMS == 0 {
		cfg.Hitomi.MinIntervalMS = defs.MinIntervalMS
	}
	if cfg.Hitomi.ImageMinIntervalMS == 0 {
		cfg.Hitomi.ImageMinIntervalMS = defs.ImageMinIntervalMS
	}
	if cfg.Hitomi.GGForceMinIntervalMS == 0 {
		cfg.Hitomi.GGForceMinIntervalMS = defs.GGForceMinIntervalMS
	}
	if cfg.Hitomi.MaxRetries == 0 {
		cfg.Hitomi.MaxRetries = defs.MaxRetries
	}
	if cfg.Hitomi.RetryBackoffMS == 0 {
		cfg.Hitomi.RetryBackoffMS = defs.RetryBackoffMS
	}
	if cfg.Hitomi.UserAgent == "" {
		cfg.Hitomi.UserAgent = defs.UserAgent
	}
	if cfg.Hitomi.ImageFormat == "" {
		cfg.Hitomi.ImageFormat = defs.ImageFormat
	}
	if cfg.AcquisitionDir == "" {
		cfg.AcquisitionDir = "acquisition"
	}
	if cfg.Viewer.Port == 0 {
		cfg.Viewer.Port = 8731
	}
	if cfg.Viewer.Bind == "" {
		cfg.Viewer.Bind = "127.0.0.1"
	}
	return cfg, nil
}

func resolveRoot(explicit string) string {
	if explicit != "" {
		abs, err := filepath.Abs(explicit)
		if err == nil {
			return abs
		}
		return explicit
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

const schemaVersion = 4

// dbExec は *sql.DB と *sql.Tx の共通部分。processFile の tx 内で同じ書き込みヘルパーを使えるようにする
type dbExec interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbDSN(path))
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		return nil, err
	}
	// viewer の handler が同時に書込むと SQLITE_BUSY になるため単一接続に固定
	db.SetMaxOpenConns(1)
	return db, nil
}

func dbDSN(path string) string {
	settings := "_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	if strings.Contains(path, "?") {
		return path + "&" + settings
	}
	return path + "?" + settings
}

func ensureSchema(db *sql.DB) error {
	var current int
	if err := db.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return err
	}
	if err := migrateSchema(db, current); err != nil {
		return err
	}
	text, err := os.ReadFile(filepath.Join(dataDir, "schema.sql"))
	if err != nil {
		return err
	}
	_, err = db.Exec(string(text))
	if err != nil {
		return err
	}
	if err := ensureFindingsUniqueIndex(db); err != nil {
		return err
	}
	if current < schemaVersion {
		_, err = db.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion))
		if err != nil {
			return err
		}
	}
	return nil
}

// migrateSchema は user_version 段階移行。既存行は削除しない。findings の unique index を
// path を key に含める定義へ揃える（定義置換のみ）。新 key に重複行が残る場合は CREATE が
// 失敗し、エラーを返してデータの対応を運用判断へ委ねる（D-03）。
func migrateSchema(db *sql.DB, current int) error {
	if current >= schemaVersion {
		return nil
	}
	var tables int
	if err := db.QueryRow("SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = 'findings'").Scan(&tables); err != nil {
		return err
	}
	if tables == 0 {
		return nil
	}
	return ensureFindingsUniqueIndex(db)
}

const findingsUniqueIndexSQL = "CREATE UNIQUE INDEX IF NOT EXISTS idx_findings_unique ON findings(path, sha256, rule, detail)"

func normalizeIndexSQL(s string) string {
	s = strings.ToLower(s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.Replace(s, "if not exists ", "", 1)
}

// ensureFindingsUniqueIndex は idx_findings_unique を findingsUniqueIndexSQL へ揃える。
// 定義の置換のみで既存行を削除しない。新 key に重複行があるままでは作成できず、
// その場合エラーを返す（データを消してよいかは運用判断）。
func ensureFindingsUniqueIndex(db dbExec) error {
	var def sql.NullString
	err := db.QueryRow("SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'idx_findings_unique'").Scan(&def)
	if err == sql.ErrNoRows {
		_, err = db.Exec(findingsUniqueIndexSQL)
		return err
	}
	if err != nil {
		return err
	}
	if normalizeIndexSQL(def.String) == normalizeIndexSQL(findingsUniqueIndexSQL) {
		return nil
	}
	if _, err := db.Exec("DROP INDEX idx_findings_unique"); err != nil {
		return err
	}
	_, err = db.Exec(findingsUniqueIndexSQL)
	return err
}

const scanCacheTTL = 30 * 24 * time.Hour
const negativeScanCacheTTL = 2 * time.Hour

func cacheLookup(db dbExec, sha string, ttl time.Duration) (string, string, string, string, error) {
	var verdict, engine, threat, detail, cachedAt sql.NullString
	err := db.QueryRow("SELECT verdict, engine, threat_name, detail, cached_at FROM scan_cache WHERE sha256 = ?", sha).
		Scan(&verdict, &engine, &threat, &detail, &cachedAt)
	if err == sql.ErrNoRows {
		return "", "", "", "", nil
	}
	if err != nil {
		return "", "", "", "", err
	}
	if cachedAt.Valid {
		effectiveTTL := ttl
		if verdict.String == "unknown_to_vt" && negativeScanCacheTTL < effectiveTTL {
			effectiveTTL = negativeScanCacheTTL
		}
		cachedTime, parseErr := time.Parse("2006-01-02T15:04:05Z", cachedAt.String)
		if parseErr == nil && effectiveTTL > 0 && time.Since(cachedTime) > effectiveTTL {
			return "", "", "", "", nil
		}
	}
	return verdict.String, engine.String, threat.String, detail.String, nil
}

func cacheStore(db dbExec, sha, verdict, engine, threat, detail string) error {
	_, err := db.Exec(`INSERT INTO scan_cache (sha256, verdict, engine, threat_name, detail, cached_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(sha256) DO UPDATE SET verdict = excluded.verdict, engine = excluded.engine,
		threat_name = excluded.threat_name, detail = excluded.detail, cached_at = excluded.cached_at`,
		sha, verdict, engine, threat, detail, nowISO())
	return err
}

func addFinding(db dbExec, path, sha, rule, detail string) error {
	_, err := db.Exec("INSERT OR IGNORE INTO findings (path, sha256, rule, detail, found_at) VALUES (?, ?, ?, ?, ?)",
		path, sha, rule, detail, nowISO())
	return err
}

func recordFile(db dbExec, path string, galleryID sql.NullInt64, sha string, size int64, ext, magic, status, source string) error {
	_, err := db.Exec(`INSERT INTO files (path, gallery_id, sha256, size, ext, magic, scan_status, scan_source, scan_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET scan_status = excluded.scan_status, scan_source = excluded.scan_source,
		scan_time = excluded.scan_time, sha256 = excluded.sha256, magic = excluded.magic,
		gallery_id = excluded.gallery_id`,
		path, galleryID, sha, size, ext, magic, status, source, nowISO())
	return err
}

func recordScanFile(db dbExec, root string, galleryID sql.NullInt64, path string, sha string, size int64, ext, magic, status, source string) error {
	if galleryID.Valid {
		galleryIDInt := int(galleryID.Int64)
		if err := recordDiscovered(db, galleryIDInt, galleryDir(root, galleryIDInt)); err != nil {
			return err
		}
	}
	return recordFile(db, path, galleryID, sha, size, ext, magic, status, source)
}

// galleryIDForPath は `<gallery_dir>/<id>/<file>` の `<id>` を読む（scan は path からしか判断できない）
// export file の `<gallery_dir>/<id>.<format>` は gallery dir file ではないため NULL にする
func galleryIDForPath(rel, galleryDirName string) sql.NullInt64 {
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	if len(parts) < 3 || parts[0] != galleryDirName {
		return sql.NullInt64{}
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(id), Valid: true}
}

func logScanError(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan: "+err.Error())
	}
}

func verifyMissing(db *sql.DB, root string) (int, error) {
	rows, err := db.Query("SELECT path FROM files WHERE missing_at_verify = 0")
	if err != nil {
		return 0, err
	}
	paths := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return 0, err
		}
		paths = append(paths, p)
	}
	rows.Close()
	count := 0
	for _, p := range paths {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			if _, err := db.Exec("UPDATE files SET missing_at_verify = 1, scan_status = 'external_removal' WHERE path = ?", p); err != nil {
				return count, err
			}
			count++
		}
	}
	return count, nil
}

type downloadEntryResult struct {
	page  int
	path  string
	hash  string
	err   string
}

func downloadFilesParallel(client *Client, target string, entries []FileEntry, format string, maxConcurrent int) []downloadEntryResult {
	if len(entries) == 0 {
		return []downloadEntryResult{}
	}
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}

	results := make([]downloadEntryResult, len(entries))
	pending := make([]int, len(entries))
	for i := range entries {
		pending[i] = i
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for {
			mu.Lock()
			if len(pending) == 0 {
				mu.Unlock()
				return
			}
			idx := pending[0]
			pending = pending[1:]
			mu.Unlock()

			entry := entries[idx]
			path := filepath.Join(target, entry.Hash+"."+format)
			body, err := client.getImage(context.Background(), StorageURL(entry.Hash, format, client.GG()), [2]int64{-1, -1})
			if err != nil {
				results[idx] = downloadEntryResult{page: idx + 1, path: path, hash: entry.Hash, err: err.Error()}
				continue
			}
			if err := os.WriteFile(path, body, 0o644); err != nil {
				results[idx] = downloadEntryResult{page: idx + 1, path: path, hash: entry.Hash, err: err.Error()}
				continue
			}
			results[idx] = downloadEntryResult{page: idx + 1, path: path, hash: entry.Hash, err: ""}
		}
	}

	for i := 0; i < maxConcurrent; i++ {
		wg.Add(1)
		go worker()
	}
	wg.Wait()

	return results
}

type pendingFinding struct {
	rule  string
	detail string
}

type pendingCacheStore struct {
	sha     string
	verdict string
	engine  string
	threat  string
	detail  string
}

func processFile(db *sql.DB, cfg Config, state *RateState, root, path string, summary map[string]int, fetch FetchFunc) string {
	return processFileWithYara(db, cfg, state, root, path, summary, fetch, nil)
}

func processFileWithYara(db *sql.DB, cfg Config, state *RateState, root, path string, summary map[string]int, fetch FetchFunc, runYara YaraRunFunc) string {
	sha, err := Sha256File(path)
	if err != nil {
		logScanError(err)
		return ""
	}
	ext, magic, findings, status := ScanFile(path, cfg)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	galleryID := galleryIDForPath(rel, orDefault(cfg.GalleryDir, "Galleries"))

	source := "structural"
	cached := false
	var yaraFindings []pendingFinding
	var vtFindings []pendingFinding
	var cacheStoreEntry pendingCacheStore
	needsCacheStore := false

	if status != "scan_error" {
		if YaraGate(ext, cfg) {
			result := runYaraWithRunner(context.Background(), path, rulesFile(root), resolveYrPath(cfg.Yr, root), cfg.Limits, runYara)
			if result.Status == "scan_error" {
				if status != "hit" {
					status = "scan_error"
				}
				source = "structural+yara_error"
			} else if len(result.Rules) > 0 {
				status = "hit"
				source = "structural+yara"
				for _, rule := range result.Rules {
					yaraFindings = append(yaraFindings, pendingFinding{rule: "yara", detail: rule})
				}
			}
		}
	}

	if status != "hit" && status != "scan_error" && cfg.VirusTotal.Enabled && scanTargets[ext] {
		verdict, engine, threat, detail, err := cacheLookup(db, sha, scanCacheTTL)
		if err != nil {
			logScanError(err)
			return status
		}
		if verdict != "" {
			source = "structural+virustotal"
			cached = true
			if verdict == "hit" {
				status = "hit"
				vtFindings = append(vtFindings, pendingFinding{rule: engine, detail: firstNonEmpty(detail, threat)})
			} else if verdict == "unknown_to_vt" {
				vtFindings = append(vtFindings, pendingFinding{rule: engine, detail: firstNonEmpty(detail, threat)})
			} else {
				vtFindings = append(vtFindings, pendingFinding{rule: engine, detail: "cache_hit"})
			}
		} else {
			result := VTLookup(db, sha, cfg, state, fetch)
			if result.Status == "pass" || result.Status == "hit" {
				needsCacheStore = true
				cacheStoreEntry = pendingCacheStore{sha: sha, verdict: result.Status, engine: "virustotal", threat: result.ThreatName, detail: fmt.Sprintf("positives=%d", result.Positives)}
				source = "structural+virustotal"
				if result.Status == "hit" {
					status = "hit"
					vtFindings = append(vtFindings, pendingFinding{rule: "virustotal", detail: firstNonEmpty(result.ThreatName, fmt.Sprintf("positives=%d", result.Positives))})
				}
			} else if result.Status == "unknown_to_vt" {
				source = "structural+virustotal"
				vtFindings = append(vtFindings, pendingFinding{rule: "virustotal", detail: firstNonEmpty(result.ThreatName, result.Status)})
			} else if result.Status != "skipped" {
				vtFindings = append(vtFindings, pendingFinding{rule: "virustotal", detail: "api_error:" + result.Status})
			}
		}
	}

	tx, err := db.Begin()
	if err != nil {
		logScanError(err)
		return status
	}
	defer func() { _ = tx.Rollback() }()

	for _, item := range findings {
		if err := addFinding(tx, rel, sha, "structural", item); err != nil {
			logScanError(err)
			return status
		}
	}
	for _, item := range yaraFindings {
		if err := addFinding(tx, rel, sha, item.rule, item.detail); err != nil {
			logScanError(err)
			return status
		}
	}
	for _, item := range vtFindings {
		if err := addFinding(tx, rel, sha, item.rule, item.detail); err != nil {
			logScanError(err)
			return status
		}
	}
	if needsCacheStore {
		if err := cacheStore(tx, cacheStoreEntry.sha, cacheStoreEntry.verdict, cacheStoreEntry.engine, cacheStoreEntry.threat, cacheStoreEntry.detail); err != nil {
			logScanError(err)
			return status
		}
	}

	if err := recordScanFile(tx, root, galleryID, rel, sha, fileSize(path), ext, magic, status, source); err != nil {
		logScanError(err)
		return status
	}
	if err := tx.Commit(); err != nil {
		logScanError(err)
		return status
	}
	summary[status] = summary[status] + 1
	if cached {
		summary["cached"] = summary["cached"] + 1
	}
	return status
}

type flags struct {
	root     string
	gallery  string
	noVT     bool
	verify   bool
	term     string
	lang     string
	title    string
	page     int
	id       int
	urlArg   string
	limit    int
	name     string
	query    string
	complete bool
	unfav    bool
	list         string
	port         int
	format       string
	out          string
	config       string
	clearComplete bool
}

func main() {
	args := os.Args[1:]
	cmd, args := findCommand(args)

	fsFlags := flag.NewFlagSet(cmd, flag.ExitOnError)
	f := &flags{}
	fsFlags.StringVar(&f.root, "root", "", "workspace root (default: cwd)")
	fsFlags.StringVar(&f.gallery, "gallery-dir", "", "gallery dir under root (default: Galleries)")
	fsFlags.StringVar(&dataDir, "data-dir", "", "data files (schema.sql, rules/, config.example.json)")
	fsFlags.BoolVar(&f.noVT, "no-vt", false, "skip VirusTotal lookup")
	fsFlags.BoolVar(&f.verify, "verify", false, "mark recorded files that no longer exist")
	fsFlags.StringVar(&f.term, "term", "", "search term (nozomi)")
	fsFlags.StringVar(&f.lang, "lang", "all", "search language (hitomi results.js default is all)")
	fsFlags.StringVar(&f.title, "title", "", "phrase or title search via galleriesindex (B-tree)")
	fsFlags.IntVar(&f.page, "page", 1, "result page (1 origin)")
	fsFlags.IntVar(&f.id, "id", 0, "gallery id")
	fsFlags.StringVar(&f.urlArg, "url", "", "gallery URL (id is parsed from it)")
	fsFlags.IntVar(&f.limit, "limit", 0, "max files to download (0 = all)")
	fsFlags.StringVar(&f.name, "name", "", "saved query name")
	fsFlags.StringVar(&f.query, "query", "", "saved query text")
	fsFlags.BoolVar(&f.complete, "complete", false, "mark read_state as fully_read")
	fsFlags.BoolVar(&f.unfav, "unfav", false, "clear favorite instead of setting it")
	fsFlags.BoolVar(&f.clearComplete, "clear-complete", false, "clear fully_read explicitly")
	fsFlags.StringVar(&f.list, "list", "", "unread | favorites | queries")
	fsFlags.IntVar(&f.port, "port", 0, "viewer port (default 8731)")
	fsFlags.StringVar(&f.format, "format", "cbz", "export format: cbz | pdf")
	fsFlags.StringVar(&f.out, "out", "", "export output file (default Galleries/<id>.<format>)")
	fsFlags.StringVar(&f.config, "config", "", "config JSON path (default <data-dir>/config.example.json)")
	_ = fsFlags.Parse(args)

	if dataDir == "" {
		dataDir = filepath.Dir(executablePath())
	}
	root := resolveRoot(f.root)
	cfgPath := filepath.Join(dataDir, "config.example.json")
	if f.config != "" {
		cfgPath = f.config
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config: "+err.Error())
		os.Exit(1)
	}
	if f.noVT {
		cfg.VirusTotal.Enabled = false
	}

	dbPath := filepath.Join(root, orDefault(cfg.DB, "hitomi.db"))
	db, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "db: "+dbPath+": "+err.Error())
		os.Exit(1)
	}
	defer db.Close()
	if err := ensureSchema(db); err != nil {
		fmt.Fprintln(os.Stderr, "schema: "+err.Error())
		os.Exit(1)
	}

	summary := map[string]int{"pass": 0, "hit": 0, "not_evaluated": 0, "scan_error": 0, "cached": 0}
	galleryRoot := filepath.Join(root, orDefault(cfg.GalleryDir, "Galleries"))

	switch cmd {
	case "scan":
		if _, err := os.Stat(galleryRoot); err != nil {
			fmt.Fprintf(os.Stderr, "gallery dir not found: %s\n", galleryRoot)
			os.Exit(2)
		}
		files := []string{}
		_ = filepath.WalkDir(galleryRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			files = append(files, path)
			return nil
		})
		sort.Strings(files)
		state := &RateState{}
		for _, path := range files {
			processFile(db, cfg, state, root, path, summary, nil)
		}
		if f.verify {
			count, err := verifyMissing(db, root)
			if err != nil {
				fmt.Fprintf(os.Stderr, "verify: %s\n", err.Error())
			}
			summary["external_removal"] = count
		}
		printJSON(summary)
	case "search":
		client := &Client{http: &http.Client{Timeout: 30 * time.Second}, cfg: cfg.Hitomi}
		warnGG(loadGG(client))
		mode := "html"
		var shown []int
		var total int
		var err error
		if f.title != "" {
			mode = "index"
			ids, err := client.SearchTitle(f.title)
			if err != nil {
				fmt.Fprintln(os.Stderr, "search: "+err.Error())
				os.Exit(1)
			}
			total = len(ids)
			shown = searchPageSlice(ids, f.page, f.limit)
		} else {
			shown, total, err = client.SearchHTML(f.term, f.lang, f.page, f.limit)
			if err != nil {
				fmt.Fprintln(os.Stderr, "search: "+err.Error())
				os.Exit(1)
			}
		}
		for _, id := range shown {
			if err := recordDiscovered(db, id, galleryDir(root, id)); err != nil {
				fmt.Fprintf(os.Stderr, "gallery: %s\n", err.Error())
			}
		}
		if f.name != "" && f.query != "" {
			if err := addQuery(db, f.name, f.query, 0); err != nil {
				fmt.Fprintf(os.Stderr, "query: %s\n", err.Error())
			}
		}
		printJSON(map[string]any{"mode": mode, "query": orDefault(f.term, f.title), "title": f.title,
			"lang": f.lang, "page": f.page, "total": total, "ids": shown, "count": len(shown)})
	case "gallery":
		id, err := targetID(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "gallery: "+err.Error())
			os.Exit(2)
		}
		client := &Client{http: &http.Client{Timeout: 30 * time.Second}, cfg: cfg.Hitomi}
		warnGG(loadGG(client))
		info, err := client.Gallery(id)
		if err != nil {
			fmt.Fprintln(os.Stderr, "gallery: "+err.Error())
			os.Exit(1)
		}
		if err := upsertGallery(db, info, galleryDir(root, info.ID)); err != nil {
			fmt.Fprintf(os.Stderr, "upsert: %s\n", err.Error())
		}
		printJSON(info)
	case "download":
		id, err := targetID(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "download: "+err.Error())
			os.Exit(2)
		}
		client := &Client{http: &http.Client{Timeout: 60 * time.Second}, cfg: cfg.Hitomi}
		warnGG(loadGG(client))
		info, err := client.Gallery(id)
		if err != nil {
			fmt.Fprintln(os.Stderr, "gallery: "+err.Error())
			os.Exit(1)
		}
		target := filepath.Join(root, galleryDir(root, info.ID))
		if err := os.MkdirAll(target, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "mkdir: "+err.Error())
			os.Exit(1)
		}
		if err := upsertGallery(db, info, galleryDir(root, info.ID)); err != nil {
			fmt.Fprintf(os.Stderr, "upsert: %s\n", err.Error())
		}
		state := &RateState{}
		format := orDefault(cfg.Hitomi.ImageFormat, "webp")
		limit := len(info.Files)
		if f.limit > 0 && f.limit < limit {
			limit = f.limit
		}

		results := downloadFilesParallel(client, target, info.Files[:limit], format, 4)
		written := 0
		for _, result := range results {
			if result.err != "" {
				fmt.Fprintf(os.Stderr, "download: %s\n", result.err)
				continue
			}
			written++
			if err := recordPage(db, info.ID, result.page, result.hash); err != nil {
				fmt.Fprintf(os.Stderr, "pageorder: %s\n", err.Error())
			}
			processFile(db, cfg, state, root, result.path, summary, nil)
		}
		printJSON(map[string]any{"gallery_id": info.ID, "attempted": len(results), "written": written, "summary": summary})
	case "export":
		id, err := targetID(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "export: "+err.Error())
			os.Exit(2)
		}
		var result map[string]any
		switch f.format {
		case "cbz":
			result, err = exportCBZ(db, cfg, root, id, f.out)
		case "pdf":
			result, err = exportPDF(db, cfg, root, id, f.out)
		default:
			fmt.Fprintln(os.Stderr, "export: format must be cbz or pdf")
			os.Exit(2)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "export: "+err.Error())
			os.Exit(1)
		}
		printJSON(result)
	case "read":
		id, err := targetID(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read: "+err.Error())
			os.Exit(2)
		}
		if err := recordDiscovered(db, id, galleryDir(root, id)); err != nil {
			fmt.Fprintln(os.Stderr, "gallery: "+err.Error())
			os.Exit(1)
		}
		if err := setReadStateWithClear(db, id, f.page, f.complete, f.clearComplete); err != nil {
			fmt.Fprintln(os.Stderr, "read: "+err.Error())
			os.Exit(1)
		}
		row, err := readStateRow(db, id)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read_state: "+err.Error())
			os.Exit(1)
		}
		printJSON(map[string]any{"gallery_id": id, "last_page": row["last_page"], "fully_read": row["fully_read"]})
	case "fav":
		id, err := targetID(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fav: "+err.Error())
			os.Exit(2)
		}
		if err := recordDiscovered(db, id, galleryDir(root, id)); err != nil {
			fmt.Fprintln(os.Stderr, "gallery: "+err.Error())
			os.Exit(1)
		}
		if err := setFavorite(db, id, !f.unfav); err != nil {
			fmt.Fprintln(os.Stderr, "fav: "+err.Error())
			os.Exit(1)
		}
		printJSON(map[string]any{"gallery_id": id, "favorite": boolToInt(!f.unfav)})
	case "check":
		client := &Client{http: &http.Client{Timeout: 30 * time.Second}, cfg: cfg.Hitomi}
		warnGG(loadGG(client))
		results, err := runSavedQueries(db, client, root, f.lang, f.limit)
		if err != nil {
			fmt.Fprintln(os.Stderr, "check: "+err.Error())
			os.Exit(1)
		}
		printJSON(results)
	case "list":
		rows, err := listRows(db, f.list)
		if err != nil {
			fmt.Fprintln(os.Stderr, "list: "+err.Error())
			os.Exit(1)
		}
		printJSON(rows)
	case "view":
		if f.port > 0 {
			cfg.Viewer.Port = f.port
		}
		v := newViewer(db, cfg, root)
		if err := v.serve(); err != nil {
			fmt.Fprintln(os.Stderr, "viewer: "+err.Error())
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown command: "+cmd)
		os.Exit(2)
	}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// valueFlags は値を消費するフラグ。findCommand がフラグの値をサブコマンドと誤読しないため
var valueFlags = map[string]bool{
	"root": true, "gallery-dir": true, "data-dir": true, "term": true, "lang": true,
	"title": true, "page": true, "id": true, "url": true, "limit": true, "name": true, "query": true,
	"list": true, "port": true, "format": true, "out": true, "config": true,
}

// findCommand はサブコマンドを許容位置で拾い、残りをフラグ引数として返す
func findCommand(args []string) (string, []string) {
	rest := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "" || strings.HasPrefix(arg, "-") {
			rest = append(rest, arg)
			name := strings.TrimLeft(arg, "-")
			if !strings.Contains(name, "=") && valueFlags[name] && i+1 < len(args) {
				i++
				rest = append(rest, args[i])
			}
			continue
		}
		rest = append(rest, args[i+1:]...)
		return arg, rest
	}
	return "scan", rest
}

func targetID(f *flags) (int, error) {
	if f.urlArg != "" {
		return ExtractGalleryID(f.urlArg)
	}
	if f.id == 0 {
		return 0, errors.New("--id or --url is required")
	}
	return f.id, nil
}

func printJSON(value any) {
	out, err := json.Marshal(value)
	if err != nil {
		fmt.Fprintln(os.Stderr, "json: "+err.Error())
		return
	}
	fmt.Println(string(out))
}

// loadGG は 1 実行で 1 回だけ正典 gg.js を読む（rate gate 内）。live が取れなければ snapshot に戻る
func loadGG(client *Client) string {
	return client.LoadGG(context.Background())
}

func warnGG(source string) {
	if source == "snapshot" {
		fmt.Fprintln(os.Stderr, "gg: snapshot fallback (live gg.js unavailable)")
	}
}

func orDefault(value, def string) string {
	if value == "" {
		return def
	}
	return value
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func rulesFile(root string) string {
	candidate := filepath.Join(root, "rules", "structural.yar")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return filepath.Join(dataDir, "rules", "structural.yar")
}

func recordDiscovered(db dbExec, galleryID int, dir string) error {
	_, err := db.Exec(`INSERT INTO galleries (gallery_id, title, lang, dir, first_seen, last_seen)
		VALUES (?, NULL, NULL, ?, ?, ?)
		ON CONFLICT(gallery_id) DO UPDATE SET last_seen = excluded.last_seen`,
		galleryID, dir, nowISO(), nowISO())
	return err
}

func listRows(db *sql.DB, kind string) ([]map[string]any, error) {
	query := ""
	switch kind {
	case "unread":
		query = "SELECT gallery_id, last_page, fully_read, favorite FROM read_state WHERE fully_read = 0 ORDER BY gallery_id"
	case "favorites":
		query = "SELECT gallery_id, last_page, fully_read, favorite FROM read_state WHERE favorite = 1 ORDER BY gallery_id"
	case "queries":
		query = "SELECT id, name, query, last_run_at FROM bookmark_queries ORDER BY id"
	default:
		return nil, errors.New("kind must be unread, favorites or queries")
	}
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		if kind == "queries" {
			var id int
			var name, q string
			var last sql.NullString
			if err := rows.Scan(&id, &name, &q, &last); err != nil {
				return nil, err
			}
			out = append(out, map[string]any{"id": id, "name": name, "query": q, "last_run_at": last.String})
			continue
		}
		var galleryID, lastPage, fully, fav int
		if err := rows.Scan(&galleryID, &lastPage, &fully, &fav); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"gallery_id": galleryID, "last_page": lastPage,
			"fully_read": fully, "favorite": fav})
	}
	return out, nil
}
