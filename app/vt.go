package main

import (
	"context"
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
}

type RateState struct {
	window []time.Time
	total  int
}

type FetchFunc func(url, key string) (map[string]any, error)

func defaultFetch(url, key string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-apikey", key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	payload := map[string]any{}
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func VTLookup(sha string, cfg Config, state *RateState, fetch FetchFunc) VTResult {
	vt := cfg.VirusTotal
	if !vt.Enabled {
		return VTResult{Status: "skipped"}
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
	if len(kept) >= vt.MaxPerMinute {
		return VTResult{Status: "rate_limited"}
	}
	if state.total >= vt.MaxDaily {
		return VTResult{Status: "daily_quota_exhausted"}
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
	payload, err := fetch("https://www.virustotal.com/api/v3/files/"+sha, key)
	if err != nil {
		return VTResult{Status: "scan_error"}
	}
	state.window = append(state.window, now)
	state.total++

	attrs, _ := payload["data"].(map[string]any)
	stats, _ := attrs["attributes"].(map[string]any)
	rawStats, _ := stats["stats"].(map[string]any)
	positives := asInt(rawStats["malicious"]) + asInt(rawStats["suspicious"])
	status := "pass"
	if positives > 0 {
		status = "hit"
	}
	names := []string{}
	results, _ := stats["last_analysis_results"].(map[string]any)
	for _, v := range results {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if entry["category"] == "malicious" || entry["category"] == "suspicious" {
			names = append(names, asString(entry["category"]))
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
	return VTResult{Status: status, Positives: positives, ThreatName: strings.Join(unique, ";")}
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
