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

func TestThumbnailURLMatchesTNBigtnPath(t *testing.T) {
	got := ThumbnailURL(sampleHash, "webp")
	want := "https://tn.gold-usergeneratedcontent.net/webpbigtn/9/b8/" + sampleHash + ".webp"
	if got != want {
		t.Fatalf("thumbnail: got %s want %s", got, want)
	}
	got = ThumbnailURL(sampleHash, "avif")
	want = "https://tn.gold-usergeneratedcontent.net/avifbigtn/9/b8/" + sampleHash + ".avif"
	if got != want {
		t.Fatalf("avif thumbnail: got %s want %s", got, want)
	}
	got = ThumbnailURL(sampleHash, "jpeg")
	want = "https://tn.gold-usergeneratedcontent.net/jpgbigtn/9/b8/" + sampleHash + ".jpg"
	if got != want {
		t.Fatalf("jpg thumbnail: got %s want %s", got, want)
	}
	if fileExt("001.avif") != "avif" || fileExt("001.jpeg") != "jpg" || fileExt("001") != "webp" {
		t.Fatalf("fileExt: %q %q %q", fileExt("001.avif"), fileExt("001.jpeg"), fileExt("001"))
	}
}
