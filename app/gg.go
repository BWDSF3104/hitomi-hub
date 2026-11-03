package main

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"time"
)

// GG は hitomi `gg.js` の正典値（bucket、switch values、polarity）。switch values と bucket は
// 回転する（正典スクリプト: `tools/gen_ggset.py`）。`ggset.go` はその snapshot で、live が取得できない
// 場合の fallback として使う。生成時点と live がずれると shard URL が 404 になる
// （`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`）。
// polarity（初期 o と case 代入）は窓ごとに rotation するため固定しない。faithful parse が
// その gg.js が実際に返す値を保持する（`docs/ai/decisions/2026-11-03-gg-faithful-parse-not-fixed-polarity.md`）。
// mapping 方向は固定: `m = 0 → w1`/`atn`、`m = 1 → w2`/`btn`。
type GG struct {
	Bucket    string
	CaseSet   map[int]bool
	InitialO  int  // `var o` の初期値（live 観測・faithful parse）
	CaseValue int  // switch 内 case 代入値（live 観測・faithful parse）
	Source    string // "live" / "snapshot"
}

var ggBucketRe = regexp.MustCompile(`b:\s*'([^']*)'`)
var ggCaseRe = regexp.MustCompile(`case (\d+):`)
var ggInitialORe = regexp.MustCompile(`var o\s*=\s*(\d+)`)
var ggSwitchRe = regexp.MustCompile(`(?s)switch\s*\(\s*g\s*\)\s*\{(.*?)\}`)
var ggCaseAssignRe = regexp.MustCompile(`o\s*=\s*(\d+)`)

func snapshotGG() GG {
	return GG{Bucket: ggBucket, CaseSet: ggCaseSet, InitialO: ggInitialO, CaseValue: ggCaseValue, Source: "snapshot"}
}

// parseGG は gg.js の本文から bucket、switch values、polarity（初期 o と case 代入）を読む。
// 構造は窓ごとに rotation する（`var o = 0; case → o = 1` 窓と `var o = 1; case → o = 0` 窓の
// 両方が実測済み）。polarity を固定せず、その gg.js が実際に返す値を保持する。
func parseGG(text string) (GG, error) {
	m := ggBucketRe.FindStringSubmatch(text)
	if m == nil {
		return GG{}, errors.New("gg.b not found")
	}
	sw := ggSwitchRe.FindStringSubmatch(text)
	if sw == nil {
		return GG{}, errors.New("gg switch not found")
	}
	init := ggInitialORe.FindStringSubmatch(text)
	if init == nil {
		return GG{}, errors.New("gg initial o not found")
	}
	assign := ggCaseAssignRe.FindStringSubmatch(sw[1])
	if assign == nil {
		return GG{}, errors.New("gg case assignment not found")
	}
	initialO, err := strconv.Atoi(init[1])
	if err != nil {
		return GG{}, errors.New("gg initial o not parseable")
	}
	caseValue, err := strconv.Atoi(assign[1])
	if err != nil {
		return GG{}, errors.New("gg case assignment not parseable")
	}
	gg := GG{Bucket: m[1], CaseSet: map[int]bool{}, InitialO: initialO, CaseValue: caseValue, Source: "live"}
	for _, c := range ggCaseRe.FindAllStringSubmatch(sw[1], -1) {
		n, err := strconv.Atoi(c[1])
		if err == nil {
			gg.CaseSet[n] = true
		}
	}
	if len(gg.CaseSet) == 0 {
		return GG{}, errors.New("gg switch values not found")
	}
	return gg, nil
}

func sameGG(a, b GG) bool {
	if a.Source != b.Source || a.Bucket != b.Bucket || len(a.CaseSet) != len(b.CaseSet) {
		return false
	}
	if a.InitialO != b.InitialO || a.CaseValue != b.CaseValue {
		return false
	}
	for value := range a.CaseSet {
		if !b.CaseSet[value] {
			return false
		}
	}
	return true
}

