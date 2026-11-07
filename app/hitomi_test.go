package main

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 2026-11-01 に ltn 実測で確認した値（HTTP 200・210784 bytes）
const sampleHash = "2d18a9a8fb2d971ec67f4287a13322738c04b8111e4cf4c8d73b2745a5942b89"

// faithful parse 基準の fixture。polarity は固定しない。窓ごとに観測される両構造を持つ:
// A = `var o = 0`・case 代入 `o = 1`（11-03 11:38Z 窓）、B = `var o = 1`・case 代入 `o = 0`（11-03 12:00 窓）。
// 同じ value に対し A と B で m が反転すること（= w1/w2・atn/btn が反転）をテストで固定する。
const ggFixtureA = "var gg = { b: '1793494802/', m: function (g) { var o = 0; switch (g) { case 2488: o = 1; break; } return o; } };"
const ggFixtureB = "var gg = { b: '1793494802/', m: function (g) { var o = 1; switch (g) { case 2488: o = 0; break; } return o; } };"

func TestClientCacheFieldsAreMutexProtected(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.cfg.MinIntervalMS = 1
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		if strings.Contains(rawurl, "/gg.js") {
			return []byte(ggFixtureA), nil
		}
		if strings.HasSuffix(rawurl, "/version") {
			return []byte("1793467632"), nil
		}
		return nil, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.EnsureGG(context.Background(), true)
			_ = c.GG()
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = c.ensureIndexVersion("galleries", false, false)
		}()
	}
	wg.Wait()
}

func TestIndexTopCacheEvictsLRUEntriesWhenOverEntryLimit(t *testing.T) {
	fetches := 0
	c := &Client{cfg: defaultHitomiConfig(), indexTopCacheMaxEntries: 2, indexTopCacheMaxBytes: 1000, testGateIntervalMS: 1}
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		return beNode(1, [][]byte{hashTerm("top")}, [][2]int64{{0, 0}}, make([]uint64, btreeOrder+1)), nil
	}

	// Windows clock granularity can tie same-instant timestamps. Fix access order
	// explicitly after each insert so LRU eviction order is deterministic.
	base := time.Now()
	for i, version := range []string{"v1", "v2", "v3"} {
		if _, _, err := c.indexTopNodeCached("galleries", version, false); err != nil {
			t.Fatalf("indexTopNodeCached %s: %s", version, err)
		}
		if entry, ok := c.indexTopCache["galleries|"+version]; ok {
			entry.lastAccess = base.Add(time.Duration(-30+10*i) * time.Second)
		}
	}
	if len(c.indexTopCache) != 2 {
		t.Fatalf("entry limit 2 should evict oldest entry: len %d", len(c.indexTopCache))
	}
	if _, ok := c.indexTopCache["galleries|v1"]; ok {
		t.Error("oldest entry should be evicted when entry limit is crossed")
	}
	if _, ok := c.indexTopCache["galleries|v2"]; !ok {
		t.Error("second entry should remain")
	}
	if _, ok := c.indexTopCache["galleries|v3"]; !ok {
		t.Error("newest entry should remain")
	}
	if c.indexTopCacheBytes > c.indexTopCacheMaxBytes {
		t.Errorf("index top cache memory must stay below cap: bytes %d", c.indexTopCacheBytes)
	}

	// LRU: hit updates access order. Next insert must evict v3 (stale access), not v2.
	if _, _, err := c.indexTopNodeCached("galleries", "v2", false); err != nil {
		t.Fatal(err)
	}
	if fetches != 3 {
		t.Fatalf("TTL hit must not refetch: fetches %d", fetches)
	}
	if _, _, err := c.indexTopNodeCached("galleries", "v4", false); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.indexTopCache["galleries|v3"]; ok {
		t.Error("stale-access entry must be evicted by LRU order")
	}
	if _, ok := c.indexTopCache["galleries|v2"]; !ok {
		t.Error("recently accessed entry must survive LRU eviction")
	}

	// Evicted key is not a TTL hit anymore: it refetches and memory stays bounded.
	if _, _, err := c.indexTopNodeCached("galleries", "v1", false); err != nil {
		t.Fatal(err)
	}
	if fetches != 5 {
		t.Errorf("evicted key must refetch: fetches %d", fetches)
	}
	if len(c.indexTopCache) > 2 || c.indexTopCacheBytes > c.indexTopCacheMaxBytes {
		t.Errorf("memory must stay below cap after refetch insert: len %d bytes %d", len(c.indexTopCache), c.indexTopCacheBytes)
	}
}

func TestNoFullCacheCleansExpiredEntriesWhenOverEntryLimit(t *testing.T) {
	if defaultNoFullCacheMaxEntries != 1024 {
		t.Fatalf("approved default entry limit must be 1024: got %d", defaultNoFullCacheMaxEntries)
	}

	c := &Client{cfg: defaultHitomiConfig(), noFullCacheMaxEntries: 4}
	c.noFullCache = make(map[string]time.Time)
	gg := snapshotGG()
	base := time.Now()
	key := func(hash string) string { return negativeNoFullKey(hash, "webp", gg) }

	// stale entry（TTL 超過済み）below the cap must not be cleaned by insert.
	c.noFullCache[key("h1")] = base.Add(-negativeNoFullCacheTTL - time.Second)
	c.noFullCache[key("h2")] = base.Add(-negativeNoFullCacheTTL - 2*time.Second)
	c.rememberNegativeNoFull("h3", "webp", gg)
	if len(c.noFullCache) != 3 {
		t.Fatalf("insert at or below cap must not clean: len %d", len(c.noFullCache))
	}

	// canUseNegativeNoFull TTL hit 意味不変: expired entry は hit 時 delete される。
	if c.canUseNegativeNoFull("h1", "webp", gg) {
		t.Fatal("expired entry must not be a TTL hit")
	}
	if _, ok := c.noFullCache[key("h1")]; ok {
		t.Error("expired entry must be deleted on TTL hit")
	}

	// re-add h1 stale, then cross the cap via insert: only stale keys are cleaned.
	c.noFullCache[key("h1")] = base.Add(-negativeNoFullCacheTTL - 3*time.Second)
	c.rememberNegativeNoFull("h4", "webp", gg)
	if len(c.noFullCache) != 4 {
		t.Fatalf("stale entries must survive until cap is crossed by insert: len %d", len(c.noFullCache))
	}
	c.rememberNegativeNoFull("h5", "webp", gg)
	if len(c.noFullCache) != 3 {
		t.Fatalf("stale-only cleaning over cap should leave only active entries: len %d", len(c.noFullCache))
	}
	for _, hash := range []string{"h1", "h2"} {
		if _, ok := c.noFullCache[key(hash)]; ok {
			t.Errorf("stale entry %s must be cleaned when entry limit is crossed", hash)
		}
	}
	for _, hash := range []string{"h3", "h4", "h5"} {
		if _, ok := c.noFullCache[key(hash)]; !ok {
			t.Errorf("active entry %s must not be evicted by stale-only cleaning", hash)
		}
	}

	// default cap 1024 動作（C-05 実測の stale 残留相当）: cap 超過 insert で stale 一掃。
	c2 := &Client{cfg: defaultHitomiConfig()}
	c2.noFullCache = make(map[string]time.Time)
	for i := 0; i < 1024; i++ {
		c2.noFullCache[negativeNoFullKey(strconv.Itoa(i), "webp", gg)] = base.Add(-negativeNoFullCacheTTL - time.Duration(i+1)*time.Second)
	}
	c2.rememberNegativeNoFull("trigger", "webp", gg)
	if len(c2.noFullCache) != 1 {
		t.Fatalf("default cap 1024 must clean stale residue on over-cap insert: len %d", len(c2.noFullCache))
	}
	if !c2.canUseNegativeNoFull("trigger", "webp", gg) {
		t.Error("active entry must survive default cap stale-only cleaning")
	}
}

func TestNoFullCacheDoesNotEvictActiveEntriesWhenOverEntryLimit(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig(), noFullCacheMaxEntries: 2}
	gg := snapshotGG()
	key := func(hash string) string { return negativeNoFullKey(hash, "webp", gg) }

	for _, hash := range []string{"h1", "h2", "h3"} {
		c.rememberNegativeNoFull(hash, "webp", gg)
	}
	// active protected, no active eviction: cap 超過でも active entry は消えない。
	if len(c.noFullCache) != 3 {
		t.Fatalf("active entries over entry limit must be protected: len %d", len(c.noFullCache))
	}
	for _, hash := range []string{"h1", "h2", "h3"} {
		if _, ok := c.noFullCache[key(hash)]; !ok {
			t.Errorf("active entry %s must not be evicted when over entry limit", hash)
		}
	}

	// canUseNegativeNoFull TTL hit 意味不変（cap 超過後も active entry は hit）。
	for _, hash := range []string{"h1", "h2", "h3"} {
		if !c.canUseNegativeNoFull(hash, "webp", gg) {
			t.Errorf("active entry %s must remain a TTL hit", hash)
		}
	}
}

