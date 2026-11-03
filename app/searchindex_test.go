package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func beNode(nKeys int, keys [][]byte, datas [][2]int64, subs []uint64) []byte {
	out := []byte{}
	writeBE := func(v uint64, size int) {
		for i := size - 1; i >= 0; i-- {
			out = append(out, byte(v>>uint(i*8)))
		}
	}
	writeBE(uint64(nKeys), 4)
	for _, k := range keys {
		writeBE(uint64(len(k)), 4)
		out = append(out, k...)
	}
	writeBE(uint64(len(datas)), 4)
	for _, d := range datas {
		writeBE(uint64(d[0]), 8)
		writeBE(uint64(uint32(d[1])), 4)
	}
	for _, s := range subs {
		writeBE(s, 8)
	}
	return out
}

func TestHashTermIsSHA256Prefix(t *testing.T) {
	sum := sha256.Sum256([]byte("kemo"))
	got := hashTerm("KEMO")
	if string(got) != string(sum[:4]) {
		t.Fatalf("hashTerm mismatch: %x vs %x", got, sum[:4])
	}
	if len(got) != 4 {
		t.Fatalf("key length %d", len(got))
	}
}

func TestDecodeNodeMatchesSearchlibLayout(t *testing.T) {
	keyA := []byte{0x20, 0x21, 0x22, 0x23}
	keyB := []byte{0x40, 0x41, 0x42, 0x43}
	subs := make([]uint64, btreeOrder+1)
	subs[1] = 464
	data := beNode(2, [][]byte{keyA, keyB}, [][2]int64{{0, 0}, {100, 14}}, subs)
	node, err := decodeNode(data)
	if err != nil {
		t.Fatalf("decodeNode: %s", err)
	}
	if len(node.keys) != 2 || string(node.keys[1]) != string(keyB) {
		t.Fatalf("keys %v", node.keys)
	}
	if node.datas[1] != [2]int64{100, 14} {
		t.Fatalf("datas %v", node.datas)
	}
	if len(node.subs) != btreeOrder+1 || node.subs[1] != 464 {
		t.Fatalf("subs %v", node.subs)
	}
	if found, where := locateKey(keyB, node); !found || where != 1 {
		t.Fatalf("locate exact: %v %d", found, where)
	}
	if found, where := locateKey([]byte{0x30, 0x30, 0x30, 0x30}, node); found || where != 1 {
		t.Fatalf("locate descend: %v %d", found, where)
	}
	if isLeaf(node) {
		t.Fatal("node with subnode address reported as leaf")
	}
}

func TestIndexTokensSplitsHyphenAndUnderscore(t *testing.T) {
	got := indexTokens("KEMO NEE-SAN_10")
	want := []string{"kemo", "nee", "san", "10"}
	if len(got) != len(want) {
		t.Fatalf("tokens %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tokens %v want %v", got, want)
		}
	}
}

func TestSearchTitleIntersectsTokensInFirstTokenOrder(t *testing.T) {
	keyKemo := hashTerm("kemo")
	keyNee := hashTerm("nee")
	// B-tree keys are stored sorted. Map data offsets to the sorted position of each token key
	type slot struct {
		key   []byte
		offset int64
	}
	slots := []slot{{keyKemo, 1000}, {keyNee, 2000}}
	if compareKey(slots[0].key, slots[1].key) > 0 {
		slots = []slot{{keyNee, 2000}, {keyKemo, 1000}}
	}
	leaf := beNode(2, [][]byte{slots[0].key, slots[1].key},
		[][2]int64{{slots[0].offset, 24}, {slots[1].offset, 16}}, make([]uint64, btreeOrder+1))
	// order preserved from the first token: kemo list
	kemoIDs := []uint32{4252697, 4294614, 1, 2, 3}
	neeIDs := []uint32{9, 4252697, 4294614, 8}
	kemoData := append([]byte{0, 0, 0, 5}, beIDs(kemoIDs)...)
	neeData := append([]byte{0, 0, 0, 4}, beIDs(neeIDs)...)

	c := &Client{cfg: defaultHitomiConfig(), fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		switch {
		case strings.HasSuffix(rawurl, "/version"):
			return []byte("1793467632"), nil
		case strings.HasSuffix(rawurl, ".index"):
			return leaf, nil
		case rng[0] == 1000:
			return kemoData, nil
		default:
			return neeData, nil
		}
	}}
	got, err := c.SearchTitle("KEMO NEE")
	if err != nil {
		t.Fatalf("SearchTitle: %s", err)
	}
	if len(got) != 2 || got[0] != 4252697 || got[1] != 4294614 {
		t.Fatalf("intersection %v", got)
	}
}