// M はその gg.js が実際に返す m(g)（faithful parse）。case 内なら case 代入値、case 外なら初期 o。
// mapping 方向は hitomi 準拠で固定: 0 → `w1`/`atn`、1 → `w2`/`btn`（`ImageURL`・`ThumbnailURLVariant`）。
func (g GG) M(value int) int {
	if g.CaseSet[value] {
		return g.CaseValue
	}
	return g.InitialO
}

// Value は gg.s(hash) と同じ（末尾 3 hex を反転して 10 進）
func (g GG) Value(hash string) int {
	if len(hash) < 3 {
		return 0
	}
	v, err := strconv.ParseInt(hash[len(hash)-1:]+hash[len(hash)-3:len(hash)-1], 16, 64)
	if err != nil {
		return 0
	}
	return int(v)
}

// LoadGG は正典 gg.js を TTL cache 経由で読み、snapshot にフォールバックする
func (c *Client) LoadGG(ctx context.Context) string {
	return c.EnsureGG(ctx, false)
}

// EnsureGG は live gg を TTL cache から返す。force refresh は minimum interval と singleflight で
// network 中に cache mutex を保持しない。
func (c *Client) EnsureGG(ctx context.Context, force bool) string {
	c.singleflightMu.Lock()
	c.cacheMu.Lock()
	source := c.gg.Source
	previous := c.gg
	age := time.Since(c.ggLoadedAt)
	useLive := c.cfg.UseLiveGG
	if !useLive {
		c.gg = snapshotGG()
		c.ggLoadedAt = time.Now()
		c.cacheMu.Unlock()
		c.singleflightMu.Unlock()
		return c.gg.Source
	}
	forceMin := time.Duration(c.cfg.GGForceMinIntervalMS) * time.Millisecond
	if force && forceMin > 0 && !c.ggForceAt.IsZero() && time.Since(c.ggForceAt) < forceMin {
		c.cacheMu.Unlock()
		c.singleflightMu.Unlock()
		return source
	}
	if !force && source == "live" && age < ggCacheTTL {
		c.cacheMu.Unlock()
		c.singleflightMu.Unlock()
		return source
	}

	key := "https://ltn." + hitomiDomain + "/gg.js"
	if c.singleflight == nil {
		c.singleflight = map[string]*singleflightCall{}
	}
	if call := c.singleflight[key]; call != nil {
		c.cacheMu.Unlock()
		c.singleflightMu.Unlock()
		<-call.done
		c.cacheMu.Lock()
		defer c.cacheMu.Unlock()
		return c.gg.Source
	}
	call := &singleflightCall{done: make(chan struct{})}
	c.singleflight[key] = call
	c.cacheMu.Unlock()
	c.singleflightMu.Unlock()

	body, err := c.get(ctx, key, [2]int64{-1, -1})

	c.singleflightMu.Lock()
	c.cacheMu.Lock()
	if c.singleflight[key] == call {
		delete(c.singleflight, key)
	}
	if force {
		c.ggForceAt = time.Now()
	}
	if err != nil {
		if source == "live" {
			c.gg = previous
		} else {
			c.gg = snapshotGG()
			source = c.gg.Source
		}
		c.ggLoadedAt = time.Now()
	} else {
		parsed, parseErr := parseGG(string(body))
		if parseErr != nil {
			if source == "live" {
				c.gg = previous
			} else {
				c.gg = snapshotGG()
				source = c.gg.Source
			}
			c.ggLoadedAt = time.Now()
		} else {
			c.gg = parsed
			c.ggLoadedAt = time.Now()
			source = c.gg.Source
		}
	}
	c.cacheMu.Unlock()
	c.singleflightMu.Unlock()

	close(call.done)
	return source
}

func (c *Client) GG() GG {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	if c.gg.CaseSet == nil || c.gg.Bucket == "" {
		return snapshotGG()
	}
	return c.gg
}
