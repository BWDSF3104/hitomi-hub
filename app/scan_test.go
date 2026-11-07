package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
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
		"7z": []byte("7z\xbc\xaf\x27\x1c"),
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

func TestZipDotDotIsPathSegmentOnly(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		entry string
		hit   bool
	}{
		{"Vol...01.webp", false},
		{"..evil.webp", false},
		{"normal/01.webp", false},
		{"a/...b.webp", false},
		{"../evil.exe", true},
		{"a/../b.webp", true},
		{"a\\..\\b.webp", true},
		{"foo/..", true},
		{"..", true},
	}
	for _, c := range cases {
		path := makeZip(t, dir, map[string]string{c.entry: "x"})
		findings, err := ScanZipEntries(path, cfg().Limits)
		if err != nil {
			t.Fatal(err)
		}
		got := false
		for _, f := range findings {
			if strings.HasPrefix(f, "path_traversal:") {
				got = true
			}
		}
		if got != c.hit {
			t.Errorf("entry %q: path_traversal got %v want %v (findings %v)", c.entry, got, c.hit, findings)
		}
	}
	clean := makeZip(t, dir, map[string]string{"Vol...01.webp": "x"})
	if _, _, findings, status := ScanFile(clean, cfg()); status != "pass" || len(findings) != 0 {
		t.Errorf("Vol...01.webp status: got %s %v, want pass with no finding", status, findings)
	}
	traversal := makeZip(t, dir, map[string]string{"../evil.exe": "x"})
	if _, _, findings, status := ScanFile(traversal, cfg()); status != "hit" || findings[0] != "path_traversal:../evil.exe" {
		t.Errorf("../evil.exe status: got %s %v", status, findings)
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

func TestScanPDFKeysDoesNotCopyDataPerKey(t *testing.T) {
	dir := t.TempDir()
	// 非 UTF-8 バイナリ 2MB（MaxScanBytes 既定相当）。bytes.Contains は byte 単位なので
	// 非 UTF-8 でも key 判定結果は string(data) 方式と同一。
	noHit := bytes.Repeat([]byte{0xff, 0xfe, 0x80, 0x01}, 524288)
	path := filepath.Join(dir, "clean.pdf")
	if err := os.WriteFile(path, noHit, 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := ScanPDFKeys(path, cfg().Limits)
	if err != nil {
		t.Fatal(err)
	}
	if findings == nil || len(findings) != 0 {
		t.Fatalf("no-hit findings: got %v", findings)
	}
	if allocs := testing.AllocsPerRun(10, func() { pdfKeyFindings(noHit) }); allocs != 0 {
		t.Errorf("key scan must not copy data per key: allocs %v", allocs)
	}
	for _, key := range pdfKeys {
		hit := append(append([]byte{}, noHit...), key...)
		got := pdfKeyFindings(hit)
		if len(got) != 1 || got[0] != "pdf_object:"+string(key) {
			t.Errorf("key %q: got %v", string(key), got)
		}
	}
}

func TestVTRateGate(t *testing.T) {
	c := cfg()
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 2, MaxDaily: 3, APIKey: "k"}
	state := &RateState{}
	fetch := func(url, key string) (map[string]any, int, error) {
		return map[string]any{"data": map[string]any{"attributes": map[string]any{"last_analysis_stats": map[string]any{"malicious": 1}}}}, 200, nil
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

func TestVT404NegativeCacheAvoidsRepeatedQuotaUsage(t *testing.T) {
	db := openTestDB(t)
	c := cfg()
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 1, MaxDaily: 2, APIKey: "k"}
	state := &RateState{}
	payload := map[string]any{}
	fetchCalls := 0
	fetch := func(url, key string) (map[string]any, int, error) {
		fetchCalls++
		return payload, 404, nil
	}

	result := VTLookup(db, "abc", c, state, fetch)
	if result.Status != "unknown_to_vt" {
		t.Fatalf("status: got %s", result.Status)
	}

	verdict, _, _, detail, err := cacheLookup(db, "abc", negativeScanCacheTTL)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != "unknown_to_vt" || detail != "unknown_to_vt" {
		t.Fatalf("negative cache: got verdict %q detail %q", verdict, detail)
	}

	result = VTLookup(db, "abc", c, state, fetch)
	if result.Status != "unknown_to_vt" {
		t.Fatalf("cached negative status: got %s", result.Status)
	}
	if fetchCalls != 1 {
		t.Errorf("negative cache should avoid repeated fetch: fetchCalls %d", fetchCalls)
	}
}

func TestVTThreatNameUsesResultAndEngineName(t *testing.T) {
	db := openTestDB(t)
	c := cfg()
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 1, MaxDaily: 2, APIKey: "k"}
	state := &RateState{}
	payload := map[string]any{"data": map[string]any{"attributes": map[string]any{
		"last_analysis_stats": map[string]any{"malicious": 2},
		"last_analysis_results": map[string]any{
			"Ad-Aware": map[string]any{"category": "malicious", "engine_name": "Ad-Aware", "result": "Trojan.GenericKD.42996961"},
			"AegisLab": map[string]any{"category": "malicious", "engine_name": "AegisLab", "result": nil},
		},
	}}}
	fetch := func(url, key string) (map[string]any, int, error) {
		return payload, 200, nil
	}
	result := VTLookup(db, "abc", c, state, fetch)
	if result.Status != "hit" {
		t.Fatalf("status: got %s", result.Status)
	}
	want := "Ad-Aware: Trojan.GenericKD.42996961;AegisLab"
	if result.ThreatName != want {
		t.Errorf("threat name: got %q want %q", result.ThreatName, want)
	}
}

func TestVTThreatNameFallsBackToCategoryWhenResultMissing(t *testing.T) {
	db := openTestDB(t)
	c := cfg()
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 1, MaxDaily: 2, APIKey: "k"}
	payload := map[string]any{"data": map[string]any{"attributes": map[string]any{
		"last_analysis_stats": map[string]any{"malicious": 1},
		"last_analysis_results": map[string]any{
			"engine": map[string]any{"category": "malicious"},
		},
	}}}
	result := VTLookup(db, "abc", c, &RateState{}, func(url, key string) (map[string]any, int, error) {
		return payload, 200, nil
	})
	if result.Status != "hit" || result.ThreatName != "malicious" {
		t.Errorf("category fallback: got %v", result)
	}
}

