package main

import (
	"context"
	"strings"
	"testing"
)

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

func TestThumbnailURLMatchesVerifiedTNPath(t *testing.T) {
	// TN base は format で決まる（実測 200: `77656270626967746e` / `61766966626967746e`）。
	// `746e626967746e` は base として存在せず 404。subdomain は `a`/`b` + `tn`（gg.m で決まる）。
	// bucket/switch values は回転するので fixture で規則を検証する（正典: `tools/gen_ggset.py`）
	gg, err := parseGG("var gg = { b: '1793494802/', m: function (g) {\nvar o = 1;\nswitch (g) {\ncase 3:\n o = 0;\n break;\n}\nreturn o;\n}};") // sampleHash（2488）は case 外 → m=1（`btn`）
	if err != nil {
		t.Fatal(err)
	}
	got := ThumbnailURL(sampleHash, "webp", gg)
	want := "https://\x62\x74\x6e." + hitomiDomain + "/\x77\x65\x62\x70\x62\x69\x67\x74\x6e/9/b8/" + sampleHash + ".\x77\x65\x62\x70"
	if got != want {
		t.Fatalf("thumbnail: got %s want %s", got, want)
	}
	got = ThumbnailURL(sampleHash, "avif", gg)
	want = "https://\x62\x74\x6e." + hitomiDomain + "/\x61\x76\x69\x66\x62\x69\x67\x74\x6e/9/b8/" + sampleHash + ".\x61\x76\x69\x66"
	if got != want {
		t.Fatalf("avif thumbnail: got %s want %s", got, want)
	}
	// TN に格納されるのは webp / avif ののみ（実測）なので、それ以外の format は webp に落ちる
	got = ThumbnailURL(sampleHash, "jpeg", gg)
	if got != "https://\x62\x74\x6e." + hitomiDomain + "/\x77\x65\x62\x70\x62\x69\x67\x74\x6e/9/b8/" + sampleHash + ".\x77\x65\x62\x70" {
		t.Fatalf("non-TN format must fall back to webp: got %s", got)
	}
	if fileExt("001.avif") != "avif" || fileExt("001.jpeg") != "jpg" || fileExt("001") != "webp" {
		t.Fatalf("fileExt: %q %q %q", fileExt("001.avif"), fileExt("001.jpeg"), fileExt("001"))
	}
}