func beIDs(ids []uint32) []byte {
	out := []byte{}
	for _, id := range ids {
		out = append(out, beID(id)...)
	}
	return out
}

func TestSearchTitleDescendsToLeafNode(t *testing.T) {
	rootKey := []byte{0x20, 0x21, 0x22, 0x23}
	childKey := hashTerm("kemo")
	subs := make([]uint64, btreeOrder+1)
	subs[1] = 464
	root := beNode(1, [][]byte{rootKey}, [][2]int64{{0, 0}}, subs)
	child := beNode(1, [][]byte{childKey}, [][2]int64{{1000, 24}}, make([]uint64, btreeOrder+1))
	childData := append([]byte{0, 0, 0, 5}, beIDs([]uint32{4252697, 4294613, 4294614, 1, 2})...)

	calls := 0
	c := &Client{cfg: defaultHitomiConfig(), fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		switch {
		case strings.HasSuffix(rawurl, "/version"):
			return []byte("1793467632"), nil
		case strings.HasSuffix(rawurl, ".index"):
			if rng[0] == 0 {
				return root, nil
			}
			return child, nil
		default:
			return childData, nil
		}
	}}
	got, err := c.SearchTitle("KEMO")
	if err != nil {
		t.Fatalf("SearchTitle: %s", err)
	}
	if len(got) != 5 || got[0] != 4252697 {
		t.Fatalf("ids %v", got)
	}
	if calls < 4 {
		t.Fatalf("expected version + 2 node reads + data read, got %d", calls)
	}
}

func beID(id uint32) []byte {
	return []byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)}
}

func TestIndexVersionCacheUsesTTL(t *testing.T) {
	calls := 0
	c := &Client{cfg: defaultHitomiConfig(), fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		return []byte("1793467632"), nil
	}}
	if _, err := c.ensureIndexVersion("galleries", false); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ensureIndexVersion("galleries", false); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("TTL cache should not refetch within TTL: calls %d", calls)
	}
	c.versionAt["galleries"] = time.Now().Add(-2 * time.Hour)
	if _, err := c.ensureIndexVersion("galleries", false); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("expired version cache should refetch: calls %d", calls)
	}
}

func TestIndexVersionForceRefetchOnlyOnceOn404(t *testing.T) {
	versionCalls := 0
	c := &Client{cfg: defaultHitomiConfig()}
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		if strings.HasSuffix(rawurl, "/version") {
			versionCalls++
			return []byte("1793467632"), nil
		}
		return nil, errors.New("status 404 for " + rawurl)
	}
	if _, err := c.ensureIndexVersion("galleries", false); err != nil {
		t.Fatal(err)
	}
	if versionCalls != 1 {
		t.Fatalf("initial version fetch should happen once, got %d", versionCalls)
	}
	if _, err := c.indexData("galleries", "1793467632", 1, 1); err == nil {
		t.Fatal("expected old version 404")
	}
	if versionCalls != 2 {
		t.Fatalf("404 should force one version refetch, version calls %d", versionCalls)
	}
	if _, err := c.indexData("galleries", "1793467632", 1, 1); err == nil {
		t.Fatal("expected old version 404 again")
	}
	if versionCalls != 2 {
		t.Errorf("force refetch should happen once, version calls %d", versionCalls)
	}
}

