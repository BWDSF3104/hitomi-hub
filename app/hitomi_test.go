package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// 2026-11-01 に ltn 実測で確認した値（HTTP 200・210784 bytes）
const sampleHash = "2d18a9a8fb2d971ec67f4287a13322738c04b8111e4cf4c8d73b2745a5942b89"

func TestImageURLMatchesLiveSample(t *testing.T) {
	want := "https://w2.gold-usergeneratedcontent.net/1793476801/2488/" + sampleHash + ".webp"
	if got := ImageURL(sampleHash, "webp", "webp"); got != want {
		t.Errorf("image url: got %s want %s", got, want)
	}
	if ggValue(sampleHash) != 2488 {
		t.Errorf("gg value: got %d", ggValue(sampleHash))
	}
	if ggM(2488) != 1 || ggM(3) != 1 || ggM(1) != 0 || ggM(3628) != 1 {
		t.Errorf("gg.m: m(2488)=%d m(3)=%d m(1)=%d m(3628)=%d", ggM(2488), ggM(3), ggM(1), ggM(3628))
	}
}

func TestSearchNozomiDecodeAndRange(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
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
	c := &Client{cfg: defaultHitomiConfig()}
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
	want := "https://w2.gold-usergeneratedcontent.net/1793476801/2488/" + sampleHash + ".webp"
	if got := StorageURL(sampleHash, "webp"); got != want {
		t.Errorf("storage url: got %s want %s", got, want)
	}
	if got := StorageURL(sampleHash, ""); got != want {
		t.Errorf("default format: got %s", got)
	}
	if !strings.HasSuffix(StorageURL(sampleHash, "avif"), ".avif") {
		t.Errorf("avif format: got %s", StorageURL(sampleHash, "avif"))
	}
}

func TestGalleryParse(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		if !strings.Contains(rawurl, "/galleries/4294614.js") {
			t.Errorf("url: got %s", rawurl)
		}
		return []byte(`var galleryinfo = {"id":"4294614","title":"Heroine Profile","language":"english","type":"doujinshi","tags":[{"tag":"corruption","female":"1","male":""}],"artists":[{"artist":"alfre-tori"}],"groups":null,"files":[{"hash":"abc","name":"01_1.webp","width":1280,"height":1791}]}`), nil
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
	if len(info.Files) != 1 || info.Files[0].Hash != "abc" {
		t.Errorf("files: got %v", info.Files)
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
