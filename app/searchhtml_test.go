package main

import (
	"context"
	"strings"
	"testing"
)

func TestSearchNozomiPartsUsesTTLCacheAndCopies(t *testing.T) {
	fetches := 0
	c := &Client{cfg: defaultHitomiConfig(), fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		if strings.Contains(rawurl, "/n/tag/female:yuri-all.nozomi") {
			return beIDs([]uint32{10, 20, 30}), nil
		}
		return nil, nil
	}}

	ids1, err := c.SearchNozomiParts("tag", "female:yuri", "all")
	if err != nil {
		t.Fatalf("SearchNozomiParts: %s", err)
	}
	if len(ids1) != 3 || ids1[0] != 10 || ids1[1] != 20 || ids1[2] != 30 {
		t.Fatalf("first cache result: %v", ids1)
	}

	ids2, err := c.SearchNozomiParts("tag", "female:yuri", "all")
	if err != nil {
		t.Fatalf("SearchNozomiParts cache: %s", err)
	}
	if fetches != 1 {
		t.Fatalf("TTL cache should not refetch within TTL: fetches %d", fetches)
	}
	if len(ids2) != 3 || ids2[0] != 10 || ids2[1] != 20 || ids2[2] != 30 {
		t.Fatalf("second cache result: %v", ids2)
	}

	ids1[0] = 99
	ids3, err := c.SearchNozomiParts("tag", "female:yuri", "all")
	if err != nil {
		t.Fatalf("SearchNozomiParts after mutation: %s", err)
	}
	if len(ids3) != 3 || ids3[0] != 10 || ids3[1] != 20 || ids3[2] != 30 {
		t.Fatalf("cache must not be mutated by caller slice: %v", ids3)
	}
}

func TestSearchHTMLPlainTermUsesIndexExactTerm(t *testing.T) {
	key := hashTerm("furry")
	leaf := beNode(1, [][]byte{key}, [][2]int64{{1000, 24}}, make([]uint64, btreeOrder+1))
	data := append([]byte{0, 0, 0, 3}, beIDs([]uint32{10, 20, 30})...)

	c := &Client{cfg: defaultHitomiConfig(), fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		switch {
		case strings.HasSuffix(rawurl, "/version"):
			return []byte("1793467632"), nil
		case strings.HasSuffix(rawurl, ".index"):
			return leaf, nil
		default:
			return data, nil
		}
	}}

	ids, total, err := c.SearchHTML("FURRY", "all", 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(ids) != 2 || ids[0] != 10 || ids[1] != 20 {
		t.Fatalf("plain term: total %d ids %v", total, ids)
	}
}

func TestSearchHTMLPreservesUnderscoreTagTerm(t *testing.T) {
	var urls []string
	c := &Client{cfg: defaultHitomiConfig(), fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		urls = append(urls, rawurl)
		if strings.Contains(rawurl, "/n/tag/female:big_breasts-all.nozomi") {
			return beIDs([]uint32{10, 20}), nil
		}
		return nil, nil
	}}

	ids, total, err := c.SearchHTML("female:big_breasts", "all", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(ids) != 2 || ids[0] != 10 || ids[1] != 20 {
		t.Fatalf("underscore term: total %d ids %v", total, ids)
	}
	if len(urls) != 1 || !strings.Contains(urls[0], "/n/tag/female:big_breasts-all.nozomi") {
		t.Fatalf("underscore term must not split: got %v", urls)
	}
}

func TestSearchHTMLColonTermUsesNozomiState(t *testing.T) {
	var urls []string
	c := &Client{cfg: defaultHitomiConfig(), fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		urls = append(urls, rawurl)
		switch {
		case strings.Contains(rawurl, "/n/tag/female:yuri-all.nozomi"):
			return beIDs([]uint32{10, 20, 30}), nil
		case strings.Contains(rawurl, "/n/tag/female:yuri-english.nozomi"):
			return beIDs([]uint32{20, 30, 99}), nil
		default:
			return nil, nil
		}
	}}

	ids, total, err := c.SearchHTML("female:yuri language:english", "all", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(ids) != 2 || ids[0] != 20 || ids[1] != 30 {
		t.Fatalf("nozomi state: total %d ids %v", total, ids)
	}
	if len(urls) != 2 || !strings.Contains(urls[0], "/n/tag/female:yuri-all.nozomi") || !strings.Contains(urls[1], "/n/tag/female:yuri-english.nozomi") {
		t.Fatalf("paths: got %v", urls)
	}
}

func TestSearchHTMLLanguageOnlyUsesNozomiIndex(t *testing.T) {
	var urls []string
	c := &Client{cfg: defaultHitomiConfig(), fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		urls = append(urls, rawurl)
		if strings.Contains(rawurl, "/n/index-japanese.nozomi") {
			return beIDs([]uint32{7, 8, 9}), nil
		}
		return nil, nil
	}}
	ids, total, err := c.SearchHTML("language:japanese", "all", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(ids) != 3 || ids[0] != 7 {
		t.Fatalf("language-only: total %d ids %v", total, ids)
	}
	if len(urls) != 1 || !strings.Contains(urls[0], "/n/index-japanese.nozomi") {
		t.Fatalf("paths: got %v", urls)
	}
}