func TestCanonicalGGRuleComposition(t *testing.T) {
	// mapping 方向は固定（0 → `w1`/`atn`、1 → `w2`/`btn`）。polarity は窓ごとに rotation するため
	// faithful parse が保持する。両構造で同じ value の w1/w2（atn/btn）が反転することを固定する。
	// 構造 A: `var o = 0`・case → `o = 1`。構造 B: `var o = 1`・case → `o = 0`。
	ggA, err := parseGG(ggFixtureA)
	if err != nil {
		t.Fatal(err)
	}
	ggB, err := parseGG(ggFixtureB)
	if err != nil {
		t.Fatal(err)
	}

	inSet := sampleHash // value 2488 → 両 fixture の case
	outSet := "21844c51814f950f39d448afca0c701a49c3c0cbd1b513ebf18e936ad9290065" // value 1286 → case 外
	if ggA.Value(inSet) != 2488 || ggA.Value(outSet) != 1286 {
		t.Errorf("gg value: %d %d", ggA.Value(inSet), ggA.Value(outSet))
	}

	// fixture A の polarity: 初期 o=0・case 代入=1。
	if ggA.Source != "live" || ggA.Bucket != "1793494802/" || len(ggA.CaseSet) != 1 {
		t.Errorf("parsed gg A: got %+v", ggA)
	}
	if ggA.InitialO != 0 || ggA.CaseValue != 1 || ggA.M(2488) != 1 || ggA.M(1286) != 0 {
		t.Errorf("gg A polarity: init=%d case=%d in=%d out=%d", ggA.InitialO, ggA.CaseValue, ggA.M(2488), ggA.M(1286))
	}
	// fixture B の polarity: 初期 o=1・case 代入=0（A と反転）。
	if ggB.InitialO != 1 || ggB.CaseValue != 0 || ggB.M(2488) != 0 || ggB.M(1286) != 1 {
		t.Errorf("gg B polarity: init=%d case=%d in=%d out=%d", ggB.InitialO, ggB.CaseValue, ggB.M(2488), ggB.M(1286))
	}

	// mapping 方向固定（構造 A 基準）
	if got := ImageURL(inSet, "webp", "webp", ggA); got != "https://w2.gold-usergeneratedcontent.net/1793494802/2488/"+inSet+".webp" {
		t.Errorf("image url A in (m=1): got %s", got)
	}
	if got := ImageURL(outSet, "webp", "webp", ggA); got != "https://w1.gold-usergeneratedcontent.net/1793494802/1286/"+outSet+".webp" {
		t.Errorf("image url A out (m=0): got %s", got)
	}

	// 構造差分で同じ value の w1/w2 が反転する（A in=w2 → B in=w1、A out=w1 → B out=w2）
	if got := ImageURL(inSet, "webp", "webp", ggB); got != "https://w1.gold-usergeneratedcontent.net/1793494802/2488/"+inSet+".webp" {
		t.Errorf("image url B in (m=0): got %s", got)
	}
	if got := ImageURL(outSet, "webp", "webp", ggB); got != "https://w2.gold-usergeneratedcontent.net/1793494802/1286/"+outSet+".webp" {
		t.Errorf("image url B out (m=1): got %s", got)
	}

	// TN subdomain も構造差分で反転（A in=b、A out=a → B in=a、B out=b）。base/format は構造に依存しない。
	if got := ThumbnailURL(inSet, "webp", ggA); got != "https://btn."+hitomiDomain+"/webpbigtn/9/b8/"+inSet+".webp" {
		t.Errorf("tn A in (m=1): got %s", got)
	}
	if got := ThumbnailURL(outSet, "avif", ggA); got != "https://atn."+hitomiDomain+"/avifbigtn/5/06/"+outSet+".avif" {
		t.Errorf("tn A out avif (m=0): got %s", got)
	}
	if got := ThumbnailURL(inSet, "webp", ggB); got != "https://atn."+hitomiDomain+"/webpbigtn/9/b8/"+inSet+".webp" {
		t.Errorf("tn B in (m=0): got %s", got)
	}
	if got := ThumbnailURL(outSet, "avif", ggB); got != "https://btn."+hitomiDomain+"/avifbigtn/5/06/"+outSet+".avif" {
		t.Errorf("tn B out avif (m=1): got %s", got)
	}
	if strings.Contains(ThumbnailURL(outSet, "avif", ggA), "tnbigtn") {
		t.Errorf("tn name must not appear as base: got %s", ThumbnailURL(outSet, "avif", ggA))
	}
}

func TestEnsureGGUsesTTLAndForceRefresh(t *testing.T) {
		fixture := ggFixtureA
	calls := 0
	c := &Client{cfg: defaultHitomiConfig()}
	c.cfg.MinIntervalMS = 1
	c.cfg.GGForceMinIntervalMS = 0
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		if !strings.Contains(rawurl, "/gg.js") {
			return nil, errors.New("expected gg.js")
		}
		return []byte(fixture), nil
	}
	if source := c.EnsureGG(context.Background(), false); source != "live" {
		t.Fatalf("first source: got %s", source)
	}
	if source := c.EnsureGG(context.Background(), false); source != "live" {
		t.Fatalf("cached source: got %s", source)
	}
	if calls != 1 {
		t.Errorf("TTL cache should not refetch within TTL: calls %d", calls)
	}
	c.ggLoadedAt = time.Now().Add(-2 * time.Hour)
	if source := c.EnsureGG(context.Background(), false); source != "live" {
		t.Fatalf("expired source: got %s", source)
	}
	if calls != 2 {
		t.Errorf("expired cache should refetch: calls %d", calls)
	}
	if source := c.EnsureGG(context.Background(), true); source != "live" {
		t.Fatalf("force source: got %s", source)
	}
	if calls != 3 {
		t.Errorf("force refresh should refetch: calls %d", calls)
	}
}

func TestEnsureGGForceMinIntervalSkipsSecondForce(t *testing.T) {
		fixture := ggFixtureA
	calls := 0
	c := &Client{cfg: defaultHitomiConfig()}
	c.cfg.MinIntervalMS = 1
	c.cfg.GGForceMinIntervalMS = 60000
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		if !strings.Contains(rawurl, "/gg.js") {
			return nil, errors.New("expected gg.js")
		}
		return []byte(fixture), nil
	}
	if source := c.EnsureGG(context.Background(), true); source != "live" {
		t.Fatalf("first force source: got %s", source)
	}
	if calls != 1 {
		t.Fatalf("first force calls: got %d", calls)
	}
	if source := c.EnsureGG(context.Background(), true); source != "live" {
		t.Fatalf("second force source: got %s", source)
	}
	if calls != 1 {
		t.Errorf("force within minimum interval should not refetch: calls %d", calls)
	}
	c.ggForceAt = time.Now().Add(-2 * time.Minute)
	if source := c.EnsureGG(context.Background(), true); source != "live" {
		t.Fatalf("third force source: got %s", source)
	}
	if calls != 2 {
		t.Errorf("force after minimum interval should refetch: calls %d", calls)
	}
}

func TestEnsureGGSingleflightDoesNotRepeatConcurrentForce(t *testing.T) {
		fixture := ggFixtureA
	c := &Client{cfg: defaultHitomiConfig()}
	c.cfg.MinIntervalMS = 1
	c.cfg.GGForceMinIntervalMS = 60000
	var calls int32
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		if !strings.Contains(rawurl, "/gg.js") {
			return nil, errors.New("expected gg.js")
		}
		atomic.AddInt32(&calls, 1)
		return []byte(fixture), nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if source := c.EnsureGG(context.Background(), true); source != "live" {
				t.Errorf("concurrent force source: got %s", source)
			}
		}()
	}
	wg.Wait()
	if calls != 1 {
		t.Errorf("singleflight should fetch once, got %d", calls)
	}
}

func TestEnsureGGFailurePreservesLiveGG(t *testing.T) {
		fixture := ggFixtureA
	c := &Client{cfg: defaultHitomiConfig()}
	c.cfg.MinIntervalMS = 1
	c.cfg.GGForceMinIntervalMS = 0
	calls := 0
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte(fixture), nil
		}
		return nil, errors.New("offline")
	}

	if source := c.EnsureGG(context.Background(), false); source != "live" {
		t.Fatalf("first source: got %s", source)
	}
	bucket := c.gg.Bucket

	c.ggLoadedAt = time.Now().Add(-2 * time.Hour)
	if source := c.EnsureGG(context.Background(), false); source != "live" {
		t.Fatalf("failure source must stay live: got %s", source)
	}
	if c.gg.Bucket != bucket {
		t.Fatalf("failure must preserve live gg: bucket %s want %s", c.gg.Bucket, bucket)
	}
	if calls != 2 {
		t.Fatalf("calls: got %d", calls)
	}
}

