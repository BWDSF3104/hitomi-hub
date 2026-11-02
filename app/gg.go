package main

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"time"
)

// GG は hitomi `gg.js` の正典値（bucket と switch values）。switch values と bucket は回転する
// （正典スクリプト: `tools/gen_ggset.py`）。`ggset.go` はその snapshot で、live が取得できない
// 場合の fallback として使う。生成時点と live がずれると shard URL が 404 になる
// （`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`）。
type GG struct {
	Bucket   string
	CaseSet  map[int]bool
	Source   string // "live" / "snapshot"
}

var ggBucketRe = regexp.MustCompile(`b:\s*'([^']*)'`)
var ggCaseRe = regexp.MustCompile(`case (\d+):`)

func snapshotGG() GG {
	return GG{Bucket: ggBucket, CaseSet: ggCaseSet, Source: "snapshot"}
}

// parseGG は gg.js の本文から bucket と switch values を読む。
// 2026-11-01 実測の live `gg.js` は `var o = 0; switch (g) { <cases> o = 1; break; } return o;`。
// よって live では cases 内の value は 1、cases 外の value は 0 を返す。
func parseGG(text string) (GG, error) {
	m := ggBucketRe.FindStringSubmatch(text)
	if m == nil {
		return GG{}, errors.New("gg.b not found")
	}
	gg := GG{Bucket: m[1], CaseSet: map[int]bool{}, Source: "live"}
	for _, c := range ggCaseRe.FindAllStringSubmatch(text, -1) {
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
	for value := range a.CaseSet {
		if !b.CaseSet[value] {
			return false
		}
	}
	return true
}

func (g GG) M(value int) int {
	if g.CaseSet[value] {
		return 1
	}
	return 0
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
		c.gg = snapshotGG()
		c.ggLoadedAt = time.Now()
		if source != "live" {
			source = c.gg.Source
		}
	} else {
		parsed, parseErr := parseGG(string(body))
		if parseErr != nil {
			c.gg = snapshotGG()
			c.ggLoadedAt = time.Now()
			if source != "live" {
				source = c.gg.Source
			}
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
