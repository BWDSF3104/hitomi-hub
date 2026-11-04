package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type VTResult struct {
	Status     string
	Positives  int
	ThreatName string
	HTTPStatus int
}

type RateState struct {
	window []time.Time
	total  int
}

type FetchFunc func(url, key string) (map[string]any, int, error)

func defaultFetch(url, key string) (map[string]any, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("x-apikey", key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	payload := map[string]any{}
	if err := decoder.Decode(&payload); err != nil {
		return nil, resp.StatusCode, err
	}
	return payload, resp.StatusCode, nil
}

func VTLookup(db dbExec, sha string, cfg Config, state *RateState, fetch FetchFunc) VTResult {
	vt := cfg.VirusTotal
	if !vt.Enabled {
		return VTResult{Status: "skipped"}
	}
	if state == nil {
		state = &RateState{}
	}

	if db != nil {
		verdict, _, threat, _, err := cacheLookup(db, sha, scanCacheTTL)
		if err != nil {
			return VTResult{Status: "scan_error"}
		}
		if verdict != "" {
			return VTResult{Status: verdict, ThreatName: threat, HTTPStatus: 200}
		}
	}

	now := time.Now()
	cutoff := now.Add(-time.Minute)
	kept := []time.Time{}
	for _, t := range state.window {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	state.window = kept
	if vt.MaxPerMinute > 0 && len(kept) >= vt.MaxPerMinute {
		return VTResult{Status: "rate_limited"}
	}
	if vt.MaxDaily > 0 && state.total >= vt.MaxDaily {
		return VTResult{Status: "daily_quota_exhausted"}
	}
	if db != nil && vt.MaxDaily > 0 {
		total, err := vtDailyTotal(db, vtDay(now))
		if err != nil {
			return VTResult{Status: "scan_error"}
		}
		if total >= vt.MaxDaily {
			return VTResult{Status: "daily_quota_exhausted"}
		}
	}

	key := vt.APIKey
	if key == "" {
		key = os.Getenv(vt.APIKeyEnv)
	}
	if key == "" {
		return VTResult{Status: "no_key"}
	}
	if fetch == nil {
		fetch = defaultFetch
	}
	payload, status, err := fetch("https://www.virustotal.com/api/v3/files/"+sha, key)
	if err != nil {
		return VTResult{Status: "scan_error"}
	}
	state.window = append(state.window, now)
	state.total++
	if db != nil {
		if err := incrementVTUsage(db, vtDay(now)); err != nil {
			return VTResult{Status: "scan_error", HTTPStatus: status}
		}
	}
	if status != 200 {
		resultStatus := "not_evaluated"
		switch status {
		case 401:
			resultStatus = "auth_error"
		case 429:
			resultStatus = "rate_limited"
		case 404:
			resultStatus = "unknown_to_vt"
		}
		if db != nil && resultStatus == "unknown_to_vt" {
			if err := cacheStore(db, sha, resultStatus, "virustotal", "", resultStatus); err != nil {
				return VTResult{Status: "scan_error", HTTPStatus: status}
			}
		}
		return VTResult{Status: resultStatus, HTTPStatus: status}
	}

	attrs, ok := payload["data"].(map[string]any)
	if !ok {
		return VTResult{Status: "not_evaluated", HTTPStatus: status}
	}
	attributes, ok := attrs["attributes"].(map[string]any)
	if !ok {
		return VTResult{Status: "not_evaluated", HTTPStatus: status}
	}
	analysisStats, ok := attributes["last_analysis_stats"].(map[string]any)
	if !ok {
		return VTResult{Status: "not_evaluated", HTTPStatus: status}
	}
	positives := asInt(analysisStats["malicious"]) + asInt(analysisStats["suspicious"])
	results, _ := attributes["last_analysis_results"].(map[string]any)
	// B-06 基準（Python 参照実装と一致）: key 有無ではなく empty results 基準。
	// stats.total>0 で last_analysis_results が空 map の 200 応答も not_evaluated（pass にしない）
	if len(results) == 0 && positives == 0 {
		return VTResult{Status: "not_evaluated", HTTPStatus: status}
	}
	statusString := "pass"
	if positives > 0 {
		statusString = "hit"
	}
	names := []string{}
	for _, v := range results {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if entry["category"] == "malicious" || entry["category"] == "suspicious" {
			names = append(names, engineDisplayName(entry))
		}
	}
	sort.Strings(names)
	unique := []string{}
	for _, name := range names {
		if len(unique) == 0 || unique[len(unique)-1] != name {
			unique = append(unique, name)
		}
	}
	if len(unique) > 5 {
		unique = unique[:5]
	}
	return VTResult{Status: statusString, Positives: positives, ThreatName: strings.Join(unique, ";"), HTTPStatus: status}
}

func engineDisplayName(entry map[string]any) string {
	result := asString(entry["result"])
	engine := asString(entry["engine_name"])
	if engine != "" && result != "" {
		return engine + ": " + result
	}
	if result != "" {
		return result
	}
	if engine != "" {
		return engine
	}
	return asString(entry["category"])
}

func vtDay(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func vtDailyTotal(db dbExec, day string) (int, error) {
	var total int
	err := db.QueryRow("SELECT total FROM vt_rate_state WHERE day = ?", day).Scan(&total)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return total, err
}

func incrementVTUsage(db dbExec, day string) error {
	_, err := db.Exec("INSERT INTO vt_rate_state (day, total) VALUES (?, 1) ON CONFLICT(day) DO UPDATE SET total = total + 1", day)
	return err
}

func asInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

type YaraResult struct {
	Rules  []string
	Status string
	Error  string
	Exit   int
}

type YaraRunFunc func(ctx context.Context, exe string, args []string) ([]byte, int, error)

func defaultYaraRun(ctx context.Context, exe string, args []string) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	out, err := cmd.Output()
	exit := 0
	if cmd.ProcessState != nil {
		exit = cmd.ProcessState.ExitCode()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exit = exitErr.ExitCode()
	}
	return out, exit, err
}

func runYaraWithRunner(ctx context.Context, path, rulesPath, yr string, l Limits, run YaraRunFunc) YaraResult {
	if run == nil {
		run = defaultYaraRun
	}
	args := []string{"scan", rulesPath, path, "--output-format", "ndjson",
		"--timeout", fmt.Sprintf("%d", l.YrTimeoutSec),
		"--cpu-limit", fmt.Sprintf("%d", l.YrCpuLimit),
		"--skip-larger", fmt.Sprintf("%d", l.YrSkipLarger)}
	if l.YrFastScan {
		args = append(args, "--fast-scan")
	}
	out, exit, err := run(ctx, yr, args)
	rules := ParseYaraOutput(string(out))
	if err != nil || exit != 0 {
		errText := ""
		if err != nil {
			errText = err.Error()
		} else {
			errText = fmt.Sprintf("exit status %d", exit)
		}
		return YaraResult{Status: "scan_error", Error: errText, Exit: exit}
	}
	if len(rules) > 0 {
		return YaraResult{Rules: rules, Status: "hit", Exit: exit}
	}
	return YaraResult{Status: "pass", Exit: exit}
}

func resolveYrPath(yr, root string) string {
	if yr == "" {
		return ""
	}
	if filepath.IsAbs(yr) {
		return yr
	}
	candidates := []string{
		filepath.Join(dataDir, yr),
		filepath.Join(root, yr),
	}
	if abs, err := filepath.Abs(yr); err == nil {
		candidates = append(candidates, abs)
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return yr
}

func RunYaraScan(path, rulesPath, yr string, l Limits) YaraResult {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(l.YrTimeoutSec+15)*time.Second)
	defer cancel()
	return runYaraWithRunner(ctx, path, rulesPath, yr, l, defaultYaraRun)
}

func ParseYaraOutput(text string) []string {
	rules := []string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		obj := map[string]any{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		if list, ok := obj["rules"].([]any); ok {
			for _, item := range list {
				if entry, ok := item.(map[string]any); ok {
					if id := asString(entry["identifier"]); id != "" {
						rules = append(rules, id)
					}
				} else if s, ok := item.(string); ok {
					rules = append(rules, s)
				}
			}
		} else if rule := asString(obj["rule"]); rule != "" {
			rules = append(rules, rule)
		}
	}
	return rules
}

func RunYara(path, rulesPath, yr string, l Limits) []string {
	result := RunYaraScan(path, rulesPath, yr, l)
	if result.Status == "scan_error" {
		return []string{"yara_error:" + result.Error}
	}
	return result.Rules
}

func executablePath() string {
	path, err := os.Executable()
	if err != nil {
		return "."
	}
	return path
}