func TestSnapshotGGIsPopulatedAndUsedAsFallback(t *testing.T) {
	gg := snapshotGG()
	if gg.Bucket == "" || len(gg.CaseSet) == 0 {
		t.Fatalf("snapshot gg is empty: %+v", gg)
	}
	// snapshot は生成時点の live で観測した polarity をそのまま保持する（正典生成: `tools/gen_ggset.py`）。
	// rotating な実値をハードコードしない。polarity 反転（case 代入 ≠ 初期 o）と M の faithful 挙動を固定する。
	if gg.InitialO == gg.CaseValue {
		t.Fatalf("snapshot polarity not populated (observed structure must have distinct values): %+v", gg)
	}
	if gg.InitialO != ggInitialO || gg.CaseValue != ggCaseValue {
		t.Fatalf("snapshot must carry observed polarity: got init=%d case=%d want init=%d case=%d", gg.InitialO, gg.CaseValue, ggInitialO, ggCaseValue)
	}
	inCase := -1
	for value := range gg.CaseSet {
		inCase = value
		break
	}
	outCase := 4096
	for gg.CaseSet[outCase] {
		outCase++
	}
	if gg.M(inCase) != gg.CaseValue || gg.M(outCase) != gg.InitialO {
		t.Errorf("snapshot M not faithful: in=%d want %d, out=%d want %d", gg.M(inCase), gg.CaseValue, gg.M(outCase), gg.InitialO)
	}

	offline := &Client{cfg: defaultHitomiConfig()}
	offline.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return nil, errors.New("offline")
	}
	if source := offline.LoadGG(context.Background()); source != "snapshot" {
		t.Errorf("offline source: got %s", source)
	}
	if offline.GG().Bucket != gg.Bucket {
		t.Errorf("fallback bucket: got %s", offline.GG().Bucket)
	}

	live := &Client{cfg: defaultHitomiConfig()}
	live.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		if !strings.Contains(rawurl, "/gg.js") {
			t.Errorf("expected gg.js fetch, got %s", rawurl)
		}
		return []byte(ggFixtureA), nil
	}
	if source := live.LoadGG(context.Background()); source != "live" {
		t.Errorf("live source: got %s", source)
	}

	off := &Client{cfg: defaultHitomiConfig()}
	off.cfg.UseLiveGG = false
	if source := off.LoadGG(context.Background()); source != "snapshot" {
		t.Errorf("config off: got %s", source)
	}
}