func TestVTLegacyStatsKeyAloneIsNotEvaluated(t *testing.T) {
	db := openTestDB(t)
	c := cfg()
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 1, MaxDaily: 2, APIKey: "k"}
	payload := map[string]any{"data": map[string]any{"attributes": map[string]any{
		"stats": map[string]any{"malicious": 1},
		"last_analysis_results": map[string]any{
			"engine": map[string]any{"category": "malicious", "result": "dropper"},
		},
	}}}
	result := VTLookup(db, "abc", c, &RateState{}, func(url, key string) (map[string]any, int, error) {
		return payload, 200, nil
	})
	if result.Status != "not_evaluated" {
		t.Errorf("legacy stats key must not be treated as analysis: got %v", result)
	}
}

func TestVTUnanalyzedWithoutResultsIsNotEvaluated(t *testing.T) {
	db := openTestDB(t)
	c := cfg()
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 1, MaxDaily: 2, APIKey: "k"}
	payload := map[string]any{"data": map[string]any{"attributes": map[string]any{
		"last_analysis_stats": map[string]any{"malicious": 0, "undetected": 0},
	}}}
	result := VTLookup(db, "abc", c, &RateState{}, func(url, key string) (map[string]any, int, error) {
		return payload, 200, nil
	})
	if result.Status != "not_evaluated" {
		t.Errorf("unanalyzed: got %v", result)
	}
	verdict, _, _, _, err := cacheLookup(db, "abc", scanCacheTTL)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != "" {
		t.Errorf("unanalyzed must not cache pass: got %s", verdict)
	}
}

