package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
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

func VTLookup(db *sql.DB, sha string, cfg Config, state *RateState, fetch FetchFunc) VTResult {
	vt := cfg.VirusTotal
	if !vt.Enabled {
		return VTResult{Status: "skipped"}
	}
	if state == nil {
		state = &RateState{}
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
		}
		return VTResult{Status: resultStatus, HTTPStatus: status}
	}

	attrs, _ := payload["data"].(map[string]any)
	stats, _ := attrs["attributes"].(map[string]any)
	rawStats, _ := stats["stats"].(map[string]any)
	positives := asInt(rawStats["malicious"]) + asInt(rawStats["suspicious"])
	statusString := "pass"
	if positives > 0 {
		statusString = "hit"
	}
	names := []string{}
	results, _ := stats["last_analysis_results"].(map[string]any)
	for _, v := range results {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if entry["category"] == "malicious" || entry["category"] == "suspicious" {
			name := asString(entry["name"])
			if name == "" {
				name = asString(entry["category"])
			}
			names = append(names, name)
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

func vtDay(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func vtDailyTotal(db *sql.DB, day string) (int, error) {
	var total int
	err := db.QueryRow("SELECT total FROM vt_rate_state WHERE day = ?", day).Scan(&total)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return total, err
}

func incrementVTUsage(db *sql.DB, day string) error {
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(l.YrTimeoutSec+15)*time.Second)
	defer cancel()
	args := []string{"scan", rulesPath, path, "--output-format", "ndjson",
		"--timeout", fmt.Sprintf("%d", l.YrTimeoutSec),
		"--cpu-limit", fmt.Sprintf("%d", l.YrCpuLimit),
		"--skip-larger", fmt.Sprintf("%d", l.YrSkipLarger)}
	if l.YrFastScan {
		args = append(args, "--fast-scan")
	}
	cmd := exec.CommandContext(ctx, yr, args...)
	out, err := cmd.Output()
	if err != nil {
		return []string{"yara_error:" + err.Error()}
	}
	rules := ParseYaraOutput(string(out))
	if len(rules) == 0 && cmd.ProcessState != nil && cmd.ProcessState.ExitCode() > 1 {
		return []string{fmt.Sprintf("yara_error:exit_%d", cmd.ProcessState.ExitCode())}
	}
	return rules
}

func executablePath() string {
	path, err := os.Executable()
	if err != nil {
		return "."
	}
	return path
}