func TestHitomiConfigPreservesUseLiveGGDefaultWhenJSONOmitted(t *testing.T) {
	cfg := HitomiConfig{}
	if err := json.Unmarshal([]byte(`{"min_interval_ms":1000}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.UseLiveGG {
		t.Errorf("omitted use_live_gg should remain true: got %v", cfg.UseLiveGG)
	}

	explicit := HitomiConfig{}
	if err := json.Unmarshal([]byte(`{"min_interval_ms":1000,"use_live_gg":false}`), &explicit); err != nil {
		t.Fatal(err)
	}
	if explicit.UseLiveGG {
		t.Errorf("explicit use_live_gg false should remain false: got %v", explicit.UseLiveGG)
	}
}

func TestSearchNozomiDecodeAndRange(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1}
	var gotURL string
	var gotRange [2]int64
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		gotURL = rawurl
		gotRange = rng
		return []byte{0, 0, 41, 207, 0, 0, 41, 196, 0, 0, 41, 180}, nil
	}
	ids, err := c.Search("type:doujinshi", "japanese", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || ids[0] != 10703 {
		t.Errorf("ids: got %v", ids)
	}
	if !strings.Contains(gotURL, "/n/type/doujinshi-japanese.nozomi") {
		t.Errorf("namespace area: got %s", gotURL)
	}
	if gotRange[0] != 100 || gotRange[1] != 199 {
		t.Errorf("range: got %v", gotRange)
	}
	if _, err := c.Search("female:yuri", "english", 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotURL, "/n/tag/female:yuri-english.nozomi") {
		t.Errorf("female area: got %s", gotURL)
	}
	if _, err := c.Search("goudoushi", "japanese", 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotURL, "/n/goudoushi-japanese.nozomi") {
		t.Errorf("plain tag: got %s", gotURL)
	}
}

func TestSearchQueryIntersectsTermsAndLanguage(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1}
	var urls []string
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		urls = append(urls, rawurl)
		if strings.Contains(rawurl, "/n/tag/female:yuri-english.nozomi") {
			return beIDs([]uint32{10, 20, 30}), nil
		}
		return beIDs([]uint32{20, 30, 99}), nil
	}
	ids, err := c.SearchQuery("female:yuri type:doujinshi language:english", "english", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != 20 || ids[1] != 30 {
		t.Errorf("intersection: got %v", ids)
	}
	if len(urls) != 2 {
		t.Errorf("expected one fetch per term, got %v", urls)
	}
	if !strings.Contains(urls[0], "/n/tag/female:yuri-english.nozomi") || !strings.Contains(urls[1], "/n/type/doujinshi-english.nozomi") {
		t.Errorf("paths: got %v", urls)
	}
	ids, err = c.SearchQuery("language:japanese", "english", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 3 || !strings.Contains(urls[2], "/n/index-japanese.nozomi") {
		t.Errorf("language-only query: got %v ids %v", urls, ids)
	}
}

// Search は production 未使用（D-08）。テスト参照のためテスト側へ移動。
// URL 構築は production の nozomiPathParts/nozomiPath を使う自己完結形（旧 nozomiURL は削除）
func (c *Client) Search(term, lang string, page int) ([]int, error) {
	if page < 1 {
		page = 1
	}
	area, tag, lang, err := nozomiPathParts(term, lang)
	if err != nil {
		return nil, err
	}
	full := "https://ltn." + hitomiDomain + nozomiPath(area, tag, lang)
	start := int64((page-1)*galleriesPerPage*4)
	body, err := c.get(context.Background(), full, [2]int64{start, start + int64(galleriesPerPage*4) - 1}, false)
	if err != nil {
		return nil, err
	}
	ids := []int{}
	for i := 0; i+4 <= len(body); i += 4 {
		ids = append(ids, int(binary.BigEndian.Uint32(body[i:i+4])))
	}
	return ids, nil
}

// D-10 追加（decision 2026-11-04-nozomi-dataset-name-space-form-correction・合格条件3）:
// app 構築 URL（nozomiPathParts/nozomiPath）で多語 tag の dataset が live で 200 を実測。
// 既存実測（space 200 / underscore 404・呼出し 4）と同一 URL 形。explicit live test mode
// （HITOMI_LIVE_TEST）で実行。gate は本番既定 1000ms（testGateIntervalMS 非 inject）を維持。
func TestNozomiURLSpaceFormLiveReturns200WhenLiveTestEnabled(t *testing.T) {
	if os.Getenv("HITOMI_LIVE_TEST") == "" {
		t.Skip("HITOMI_LIVE_TEST is not set (explicit live test mode)")
	}
	c := &Client{cfg: defaultHitomiConfig(), http: &http.Client{Timeout: 30 * time.Second}}
	calls := 0
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		req, err := http.NewRequestWithContext(ctx, "GET", rawurl, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("user-agent", c.cfg.UserAgent)
		req.Header.Set("referer", "https://hitomi.la/")
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		status := resp.StatusCode
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		t.Logf("live call %d: %s status %d bytes %d", calls, rawurl, status, len(body))
		if status != 200 {
			return nil, errors.New("live status " + strconv.Itoa(status))
		}
		return body, nil
	}
	ids, err := c.Search("female:sole female", "all", 1)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(ids) == 0 {
		t.Fatalf("live 200 dataset must decode ids: calls %d ids %v", calls, ids)
	}
}

// SearchQuery は production 未使用（D-08）。テスト参照のためテスト側へ移動
func (c *Client) SearchQuery(query, lang string, page int) ([]int, error) {
	if page < 1 {
		page = 1
	}
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return nil, errors.New("query has no term")
	}
	var ordered []int
	languageOnly := true
	for _, term := range terms {
		if strings.HasPrefix(strings.ToLower(term), "language:") {
			lang = strings.ToLower(strings.TrimPrefix(strings.ToLower(term), "language:"))
			continue
		}
		languageOnly = false
		ids, err := c.Search(term, lang, 1)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, nil
		}
		if ordered == nil {
			ordered = ids
			continue
		}
		set := map[int]bool{}
		for _, id := range ids {
			set[id] = true
		}
		next := []int{}
		for _, id := range ordered {
			if set[id] {
				next = append(next, id)
			}
		}
		ordered = next
		if len(ordered) == 0 {
			return nil, nil
		}
	}
	if languageOnly {
		return c.Search("language:"+lang, lang, page)
	}
	// 共通部分是 25 件/page の slice より後に適用される（nozomi の page は各 term の 1 page）
	start := (page - 1) * galleriesPerPage
	if start > len(ordered) {
		return nil, nil
	}
	end := start + galleriesPerPage
	if end > len(ordered) {
		end = len(ordered)
	}
	return ordered[start:end], nil
}

func TestStorageURLUsesFormatNotNameExt(t *testing.T) {
	// galleryinfo の name が 001.jpg でも shard 保存形式は webp（同じ hash の .jpg は 404 実測）
	// fixture の switch に sampleHash の値（2488）が無い。polarity は fixture 構造から読む（faithful parse）。
	// 構造 A（`var o = 0`・case → `o = 1`）: case 外 → m=0（`w1`）。構造 B（`var o = 1`・case → `o = 0`）: case 外 → m=1（`w2`）。
	ggA, err := parseGG("var gg = { b: '1793494802/', m: function (g) {\nvar o = 0;\nswitch (g) {\ncase 3:\n o = 1;\n break;\n}\nreturn o;\n}};")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://w1.gold-usergeneratedcontent.net/1793494802/2488/" + sampleHash + ".webp"
	if got := StorageURL(sampleHash, "webp", ggA); got != want {
		t.Errorf("storage url: got %s want %s", got, want)
	}
	if got := StorageURL(sampleHash, "", ggA); got != want {
		t.Errorf("default format: got %s", got)
	}
	if !strings.HasSuffix(StorageURL(sampleHash, "avif", ggA), ".avif") {
		t.Errorf("avif format: got %s", StorageURL(sampleHash, "avif", ggA))
	}
	// 構造差分で同じ hash の subdomain が w1 → w2 へ反転（format・base は不変）
	ggB, err := parseGG("var gg = { b: '1793494802/', m: function (g) {\nvar o = 1;\nswitch (g) {\ncase 3:\n o = 0;\n break;\n}\nreturn o;\n}};")
	if err != nil {
		t.Fatal(err)
	}
	wantB := "https://w2.gold-usergeneratedcontent.net/1793494802/2488/" + sampleHash + ".webp"
	if got := StorageURL(sampleHash, "webp", ggB); got != wantB {
		t.Errorf("polarity flip storage url: got %s want %s", got, wantB)
	}
	if got := StorageURL(sampleHash, "avif", ggB); got != "https://a2.gold-usergeneratedcontent.net/1793494802/2488/" + sampleHash + ".avif" {
		t.Errorf("polarity flip avif (letter a, digit 2): got %s", got)
	}
}

func TestTNVariantPathsMatchHitomiWorkPage(t *testing.T) {
	// live 実測構造では fixture の switch に sampleHash の値（2488）が無い → m=0（`atn`）
	gg, err := parseGG("var gg = { b: '1793494802/', m: function (g) {\nvar o = 0;\nswitch (g) {\ncase 3:\n o = 1;\n break;\n}\nreturn o;\n}};")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		variant string
		want    string
	}{
		{variant: "bigtn", want: "https://atn." + hitomiDomain + "/webpbigtn/9/b8/" + sampleHash + ".webp"},
		{variant: "smalltn", want: "https://atn." + hitomiDomain + "/webpsmalltn/9/b8/" + sampleHash + ".webp"},
		{variant: "smallsmalltn", want: "https://atn." + hitomiDomain + "/webpsmallsmalltn/9/b8/" + sampleHash + ".webp"},
		{variant: "smallbigtn", want: "https://atn." + hitomiDomain + "/webpsmallbigtn/9/b8/" + sampleHash + ".webp"},
	}
	for _, c := range cases {
		if got := ThumbnailURLVariant(sampleHash, "webp", c.variant, gg); got != c.want {
			t.Errorf("variant %q: got %s want %s", c.variant, got, c.want)
		}
	}

	// hitomi の TN に格納されるのは webp / avif ののみ。name の ext は TN path に使わない。
	if got := ThumbnailURLVariant(sampleHash, "png", "smalltn", gg); got != "https://atn." + hitomiDomain + "/webpsmalltn/9/b8/" + sampleHash + ".webp" {
		t.Errorf("name ext must not appear in TN path: got %s", got)
	}
	if got := ThumbnailURLVariant("21844c51814f950f39d448afca0c701a49c3c0cbd1b513ebf18e936ad9290065", "avif", "smalltn", gg); got != "https://atn." + hitomiDomain + "/avifsmalltn/5/06/21844c51814f950f39d448afca0c701a49c3c0cbd1b513ebf18e936ad9290065.avif" {
		t.Errorf("avif TN variant: got %s", got)
	}

	// 構造差分（`var o = 1`・case → `o = 0`）で TN subdomain が atn → btn へ反転。base・variant・path は不変。
	ggFlip, err := parseGG("var gg = { b: '1793494802/', m: function (g) {\nvar o = 1;\nswitch (g) {\ncase 3:\n o = 0;\n break;\n}\nreturn o;\n}};")
	if err != nil {
		t.Fatal(err)
	}
	if got := ThumbnailURLVariant(sampleHash, "webp", "bigtn", ggFlip); got != "https://btn." + hitomiDomain + "/webpbigtn/9/b8/" + sampleHash + ".webp" {
		t.Errorf("polarity flip TN subdomain: got %s", got)
	}
}

func TestTNBaseUsesExactFormatBytesNotTnName(t *testing.T) {
	// TN base は format で決まる（実測 200: webpbigtn / avifbigtn）。`746e626967746e` は存在せず 404。
	// subdomain は gg.m で `a`/`b` + `tn`（polarity は fixture 構造から読む）。hash の末尾は `(last)/(prev two)`。
	gg, err := parseGG("var gg = { b: '1793494802/', m: function (g) {\nvar o = 0;\nswitch (g) {\ncase 3:\n o = 1;\n break;\n}\nreturn o;\n}};") // 構造 A: case 外の sampleHash（2488）→ m=0（`atn`）
	if err != nil {
		t.Fatal(err)
	}
	hash := sampleHash // 末尾 9/b8

	want := "https://atn." + hitomiDomain + "/webpbigtn/9/b8/" + hash + ".webp"
	if got := ThumbnailURL(hash, "webp", gg); got != want {
		t.Errorf("tn webp: got %s want %s", got, want)
	}
	want = "https://atn." + hitomiDomain + "/avifbigtn/9/b8/" + hash + ".avif"
	if got := ThumbnailURL(hash, "avif", gg); got != want {
		t.Errorf("tn avif: got %s want %s", got, want)
	}
	if strings.Contains(ThumbnailURL(hash, "avif", gg), "tnbigtn") {
		t.Errorf("tn name must not appear as base: got %s", ThumbnailURL(hash, "avif", gg))
	}
	// TN に格納されるのは webp / avif ののみ（実測）なので、それ以外の format は webp に落ちる
	if got := ThumbnailURL(hash, "jpeg", gg); got != "https://atn." + hitomiDomain + "/webpbigtn/9/b8/" + hash + ".webp" {
		t.Errorf("non-TN format must fall back to webp: got %s", got)
	}
	// 構造差分で subdomain は反転するが base（format 由来）は反転しない
	ggFlip, err := parseGG("var gg = { b: '1793494802/', m: function (g) {\nvar o = 1;\nswitch (g) {\ncase 3:\n o = 0;\n break;\n}\nreturn o;\n}};")
	if err != nil {
		t.Fatal(err)
	}
	if got := ThumbnailURL(hash, "webp", ggFlip); got != "https://btn." + hitomiDomain + "/webpbigtn/9/b8/" + hash + ".webp" {
		t.Errorf("polarity flip TN subdomain: got %s", got)
	}
	if got := ThumbnailURL(hash, "avif", ggFlip); got != "https://btn." + hitomiDomain + "/avifbigtn/9/b8/" + hash + ".avif" {
		t.Errorf("polarity flip TN avif: got %s", got)
	}
}

func TestGalleryParse(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		if !strings.Contains(rawurl, "/galleries/4294614.js") {
			t.Errorf("url: got %s", rawurl)
		}
		return []byte(`var galleryinfo = {"id":"4294614","title":"Heroine Profile","language":"english","type":"doujinshi","tags":[{"tag":"corruption","female":"1","male":""}],"artists":[{"artist":"alfre-tori"}],"groups":null,"scene_indexes":[0,13,15],"files":[{"hash":"abc","name":"01_1.webp","width":1280,"height":1791,"hasavif":1}]}`), nil
	}
	info, err := c.Gallery(4294614, false)
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != 4294614 || info.Title != "Heroine Profile" {
		t.Errorf("info: got %+v", info)
	}
	if len(info.Tags) != 1 || info.Tags[0] != "female:corruption" {
		t.Errorf("tags: got %v", info.Tags)
	}
	if len(info.Artists) != 1 || info.Artists[0] != "alfre-tori" {
		t.Errorf("artists: got %v", info.Artists)
	}
	if len(info.Files) != 1 || info.Files[0].Hash != "abc" || info.Files[0].HasAVIF != 1 {
		t.Errorf("files: got %v", info.Files)
	}
	if len(info.SceneIndexes) != 3 || info.SceneIndexes[0] != 0 || info.SceneIndexes[1] != 13 {
		t.Errorf("scene_indexes: got %v", info.SceneIndexes)
	}

	// 旧形式 fixture（KI-20261105-06）: id・tags value が number。live 実測 296403 と同じ形
	//（false フラグは実測で "" のまま。number は 1 の字面のみ観測）
	c2 := &Client{cfg: defaultHitomiConfig()}
	c2.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		if !strings.Contains(rawurl, "/galleries/296403.js") {
			t.Errorf("url: got %s", rawurl)
		}
		return []byte(`var galleryinfo = {"id":296403,"title":"Sailor Moon - Secret Lingerin Scent","language":"polish","type":"doujinshi","tags":[{"tag":"futanari","female":1,"male":""}],"characters":[{"character":"sailor mars"}],"scene_indexes":[],"files":[{"hash":"78a2c688096c34e56680ec2eee30b4e3d20a15f054c8165312cae8e0374a8bfb","name":"01.jpg","width":517,"height":718,"hasavif":1}]}`), nil
	}
	num, err := c2.Gallery(296403, false)
	if err != nil {
		t.Fatal(err)
	}
	if num.ID != 296403 || num.Title != "Sailor Moon - Secret Lingerin Scent" {
		t.Errorf("numeric fixture: got %+v", num)
	}
	// names() 出力は string 形式と同一意味（female:1 → truthy → "female:<tag>"）
	if len(num.Tags) != 1 || num.Tags[0] != "female:futanari" {
		t.Errorf("numeric tags: got %v", num.Tags)
	}
	if len(num.Characters) != 1 || num.Characters[0] != "sailor mars" {
		t.Errorf("numeric characters: got %v", num.Characters)
	}
	if len(num.Files) != 1 || num.Files[0].HasAVIF != 1 {
		t.Errorf("numeric files: got %v", num.Files)
	}
}

func TestRateGatePerRequest(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.cfg.MinIntervalMS = 1
	calls := 0
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		return []byte{}, nil
	}
	for i := 0; i < 3; i++ {
		if _, err := c.get(context.Background(), "https://ltn."+hitomiDomain+"/x", [2]int64{0, 3}, false); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 3 {
		t.Errorf("calls: got %d", calls)
	}
	if c.dataGate.last.IsZero() {
		t.Error("gate did not record request time")
	}
}

// A-06: gate 中の ctx キャンセルで呼出が直ちに return すること。
// 本番 gate interval は re-anchor 済み（90 ms・decision 2026-11-06-review-findings-approval-decisions.md・100 ms の点のみ supersede）。mocked fetch
// （cancel 後は呼ばれない）で確認する。
func TestRateGateReturnsImmediatelyOnContextCancellation(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	if c.cfg.MinIntervalMS != 90 {
		t.Fatalf("production gate interval must remain 90ms (decision 2026-11-06-review-findings-approval-decisions.md): got %d", c.cfg.MinIntervalMS)
	}
	calls := 0
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		return []byte{}, nil
	}
	if _, err := c.get(context.Background(), "https://ltn."+hitomiDomain+"/x", [2]int64{0, 3}, false); err != nil {
		t.Fatal(err)
	}
	beforeCancel := calls
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, err := c.get(ctx, "https://ltn."+hitomiDomain+"/y", [2]int64{0, 3}, false)
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error: got %v", err)
	}
  if elapsed >= 90*time.Millisecond {
    t.Errorf("gate elapsed: got %s, expected immediate return despite 90ms interval", elapsed)
  }
	if calls != beforeCancel {
		t.Errorf("fetch calls after cancellation: got %d (expected no call beyond %d)", calls, beforeCancel)
	}
	if c.dataGate.last.IsZero() {
		t.Error("gate did not record request time")
	}
}

// A-06: retry backoff sleep 中の ctx キャンセルで呼出が直ちに return すること。
// backoff を長め（5000ms）に固定し、待機完了との区別をelapsedで証明する（本番既定 2000ms は変更しない）。
func TestRetryBackoffReturnsImmediatelyOnContextCancellation(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.cfg.MinIntervalMS = 1
	c.cfg.MaxRetries = 3
	c.cfg.RetryBackoffMS = 5000
	var requests int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requests, 1) == 1 {
			cancel()
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c.http = srv.Client()
	start := time.Now()
	_, err := c.fetchRemoteWithGate(ctx, srv.URL, [2]int64{-1, -1}, c.cfg.MinIntervalMS, &c.dataGate, false)
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error: got %v", err)
	}
	if elapsed >= time.Second {
		t.Errorf("backoff elapsed: got %s, expected immediate return despite 5000ms backoff", elapsed)
	}
	if requests != 1 {
		t.Errorf("requests after cancellation: got %d, retry must not continue", requests)
	}
}

func TestImageGateUsesLowerImageIntervalSeparately(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.cfg.MinIntervalMS = 1000
	c.cfg.ImageMinIntervalMS = 1
	calls := 0
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		return []byte{}, nil
	}
	if _, err := c.getImage(context.Background(), "https://btn."+hitomiDomain+"/x", [2]int64{0, 3}, false); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("image calls: got %d", calls)
	}
	if c.imageGate.last.IsZero() {
		t.Error("image gate did not record image request time")
	}
	if !c.dataGate.last.IsZero() {
		t.Error("image gate should not use normal gate last time")
	}
	if defaultHitomiConfig().ImageMinIntervalMS != 60 {
		t.Errorf("image gate default: got %d (decision 2026-11-06)", defaultHitomiConfig().ImageMinIntervalMS)
	}
}

// Phase 4 固定テスト（decision 2026-11-05・2レーン）。gate inject（C-10）で interval を
// 制御し、metrics kind 分離・A-06 ctx sleep 意味は変更しない。

// ①フォア待機（fgPending>0）中にバックは slot を予約しない（共有 last 連鎖の境界で
// 再チェックするだけ・未来予約禁止）。
func TestBackgroundGateDoesNotReserveSlotWhileForegroundPending(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.testGateIntervalMS = 200
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return []byte{}, nil
	}
	// fg #1: idle → 現时刻記録（last=now）。
	if _, err := c.get(context.Background(), "https://ltn."+hitomiDomain+"/x", [2]int64{0, 3}, false); err != nil {
		t.Fatal(err)
	}
	fgDone := make(chan struct{})
	go func() {
		// fg #2: last+interval 未来 slot を予約（sleep 中は fgPending=1）。
		_, _ = c.get(context.Background(), "https://ltn."+hitomiDomain+"/y", [2]int64{0, 3}, false)
		close(fgDone)
	}()
	if c.dataGate.fgPending == 0 {
		deadline := time.Now().Add(100 * time.Millisecond)
		for c.dataGate.fgPending == 0 && time.Now().Before(deadline) {
			time.Sleep(2 * time.Millisecond)
		}
	}
	if c.dataGate.fgPending == 0 {
		t.Fatal("fg pending window not observed")
	}
	lastBefore := c.dataGate.last
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	err := c.gateLane(ctx, c.cfg.MinIntervalMS, &c.dataGate, true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("background gate must not reserve while fg pending, must return on cancel: got %v", err)
	}
	if !c.dataGate.lastBg.IsZero() {
		t.Error("background gate reserved while foreground pending (must not reserve)")
	}
	if !c.dataGate.last.Equal(lastBefore) {
		t.Error("background gate advanced shared last chain while fg pending")
	}
	if time.Since(start) >= time.Duration(c.testGateIntervalMS)*time.Millisecond {
		t.Errorf("bg elapsed during fg pending must be immediate on cancel: got %s", time.Since(start))
	}
	<-fgDone
}

// ②フォア不在時、バックは現行 interval で進む（飢餓なし・interval 遵守・未来予約禁止）。
func TestBackgroundGateAdvancesAtCurrentIntervalWhenNoForeground(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.testGateIntervalMS = 100
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return []byte{}, nil
	}
	if defaultHitomiConfig().BgGraceMS != 300 {
		t.Errorf("bg grace default: got %d", defaultHitomiConfig().BgGraceMS)
	}
	ctx := context.Background()
	start := time.Now()
	if err := c.gateLane(ctx, c.cfg.MinIntervalMS, &c.dataGate, true); err != nil {
		t.Fatal(err)
	}
	if c.dataGate.lastBg.IsZero() || time.Since(start) > 60*time.Millisecond {
		t.Errorf("idle bg must reserve now: elapsed %s lastBg %v", time.Since(start), c.dataGate.lastBg)
	}
	start2 := time.Now()
	if err := c.gateLane(ctx, c.cfg.MinIntervalMS, &c.dataGate, true); err != nil {
		t.Fatal(err)
	}
	if elapsed2 := time.Since(start2); elapsed2 < 70*time.Millisecond || elapsed2 > 160*time.Millisecond {
		t.Errorf("bg must advance at current interval only: elapsed %s", elapsed2)
	}
	// 飢餓防止（公平枠）: フォア長期待機（3×interval 継続）で interval 条件を満たす
	// 時点で即予約可。将来予約はしない。
	c.dataGate.fgPending = 1
	c.dataGate.bgWaitSince = time.Now().Add(-4 * time.Duration(c.testGateIntervalMS) * time.Millisecond)
	c.dataGate.last = time.Now().Add(-2 * time.Duration(c.testGateIntervalMS) * time.Millisecond)
	start3 := time.Now()
	if err := c.gateLane(ctx, c.cfg.MinIntervalMS, &c.dataGate, true); err != nil {
		t.Fatal(err)
	}
	if elapsed3 := time.Since(start3); elapsed3 > 60*time.Millisecond {
		t.Errorf("starved bg must advance at interval boundary (fairness slot): elapsed %s", elapsed3)
	}
	c.dataGate.fgPending = 0
}

// ③バックの ctx キャンセルは即 return（A-06 意味）。
func TestBackgroundGateReturnsImmediatelyOnContextCancellation(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.testGateIntervalMS = 200
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return []byte{}, nil
	}
	ctx := context.Background()
	if err := c.gateLane(ctx, c.cfg.MinIntervalMS, &c.dataGate, true); err != nil {
		t.Fatal(err)
	}
	firstLastBg := c.dataGate.lastBg
	ctx2, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	err := c.gateLane(ctx2, c.cfg.MinIntervalMS, &c.dataGate, true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error: got %v", err)
	}
	if time.Since(start) >= time.Duration(c.testGateIntervalMS)*time.Millisecond {
		t.Errorf("bg cancel elapsed: got %s, expected immediate return despite 200ms interval", time.Since(start))
	}
	if !c.dataGate.lastBg.Equal(firstLastBg) {
		t.Error("cancelled bg must not reserve")
	}
}

// ④バックは grace（既定 300ms・lastFg+grace<=now）を尊重する。
func TestBackgroundGateRespectsGraceAfterForegroundRecord(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.testGateIntervalMS = 100
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return []byte{}, nil
	}
	ctx := context.Background()
	if err := c.gateLane(ctx, c.cfg.MinIntervalMS, &c.dataGate, false); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := c.gateLane(ctx, c.cfg.MinIntervalMS, &c.dataGate, true); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond || elapsed > 450*time.Millisecond {
		t.Errorf("bg must wait lastFg+grace(300ms): elapsed %s", elapsed)
	}
}

func TestDownloadFilesParallelWritesResultsInOriginalOrder(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.cfg.MinIntervalMS = 1000
	c.cfg.ImageMinIntervalMS = 1
	// R-16: download 書き込みは shared core（magic 照合）を通る。fixture は webp magic bytes。
	payload := webpMagicBytes("x")
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return payload, nil
	}

	target := t.TempDir()
	entries := []FileEntry{{Hash: "a"}, {Hash: "b"}, {Hash: "c"}}
	results := downloadFilesParallel(c, target, entries, "webp", 4, nil, nil)
	if len(results) != 3 {
		t.Fatalf("results: got %d", len(results))
	}
	for i, result := range results {
		if result.err != "" {
			t.Fatalf("result %d error: %s", i, result.err)
		}
		if result.skipped {
			t.Errorf("result %d must not be skipped without recorded size: got %v", i, result)
		}
		if result.page != i+1 {
			t.Errorf("result %d page: got %d", i, result.page)
		}
		if filepath.Base(result.path) != entries[i].Hash+".webp" {
			t.Errorf("result %d path: got %q", i, result.path)
		}
		data, err := os.ReadFile(result.path)
		if err != nil || string(data) != string(payload) {
			t.Errorf("result %d file: err %v data %q", i, err, string(data))
		}
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dataDir = dataRoot()
	db, err := openDB(filepath.Join(t.TempDir(), "hitomi.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureSchema(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func recordTestFileSize(db *sql.DB, rel string, galleryID sql.NullInt64, size int64) error {
	return recordFile(db, rel, galleryID, "test-sha", size, "webp", "webp", "not_evaluated", "structural")
}

func TestDownloadFilesParallelSkipsExistingFileWhenRecordedSizeMatches(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	calls := 0
	// R-16: webp magic fixture（shared core の magic 照合を通る）。
	payload1 := webpMagicBytes("x")
	content := payload1
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		return content, nil
	}
	db := openTestDB(t)
	if err := upsertGallery(db, &GalleryInfo{ID: 125, Title: "t", Language: "all"}, "Galleries/125"); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	target := filepath.Join(root, galleryDir(root, 125))
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	entries := []FileEntry{{Hash: "a"}}

	// 1 run: no recorded size yet -> fetch and write. scan 相当の記録後、size 一致。
	results := downloadFilesParallel(c, target, entries, "webp", 1, nil, nil)
	if len(results) != 1 || results[0].skipped || results[0].err != "" {
		t.Fatalf("run 1: got %v", results)
	}
	if calls != 1 {
		t.Fatalf("run 1 image calls: got %d", calls)
	}
	if err := recordTestFileSize(db, filepath.Join(galleryDir(root, 125), "a.webp"), sql.NullInt64{Int64: 125, Valid: true}, int64(len(payload1))); err != nil {
		t.Fatal(err)
	}
	content = webpMagicBytes("z") // 2 run で fetch したら内容が変わる。skip なら内容保持のはず。

	results = downloadFilesParallel(c, target, entries, "webp", 1, recordedSizes(db, root, 125), nil)
	if len(results) != 1 || !results[0].skipped || results[0].err != "" {
		t.Fatalf("run 2 must report skipped: got %v", results)
	}
	if calls != 1 {
		t.Errorf("run 2 must not call remote fetch: calls %d", calls)
	}
	data, err := os.ReadFile(results[0].path)
	if err != nil || string(data) != string(payload1) {
		t.Errorf("skipped file must keep original content: err %v data %q", err, string(data))
	}
}

func TestDownloadFilesParallelRefetchesOnSizeMismatchMissingOrUnrecorded(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	calls := 0
	payloadZ := webpMagicBytes("z") // R-16: shared core の magic 照合を通る fixture。
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		return payloadZ, nil
	}
	db := openTestDB(t)
	if err := upsertGallery(db, &GalleryInfo{ID: 126, Title: "t", Language: "all"}, "Galleries/126"); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	target := filepath.Join(root, galleryDir(root, 126))
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	galleryID := sql.NullInt64{Int64: 126, Valid: true}
	entries := []FileEntry{{Hash: "match"}, {Hash: "mismatch"}, {Hash: "missing"}, {Hash: "unrecorded"}}

	// size 一致・size 不一致・DB 記録あるが欠損・DB 記録なしの既存ファイル
	for _, hash := range []string{"match", "mismatch", "unrecorded"} {
		if err := os.WriteFile(filepath.Join(target, hash+".webp"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(target, "mismatch.webp"), []byte("xxx"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, hash := range []string{"match", "mismatch", "missing"} {
		if err := recordTestFileSize(db, filepath.Join(galleryDir(root, 126), hash+".webp"), galleryID, 1); err != nil {
			t.Fatal(err)
		}
	}

	results := downloadFilesParallel(c, target, entries, "webp", 1, recordedSizes(db, root, 126), nil)
	if len(results) != 4 {
		t.Fatalf("results: got %d", len(results))
	}
	if !results[0].skipped {
		t.Errorf("match must be skipped: got %v", results[0])
	}
	for i, result := range results[1:] {
		if result.skipped {
			t.Errorf("result %d must fetch: got %v", i+1, result)
		}
		if result.err != "" {
			t.Errorf("result %d error: %s", i+1, result.err)
		}
	}
	if calls != 3 {
		t.Errorf("remote fetch calls: got %d want 3", calls)
	}
	for _, hash := range []string{"mismatch", "missing", "unrecorded"} {
		data, err := os.ReadFile(filepath.Join(target, hash+".webp"))
		if err != nil || string(data) != string(payloadZ) {
			t.Errorf("%s must be re-fetched: err %v data %q", hash, err, string(data))
		}
	}
	// R-16: size 不一致の既存 file は rename 上書きで self-repair（existing skip は core 側にない）。
	// target folder に `.tmp` 残りがあってはならない。
	assertNoCacheTempResidue(t, target)
}

// R-16 download 書込点の失敗注入。合格条件: write / close / rename 失敗で完成フォルダに
// 壊れた target が残らない・target フォルダ内に `.tmp` 残りが消える（正典 correction の失敗時テスト要求を
// download 書込点へ拡張）。seam は shared core の test seam（download は nil seam でも動く）。
func TestDownloadFilesParallelAtomicFailureLeavesNoBrokenTarget(t *testing.T) {
	for _, stage := range []string{"write", "close", "rename"} {
		c := &Client{cfg: defaultHitomiConfig()}
		c.cfg.MinIntervalMS = 1000
		c.cfg.ImageMinIntervalMS = 1
		c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
			return webpMagicBytes("p"), nil
		}
		seam := &atomicWriteSeam{}
		injected := errors.New("inject " + stage)
		hook := func(path string) error { return injected }
		switch stage {
		case "write":
			seam.cacheWriteFailureHook = hook
		case "close":
			seam.cacheCloseFailureHook = hook
		case "rename":
			seam.cacheRenameFailureHook = hook
		}
		target := t.TempDir()
		results := downloadFilesParallel(c, target, []FileEntry{{Hash: "a"}}, "webp", 1, nil, seam)
		if len(results) != 1 || results[0].err == "" {
			t.Fatalf("%s failure must return error: %v", stage, results)
		}
		if results[0].err != injected.Error() {
			t.Errorf("%s result error: got %q want %q", stage, results[0].err, injected.Error())
		}
		path := filepath.Join(target, "a.webp")
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s failure must not create partial target: %s", stage, path)
		}
		assertNoCacheTempResidue(t, target)
		if seam.cacheWriteAttemptCount != 1 || seam.cacheRenameSuccessCount != 0 {
			t.Errorf("%s failure counts: attempt %d rename %d", stage, seam.cacheWriteAttemptCount, seam.cacheRenameSuccessCount)
		}
	}
}

// R-16: core の magic 照合は download 書込点にも適用（scan 側 magic_mismatch 検出と整合）。
// 不一致バイトは error として result に記録され、target を書かない・`.tmp` 残りが消える。
func TestDownloadFilesParallelMagicMismatchErrorLeavesNoTarget(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.cfg.MinIntervalMS = 1000
	c.cfg.ImageMinIntervalMS = 1
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return []byte("not an image"), nil
	}
	target := t.TempDir()
	results := downloadFilesParallel(c, target, []FileEntry{{Hash: "a"}}, "webp", 1, nil, nil)
	if len(results) != 1 || results[0].err == "" || !strings.Contains(results[0].err, "magic mismatch") {
		t.Fatalf("magic mismatch must be reported as error: %v", results)
	}
	path := filepath.Join(target, "a.webp")
	if _, err := os.Stat(path); err == nil {
		t.Errorf("magic mismatch must not write target: %s", path)
	}
	assertNoCacheTempResidue(t, target)
}

// A-10 実測（明示テストモード）。docs/ai/decisions/2026-11-01-hitomi-hub-tests-may-call-external-api.md に
// 従い、HITOMI_HUB_LIVE=1 のときだけ実在 gallery を呼ぶ。書き込みは docs/ai/tmp/ scratch 内のみ。
// 未設定時は skip。shard data は live gg.js の窓（約1時間）に在る分のみ解けるため、窓Membership
// 漏れ・polarity drift で run 1 が 404 の場合は skip（合格条件の live 固定は窓 data が解ける時のみ）。
func TestDownloadFilesParallelLiveSecondRunSkipsExistingFiles(t *testing.T) {
	if os.Getenv("HITOMI_HUB_LIVE") == "" {
		t.Skip("explicit test mode: set HITOMI_HUB_LIVE=1")
	}
	dataDir = dataRoot()
	scratch := filepath.Join(dataRoot(), "docs", "ai", "tmp", "assess-20261103-1220", "a10-live")
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := openDB(filepath.Join(scratch, "hitomi.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := ensureSchema(db); err != nil {
		t.Fatal(err)
	}
	cfg := Config{GalleryDir: "Galleries"} // VT 無効: 外部呼出しは hitomi のみ
	client := &Client{http: &http.Client{Timeout: 60 * time.Second}, cfg: defaultHitomiConfig()}
	warnGG(loadGG(client))
	info, err := client.Gallery(4294614, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Files) < 2 {
		t.Fatalf("live gallery too small: %d files", len(info.Files))
	}
	target := filepath.Join(scratch, galleryDir(scratch, info.ID))
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := upsertGallery(db, info, galleryDir(scratch, info.ID)); err != nil {
		t.Fatal(err)
	}

	run1 := downloadFilesParallel(client, target, info.Files[:2], "webp", 4, nil, nil)
	written1 := 0
	summary := map[string]int{}
	state := &RateState{}
	for _, result := range run1 {
		if result.err != "" {
			if strings.Contains(result.err, "status 404") {
				t.Skipf("run 1 remote fetch 404: shard window membership / gg switch polarity drift (see docs/ai/tasks/2026-11-03-a10-download-existing-check.md); raw: %s", result.err)
			}
			t.Fatalf("run 1 error: %s", result.err)
		}
		if result.skipped {
			t.Fatalf("run 1 must not skip: got %v", result)
		}
		written1++
		if err := recordPage(db, info.ID, result.page, result.hash); err != nil {
			t.Fatal(err)
		}
		if status := processFile(db, cfg, state, scratch, result.path, summary, nil); status == "" {
			t.Fatalf("run 1 processFile failed: %s", result.path)
		}
	}
	calls1 := liveDownloadFetchCalls(client)
	if len(run1) != 2 || written1 != len(run1) || calls1 != int64(len(run1)) {
		t.Fatalf("run 1 evidence: attempted %d written %d download fetch calls %d", len(run1), written1, calls1)
	}

	run2 := downloadFilesParallel(client, target, info.Files[:2], "webp", 4, recordedSizes(db, scratch, info.ID), nil)
	skipped2 := 0
	for _, result := range run2 {
		if result.err != "" {
			t.Fatalf("run 2 error: %s", result.err)
		}
		if !result.skipped {
			t.Fatalf("run 2 must skip existing recorded file: got %v", result)
		}
		skipped2++
	}
	calls2 := liveDownloadFetchCalls(client) - calls1
	if skipped2 != len(run2) || calls2 != 0 {
		t.Fatalf("run 2 evidence: skipped %d download fetch calls %d", skipped2, calls2)
	}
	if tnCalls := liveTNFetchCalls(client); tnCalls != 0 {
		t.Fatalf("download must not use TN as source: TN calls %d", tnCalls)
	}
	t.Logf("A-10 live evidence: run1 attempted=%d written=%d download fetch calls=%d, run2 attempted=%d skipped=%d download fetch calls=%d, TN calls=%d, metrics=%v",
		len(run1), written1, calls1, len(run2), skipped2, calls2, liveTNFetchCalls(client), client.metricsSnapshot())
}

// download の shard full-image fetch は classifyFetchURL の "data" kind に計上される（shard URL は
// TN path ではない）。TN（"image" kind）は download の表示源ではないので 0 のまま。
func liveDownloadFetchCalls(c *Client) int64 {
	metric, ok := c.metricsSnapshot()["data"].(map[string]int64)
	if !ok {
		return 0
	}
	return metric["calls"]
}

func liveTNFetchCalls(c *Client) int64 {
	metric, ok := c.metricsSnapshot()["image"].(map[string]int64)
	if !ok {
		return 0
	}
	return metric["calls"]
}

func TestReadStateFavoriteAndQuery(t *testing.T) {
	db := openTestDB(t)
	if err := upsertGallery(db, &GalleryInfo{ID: 125, Title: "t", Language: "english"}, "Galleries/125"); err != nil {
		t.Fatal(err)
	}
	if err := setReadState(db, 125, 3, false); err != nil {
		t.Fatal(err)
	}
	if err := setFavorite(db, 125, true); err != nil {
		t.Fatal(err)
	}
	rows, err := listRows(db, "favorites")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["gallery_id"] != 125 || rows[0]["favorite"] != 1 || rows[0]["last_page"] != 3 {
		t.Errorf("favorites: got %v", rows)
	}
	if err := setFavorite(db, 125, false); err != nil {
		t.Fatal(err)
	}
	rows, err = listRows(db, "favorites")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("unfav: got %v", rows)
	}
	unread, err := listRows(db, "unread")
	if len(unread) != 1 || unread[0]["fully_read"] != 0 {
		t.Errorf("unread: got %v", unread)
	}
	if err := addQuery(db, "color", "tag:full_color", 0); err != nil {
		t.Fatal(err)
	}
	queries, err := listRows(db, "queries")
	if len(queries) != 1 || queries[0]["name"] != "color" {
		t.Errorf("queries: got %v", queries)
	}
	if err := markQueryRun(db, queries[0]["id"].(int)); err != nil {
		t.Fatal(err)
	}
	queries, err = listRows(db, "queries")
	if err != nil {
		t.Fatal(err)
	}
	if queries[0]["last_run_at"] == "" {
		t.Error("last_run_at not set")
	}
}

// 項目2（assess-20261105-1015・block 検出・Retry-After 尊重・クールダウン）。
// Go テストは mock/httptest を使う（外部 API 呼出しなし）。既存テストリテラルは data `90`、image `60`、
// politeness退避 `1000` を固定し、metrics kind 分離・FIFO・共有 last 連鎖・A-06 意味は変更しない。

func TestBlockSignal403CFMitigatedRetriedWithRetryAfterRespected(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.testGateIntervalMS = 1
	var requests int32
	var firstAt time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requests, 1) == 1 {
			firstAt = time.Now()
			w.Header().Set("cf-mitigated", "challenge")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(403)
			return
		}
		gap := time.Since(firstAt)
		if gap < 1000*time.Millisecond || gap >= 1500*time.Millisecond {
			t.Errorf("Retry-After gap between attempts: got %s (expected ~1000 ms・not static 2000 ms backoff)", gap)
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c.http = srv.Client()
	body, err := c.fetchRemoteWithGate(context.Background(), srv.URL, [2]int64{-1, -1}, c.cfg.MinIntervalMS, &c.dataGate, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body after retry: got %q", string(body))
	}
	if requests != 2 {
		t.Fatalf("requests: got %d (block must be retried)", requests)
	}
	metric, ok := c.metricsSnapshot()["data"].(map[string]int64)
	if !ok {
		t.Fatalf("metrics data missing: %v", c.metricsSnapshot())
	}
	if metric["retry_sleep_ms"] != 1000 {
		t.Errorf("retry_sleep_ms: got %d (expected 1000 from Retry-After, not 2000 static)", metric["retry_sleep_ms"])
	}
	if c.blockCooldownUntil.IsZero() {
		t.Fatal("block signal did not engage cooldown")
	}
	d := c.blockCooldownUntil.Sub(firstAt)
	if d < blockCooldownDuration-100*time.Millisecond || d > blockCooldownDuration+100*time.Millisecond {
		t.Errorf("cooldown duration: got %s (expected blockCooldownDuration %s・canonical convention negativeNoFullCacheTTL)", d, blockCooldownDuration)
	}
}

func TestBlockStatusesRetriedWithExistingBackoffAndEngageCooldown(t *testing.T) {
	for _, status := range []int{403, 503, 522} {
		c := &Client{cfg: defaultHitomiConfig()}
		c.testGateIntervalMS = 1
		c.cfg.MaxRetries = 1
		c.cfg.RetryBackoffMS = 20
		var requests int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt32(&requests, 1) == 1 {
				w.WriteHeader(status)
				return
			}
			w.Write([]byte("ok"))
		}))
		c.http = srv.Client()
		body, err := c.fetchRemoteWithGate(context.Background(), srv.URL, [2]int64{-1, -1}, c.cfg.MinIntervalMS, &c.dataGate, false)
		srv.Close()
		if err != nil {
			t.Fatalf("status %d: %v", status, err)
		}
		if string(body) != "ok" {
			t.Errorf("status %d body: got %q", status, string(body))
		}
		if requests != 2 {
			t.Errorf("status %d requests: got %d (block must be retried・403 was immediate return before item 2)", status, requests)
		}
		metric, ok := c.metricsSnapshot()["data"].(map[string]int64)
		if !ok {
			t.Fatalf("status %d metrics missing: %v", status, c.metricsSnapshot())
		}
		if metric["retry_sleep_ms"] != 20 {
			t.Errorf("status %d retry_sleep_ms: got %d (expected existing backoff formula 20 ms・no Retry-After)", status, metric["retry_sleep_ms"])
		}
		if c.blockCooldownUntil.IsZero() || !c.blockCooldownUntil.After(time.Now()) {
			t.Errorf("status %d: cooldown not engaged", status)
		}
	}
}

func Test429WithoutRetryAfterKeepsExistingBackoffAndNoCooldown(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.testGateIntervalMS = 1
	c.cfg.MaxRetries = 1
	c.cfg.RetryBackoffMS = 100
	var requests int32
	var firstAt time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requests, 1) == 1 {
			firstAt = time.Now()
			w.WriteHeader(429)
			return
		}
		gap := time.Since(firstAt)
		if gap < 100*time.Millisecond || gap >= 200*time.Millisecond {
			t.Errorf("429 static backoff gap: got %s (expected ~100 ms existing formula)", gap)
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c.http = srv.Client()
	_, err := c.fetchRemoteWithGate(context.Background(), srv.URL, [2]int64{-1, -1}, c.cfg.MinIntervalMS, &c.dataGate, false)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests: got %d", requests)
	}
	metric, ok := c.metricsSnapshot()["data"].(map[string]int64)
	if !ok {
		t.Fatalf("metrics data missing: %v", c.metricsSnapshot())
	}
	if metric["retry_sleep_ms"] != 100 {
		t.Errorf("retry_sleep_ms: got %d (expected existing 100 ms formula)", metric["retry_sleep_ms"])
	}
	if !c.blockCooldownUntil.IsZero() {
		t.Errorf("429 alone must not engage block cooldown: got %v", c.blockCooldownUntil)
	}
}

func TestParseRetryAfterRespectsValueAndCapsAtCanonicalConvention(t *testing.T) {
	if blockCooldownDuration != negativeNoFullCacheTTL {
		t.Fatal("cooldown duration must remain the existing canonical convention (no invented number)")
	}
	if politenessDefaultIntervalMS != 1000 {
		t.Errorf("politeness retreat target must remain the decision value 1000 (2026-11-05): got %d", politenessDefaultIntervalMS)
	}
	if politenessDefaultIntervalMS <= defaultHitomiConfig().MinIntervalMS {
		t.Errorf("politeness retreat must exceed canonical data lane default (retreat semantics): got %d vs %d", politenessDefaultIntervalMS, defaultHitomiConfig().MinIntervalMS)
	}
	cases := []struct {
		value   string
		want    time.Duration
		ok      bool
	}{
		{"1", 1 * time.Second, true},
		{"300", 300 * time.Second, true},
		{"600", blockCooldownDuration, true}, // unreasonable over cap → clamp to 5 min
		{"-5", 0, false},
		{"not-a-number", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseRetryAfter(tc.value)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("parseRetryAfter %q: got (%v, %v), want (%v, %v)", tc.value, got, ok, tc.want, tc.ok)
		}
	}
	future := time.Now().UTC().Add(2 * time.Second).Format(http.TimeFormat)
	d, ok := parseRetryAfter(future)
	if !ok || d < 0 || d > 2*time.Second {
		t.Errorf("parseRetryAfter HTTP-date %q: got (%v, %v), want 0..2 s", future, d, ok)
	}
	past := time.Now().UTC().Add(-10 * time.Second).Format(http.TimeFormat)
	d, ok = parseRetryAfter(past)
	if !ok || d != 0 {
		t.Errorf("parseRetryAfter past HTTP-date %q: got (%v, %v), want (0, true)", past, d, ok)
	}
}

func TestBlockCooldownRetreatsBothLanesToPolitenessDefaultAndRestoresOnExpiry(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.testGateIntervalMS = 1
	c.cfg.MaxRetries = 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403) // block signal（status のみ・cf-mitigated なしでも block）
	}))
	defer srv.Close()
	c.http = srv.Client()
	if _, err := c.fetchRemoteWithGate(context.Background(), srv.URL, [2]int64{-1, -1}, c.cfg.MinIntervalMS, &c.dataGate, false); err == nil {
		t.Fatal("block with MaxRetries 0 must return error")
	}
	if c.blockCooldownUntil.IsZero() || !c.blockCooldownUntil.After(time.Now()) {
		t.Fatal("cooldown not engaged after block")
	}
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return []byte{}, nil
	}
	// data lane: block 呼出しの gate が last を記録済み。次呼出しは politeness 既定 1000 ms
	// の枠まで待機する（退避なしなら inject 1 ms で即時）。
	start := time.Now()
	if _, err := c.get(context.Background(), "https://ltn."+hitomiDomain+"/x", [2]int64{0, 3}, false); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 950*time.Millisecond || elapsed > 1300*time.Millisecond {
		t.Errorf("data lane gate wait during cooldown: got %s (expected ~1000 ms retreat, not inject 1 ms)", elapsed)
	}
	// image lane（既定 250 ms）: FIFO 意味を変えない（lane 初呼出しは即時・last 記録）。
	// warmup 後の次呼出し wait を実測（250 ms なら ~250 ms・退避なら ~1000 ms）。
	if _, err := c.getImage(context.Background(), "https://btn."+hitomiDomain+"/x", [2]int64{0, 3}, false); err != nil {
		t.Fatal(err) // warmup（lane 初呼出し・即時・last 記録）
	}
	start = time.Now()
	if _, err := c.getImage(context.Background(), "https://btn."+hitomiDomain+"/x", [2]int64{0, 3}, false); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 950*time.Millisecond || elapsed > 1300*time.Millisecond {
		t.Errorf("image lane gate wait during cooldown: got %s (expected ~1000 ms retreat, not 250 ms)", elapsed)
	}
	// 満了後は現行 interval（test inject 1 ms）へ復帰
	c.blockCooldownUntil = time.Now().Add(-1 * time.Second)
	start = time.Now()
	if _, err := c.getImage(context.Background(), "https://btn."+hitomiDomain+"/y", [2]int64{0, 3}, false); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed >= 200*time.Millisecond {
		t.Errorf("after cooldown expiry: got %s (expected immediate・restore to current interval)", elapsed)
	}
}

func TestClientRecordsFetchMetricsForMockFetch(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig(), fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		if classifyFetchURL(rawurl) != "version" {
			t.Errorf("classifyFetchURL version: got %s", classifyFetchURL(rawurl))
		}
		return []byte("x"), nil
	}}
	_, err := c.get(context.Background(), "https://gold-usergeneratedcontent.net/common/version", [2]int64{-1, -1}, false)
	if err != nil {
		t.Fatal(err)
	}
	metrics := c.metricsSnapshot()
	metric, ok := metrics["version"].(map[string]int64)
	if !ok {
		t.Fatalf("metrics version missing: got %v", metrics)
	}
	if metric["calls"] != 1 {
		t.Errorf("metrics version calls: got %v", metric)
	}
}

// R-01 回帰テスト: 同一 key の上書き insert は old entry の entrySize を引いてから
// new を加算する（正味 accounting）。docs/ai/tasks/2026-11-06-r01-bounded-cache-put-overwrite-double-count.md。
func TestBoundedCachePutOverwriteReplacesOldEntryAccounting(t *testing.T) {
	cache := map[string]*searchCacheEntry{}
	total := 0
	base := time.Now()

	boundedCachePut(cache, &total, 10, 1000, "a", &searchCacheEntry{ids: []int{1, 2, 3, 4, 5}, fetchedAt: base, lastAccess: base})
	if total != 40 || len(cache) != 1 {
		t.Fatalf("baseline wrong: total=%d len=%d", total, len(cache))
	}

	boundedCachePut(cache, &total, 10, 1000, "a", &searchCacheEntry{ids: []int{6}, fetchedAt: base.Add(time.Second), lastAccess: base.Add(time.Second)})
	if total != 8 || len(cache) != 1 {
		t.Errorf("same-key overwrite must replace accounting: total=%d (want 8) len=%d", total, len(cache))
	}
}

func TestBoundedCachePutOverwriteDoesNotPrematurelyEvictWithinCap(t *testing.T) {
	cache := map[string]*searchCacheEntry{}
	total := 0
	base := time.Now()

	boundedCachePut(cache, &total, 10, 48, "a", &searchCacheEntry{ids: []int{1, 2, 3, 4, 5}, fetchedAt: base, lastAccess: base})
	boundedCachePut(cache, &total, 10, 48, "b", &searchCacheEntry{ids: []int{6}, fetchedAt: base, lastAccess: base})
	if len(cache) != 2 || total != 48 {
		t.Fatalf("baseline wrong: total=%d len=%d", total, len(cache))
	}

	boundedCachePut(cache, &total, 10, 48, "a", &searchCacheEntry{ids: []int{7}, fetchedAt: base.Add(time.Second), lastAccess: base.Add(time.Second)})
	if _, ok := cache["a"]; !ok {
		t.Error("overwritten key must survive")
	}
	if _, ok := cache["b"]; !ok {
		t.Errorf("survivor within true-bytes cap must not be evicted by inflated accounting: total=%d len=%d", total, len(cache))
	}
	if total != 16 || len(cache) != 2 {
		t.Errorf("net accounting after overwrite: total=%d (want 16) len=%d (want 2)", total, len(cache))
	}
}

func TestSearchNozomiPartsTTLRefetchOverwriteKeepsNetAccounting(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig(), searchCacheMaxEntries: 10, searchCacheMaxBytes: 1000, testGateIntervalMS: 1}
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return beIDs([]uint32{10, 11, 12, 13, 14}), nil
	}
	state := &searchState{area: "tag", tag: "female:yuri", language: "all"}
	if _, err := c.SearchNozomiParts(state); err != nil {
		t.Fatal(err)
	}
	if c.searchCacheBytes != 40 {
		t.Fatalf("baseline wrong: bytes=%d", c.searchCacheBytes)
	}

	// TTL miss は entry を map から消さない。refetch は同一 key 上書き put になる。
	c.searchCache["tag|female:yuri|all"].fetchedAt = time.Now().Add(-2 * time.Hour)
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return beIDs([]uint32{20}), nil
	}
	if _, err := c.SearchNozomiParts(state); err != nil {
		t.Fatal(err)
	}
	if c.searchCacheBytes != 8 || len(c.searchCache) != 1 {
		t.Errorf("TTL-miss refetch overwrite must keep net accounting: bytes=%d (want 8) len=%d", c.searchCacheBytes, len(c.searchCache))
	}
}