func TestVTEmptyResultsWithStatsTotalIsNotEvaluated(t *testing.T) {
	// B-06 合格条件（Go 側・Python 参照実装 test_vt_empty_results_with_stats_total_is_not_evaluated と一致）:
	// key あり＋空 results（stats.total>0）の 200 応答は key 有無判定ではなく empty results 基準で not_evaluated
	db := openTestDB(t)
	c := cfg()
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 1, MaxDaily: 2, APIKey: "k"}
	payload := map[string]any{"data": map[string]any{"attributes": map[string]any{
		"last_analysis_stats":   map[string]any{"malicious": 0, "suspicious": 0, "harmless": 60, "undetected": 40, "total": 100},
		"last_analysis_results": map[string]any{},
	}}}
	result := VTLookup(db, "abc", c, &RateState{}, func(url, key string) (map[string]any, int, error) {
		return payload, 200, nil
	})
	if result.Status != "not_evaluated" {
		t.Fatalf("empty results with stats.total must be not_evaluated, got %v", result)
	}
	if result.Positives != 0 {
		t.Errorf("positives: got %d", result.Positives)
	}
	verdict, _, _, _, err := cacheLookup(db, "abc", scanCacheTTL)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != "" {
		t.Errorf("empty results must not cache pass: got %s", verdict)
	}
}

func TestVTV3RealAPIShapeWhenKeyAvailable(t *testing.T) {
	key := os.Getenv("VIRUSTOTAL_API_KEY")
	if key == "" {
		t.Skip("VIRUSTOTAL_API_KEY is not set")
	}
	c := cfg()
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 1, MaxDaily: 1, APIKey: key}
	payload, status, err := defaultFetch("https://www.virustotal.com/api/v3/files/27545216921b2559534379243d19446661534481345149624164814776710798", key)
	if err != nil {
		t.Fatalf("real VT API call: %v", err)
	}
	if status == 429 {
		t.Skip("VT real API rate limited")
	}
	if status != 200 {
		t.Fatalf("VT real API status: %d", status)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("VT real API payload has no data")
	}
	attrs, ok := data["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("VT real API payload has no data.attributes")
	}
	stats, ok := attrs["last_analysis_stats"].(map[string]any)
	if !ok {
		t.Fatalf("VT real API attributes has no last_analysis_stats")
	}
	if asInt(stats["malicious"]) <= 0 {
		t.Fatalf("known malicious hash must have malicious stats, got %v", stats)
	}
	results, ok := attrs["last_analysis_results"].(map[string]any)
	if !ok {
		t.Fatalf("VT real API attributes has no last_analysis_results")
	}
	sawResult := false
	sawEngineName := false
	for _, entryAny := range results {
		entry, ok := entryAny.(map[string]any)
		if !ok {
			continue
		}
		if asString(entry["result"]) != "" {
			sawResult = true
		}
		if asString(entry["engine_name"]) != "" {
			sawEngineName = true
		}
		if sawResult && sawEngineName {
			break
		}
	}
	if !sawResult {
		t.Fatalf("VT real API malicious entry did not contain result")
	}
	if !sawEngineName {
		t.Fatalf("VT real API entry must contain engine_name")
	}
	result := VTLookup(nil, "27545216921b2559534379243d19446661534481345149624164814776710798", c, &RateState{}, func(url, key string) (map[string]any, int, error) {
		return payload, status, nil
	})
	if result.Status != "hit" {
		t.Fatalf("VT real API shape must produce hit, got %v", result)
	}
}

func TestProcessFileVTFailureKeepsStructuralPassStatus(t *testing.T) {
	cases := []struct {
		name   string
		apiKey string
		maxPer int
		state  *RateState
		fetch  FetchFunc
		want   string
	}{
		{"fetch_error", "k", 5, &RateState{}, func(url, key string) (map[string]any, int, error) {
			return nil, 0, errors.New("network down")
		}, "api_error:scan_error"},
		{"no_key", "", 5, &RateState{}, nil, "api_error:no_key"},
		{"rate_limited", "k", 1, &RateState{window: []time.Time{time.Now()}}, nil, "api_error:rate_limited"},
	}
	for _, tc := range cases {
		db := openTestDB(t)
		dir := t.TempDir()
		path := makeZip(t, dir, map[string]string{"ok.txt": "y"})
		c := cfg()
		c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: tc.maxPer, MaxDaily: 5, APIKey: tc.apiKey}
		summary := map[string]int{}
		status := processFile(db, c, tc.state, dir, path, summary, tc.fetch)
		if status != "pass" {
			t.Errorf("%s: structural status must survive VT failure, got %s", tc.name, status)
		}
		if summary["pass"] != 1 {
			t.Errorf("%s: summary: got %v", tc.name, summary)
		}
		var detail string
		if err := db.QueryRow("SELECT detail FROM findings WHERE rule = 'virustotal'").Scan(&detail); err != nil {
			t.Errorf("%s: VT status finding missing: %v", tc.name, err)
			continue
		}
		if detail != tc.want {
			t.Errorf("%s: finding detail: got %q want %q", tc.name, detail, tc.want)
		}
		var source, recorded string
		if err := db.QueryRow("SELECT scan_source, scan_status FROM files").Scan(&source, &recorded); err != nil {
			t.Errorf("%s: file row: %v", tc.name, err)
			continue
		}
		if source != "structural" || recorded != "pass" {
			t.Errorf("%s: recorded row: source %s status %s", tc.name, source, recorded)
		}
	}
}

