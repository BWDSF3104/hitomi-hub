package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
			_, _ = c.ensureIndexVersion("galleries", false)
		}()
	}
	wg.Wait()
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
	info, err := c.Gallery(4294614)
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
		if _, err := c.get(context.Background(), "https://ltn."+hitomiDomain+"/x", [2]int64{0, 3}); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 3 {
		t.Errorf("calls: got %d", calls)
	}
	if c.last.IsZero() {
		t.Error("gate did not record request time")
	}
}

// A-06: gate 中の ctx キャンセルで呼出が直ちに return すること。
// 本番 gate interval（1000ms）は変更せず、mocked fetch（cancel 後は呼ばれない）で確認する。
func TestRateGateReturnsImmediatelyOnContextCancellation(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	if c.cfg.MinIntervalMS != 1000 {
		t.Fatalf("production gate interval must remain 1000ms: got %d", c.cfg.MinIntervalMS)
	}
	calls := 0
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		return []byte{}, nil
	}
	if _, err := c.get(context.Background(), "https://ltn."+hitomiDomain+"/x", [2]int64{0, 3}); err != nil {
		t.Fatal(err)
	}
	beforeCancel := calls
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, err := c.get(ctx, "https://ltn."+hitomiDomain+"/y", [2]int64{0, 3})
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error: got %v", err)
	}
	if elapsed >= 250*time.Millisecond {
		t.Errorf("gate elapsed: got %s, expected immediate return despite 1000ms interval", elapsed)
	}
	if calls != beforeCancel {
		t.Errorf("fetch calls after cancellation: got %d (expected no call beyond %d)", calls, beforeCancel)
	}
	if c.last.IsZero() {
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
	_, err := c.fetchRemoteWithGate(ctx, srv.URL, [2]int64{-1, -1}, c.cfg.MinIntervalMS, &c.last)
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
	if _, err := c.getImage(context.Background(), "https://btn."+hitomiDomain+"/x", [2]int64{0, 3}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("image calls: got %d", calls)
	}
	if c.imageLast.IsZero() {
		t.Error("image gate did not record image request time")
	}
	if !c.last.IsZero() {
		t.Error("image gate should not use normal gate last time")
	}
	if defaultHitomiConfig().ImageMinIntervalMS != 250 {
		t.Errorf("image gate default: got %d", defaultHitomiConfig().ImageMinIntervalMS)
	}
}

func TestDownloadFilesParallelWritesResultsInOriginalOrder(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.cfg.MinIntervalMS = 1000
	c.cfg.ImageMinIntervalMS = 1
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return []byte("x"), nil
	}

	target := t.TempDir()
	entries := []FileEntry{{Hash: "a"}, {Hash: "b"}, {Hash: "c"}}
	results := downloadFilesParallel(c, target, entries, "webp", 4, nil)
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
		if err != nil || string(data) != "x" {
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
	content := "x"
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		return []byte(content), nil
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
	results := downloadFilesParallel(c, target, entries, "webp", 1, nil)
	if len(results) != 1 || results[0].skipped || results[0].err != "" {
		t.Fatalf("run 1: got %v", results)
	}
	if calls != 1 {
		t.Fatalf("run 1 image calls: got %d", calls)
	}
	if err := recordTestFileSize(db, filepath.Join(galleryDir(root, 125), "a.webp"), sql.NullInt64{Int64: 125, Valid: true}, 1); err != nil {
		t.Fatal(err)
	}
	content = "z" // 2 run で fetch したら内容が変わる。skip なら内容保持のはず。

	results = downloadFilesParallel(c, target, entries, "webp", 1, recordedSizes(db, root, 125))
	if len(results) != 1 || !results[0].skipped || results[0].err != "" {
		t.Fatalf("run 2 must report skipped: got %v", results)
	}
	if calls != 1 {
		t.Errorf("run 2 must not call remote fetch: calls %d", calls)
	}
	data, err := os.ReadFile(results[0].path)
	if err != nil || string(data) != "x" {
		t.Errorf("skipped file must keep original content: err %v data %q", err, string(data))
	}
}

func TestDownloadFilesParallelRefetchesOnSizeMismatchMissingOrUnrecorded(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	calls := 0
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		return []byte("z"), nil
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

	results := downloadFilesParallel(c, target, entries, "webp", 1, recordedSizes(db, root, 126))
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
		if err != nil || string(data) != "z" {
			t.Errorf("%s must be re-fetched: err %v data %q", hash, err, string(data))
		}
	}
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
	info, err := client.Gallery(4294614)
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

	run1 := downloadFilesParallel(client, target, info.Files[:2], "webp", 4, nil)
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

	run2 := downloadFilesParallel(client, target, info.Files[:2], "webp", 4, recordedSizes(db, scratch, info.ID))
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

func TestClientRecordsFetchMetricsForMockFetch(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig(), fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		if classifyFetchURL(rawurl) != "version" {
			t.Errorf("classifyFetchURL version: got %s", classifyFetchURL(rawurl))
		}
		return []byte("x"), nil
	}}
	_, err := c.get(context.Background(), "https://gold-usergeneratedcontent.net/common/version", [2]int64{-1, -1})
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
