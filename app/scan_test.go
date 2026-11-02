package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func dataRoot() string {
	abs, err := filepath.Abs("..")
	if err != nil {
		return ".."
	}
	return abs
}

func cfg() Config {
	return Config{Limits: defaultLimits()}
}

func makeZip(t *testing.T, dir string, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "fixture.cbz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for name, content := range entries {
		hw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := hw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDataFilesAtRoot(t *testing.T) {
	for _, name := range []string{"schema.sql", "config.example.json", "rules/structural.yar"} {
		if _, err := os.Stat(filepath.Join(dataRoot(), name)); err != nil {
			t.Errorf("missing data file: %s", name)
		}
	}
}

func TestDetectMagic(t *testing.T) {
	cases := map[string][]byte{
		"zip":  []byte("PK\x03\x04abc"),
		"pdf":  []byte("%PDF-1.4"),
		"pe":   []byte("MZ\x90\x00"),
		"png":  []byte("\x89PNG\r\n\x1a\n"),
		"jpeg": []byte("\xff\xd8\xff"),
		"webp": append(append([]byte("RIFF\x00\x00\x00\x00"), []byte("WEBP")...), 0),
		"unknown": []byte("hello"),
	}
	for want, head := range cases {
		if got := DetectMagic(head); got != want {
			t.Errorf("magic %q: got %s", want, got)
		}
	}
}

func TestZipFindings(t *testing.T) {
	dir := t.TempDir()
	path := makeZip(t, dir, map[string]string{"../evil.exe": "x", "ok.txt": "y"})
	findings, err := ScanZipEntries(path, cfg().Limits)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(findings, ",")
	for _, want := range []string{"path_traversal:../evil.exe", "executable_entry:../evil.exe"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %s in %v", want, findings)
		}
	}
}

func TestRatioExceeded(t *testing.T) {
	dir := t.TempDir()
	filler := strings.Repeat("\x00", 2000000)
	path := makeZip(t, dir, map[string]string{"bomb.webp": filler})
	findings, err := ScanZipEntries(path, cfg().Limits)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) == 0 {
		t.Fatalf("expected ratio finding, got %v", findings)
	}
}

