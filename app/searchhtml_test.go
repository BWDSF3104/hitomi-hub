package main

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"
)

// fileExt は production 未使用（D-08）。テスト参照のためテスト側へ移動。
// galleryinfo `name` の ext。tn path の format には使わない
func fileExt(name string) string {
	i := strings.LastIndex(name, ".")
	if i < 0 || i+1 >= len(name) {
		return "webp"
	}
	ext := strings.ToLower(name[i+1:])
	if ext == "jpeg" {
		ext = "jpg"
	}
	if ext == "" {
		return "webp"
	}
	return ext
}

func TestSearchNozomiPartsUsesTTLCacheAndCopies(t *testing.T) {
	fetches := 0
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		if strings.Contains(rawurl, "/n/tag/female:yuri-all.nozomi") {
			return beIDs([]uint32{10, 20, 30}), nil
		}
		return nil, nil
	}}

	ids1, err := c.SearchNozomiParts(&searchState{area: "tag", tag: "female:yuri", language: "all"})
	if err != nil {
		t.Fatalf("SearchNozomiParts: %s", err)
	}
	if len(ids1) != 3 || ids1[0] != 10 || ids1[1] != 20 || ids1[2] != 30 {
		t.Fatalf("first cache result: %v", ids1)
	}

	ids2, err := c.SearchNozomiParts(&searchState{area: "tag", tag: "female:yuri", language: "all"})
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
	ids3, err := c.SearchNozomiParts(&searchState{area: "tag", tag: "female:yuri", language: "all"})
	if err != nil {
		t.Fatalf("SearchNozomiParts after mutation: %s", err)
	}
	if len(ids3) != 3 || ids3[0] != 10 || ids3[1] != 20 || ids3[2] != 30 {
		t.Fatalf("cache must not be mutated by caller slice: %v", ids3)
	}
}