func TestProcessFileVTEmptyResultsDoesNotStorePass(t *testing.T) {
	// B-06 残件（Python 参照実装 test_process_file_vt_empty_results_does_not_store_pass と一致）:
	// empty results → 構造 pass は維持・scan_cache に pass を保存しない・findings は api_error:not_evaluated
	// Go 側は A-12 scan meta 行（verdict=''）を常に書くため、固定は「pass verdict 行なし」で行う
	db := openTestDB(t)
	dir := t.TempDir()
	path := makeZip(t, dir, map[string]string{"ok.txt": "y"})
	c := cfg()
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 5, MaxDaily: 5, APIKey: "k"}
	summary := map[string]int{}
	fetch := func(url, key string) (map[string]any, int, error) {
		return map[string]any{"data": map[string]any{"attributes": map[string]any{
			"last_analysis_stats":   map[string]any{"malicious": 0, "suspicious": 0, "total": 100},
			"last_analysis_results": map[string]any{},
		}}}, 200, nil
	}
	sha, err := Sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	status := processFile(db, c, &RateState{}, dir, path, summary, fetch)
	if status != "pass" {
		t.Fatalf("structural pass must survive empty results, got %s", status)
	}
	verdict, _, _, _, err := cacheLookup(db, sha, scanCacheTTL)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != "" {
		t.Errorf("empty results must not store pass in scan_cache: got %s", verdict)
	}
	var cacheCount int
	if err := db.QueryRow("SELECT COUNT(1) FROM scan_cache WHERE verdict = 'pass'").Scan(&cacheCount); err != nil {
		t.Fatal(err)
	}
	if cacheCount != 0 {
		t.Errorf("scan_cache must not store pass verdict: got %d", cacheCount)
	}
	var detail string
	if err := db.QueryRow("SELECT detail FROM findings WHERE rule = 'virustotal'").Scan(&detail); err != nil {
		t.Fatalf("VT status finding missing: %v", err)
	}
	if detail != "api_error:not_evaluated" {
		t.Errorf("finding detail: got %q want %q", detail, "api_error:not_evaluated")
	}
	var source, recorded string
	if err := db.QueryRow("SELECT scan_source, scan_status FROM files").Scan(&source, &recorded); err != nil {
		t.Fatal(err)
	}
	if source != "structural" || recorded != "pass" {
		t.Errorf("file row: source %s status %s", source, recorded)
	}
}