func TestIndexIDsUsesTopNodeCache(t *testing.T) {
	top := beNode(1, [][]byte{hashTerm("kemo")}, [][2]int64{{0, 0}}, make([]uint64, btreeOrder+1))
	fetches := 0
	c := &Client{cfg: defaultHitomiConfig(), fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		switch {
		case strings.HasSuffix(rawurl, "/version"):
			return []byte("1793467632"), nil
		case strings.HasSuffix(rawurl, ".index"):
			return top, nil
		default:
			return nil, nil
		}
	}}

	ids, err := c.IndexIDs("galleries", "miss")
	if err != nil {
		t.Fatalf("IndexIDs: %s", err)
	}
	if len(ids) != 0 {
		t.Fatalf("missing term should return no ids: %v", ids)
	}
	if fetches != 2 {
		t.Fatalf("first IndexIDs should fetch version and top node once, fetches %d", fetches)
	}

	ids, err = c.IndexIDs("galleries", "miss")
	if err != nil {
		t.Fatalf("IndexIDs cache: %s", err)
	}
	if len(ids) != 0 {
		t.Fatalf("cached top result: %v", ids)
	}
	if fetches != 2 {
		t.Fatalf("top node cache should not refetch within TTL: fetches %d", fetches)
	}
}

func TestIndexIDsCachesInternalNodeAndDataForSameTerm(t *testing.T) {
	rootKey := []byte{0x20, 0x21, 0x22, 0x23}
	childKey := hashTerm("kemo")
	subs := make([]uint64, btreeOrder+1)
	subs[1] = 464
	root := beNode(1, [][]byte{rootKey}, [][2]int64{{0, 0}}, subs)
	child := beNode(1, [][]byte{childKey}, [][2]int64{{1000, 24}}, make([]uint64, btreeOrder+1))
	childData := append([]byte{0, 0, 0, 2}, beIDs([]uint32{4294614, 4252697})...)

	fetches := 0
	dataFetches := 0
	c := &Client{cfg: defaultHitomiConfig(), fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		switch {
		case strings.HasSuffix(rawurl, "/version"):
			return []byte("1793467632"), nil
		case strings.HasSuffix(rawurl, ".index"):
			if rng[0] == 0 {
				return root, nil
			}
			return child, nil
		default:
			dataFetches++
			return childData, nil
		}
	}}

	ids, err := c.IndexIDs("galleries", "kemo")
	if err != nil {
		t.Fatalf("IndexIDs: %s", err)
	}
	if len(ids) != 2 || ids[0] != 4294614 || ids[1] != 4252697 {
		t.Fatalf("ids %v", ids)
	}
	if fetches != 4 || dataFetches != 1 {
		t.Fatalf("first descent fetches %d data fetches %d", fetches, dataFetches)
	}

	ids, err = c.IndexIDs("galleries", "kemo")
	if err != nil {
		t.Fatalf("IndexIDs cache: %s", err)
	}
	if len(ids) != 2 || ids[0] != 4294614 || ids[1] != 4252697 {
		t.Fatalf("cached ids %v", ids)
	}
	if fetches != 4 || dataFetches != 1 {
		t.Fatalf("internal node and data cache should not refetch within TTL: fetches %d data fetches %d", fetches, dataFetches)
	}
}