func TestSearchCacheEvictsLRUEntriesWhenOverEntryLimit(t *testing.T) {
	fetches := 0
	c := &Client{cfg: defaultHitomiConfig(), searchCacheMaxEntries: 2, searchCacheMaxBytes: 1024, testGateIntervalMS: 1}
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		switch {
		case strings.Contains(rawurl, "female:yuri"):
			return beIDs([]uint32{10}), nil
		case strings.Contains(rawurl, "male:yuri"):
			return beIDs([]uint32{20}), nil
		case strings.Contains(rawurl, "female:cosplay"):
			return beIDs([]uint32{30}), nil
		case strings.Contains(rawurl, "female:lolita"):
			return beIDs([]uint32{40}), nil
		}
		return nil, nil
	}

	// Windows clock granularity can tie same-instant timestamps. Fix access order
	// explicitly after each insert so LRU eviction order is deterministic.
	base := time.Now()
	if _, err := c.SearchNozomiParts(&searchState{area: "tag", tag: "female:yuri", language: "all"}); err != nil {
		t.Fatal(err)
	}
	c.searchCache["tag|female:yuri|all"].lastAccess = base.Add(-30 * time.Second)
	if _, err := c.SearchNozomiParts(&searchState{area: "tag", tag: "male:yuri", language: "all"}); err != nil {
		t.Fatal(err)
	}
	c.searchCache["tag|male:yuri|all"].lastAccess = base.Add(-20 * time.Second)
	if _, err := c.SearchNozomiParts(&searchState{area: "tag", tag: "female:cosplay", language: "all"}); err != nil {
		t.Fatal(err)
	}
	c.searchCache["tag|female:cosplay|all"].lastAccess = base.Add(-10 * time.Second)
	if len(c.searchCache) != 2 {
		t.Fatalf("entry limit 2 should evict oldest entry: len %d", len(c.searchCache))
	}
	if _, ok := c.searchCache["tag|female:yuri|all"]; ok {
		t.Error("oldest entry should be evicted when entry limit is crossed")
	}
	if _, ok := c.searchCache["tag|male:yuri|all"]; !ok {
		t.Error("second entry should remain")
	}
	if _, ok := c.searchCache["tag|female:cosplay|all"]; !ok {
		t.Error("newest entry should remain")
	}
	if c.searchCacheBytes > c.searchCacheMaxBytes {
		t.Errorf("search cache memory must stay below cap: bytes %d", c.searchCacheBytes)
	}

	// LRU: hit updates access order. Insert D must evict C (stale access), not B (accessed).
	if _, err := c.SearchNozomiParts(&searchState{area: "tag", tag: "male:yuri", language: "all"}); err != nil {
		t.Fatal(err)
	}
	if fetches != 3 {
		t.Fatalf("TTL hit must not refetch: fetches %d", fetches)
	}
	if _, err := c.SearchNozomiParts(&searchState{area: "tag", tag: "female:lolita", language: "all"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.searchCache["tag|female:cosplay|all"]; ok {
		t.Error("stale-access entry must be evicted by LRU order")
	}
	if _, ok := c.searchCache["tag|male:yuri|all"]; !ok {
		t.Error("recently accessed entry must survive LRU eviction")
	}

	// Evicted key is not a TTL hit anymore: it refetches.
	if _, err := c.SearchNozomiParts(&searchState{area: "tag", tag: "female:yuri", language: "all"}); err != nil {
		t.Fatal(err)
	}
	if fetches != 5 {
		t.Errorf("evicted key must refetch, memory stays bounded: fetches %d bytes %d", fetches, c.searchCacheBytes)
	}
	if len(c.searchCache) > 2 || c.searchCacheBytes > c.searchCacheMaxBytes {
		t.Errorf("memory must stay below cap after refetch insert: len %d bytes %d", len(c.searchCache), c.searchCacheBytes)
	}
}

func TestSearchCacheSizeCapBoundsMemory(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig(), searchCacheMaxEntries: 10, searchCacheMaxBytes: 40, testGateIntervalMS: 1}
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		switch {
		case strings.Contains(rawurl, "female:yuri"):
			return beIDs([]uint32{10, 11, 12, 13, 14}), nil
		case strings.Contains(rawurl, "male:yuri"):
			return beIDs([]uint32{20, 21, 22, 23, 24}), nil
		default:
			return beIDs([]uint32{30, 31, 32, 33, 34}), nil
		}
	}

	// Windows clock granularity can tie same-instant timestamps and make LRU
	// eviction order nondeterministic. Fix access order explicitly after each
	// insert so the eviction target no longer depends on a tie. The size cap
	// itself (each entry 5 ids * 8 = 40 bytes against cap 40) and which entry
	// survives are unchanged; the assertion below still verifies cap bounds memory.
	base := time.Now()
	if _, err := c.SearchNozomiParts(&searchState{area: "tag", tag: "female:yuri", language: "all"}); err != nil {
		t.Fatal(err)
	}
	c.searchCache["tag|female:yuri|all"].lastAccess = base.Add(-30 * time.Second)
	if _, err := c.SearchNozomiParts(&searchState{area: "tag", tag: "male:yuri", language: "all"}); err != nil {
		t.Fatal(err)
	}
	c.searchCache["tag|male:yuri|all"].lastAccess = base.Add(-20 * time.Second)
	if _, err := c.SearchNozomiParts(&searchState{area: "tag", tag: "female:cosplay", language: "all"}); err != nil {
		t.Fatal(err)
	}
	c.searchCache["tag|female:cosplay|all"].lastAccess = base.Add(-10 * time.Second)
	if c.searchCacheBytes > c.searchCacheMaxBytes {
		t.Fatalf("size cap must keep memory below cap: bytes %d", c.searchCacheBytes)
	}
	if len(c.searchCache) != 1 {
		t.Fatalf("size cap should evict oldest entries: len %d", len(c.searchCache))
	}
	if _, ok := c.searchCache["tag|female:cosplay|all"]; !ok {
		t.Error("newest entry should remain within size cap")
	}
}