func TestVTDailyQuotaIsStoredInDB(t *testing.T) {
	db := openTestDB(t)
	c := cfg()
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 10, MaxDaily: 1, APIKey: "k"}
	state := &RateState{}
	fetch := func(url, key string) (map[string]any, int, error) {
		return map[string]any{"data": map[string]any{"attributes": map[string]any{"last_analysis_stats": map[string]any{"malicious": 1}}}}, 200, nil
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

func TestRunYaraExit1ErrorIsScanError(t *testing.T) {
	result := runYaraWithRunner(context.Background(), "file.cbz", "rules.yar", "yr", defaultLimits(), func(ctx context.Context, exe string, args []string) ([]byte, int, error) {
		return []byte(`{"rules":[{"identifier":"Zip_Path_Traversal"}]}`), 1, errors.New("exit status 1")
	})
	if result.Status != "scan_error" || len(result.Rules) != 0 || result.Exit != 1 {
		t.Fatalf("yara exit1 error: got %v %v exit %d", result.Status, result.Rules, result.Exit)
	}
}

func TestRunYaraErrorExit1WithoutRulesIsScanError(t *testing.T) {
	result := runYaraWithRunner(context.Background(), "file.cbz", "rules.yar", "yr", defaultLimits(), func(ctx context.Context, exe string, args []string) ([]byte, int, error) {
		return nil, 1, errors.New("invalid rule")
	})
	if result.Status != "scan_error" || len(result.Rules) != 0 || result.Exit != 1 || result.Error != "invalid rule" {
		t.Fatalf("yara exit1 error without rules: got %v %v exit %d error %q", result.Status, result.Rules, result.Exit, result.Error)
	}
}

func TestRunYaraExecutionFailureIsScanError(t *testing.T) {
	result := runYaraWithRunner(context.Background(), "file.cbz", "rules.yar", "yr", defaultLimits(), func(ctx context.Context, exe string, args []string) ([]byte, int, error) {
		return nil, 2, errors.New("missing yr")
	})
	if result.Status != "scan_error" || len(result.Rules) != 0 {
		t.Fatalf("yara execution failure: got %v %v", result.Status, result.Rules)
	}
}

func TestProcessFileYaraExecutionFailureKeepsNotEvaluated(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	path := makeZip(t, dir, map[string]string{"ok.txt": "y"})
	c := cfg()
	c.Yr = "tools/yr.exe"
	summary := map[string]int{}

	if status := processFileWithYara(db, c, &RateState{}, dir, path, summary, nil, func(ctx context.Context, exe string, args []string) ([]byte, int, error) {
		return nil, 2, errors.New("missing yr")
	}); status != "not_evaluated" {
		t.Fatalf("status: got %s", status)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("yara execution failure must not add findings: got %d", count)
	}
	if summary["not_evaluated"] != 1 || summary["scan_error"] != 0 {
		t.Errorf("summary: got %v", summary)
	}
	var source string
	if err := db.QueryRow("SELECT scan_source FROM files").Scan(&source); err != nil {
		t.Fatal(err)
	}
	if source != "structural+yara_error" {
		t.Errorf("source: got %s", source)
	}
}

func TestProcessFileYaraExit1ErrorKeepsNotEvaluatedAndNotFinding(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	path := makeZip(t, dir, map[string]string{"ok.txt": "y"})
	c := cfg()
	c.Yr = "tools/yr.exe"
	summary := map[string]int{}

	if status := processFileWithYara(db, c, &RateState{}, dir, path, summary, nil, func(ctx context.Context, exe string, args []string) ([]byte, int, error) {
		return []byte(`{"rules":[{"identifier":"Zip_Rule"}]}`), 1, errors.New("exit status 1")
	}); status != "not_evaluated" {
		t.Fatalf("status: got %s", status)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("yara exit1 error must not add findings: got %d", count)
	}
	if summary["not_evaluated"] != 1 || summary["scan_error"] != 0 {
		t.Errorf("summary: got %v", summary)
	}

	var source string
	if err := db.QueryRow("SELECT scan_source FROM files").Scan(&source); err != nil {
		t.Fatal(err)
	}
	if source != "structural+yara_error" {
		t.Errorf("source: got %s", source)
	}
}

func TestProcessFileStructuralHitKeepsHitOnYaraError(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	path := makeZip(t, dir, map[string]string{"../evil.txt": "y"})
	c := cfg()
	c.Yr = "tools/yr.exe"
	summary := map[string]int{}

	if status := processFileWithYara(db, c, &RateState{}, dir, path, summary, nil, func(ctx context.Context, exe string, args []string) ([]byte, int, error) {
		return nil, 1, errors.New("yara error")
	}); status != "hit" {
		t.Fatalf("status: got %s", status)
	}
	if summary["hit"] != 1 {
		t.Errorf("summary: got %v", summary)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("structural hit findings: got %d", count)
	}
	var fileStatus, source string
	if err := db.QueryRow("SELECT scan_status, scan_source FROM files").Scan(&fileStatus, &source); err != nil {
		t.Fatal(err)
	}
	if fileStatus != "hit" || source != "structural+yara_error" {
		t.Errorf("file row: got %s %s", fileStatus, source)
	}
}

func TestProcessFileBadZipScanErrorKeepsScanErrorWithoutYaraCall(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.cbz")
	if err := os.WriteFile(path, []byte("PK\x03\x04garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := cfg()
	c.Yr = "tools/yr.exe"
	summary := map[string]int{}

	calls := 0
	if status := processFileWithYara(db, c, &RateState{}, dir, path, summary, nil, func(ctx context.Context, exe string, args []string) ([]byte, int, error) {
		calls++
		return nil, 1, errors.New("yara error")
	}); status != "scan_error" {
		t.Fatalf("status: got %s", status)
	}
	if calls != 0 {
		t.Errorf("structural scan_error must not run yara: got %d calls", calls)
	}
	if summary["scan_error"] != 1 {
		t.Errorf("summary: got %v", summary)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("bad_zip finding: got %d", count)
	}
	var source string
	if err := db.QueryRow("SELECT scan_source FROM files").Scan(&source); err != nil {
		t.Fatal(err)
	}
	if source != "structural" {
		t.Errorf("source: got %s", source)
	}
}

func TestProcessFileYaraSuccessNoRulesKeepsPass(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	path := makeZip(t, dir, map[string]string{"ok.txt": "y"})
	c := cfg()
	c.Yr = "tools/yr.exe"
	summary := map[string]int{}

	if status := processFileWithYara(db, c, &RateState{}, dir, path, summary, nil, func(ctx context.Context, exe string, args []string) ([]byte, int, error) {
		return nil, 0, nil
	}); status != "pass" {
		t.Fatalf("status: got %s", status)
	}
	if summary["pass"] != 1 {
		t.Errorf("summary: got %v", summary)
	}
	var source string
	if err := db.QueryRow("SELECT scan_source FROM files").Scan(&source); err != nil {
		t.Fatal(err)
	}
	if source != "structural" {
		t.Errorf("source: got %s", source)
	}
}

func TestExportCBZExportsStructuralPassWithYaraErrorAsNotEvaluated(t *testing.T) {
	db := openTestDB(t)
	root := t.TempDir()
	galleryDir := filepath.Join(root, "Galleries", "105")
	if err := os.MkdirAll(galleryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := makeZip(t, galleryDir, map[string]string{"ok.txt": "y"})
	c := cfg()
	c.Yr = "tools/yr.exe"
	summary := map[string]int{}

	if status := processFileWithYara(db, c, &RateState{}, root, path, summary, nil, func(ctx context.Context, exe string, args []string) ([]byte, int, error) {
		return nil, 2, errors.New("missing yr")
	}); status != "not_evaluated" {
		t.Fatalf("status: got %s", status)
	}
	result, err := exportCBZ(db, exportTestCfg(), root, 105, "")
	if err != nil {
		t.Fatal(err)
	}
	if result["entries"] != 1 {
		t.Errorf("export entries: got %v", result)
	}
	skipped, _ := result["skipped"].(map[string]int)
	if _, ok := skipped["scan_error"]; ok {
		t.Errorf("export must not skip scan_error: got %v", skipped)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped must be empty: got %v", skipped)
	}
	var fileStatus, source string
	if err := db.QueryRow("SELECT scan_status, scan_source FROM files").Scan(&fileStatus, &source); err != nil {
		t.Fatal(err)
	}
	if fileStatus != "not_evaluated" || source != "structural+yara_error" {
		t.Errorf("file row: got %s %s", fileStatus, source)
	}
}

func TestProcessFileYaraExternalLookupBeforeFindingsWrite(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	path := makeZip(t, dir, map[string]string{"ok.txt": "y"})
	c := cfg()
	c.Yr = "tools/yr.exe"
	c.VirusTotal.Enabled = false
	summary := map[string]int{}

	called := false
	status := processFileWithYara(db, c, &RateState{}, dir, path, summary, nil, func(ctx context.Context, exe string, args []string) ([]byte, int, error) {
		called = true
		var count int
		if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
			t.Fatalf("external yara count: %s", err)
		}
		if count != 0 {
			t.Fatalf("external yara must run before committed findings write, count %d", count)
		}
		return []byte(`{"rules":[{"identifier":"Yara_Rule"}]}`), 0, nil
	})

	if !called {
		t.Fatal("yara external lookup not called")
	}
	if status != "hit" {
		t.Fatalf("status: got %s", status)
	}
	if summary["hit"] != 1 {
		t.Errorf("summary: got %v", summary)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("committed findings: got %d", count)
	}
}

func TestProcessFileUsesCachedNegativeVTWithoutFetch(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	path := makeZip(t, dir, map[string]string{"ok.txt": "y"})
	sha, err := Sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cacheStore(db, sha, "unknown_to_vt", "virustotal", "", "unknown_to_vt"); err != nil {
		t.Fatal(err)
	}
	c := cfg()
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 5, MaxDaily: 5, APIKey: "k"}

	fetchCalls := 0
	fetch := func(url, key string) (map[string]any, int, error) {
		fetchCalls++
		return nil, 404, nil
	}
	summary := map[string]int{}
	status := processFile(db, c, &RateState{}, dir, path, summary, fetch)
	if status != "pass" {
		t.Fatalf("status: got %s", status)
	}
	if fetchCalls != 0 {
		t.Errorf("cached negative VT should avoid fetch: fetchCalls %d", fetchCalls)
	}
	if summary["pass"] != 1 || summary["cached"] != 1 {
		t.Errorf("summary: got %v", summary)
	}

	var rule, detail string
	if err := db.QueryRow("SELECT rule, detail FROM findings WHERE rule = 'virustotal'").Scan(&rule, &detail); err != nil {
		t.Fatal(err)
	}
	if rule != "virustotal" || detail != "unknown_to_vt" {
		t.Errorf("finding: got rule=%q detail=%q", rule, detail)
	}
	var source string
	if err := db.QueryRow("SELECT scan_source FROM files").Scan(&source); err != nil {
		t.Fatal(err)
	}
	if source != "structural+virustotal" {
		t.Errorf("source: got %s", source)
	}
}

func TestProcessFileVTExternalLookupBeforeFindingsWrite(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	path := makeZip(t, dir, map[string]string{"ok.txt": "y"})
	c := cfg()
	c.Yr = ""
	c.VirusTotal = VTConfig{Enabled: true, MaxPerMinute: 5, MaxDaily: 5, APIKey: "k"}

	sha, err := Sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"data": map[string]any{"attributes": map[string]any{
		"last_analysis_stats":   map[string]any{"malicious": 1},
		"last_analysis_results": map[string]any{"e": map[string]any{"category": "malicious", "engine_name": "E", "result": "R"}},
	}}}
	called := false
	fetch := func(url, key string) (map[string]any, int, error) {
		called = true
		var count int
		if err := db.QueryRow("SELECT COUNT(1) FROM findings").Scan(&count); err != nil {
			t.Fatalf("external VT count: %s", err)
		}
		if count != 0 {
			t.Fatalf("external VT must run before committed findings write, count %d", count)
		}
		return payload, 200, nil
	}
	summary := map[string]int{}
	status := processFile(db, c, &RateState{}, dir, path, summary, fetch)
	if !called {
		t.Fatal("VT external lookup not called")
	}
	if status != "hit" {
		t.Fatalf("status: got %s", status)
	}
	if summary["hit"] != 1 {
		t.Errorf("summary: got %v", summary)
	}

	var rule, detail string
	if err := db.QueryRow("SELECT rule, detail FROM findings WHERE rule = 'virustotal'").Scan(&rule, &detail); err != nil {
		t.Fatal(err)
	}
	if rule != "virustotal" || detail != "E: R" {
		t.Errorf("finding: got rule=%q detail=%q", rule, detail)
	}
	var cachedCount int
	if err := db.QueryRow("SELECT COUNT(1) FROM scan_cache").Scan(&cachedCount); err != nil {
		t.Fatal(err)
	}
	if cachedCount != 1 {
		t.Errorf("VT cache must be committed, got %d", cachedCount)
	}
	_ = sha
}

func TestResolveYrPathUsesDataDirForRelativePath(t *testing.T) {
	oldDataDir := dataDir
	dataDir = t.TempDir()
	t.Cleanup(func() { dataDir = oldDataDir })

	yrPath := filepath.Join(dataDir, "tools", "yr.exe")
	if err := os.MkdirAll(filepath.Dir(yrPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(yrPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := resolveYrPath("tools/yr.exe", t.TempDir()); got != yrPath {
		t.Errorf("resolve yr path: got %s want %s", got, yrPath)
	}
}
