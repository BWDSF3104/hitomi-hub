package main

import (
	"context"
	"errors"
	"regexp"
	"strconv"
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

// LoadGG は正典 gg.js を 1 回だけ読み、snapshot にフォールバックする
func (c *Client) LoadGG(ctx context.Context) string {
	if !c.cfg.UseLiveGG {
		c.gg = snapshotGG()
		return c.gg.Source
	}
	body, err := c.get(ctx, "https://ltn."+hitomiDomain+"/gg.js", [2]int64{-1, -1})
	if err != nil {
		c.gg = snapshotGG()
		return c.gg.Source
	}
	parsed, err := parseGG(string(body))
	if err != nil {
		c.gg = snapshotGG()
		return c.gg.Source
	}
	c.gg = parsed
	return c.gg.Source
}

func (c *Client) GG() GG {
	if c.gg.CaseSet == nil || c.gg.Bucket == "" {
		return snapshotGG()
	}
	return c.gg
}
