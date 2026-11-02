package main

import (
	"context"
	"crypto/sha256"
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