func TestSearchHTMLPlainTermUsesIndexExactTerm(t *testing.T) {
	key := hashTerm("furry")
	leaf := beNode(1, [][]byte{key}, [][2]int64{{1000, 24}}, make([]uint64, btreeOrder+1))
	data := append([]byte{0, 0, 0, 3}, beIDs([]uint32{10, 20, 30})...)

	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
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

func TestSearchHTMLPlainTermIgnoresLangPerCanonicalHitomi(t *testing.T) {
	// A-04 固定（hitomi 正典確認 2026-11-04）: search.js `get_galleryids_for_query` plain term 分岐は
	// key = hash_term(term) の B-tree 降下だけで state.language を使わない。galleriesindex に言語キーは
	// なく（live 実測: `furry` 32901件中 26949 件が index-english 外）、lang は `language:` term の
	// dataset としてのみ結果へ入る。plain term + lang は言語条件なし集合になる挙動を固定する。
	key := hashTerm("furry")
	leaf := beNode(1, [][]byte{key}, [][2]int64{{1000, 24}}, make([]uint64, btreeOrder+1))
	data := append([]byte{0, 0, 0, 3}, beIDs([]uint32{10, 20, 30})...)

	var urls []string
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		urls = append(urls, rawurl)
		switch {
		case strings.HasSuffix(rawurl, "/version"):
			return []byte("1793467632"), nil
		case strings.HasSuffix(rawurl, ".index"):
			return leaf, nil
		default:
			return data, nil
		}
	}}

	ids, total, err := c.SearchHTML("furry", "english", 1, 25)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(ids) != 3 || ids[0] != 10 || ids[1] != 20 || ids[2] != 30 {
		t.Fatalf("plain term must not be language-filtered: total %d ids %v", total, ids)
	}
	for _, u := range urls {
		if strings.Contains(u, ".nozomi") || strings.Contains(u, "/n/") {
			t.Fatalf("plain term must not fetch language dataset: %v", urls)
		}
		if !strings.Contains(u, "galleriesindex") {
			t.Fatalf("plain term must descend galleriesindex only: %v", urls)
		}
	}
}