func TestIndexDataCacheEvictsLRUEntriesWhenOverEntryLimit(t *testing.T) {
	fetches := 0
	c := &Client{cfg: defaultHitomiConfig(), indexDataCacheMaxEntries: 2, indexDataCacheMaxBytes: 200}
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		return append([]byte{0, 0, 0, 1}, beID(uint32(rng[0]))...), nil
	}

	// Windows clock granularity can tie same-instant timestamps. Fix access order
	// explicitly after each insert so LRU eviction order is deterministic.
	base := time.Now()
	for i, offset := range []int64{100, 200, 300} {
		if _, err := c.indexDataIDsCached("galleries", "v1", offset, 8); err != nil {
			t.Fatalf("indexDataIDsCached %d: %s", offset, err)
		}
		if entry, ok := c.indexDataCache[fmt.Sprintf("galleries|v1|%d|8", offset)]; ok {
			entry.lastAccess = base.Add(time.Duration(-30+10*i) * time.Second)
		}
	}
	if len(c.indexDataCache) != 2 {
		t.Fatalf("entry limit 2 should evict oldest entry: len %d", len(c.indexDataCache))
	}
	if _, ok := c.indexDataCache["galleries|v1|100|8"]; ok {
		t.Error("oldest entry should be evicted when entry limit is crossed")
	}
	if _, ok := c.indexDataCache["galleries|v1|200|8"]; !ok {
		t.Error("second entry should remain")
	}
	if _, ok := c.indexDataCache["galleries|v1|300|8"]; !ok {
		t.Error("newest entry should remain")
	}
	if c.indexDataCacheBytes > c.indexDataCacheMaxBytes {
		t.Errorf("index data cache memory must stay below cap: bytes %d", c.indexDataCacheBytes)
	}

	// LRU: hit updates access order. Next insert must evict 300 (stale access), not 200.
	if _, err := c.indexDataIDsCached("galleries", "v1", 200, 8); err != nil {
		t.Fatal(err)
	}
	if fetches != 3 {
		t.Fatalf("TTL hit must not refetch: fetches %d", fetches)
	}
	if _, err := c.indexDataIDsCached("galleries", "v1", 400, 8); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.indexDataCache["galleries|v1|300|8"]; ok {
		t.Error("stale-access entry must be evicted by LRU order")
	}
	if _, ok := c.indexDataCache["galleries|v1|200|8"]; !ok {
		t.Error("recently accessed entry must survive LRU eviction")
	}

	// Evicted key is not a TTL hit anymore: it refetches and memory stays bounded.
	if _, err := c.indexDataIDsCached("galleries", "v1", 100, 8); err != nil {
		t.Fatal(err)
	}
	if fetches != 5 {
		t.Errorf("evicted key must refetch: fetches %d", fetches)
	}
	if len(c.indexDataCache) > 2 || c.indexDataCacheBytes > c.indexDataCacheMaxBytes {
		t.Errorf("memory must stay below cap after refetch insert: len %d bytes %d", len(c.indexDataCache), c.indexDataCacheBytes)
	}
}

func TestIndexDataCacheSizeCapBoundsMemory(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig(), indexDataCacheMaxEntries: 10, indexDataCacheMaxBytes: 20}
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return append([]byte{0, 0, 0, 1}, beID(uint32(rng[0]))...), nil
	}

	for _, offset := range []int64{100, 200, 300} {
		if _, err := c.indexDataIDsCached("galleries", "v1", offset, 8); err != nil {
			t.Fatalf("indexDataIDsCached %d: %s", offset, err)
		}
		if c.indexDataCacheBytes > c.indexDataCacheMaxBytes {
			t.Fatalf("size cap must keep memory below cap after insert %d: bytes %d", offset, c.indexDataCacheBytes)
		}
	}
	if len(c.indexDataCache) != 2 {
		t.Fatalf("size cap should evict oldest entries: len %d", len(c.indexDataCache))
	}
	if _, ok := c.indexDataCache["galleries|v1|100|8"]; ok {
		t.Error("oldest entry should be evicted by size cap")
	}
	if _, ok := c.indexDataCache["galleries|v1|300|8"]; !ok {
		t.Error("newest entry should remain within size cap")
	}
}

