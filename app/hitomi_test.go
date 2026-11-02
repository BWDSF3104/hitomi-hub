package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

func TestClientCacheFieldsAreMutexProtected(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.cfg.MinIntervalMS = 1
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		if strings.Contains(rawurl, "/gg.js") {
			return []byte("var gg = { b: '1793494802/', m: function (g) { return 1; } }; case 2488:"), nil
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
	// 2026-11-01 実測の live `gg.js` 構造: `var o = 0; switch (g) { case ... o = 1; break; } return o;`
	// cases 内の value は m=1（`w2` / `btn`）、case 外の value は m=0（`w1` / `atn`）。
	fixture := "var gg = { m: function (g) {\nvar o = 0;\nswitch (g) {\ncase 2488:\n o = 1;\n break;\n}\nreturn o;\n}, b: '1793494802/' };"
	gg, err := parseGG(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if gg.Source != "live" || gg.Bucket != "1793494802/" || len(gg.CaseSet) != 1 {
		t.Errorf("parsed gg: got %+v", gg)
	}

	inSet := sampleHash // value 2488 → fixture の case
	outSet := "21844c51814f950f39d448afca0c701a49c3c0cbd1b513ebf18e936ad9290065" // value 1286 → case 外
	if gg.Value(inSet) != 2488 || gg.Value(outSet) != 1286 {
		t.Errorf("gg value: %d %d", gg.Value(inSet), gg.Value(outSet))
	}
	if gg.M(2488) != 1 || gg.M(1286) != 0 {
		t.Errorf("gg M: case=%d out=%d", gg.M(2488), gg.M(1286))
	}

	want := "https://w2.gold-usergeneratedcontent.net/1793494802/2488/" + inSet + ".webp"
	if got := ImageURL(inSet, "webp", "webp", gg); got != want {
		t.Errorf("image url (m=1): got %s want %s", got, want)
	}
	want = "https://w1.gold-usergeneratedcontent.net/1793494802/1286/" + outSet + ".webp"
	if got := ImageURL(outSet, "webp", "webp", gg); got != want {
		t.Errorf("image url (m=0): got %s want %s", got, want)
	}

	want = "https://btn." + hitomiDomain + "/webpbigtn/9/b8/" + inSet + ".webp"
	if got := ThumbnailURL(inSet, "webp", gg); got != want {
		t.Errorf("tn (m=1): got %s want %s", got, want)
	}
	want = "https://atn." + hitomiDomain + "/avifbigtn/5/06/" + outSet + ".avif"
	if got := ThumbnailURL(outSet, "avif", gg); got != want {
		t.Errorf("tn avif (m=0): got %s want %s", got, want)
	}
	if strings.Contains(ThumbnailURL(outSet, "avif", gg), "tnbigtn") {
		t.Errorf("tn name must not appear as base: got %s", ThumbnailURL(outSet, "avif", gg))
	}
}

func TestEnsureGGUsesTTLAndForceRefresh(t *testing.T) {
	fixture := "var gg = { b: '1793494802/', m: function (g) { return 1; } }; case 2488:"
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
	fixture := "var gg = { b: '1793494802/', m: function (g) { return 1; } }; case 2488:"
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
	fixture := "var gg = { b: '1793494802/', m: function (g) { return 1; } }; case 2488:"
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

func TestSnapshotGGIsPopulatedAndUsedAsFallback(t *testing.T) {
	gg := snapshotGG()
	if gg.Bucket == "" || len(gg.CaseSet) == 0 {
		t.Fatalf("snapshot gg is empty: %+v", gg)
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
		return []byte("var gg = { b: '1793494802/', m: function (g) { return 1; } }; case 2488:"), nil
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
	// live 実測構造では fixture の switch に sampleHash の値（2488）が無い → m=0（`w1`）
	gg, err := parseGG("var gg = { b: '1793494802/', m: function (g) {\nvar o = 0;\nswitch (g) {\ncase 3:\n o = 1;\n break;\n}\nreturn o;\n}};")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://w1.gold-usergeneratedcontent.net/1793494802/2488/" + sampleHash + ".webp"
	if got := StorageURL(sampleHash, "webp", gg); got != want {
		t.Errorf("storage url: got %s want %s", got, want)
	}
	if got := StorageURL(sampleHash, "", gg); got != want {
		t.Errorf("default format: got %s", got)
	}
	if !strings.HasSuffix(StorageURL(sampleHash, "avif", gg), ".avif") {
		t.Errorf("avif format: got %s", StorageURL(sampleHash, "avif", gg))
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
}

func TestTNBaseUsesExactFormatBytesNotTnName(t *testing.T) {
	// TN base は format で決まる（実測 200: webpbigtn / avifbigtn）。`746e626967746e` は存在せず 404。
	// subdomain は gg.m で `a`/`b` + `tn`。hash の末尾は `(last)/(prev two)`。
	gg, err := parseGG("var gg = { b: '1793494802/', m: function (g) {\nvar o = 0;\nswitch (g) {\ncase 3:\n o = 1;\n break;\n}\nreturn o;\n}};") // live 実測構造: case 外の sampleHash（2488）→ m=0（`atn`）
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
	results := downloadFilesParallel(c, target, entries, "webp", 4)
	if len(results) != 3 {
		t.Fatalf("results: got %d", len(results))
	}
	for i, result := range results {
		if result.err != "" {
			t.Fatalf("result %d error: %s", i, result.err)
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