func TestStatusSemantics(t *testing.T) {
	dir := t.TempDir()
	pdf := filepath.Join(dir, "clean.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.4 no dangerous objects"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, status := ScanFile(pdf, cfg()); status != "pass" {
		t.Errorf("clean pdf status: got %s", status)
	}
	if err := os.WriteFile(pdf, []byte("%PDF-1.4 /Launch"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, findings, status := ScanFile(pdf, cfg()); status != "hit" || findings[0] != "pdf_object:/Launch" {
		t.Errorf("pdf hit: got %s %v", status, findings)
	}
	txt := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(txt, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, status := ScanFile(txt, cfg()); status != "not_evaluated" {
		t.Errorf("txt status: got %s", status)
	}
	spoof := filepath.Join(dir, "spoof.cbz")
	if err := os.WriteFile(spoof, []byte("\x89PNG\r\n\x1a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, findings, status := ScanFile(spoof, cfg()); status != "hit" || findings[0] != "magic_mismatch:cbz_vs_png" {
		t.Errorf("spoof: got %s %v", status, findings)
	}
	bad := filepath.Join(dir, "bad.cbz")
	if err := os.WriteFile(bad, []byte("PK\x03\x04garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, findings, status := ScanFile(bad, cfg()); status != "scan_error" || findings[0] != "bad_zip" {
		t.Errorf("bad zip: got %s %v", status, findings)
	}
}

func TestVTRateGate(t *testing.T) {
	c := cfg()
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 2, MaxDaily: 3, APIKey: "k"}
	state := &RateState{}
	fetch := func(url, key string) (map[string]any, int, error) {
		return map[string]any{"data": map[string]any{"attributes": map[string]any{"stats": map[string]any{"malicious": 1}}}}, 200, nil
	}
	for i := 0; i < 2; i++ {
		if r := VTLookup(nil, "abc", c, state, fetch); r.Status != "hit" {
			t.Fatalf("request %d: got %s", i, r.Status)
		}
	}
	if r := VTLookup(nil, "abc", c, state, fetch); r.Status != "rate_limited" {
		t.Errorf("rate gate: got %s", r.Status)
	}
	state.window = []time.Time{}
	state.total = 3
	if r := VTLookup(nil, "abc", c, state, fetch); r.Status != "daily_quota_exhausted" {
		t.Errorf("daily gate: got %s", r.Status)
	}
	c.VirusTotal.Enabled = false
	if r := VTLookup(nil, "abc", c, state, fetch); r.Status != "skipped" {
		t.Errorf("disabled: got %s", r.Status)
	}
}

func TestParseYaraOutput(t *testing.T) {
	line := `{"path":"a/100.cbz","rules":[{"identifier":"Zip_Path_Traversal"},{"identifier":"Zip_Executable_Entry"}]}`
	rules := ParseYaraOutput(line)
	if len(rules) != 2 || rules[0] != "Zip_Path_Traversal" {
		t.Errorf("parse: got %v", rules)
	}
}

func TestYaraGateRunsForAllScanTargets(t *testing.T) {
	c := cfg()
	c.Yr = "tools/yr.exe"
	if !YaraGate("cbz", c) || !YaraGate("zip", c) || !YaraGate("pdf", c) {
		t.Errorf("scan targets must run yara even after a structural hit")
	}
	if YaraGate("webp", c) {
		t.Errorf("non-target ext must not run yara: %s", "webp")
	}
	c.Yr = ""
	if YaraGate("cbz", c) {
		t.Errorf("no yr configured must not run yara")
	}
}

func TestReferenceEntryIsYaraOnlyFinding(t *testing.T) {
	// 参照 entry（.url）は構造検査の exec ext に無い → YARA 側の検知（役割分担）
	dir := t.TempDir()
	cbz := makeZip(t, dir, map[string]string{"startup.url": "[InternetShortcut]\nURL=https://example.com\n"})
	_, _, findings, status := ScanFile(cbz, cfg())
	if status != "pass" || len(findings) != 0 {
		t.Errorf("structural must not flag reference entry: status %s findings %v", status, findings)
	}
}

func TestVTNon200DoesNotCachePass(t *testing.T) {
	db := openTestDB(t)
	c := cfg()
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 1, MaxDaily: 2, APIKey: "k"}
	state := &RateState{}
	payload := map[string]any{"data": map[string]any{"attributes": map[string]any{"stats": map[string]any{"malicious": 0}}}}
	fetch := func(url, key string) (map[string]any, int, error) {
		return payload, 404, nil
	}
	result := VTLookup(db, "abc", c, state, fetch)
	if result.Status != "not_evaluated" {
		t.Fatalf("status: got %s", result.Status)
	}
	verdict, _, _, _, err := cacheLookup(db, "abc", scanCacheTTL)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != "" {
		t.Errorf("non-200 VT must not cache pass: got %s", verdict)
	}
}

func TestVTThreatNameUsesEngineResultName(t *testing.T) {
	db := openTestDB(t)
	c := cfg()
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 1, MaxDaily: 2, APIKey: "k"}
	state := &RateState{}
	payload := map[string]any{"data": map[string]any{"attributes": map[string]any{
		"stats": map[string]any{"malicious": 1},
		"last_analysis_results": map[string]any{
			"engine": map[string]any{"category": "malicious", "name": "Engine A"},
		},
	}}}
	fetch := func(url, key string) (map[string]any, int, error) {
		return payload, 200, nil
	}
	result := VTLookup(db, "abc", c, state, fetch)
	if result.Status != "hit" || result.ThreatName != "Engine A" {
		t.Fatalf("result: %v", result)
	}
}

func TestVTDailyQuotaIsStoredInDB(t *testing.T) {
	db := openTestDB(t)
	c := cfg()
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 10, MaxDaily: 1, APIKey: "k"}
	state := &RateState{}
	fetch := func(url, key string) (map[string]any, int, error) {
		return map[string]any{"data": map[string]any{"attributes": map[string]any{"stats": map[string]any{"malicious": 1}}}}, 200, nil
	}
	if result := VTLookup(db, "abc", c, state, fetch); result.Status != "hit" {
		t.Fatalf("first lookup: got %s", result.Status)
	}
	total, err := vtDailyTotal(db, vtDay(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("daily total: got %d", total)
	}
	if result := VTLookup(db, "abc", c, state, fetch); result.Status != "daily_quota_exhausted" {
		t.Errorf("daily db quota: got %s", result.Status)
	}
}

func TestScanCacheTTLStalePassIsNotUsed(t *testing.T) {
	db := openTestDB(t)
	if err := cacheStore(db, "abc", "pass", "virustotal", "", "positives=0"); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-31 * 24 * time.Hour).Format("2006-01-02T15:04:05Z")
	if _, err := db.Exec("UPDATE scan_cache SET cached_at = ? WHERE sha256 = ?", old, "abc"); err != nil {
		t.Fatal(err)
	}
	verdict, _, _, _, err := cacheLookup(db, "abc", scanCacheTTL)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != "" {
		t.Errorf("stale cache must not be used: got %s", verdict)
	}
}