func TestSearchHTMLPlainTermUnderscoreUsesSpaceFormKeyPerCanonicalHitomi(t *testing.T) {
	// D-10（正典確認 2026-11-04・実測）: hitomi results.js:30（`term = term.replace(/_/g, ' ')`）と
	// search.js:653（`get_galleryids_for_query` plain term 分岐で key = hash_term(query) は 674 行）は
	// `_` を space へ直してから hash する。live galleriesindex（version 1793765075）は `big breasts` の
	// space 形 key 92ef4857（1 件）を持つが underscore 形 key 95e737a6 は不在 → index key は space 型。
	// アプリ現挙動（normalizeSearchTerm）は正典一致。挙動変更なしでこの対応をテストで固定する。
	spaceKey := hashTerm("big breasts")
	underscoreKey := hashTerm("big_breasts")
	if string(spaceKey) == string(underscoreKey) {
		t.Fatal("fixture must distinguish the two key forms")
	}
	slots := []struct {
		key    []byte
		offset int64
		ids    []uint32
	}{
		{spaceKey, 1000, []uint32{10, 20, 30}},
		{hashTerm("big"), 2000, []uint32{10, 20, 30, 40}},
		{hashTerm("breasts"), 3000, []uint32{20, 30, 50}},
	}
	sort.Slice(slots, func(i, j int) bool { return compareKey(slots[i].key, slots[j].key) < 0 })

	keys := [][]byte{}
	datas := [][2]int64{}
	byOffset := map[int64][]uint32{}
	for _, s := range slots {
		keys = append(keys, s.key)
		datas = append(datas, [2]int64{s.offset, int64(4 + 4*len(s.ids))})
		byOffset[s.offset] = s.ids
	}
	leaf := beNode(len(slots), keys, datas, make([]uint64, btreeOrder+1))

	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		switch {
		case strings.HasSuffix(rawurl, "/version"):
			return []byte("1793765075"), nil
		case strings.HasSuffix(rawurl, ".index"):
			return leaf, nil
		default:
			ids := byOffset[rng[0]]
			out := []byte{byte(len(ids) >> 24), byte(len(ids) >> 16), byte(len(ids) >> 8), byte(len(ids))}
			return append(out, beIDs(ids)...), nil
		}
	}}

	if got := normalizeSearchTerm("big_breasts"); got != "big breasts" {
		t.Fatalf("results.js:30 normalize must map underscore to space: %q", got)
	}

	// `_` を含む 1 term は space 形 key で降下する。underscore 形 key は live index に不在（実測）なので、
	// 正典とちがえる実装なら total 0 になる。
	ids, total, err := c.SearchHTML("big_breasts", "all", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(ids) != 3 || ids[0] != 10 || ids[1] != 20 || ids[2] != 30 {
		t.Fatalf("underscore term must descend the space-form key: total %d ids %v", total, ids)
	}

	// results.js:17 の whitespace split は `_` 置換と別工程。`big breasts` は 2 term の lookup（共通部分）
	// で、`big_breasts`（1 term）と同じ phrase でも集合が違う（live 実測: 473712 vs 1）。
	ids2, total2, err := c.SearchHTML("big breasts", "all", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total2 != 2 || len(ids2) != 2 || ids2[0] != 20 || ids2[1] != 30 {
		t.Fatalf("whitespace terms must stay separate lookups: total %d ids %v", total2, ids2)
	}
}

// supersede（decision 2026-11-04-nozomi-dataset-name-space-form-correction）: 旧固定
// `TestSearchHTMLPreservesUnderscoreTagTerm`（underscore 形 URL）は正典矛盾。正典は colon
// split 前に `_` → space（results.js:30・search.js:653）で dataset name は space 形。
func TestSearchHTMLColonTagTermUsesCanonicalSpaceFormURL(t *testing.T) {
	var urls []string
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		urls = append(urls, rawurl)
		if strings.Contains(rawurl, "/n/tag/female:big%20breasts-all.nozomi") {
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
	if len(urls) != 1 || !strings.Contains(urls[0], "/n/tag/female:big%20breasts-all.nozomi") {
		t.Fatalf("colon term must build canonical space-form URL (no space→_ conversion): got %v", urls)
	}
}

func TestSearchHTMLColonTermUsesNozomiState(t *testing.T) {
	// C-04 fixed test: `language:` は同じ tag を language 違いで 2 回 fetch しない。
	// state.language は最初の term fetch へ反映され、female:yuri の fetch は 1 回
	//（female:yuri-english）になる。fixture は実 hitomi data の規則（language-scoped
	// dataset は all dataset の subset）に従う。
	var urls []string
	fetches := 0
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		urls = append(urls, rawurl)
		switch {
		case strings.Contains(rawurl, "/n/tag/female:yuri-all.nozomi"):
			return beIDs([]uint32{10, 20, 30}), nil
		case strings.Contains(rawurl, "/n/tag/female:yuri-english.nozomi"):
			return beIDs([]uint32{20, 30}), nil
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
	if fetches != 1 || len(urls) != 1 || !strings.Contains(urls[0], "/n/tag/female:yuri-english.nozomi") {
		t.Fatalf("same tag must fetch once with language reflected: fetches %d paths %v", fetches, urls)
	}
}

func TestSearchHTMLLanguageOnlyUsesNozomiIndex(t *testing.T) {
	var urls []string
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
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

func TestSearchHTMLLanguageTermIntersectionRegression(t *testing.T) {
	var urls []string
	fetches := 0
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		urls = append(urls, rawurl)
		switch {
		case strings.Contains(rawurl, "/n/tag/female:yuri-english.nozomi"):
			return beIDs([]uint32{10, 20, 30}), nil
		case strings.Contains(rawurl, "/n/tag/female:lolita-english.nozomi"):
			return beIDs([]uint32{20, 40}), nil
		default:
			return nil, nil
		}
	}}

	ids, total, err := c.SearchHTML("female:yuri language:english female:lolita", "all", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(ids) != 1 || ids[0] != 20 {
		t.Fatalf("intersection with language: total %d ids %v", total, ids)
	}
	if fetches != 2 || len(urls) != 2 {
		t.Fatalf("intersection must fetch each term once: fetches %d paths %v", fetches, urls)
	}
	for _, rawurl := range urls {
		if strings.Contains(rawurl, "-all.nozomi") {
			t.Fatalf("language must be reflected in term fetch, no all-language refetch: %v", urls)
		}
	}
}

func TestSearchHTMLNegativeTermExcludes(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
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

func TestSearchHTMLNegativeTermExcludesWithLanguageRegression(t *testing.T) {
	var urls []string
	fetches := 0
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		urls = append(urls, rawurl)
		switch {
		case strings.Contains(rawurl, "/n/tag/female:yuri-english.nozomi"):
			return beIDs([]uint32{10, 20, 30}), nil
		case strings.Contains(rawurl, "/n/tag/female:futanari-english.nozomi"):
			return beIDs([]uint32{20}), nil
		default:
			return nil, nil
		}
	}}

	ids, total, err := c.SearchHTML("female:yuri -female:futanari language:english", "all", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(ids) != 2 || ids[0] != 10 || ids[1] != 30 {
		t.Fatalf("negative with language: total %d ids %v", total, ids)
	}
	if fetches != 2 || len(urls) != 2 {
		t.Fatalf("negative must fetch each term once: fetches %d paths %v", fetches, urls)
	}
	for _, rawurl := range urls {
		if strings.Contains(rawurl, "-all.nozomi") {
			t.Fatalf("language must be reflected in term fetch, no all-language refetch: %v", urls)
		}
	}
}

func TestSearchHTMLNegativeLanguageTermExcludesItsOwnLanguageDataset(t *testing.T) {
	var urls []string
	fetches := 0
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		urls = append(urls, rawurl)
		switch {
		case strings.Contains(rawurl, "/n/tag/female:yuri-all.nozomi"):
			return beIDs([]uint32{10, 20, 30}), nil
		case strings.Contains(rawurl, "/n/index-english.nozomi"):
			return beIDs([]uint32{20}), nil
		default:
			return nil, nil
		}
	}}

	ids, total, err := c.SearchHTML("female:yuri -language:english", "all", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(ids) != 2 || ids[0] != 10 || ids[1] != 30 {
		t.Fatalf("negative language term: total %d ids %v", total, ids)
	}
	if fetches != 2 || len(urls) != 2 || !strings.Contains(urls[1], "/n/index-english.nozomi") {
		t.Fatalf("negative language term must contribute its own index dataset, not refetch same tag: fetches %d paths %v", fetches, urls)
	}
}

func TestSearchHTMLOrGroupLanguageTermContributesOwnLanguageDataset(t *testing.T) {
	var urls []string
	fetches := 0
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		urls = append(urls, rawurl)
		switch {
		case strings.Contains(rawurl, "/n/tag/female:yuri-all.nozomi"):
			return beIDs([]uint32{10, 20}), nil
		case strings.Contains(rawurl, "/n/index-english.nozomi"):
			return beIDs([]uint32{20, 30}), nil
		default:
			return nil, nil
		}
	}}

	ids, total, err := c.SearchHTML("female:yuri or language:english", "all", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(ids) != 3 || ids[0] != 10 || ids[1] != 20 || ids[2] != 30 {
		t.Fatalf("or group with language term: total %d ids %v", total, ids)
	}
	if fetches != 2 || len(urls) != 2 || !strings.Contains(urls[1], "/n/index-english.nozomi") {
		t.Fatalf("or group language term must fetch its own index dataset once: fetches %d paths %v", fetches, urls)
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
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
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

func TestSearchHTMLPopularOrderFetchesOrderDatasetPerCanonicalHitomi(t *testing.T) {
	// D-09（正典 mirror・実測 2026-11-04）: hitomi の正典 sort は client 側ではなく
	// `nozomi_address_from_state`（search.js 612-624）の dataset 選択。`orderby:popular` の既定 key は
	// results.js 89-95 で 'year'。colon term は order dataset を fetch し、その順序が集合順序になる
	// （`order_results` は方向のみ・desc は reverse なし）。live 実測: 200・date dataset と順序不同
	// （docs/ai/tmp/assess-20261103-1220/d09_live_observation.txt）。
	var urls []string
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		urls = append(urls, rawurl)
		if strings.Contains(rawurl, "/n/tag/popular/year/female:yuri-all.nozomi") {
			return beIDs([]uint32{5, 10, 15}), nil
		}
		return nil, nil
	}}

	ids, total, err := c.SearchHTML("female:yuri orderby:popular", "all", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 1 || !strings.Contains(urls[0], "/n/tag/popular/year/female:yuri-all.nozomi") {
		t.Fatalf("orderby:popular must fetch order-keyed dataset: %v", urls)
	}
	if total != 3 || ids[0] != 5 || ids[1] != 10 || ids[2] != 15 {
		t.Fatalf("order dataset order must be preserved: total %d ids %v", total, ids)
	}
}

func TestSearchHTMLPlainTermOrderStartsFromOrderDatasetPerCanonicalHitomi(t *testing.T) {
	// D-09: results.js 114 — 最初の positive term が plain で orderbykey != 'added'（popular 既定 'year'）
	// のとき集合順序の起点は order dataset（area 'all'）。term membership は後段 intersection で保持。
	key := hashTerm("furry")
	leaf := beNode(1, [][]byte{key}, [][2]int64{{1000, 24}}, make([]uint64, btreeOrder+1))
	data := append([]byte{0, 0, 0, 3}, beIDs([]uint32{10, 20, 30})...)

	var urls []string
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		urls = append(urls, rawurl)
		switch {
		case strings.Contains(rawurl, "/n/popular/year-all.nozomi"):
			return beIDs([]uint32{30, 10, 40, 20}), nil
		case strings.HasSuffix(rawurl, "/version"):
			return []byte("1793467632"), nil
		case strings.HasSuffix(rawurl, ".index"):
			return leaf, nil
		case strings.HasSuffix(rawurl, ".data"):
			return data, nil
		default:
			return nil, nil
		}
	}}

	ids, total, err := c.SearchHTML("furry orderby:popular", "all", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(ids) != 3 || ids[0] != 30 || ids[1] != 10 || ids[2] != 20 {
		t.Fatalf("plain term order must start from order dataset: total %d ids %v", total, ids)
	}
	orderFetch := false
	for _, u := range urls {
		if strings.Contains(u, "/n/popular/year-all.nozomi") {
			orderFetch = true
		}
		if strings.Contains(u, "/n/index-all.nozomi") {
			t.Fatalf("plain term with order key must not use date-added index dataset: %v", urls)
		}
	}
	if !orderFetch || len(urls) != 4 {
		t.Fatalf("order dataset + membership fetch required: %v", urls)
	}
}

func TestSearchHTMLPopularOrderAscReversesOrderDatasetOncePerCanonicalHitomi(t *testing.T) {
	// A-3 合格条件維持: direction は order dataset に対して 1 回だけ適用（二重 reverse なし）。
	var urls []string
	fetches := 0
	c := &Client{cfg: defaultHitomiConfig(), testGateIntervalMS: 1, fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		urls = append(urls, rawurl)
		if strings.Contains(rawurl, "/n/tag/popular/year/female:yuri-all.nozomi") {
			return beIDs([]uint32{5, 10, 15}), nil
		}
		return nil, nil
	}}

	ids, total, err := c.SearchHTML("female:yuri orderby:popular orderbydirection:asc", "all", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || ids[0] != 15 || ids[1] != 10 || ids[2] != 5 {
		t.Fatalf("asc must reverse order dataset once: ids %v", ids)
	}
	if fetches != 1 || len(urls) != 1 || !strings.Contains(urls[0], "/n/tag/popular/year/female:yuri-all.nozomi") {
		t.Fatalf("paths: fetches %d %v", fetches, urls)
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