func TestSearchHTMLNegativeTermExcludes(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig(), fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		switch rawurl {
		case "https://ltn." + hitomiDomain + "/n/tag/female:yuri-all.nozomi":
			return beIDs([]uint32{10, 20, 30}), nil
		case "https://ltn." + hitomiDomain + "/n/tag/female:futanari-all.nozomi":
			return beIDs([]uint32{20}), nil
		default:
			return nil, nil
		}
	}}
	ids, total, err := c.SearchHTML("female:yuri -female:futanari", "all", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(ids) != 2 || ids[0] != 10 || ids[1] != 30 {
		t.Fatalf("negative: total %d ids %v", total, ids)
	}
}

func TestSearchHTMLPreservesOrOnlyUnionWithoutDuplicateGroup(t *testing.T) {
	keyA := hashTerm("a")
	keyB := hashTerm("b")
	slots := []struct {
		key    []byte
		offset int64
	}{
		{keyA, 1000},
		{keyB, 1008},
	}
	if compareKey(slots[0].key, slots[1].key) > 0 {
		slots = []struct {
			key    []byte
			offset int64
		}{
			{keyB, 1008},
			{keyA, 1000},
		}
	}
	leaf := beNode(2, [][]byte{slots[0].key, slots[1].key},
		[][2]int64{{slots[0].offset, 12}, {slots[1].offset, 16}}, make([]uint64, btreeOrder+1))
	dataA := append([]byte{0, 0, 0, 2}, beIDs([]uint32{1, 2})...)
	dataB := append([]byte{0, 0, 0, 3}, beIDs([]uint32{2, 3, 4})...)

	fetches := 0
	c := &Client{cfg: defaultHitomiConfig(), fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		switch {
		case strings.HasSuffix(rawurl, "/version"):
			return []byte("1793467632"), nil
		case strings.HasSuffix(rawurl, ".index"):
			return leaf, nil
		case rng[0] == 1000:
			return dataA, nil
		case rng[0] == 1008:
			return dataB, nil
		default:
			return nil, nil
		}
	}}

	ids, total, err := c.SearchHTML("a or b", "all", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || len(ids) != 4 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 || ids[3] != 4 {
		t.Fatalf("or-only union: total %d ids %v", total, ids)
	}
	if fetches != 4 {
		t.Fatalf("or-only should not refetch initial group: fetches %d", fetches)
	}
}

func TestApplySearchOrderDoesNotDoubleReverseAsc(t *testing.T) {
	state := &searchState{orderbydir: "asc"}
	ids := []int{10, 20, 30}
	applyOrderForInitial(ids, state)
	applySearchOrder(ids, state)
	if len(ids) != 3 || ids[0] != 30 || ids[1] != 20 || ids[2] != 10 {
		t.Fatalf("asc order should reverse only once, got %v", ids)
	}
}

func TestThumbnailURLMatchesVerifiedTNPath(t *testing.T) {
	// TN base は format で決まる（実測 200: `77656270626967746e` / `61766966626967746e`）。
	// `746e626967746e` は base として存在せず 404。subdomain は `a`/`b` + `tn`（gg.m で決まる）。
	// bucket/switch values と polarity（初期 o・case 代入）は窓ごとに rotation するので fixture で規則を検証する（正典: `tools/gen_ggset.py`）
	gg, err := parseGG("var gg = { b: '1793494802/', m: function (g) {\nvar o = 0;\nswitch (g) {\ncase 3:\n o = 1;\n break;\n}\nreturn o;\n}};") // 構造 A（11-03 11:38Z 窓）: case 外の sampleHash（2488）→ m=0（`atn`）
	if err != nil {
		t.Fatal(err)
	}
	got := ThumbnailURL(sampleHash, "webp", gg)
	want := "https://atn." + hitomiDomain + "/webpbigtn/9/b8/" + sampleHash + ".webp"
	if got != want {
		t.Fatalf("thumbnail: got %s want %s", got, want)
	}
	got = ThumbnailURL(sampleHash, "avif", gg)
	want = "https://atn." + hitomiDomain + "/avifbigtn/9/b8/" + sampleHash + ".avif"
	if got != want {
		t.Fatalf("avif thumbnail: got %s want %s", got, want)
	}
	// TN に格納されるのは webp / avif ののみ（実測）なので、それ以外の format は webp に落ちる
	got = ThumbnailURL(sampleHash, "jpeg", gg)
	if got != "https://atn." + hitomiDomain + "/webpbigtn/9/b8/" + sampleHash + ".webp" {
		t.Fatalf("non-TN format must fall back to webp: got %s", got)
	}
	// 構造 B（11-03 12:00 窓: `var o = 1`・case → `o = 0`）で subdomain が反転（base・format は不変）
	ggFlip, err := parseGG("var gg = { b: '1793494802/', m: function (g) {\nvar o = 1;\nswitch (g) {\ncase 3:\n o = 0;\n break;\n}\nreturn o;\n}};")
	if err != nil {
		t.Fatal(err)
	}
	if got := ThumbnailURL(sampleHash, "webp", ggFlip); got != "https://btn." + hitomiDomain + "/webpbigtn/9/b8/" + sampleHash + ".webp" {
		t.Fatalf("polarity flip TN subdomain: got %s", got)
	}
	if fileExt("001.avif") != "avif" || fileExt("001.jpeg") != "jpg" || fileExt("001") != "webp" {
		t.Fatalf("fileExt: %q %q %q", fileExt("001.avif"), fileExt("001.jpeg"), fileExt("001"))
	}
}