func TestIndexNodeCacheEvictsLRUEntriesWhenOverEntryLimit(t *testing.T) {
	fetches := 0
	c := &Client{cfg: defaultHitomiConfig(), indexNodeCacheMaxEntries: 2, indexNodeCacheMaxBytes: 1000}
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		return beNode(1, [][]byte{{byte(rng[0]), 0, 0, 0}}, [][2]int64{{0, 0}}, make([]uint64, btreeOrder+1)), nil
	}

	// Windows clock granularity can tie same-instant timestamps. Fix access order
	// explicitly after each insert so LRU eviction order is deterministic.
	base := time.Now()
	for i, addr := range []uint64{100, 200, 300} {
		if _, err := c.indexNodeCached("galleries", "v1", addr); err != nil {
			t.Fatalf("indexNodeCached %d: %s", addr, err)
		}
		if entry, ok := c.indexNodeCache[fmt.Sprintf("galleries|v1|%d", addr)]; ok {
			entry.lastAccess = base.Add(time.Duration(-30+10*i) * time.Second)
		}
	}
	if len(c.indexNodeCache) != 2 {
		t.Fatalf("entry limit 2 should evict oldest entry: len %d", len(c.indexNodeCache))
	}
	if _, ok := c.indexNodeCache["galleries|v1|100"]; ok {
		t.Error("oldest entry should be evicted when entry limit is crossed")
	}
	if c.indexNodeCacheBytes > c.indexNodeCacheMaxBytes {
		t.Errorf("index node cache memory must stay below cap: bytes %d", c.indexNodeCacheBytes)
	}

	// LRU: hit updates access order. Next insert must evict 300 (stale access), not 200.
	if _, err := c.indexNodeCached("galleries", "v1", 200); err != nil {
		t.Fatal(err)
	}
	if fetches != 3 {
		t.Fatalf("TTL hit must not refetch: fetches %d", fetches)
	}
	if _, err := c.indexNodeCached("galleries", "v1", 400); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.indexNodeCache["galleries|v1|300"]; ok {
		t.Error("stale-access entry must be evicted by LRU order")
	}
	if _, ok := c.indexNodeCache["galleries|v1|200"]; !ok {
		t.Error("recently accessed entry must survive LRU eviction")
	}

	// Evicted key is not a TTL hit anymore: it refetches and memory stays bounded.
	if _, err := c.indexNodeCached("galleries", "v1", 100); err != nil {
		t.Fatal(err)
	}
	if fetches != 5 {
		t.Errorf("evicted key must refetch: fetches %d", fetches)
	}
	if len(c.indexNodeCache) > 2 || c.indexNodeCacheBytes > c.indexNodeCacheMaxBytes {
		t.Errorf("memory must stay below cap after refetch insert: len %d bytes %d", len(c.indexNodeCache), c.indexNodeCacheBytes)
	}
}

func TestIndexNodeCacheSizeCapBoundsMemory(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig(), indexNodeCacheMaxEntries: 10, indexNodeCacheMaxBytes: 1000}
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return beNode(1, [][]byte{{byte(rng[0]), 0, 0, 0}}, [][2]int64{{0, 0}}, make([]uint64, btreeOrder+1)), nil
	}

	for _, addr := range []uint64{100, 200, 300} {
		if _, err := c.indexNodeCached("galleries", "v1", addr); err != nil {
			t.Fatalf("indexNodeCached %d: %s", addr, err)
		}
		if c.indexNodeCacheBytes > c.indexNodeCacheMaxBytes {
			t.Fatalf("size cap must keep memory below cap after insert %d: bytes %d", addr, c.indexNodeCacheBytes)
		}
	}
	if len(c.indexNodeCache) != 2 {
		t.Fatalf("size cap should evict oldest entries: len %d", len(c.indexNodeCache))
	}
	if _, ok := c.indexNodeCache["galleries|v1|100"]; ok {
		t.Error("oldest entry should be evicted by size cap")
	}
	if _, ok := c.indexNodeCache["galleries|v1|300"]; !ok {
		t.Error("newest entry should remain within size cap")
	}
}

func TestDecodeIDsLengthGuard(t *testing.T) {
	full := append(append(append([]byte{}, []byte{0, 0, 0, 2}...), beID(4294614)...), beID(4252697)...)
	got, err := decodeIDs(full)
	if err != nil {
		t.Fatalf("decodeIDs: %s", err)
	}
	if len(got) != 2 || got[0] != 4294614 || got[1] != 4252697 {
		t.Fatalf("ids %v", got)
	}
	if _, err := decodeIDs([]byte{0, 0, 0, 2}); err == nil {
		t.Fatal("expected length mismatch error")
	}
}
