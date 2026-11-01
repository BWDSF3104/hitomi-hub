package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeGalleryPathStaysInsideGalleryDir(t *testing.T) {
	root := filepath.Join("Galleries")
	cases := []string{"4252697/899dc.webp", "4252697/../secret", "/hitomi.db",
		"../../../hitomi.db", "C:\\Windows\\win.ini"}
	for _, rel := range cases {
		full, err := safeGalleryPath(root, rel)
		if err != nil {
			t.Errorf("unexpected reject: %q -> %s", rel, err)
			continue
		}
		if !strings.HasPrefix(full, root+string(filepath.Separator)) {
			t.Errorf("escaped gallery dir: %q -> %s", rel, full)
		}
	}
	if _, err := safeGalleryPath(root, ""); err == nil {
		t.Error("empty path should be rejected")
	}
	if _, err := safeGalleryPath(root, "."); err == nil {
		t.Error("gallery dir itself should be rejected")
	}
}

func viewerFixtureGG() GG {
	// live `gg.js` の switch values は回転する。Viewer tests は snapshot の実値ではなく
	// fixture で規則を検証する。sampleHash の value（2488）を case に含め、zeroHash（1184）を case 外にする。
	return GG{
		Bucket:   "1793494802/",
		CaseSet:  map[int]bool{2488: true},
		Source:   "live",
	}
}

func newTestViewer(t *testing.T, galleryJSON string) (*viewer, string) {
	t.Helper()
	db := openTestDB(t)
	root := t.TempDir()
	galleryRoot := filepath.Join(root, "Galleries")
	if err := os.MkdirAll(galleryRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	v := newViewer(db, defaultConfigForTest(), root)
	v.client.gg = viewerFixtureGG()
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return []byte(galleryJSON), nil
	}
	return v, galleryRoot
}

func defaultConfigForTest() Config {
	cfg := Config{}
	cfg.Hitomi = defaultHitomiConfig()
	return cfg
}

func TestViewerPageSourceLocalFirst(t *testing.T) {
	galleryJSON := `var galleryinfo = {"id":"4252697","title":"Kemo","language":"japanese","type":"doujinshi",` +
		`"files":[{"hash":"899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9","name":"001.jpg","width":1813,"height":2542}]}`
	v, galleryRoot := newTestViewer(t, galleryJSON)

	// 未ダウンロード → CDN（shard 保存形式。name の ext は使わない）
	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/gallery/4252697/page/1", nil))
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["local"] != false || !strings.HasSuffix(out["src"].(string), "/899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9?format=webp") {
		t.Errorf("cdn source: got %v", out)
	}

	// ダウンロード済み → local
	if err := os.MkdirAll(filepath.Join(galleryRoot, "4252697"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(galleryRoot, "4252697", "899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9.webp"), []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/gallery/4252697/page/1", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["local"] != true || out["src"] != "/files/4252697/899dc18877a116799a4ab10f2e6e2a27a19ebd4785f7cf0ab5ede2e65d1281f9.webp" {
		t.Errorf("local source: got %v", out)
	}

	// files route は gallery dir 外を拒否
	rec = httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/files/..%2Fhitomi.db", nil))
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
		t.Errorf("guard: got %d", rec.Code)
	}
}

func TestViewerPageSourceUsesHasavifFormatWhenRemote(t *testing.T) {
	galleryJSON := `var galleryinfo = {"id":"4252697","title":"Kemo","language":"japanese","type":"doujinshi",` +
		`"files":[{"hash":"19793843306679ce3cab65db3ca0bf219477d0e8df09df23055f13ae957f5696","name":"02.png","hasavif":1}]}`
	v, _ := newTestViewer(t, galleryJSON)

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/gallery/4252697/page/1", nil))
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["local"] != false || out["format"] != "avif" || !strings.HasSuffix(out["src"].(string), "?format=avif") {
		t.Errorf("hasavif source: got %v", out)
	}
}

func TestViewerReadFavoriteAndQueryHandlers(t *testing.T) {
	v, _ := newTestViewer(t, `var galleryinfo = {"id":"125","files":[]}`)

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("POST", "/api/read",
		strings.NewReader(`{"gallery_id":125,"page":4,"complete":false}`)))
	if rec.Code != 200 {
		t.Fatalf("read: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("POST", "/api/fav",
		strings.NewReader(`{"gallery_id":125,"on":true}`)))
	if rec.Code != 200 {
		t.Fatalf("fav: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("POST", "/api/query",
		strings.NewReader(`{"name":"kemo","query":"artist:kikunyi language:japanese"}`)))
	if rec.Code != 200 {
		t.Fatalf("query: %d %s", rec.Code, rec.Body.String())
	}

	rows, err := listRows(v.db, "favorites")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["last_page"] != 4 || rows[0]["favorite"] != 1 {
		t.Errorf("favorites after handlers: got %v", rows)
	}
	queries, err := listRows(v.db, "queries")
	if err != nil {
		t.Fatal(err)
	}
	if len(queries) != 1 || queries[0]["name"] != "kemo" {
		t.Errorf("queries after handler: got %v", queries)
	}

	rec = httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/lists", nil))
	var lists map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &lists); err != nil {
		t.Fatal(err)
	}
	if len(lists["favorites"].([]any)) != 1 || len(lists["queries"].([]any)) != 1 {
		t.Errorf("lists: got %v", lists)
	}
}

func TestViewerImageProxyServesSameOriginWebp(t *testing.T) {
	v, _ := newTestViewer(t, "")
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return []byte("RIFF"), nil
	}
	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/image/"+sampleHash, nil))
	if rec.Code != http.StatusOK || rec.Header().Get("content-type") != "image/webp" || rec.Body.String() != "RIFF" {
		t.Fatalf("image proxy: code %d body %q header %q", rec.Code, rec.Body.String(), rec.Header().Get("content-type"))
	}
	if rec.Header().Get("cross-origin-resource-policy") != "same-origin" {
		t.Fatalf("image proxy CORP: got %q", rec.Header().Get("cross-origin-resource-policy"))
	}
	if rec.Header().Get("cache-control") != "private, max-age=604800" {
		t.Fatalf("image proxy cache-control: got %q", rec.Header().Get("cache-control"))
	}
}

func TestViewerImageProxyDoesNotFallBackToTNWhenStorageMissing(t *testing.T) {
	v, _ := newTestViewer(t, "")
	var urls []string
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		urls = append(urls, rawurl)
		if strings.Contains(rawurl, "tn.") {
			return []byte("tn"), nil
		}
		return nil, errors.New("storage missing")
	}
	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/image/"+sampleHash, nil))
	if rec.Code != http.StatusBadGateway || strings.Contains(rec.Body.String(), "tn") {
		t.Fatalf("image proxy should not TN fallback: code %d body %q", rec.Code, rec.Body.String())
	}
	if len(urls) != 1 || strings.Contains(urls[0], "tn.") {
		t.Fatalf("image proxy should only try storage URL: got %v", urls)
	}
}

func TestViewerImageProxyDoesNotFetchTNVariantForImage(t *testing.T) {
	v, _ := newTestViewer(t, "")
	var urls []string
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		urls = append(urls, rawurl)
		if strings.Contains(rawurl, "tn.") {
			return []byte("tn-small"), nil
		}
		return nil, errors.New("storage missing")
	}
	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/image/"+sampleHash, nil))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("image proxy code: got %d body %q", rec.Code, rec.Body.String())
	}
	if len(urls) != 1 || strings.Contains(urls[0], "tn.") {
		t.Fatalf("image proxy should not fetch TN variants: got %v", urls)
	}
}

func TestViewerImageProxyDoesNotFetchAvifTNVariantForImage(t *testing.T) {
	v, _ := newTestViewer(t, "")
	var urls []string
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		urls = append(urls, rawurl)
		if strings.Contains(rawurl, "tn.") {
			return []byte("tn-small"), nil
		}
		return nil, errors.New("storage missing")
	}
	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/image/"+sampleHash+"?format=avif", nil))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("image avif proxy code: got %d body %q", rec.Code, rec.Body.String())
	}
	if len(urls) != 1 || strings.Contains(urls[0], "tn.") {
		t.Fatalf("image avif proxy should not fetch TN variants: got %v", urls)
	}
}

func TestNormalizeFormatAndSafeCachePath(t *testing.T) {
	if normalizeFormat("JPEG") != "jpg" {
		t.Errorf("jpeg format: got %q", normalizeFormat("JPEG"))
	}
	if normalizeFormat("avif") != "avif" {
		t.Errorf("avif format: got %q", normalizeFormat("avif"))
	}
	for _, bad := range []string{"../bad", "bad/", "bad..", "bad\n", ""} {
		if normalizeFormat(bad) != "" {
			t.Errorf("bad format %q should be rejected, got %q", bad, normalizeFormat(bad))
		}
	}
	cacheRoot := filepath.Join("cache", "images")
	full, err := safeCachePath(cacheRoot, sampleHash, "webp")
	if err != nil || full != filepath.Join(cacheRoot, sampleHash+".webp") {
		t.Errorf("cache path: got %q err %v", full, err)
	}
	if _, err := safeCachePath(cacheRoot, sampleHash, "../bad"); err == nil {
		t.Error("unsafe cache format should be rejected")
	}
}

func TestViewerImageProxyWritesPersistentCacheAndServesCacheHit(t *testing.T) {
	v, _ := newTestViewer(t, "")
	calls := 0
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		return []byte("RIFF"), nil
	}

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/image/"+sampleHash, nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "RIFF" || rec.Header().Get("x-cache") != "miss" {
		t.Fatalf("image proxy cache miss: code %d body %q header %q", rec.Code, rec.Body.String(), rec.Header().Get("x-cache"))
	}
	cacheFile := filepath.Join(v.cacheRoot, sampleHash+".webp")
	data, err := os.ReadFile(cacheFile)
	if err != nil || string(data) != "RIFF" {
		t.Fatalf("persistent cache file: err %v data %q", err, string(data))
	}

	rec = httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/image/"+sampleHash, nil))
	if calls != 1 {
		t.Errorf("cache hit should not call network: calls %d", calls)
	}
	if rec.Code != http.StatusOK || rec.Body.String() != "RIFF" || rec.Header().Get("x-cache") != "cache" {
		t.Errorf("image proxy cache hit: code %d body %q header %q", rec.Code, rec.Body.String(), rec.Header().Get("x-cache"))
	}
}

func TestViewerImageProxyReturnsNoFullWhenStorage404AndDoesNotPersistTN(t *testing.T) {
	v, _ := newTestViewer(t, "")
	var urls []string
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		urls = append(urls, rawurl)
		if strings.Contains(rawurl, "tn.") {
			return []byte("tn"), nil
		}
		return nil, errors.New("status 404 for " + rawurl)
	}

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/image/"+sampleHash, nil))
	if rec.Code != http.StatusNotFound || rec.Header().Get("x-cache") != "no_full" || rec.Header().Get("cache-control") != "no-store" {
		t.Fatalf("image proxy no_full: code %d body %q header x-cache %q cache-control %q", rec.Code, rec.Body.String(), rec.Header().Get("x-cache"), rec.Header().Get("cache-control"))
	}
	if len(urls) != 1 || strings.Contains(urls[0], "tn.") {
		t.Errorf("image proxy should not fetch TN: got %v", urls)
	}
	cacheFile := filepath.Join(v.cacheRoot, sampleHash+".webp")
	if _, err := os.Stat(cacheFile); err == nil {
		t.Errorf("image proxy should not persist TN as full-image cache: %s", cacheFile)
	}
}

func TestViewerThumbProxyUsesRealTNPathAndContent(t *testing.T) {
	var gotURL string
	v, _ := newTestViewer(t, "")
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		gotURL = rawurl
		return []byte("data"), nil
	}
	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/thumb/"+sampleHash, nil))
	if rec.Code != http.StatusOK || rec.Header().Get("content-type") != "image/webp" || rec.Body.String() != "data" {
		t.Fatalf("thumb proxy: code %d body %q", rec.Code, rec.Body.String())
	}
	want := "https://\x62\x74\x6e.gold-usergeneratedcontent.net/\x77\x65\x62\x70\x62\x69\x67\x74\x6e/9/b8/" + sampleHash + ".\x77\x65\x62\x70"
	if gotURL != want {
		t.Fatalf("thumb proxy path: got %q want %q", gotURL, want)
	}
	if rec.Header().Get("cross-origin-resource-policy") != "same-origin" {
		t.Fatalf("thumb proxy CORP: got %q", rec.Header().Get("cross-origin-resource-policy"))
	}
}

func TestViewerThumbProxyUsesSmalltnVariantWhenRequested(t *testing.T) {
	var gotURL string
	v, _ := newTestViewer(t, "")
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		gotURL = rawurl
		return []byte("small"), nil
	}
	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/thumb/"+sampleHash+"?format=webp&variant=smalltn", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("content-type") != "image/webp" || rec.Body.String() != "small" {
		t.Fatalf("smalltn thumb proxy: code %d body %q header %q", rec.Code, rec.Body.String(), rec.Header().Get("content-type"))
	}
	want := "https://\x62\x74\x6e.gold-usergeneratedcontent.net/\x77\x65\x62\x70smalltn/9/b8/" + sampleHash + ".\x77\x65\x62\x70"
	if gotURL != want {
		t.Fatalf("smalltn thumb proxy path: got %q want %q", gotURL, want)
	}
	if rec.Header().Get("cross-origin-resource-policy") != "same-origin" {
		t.Fatalf("smalltn thumb proxy CORP: got %q", rec.Header().Get("cross-origin-resource-policy"))
	}
}

func TestViewerThumbProxyUsesTNZeroSubdomainAndContent(t *testing.T) {
	var gotURL string
	zeroHash := "21844c51814f950f39d448afca0c701a49c3c0cbd1b513ebf18e936ad9290065"
	v, _ := newTestViewer(t, "")
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		gotURL = rawurl
		return []byte("data"), nil
	}
	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/thumb/"+zeroHash, nil))
	if rec.Code != http.StatusOK || rec.Header().Get("content-type") != "image/webp" || rec.Body.String() != "data" {
		t.Fatalf("zero thumb proxy: code %d body %q content %q", rec.Code, rec.Body.String(), rec.Header().Get("content-type"))
	}
	want := "https://\x61\x74\x6e.gold-usergeneratedcontent.net/\x77\x65\x62\x70\x62\x69\x67\x74\x6e/5/06/" + zeroHash + ".\x77\x65\x62\x70"
	if gotURL != want {
		t.Fatalf("zero thumb proxy path: got %q want %q", gotURL, want)
	}
}

func TestViewerThumbProxyUsesAvifTNBaseAndContent(t *testing.T) {
	var gotURL string
	zeroHash := "21844c51814f950f39d448afca0c701a49c3c0cbd1b513ebf18e936ad9290065"
	v, _ := newTestViewer(t, "")
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		gotURL = rawurl
		return []byte("data"), nil
	}
	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/thumb/"+zeroHash+"?format=avif", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("content-type") != "image/avif" || rec.Body.String() != "data" {
		t.Fatalf("avif thumb proxy: code %d body %q content %q", rec.Code, rec.Body.String(), rec.Header().Get("content-type"))
	}
	want := "https://\x61\x74\x6e.gold-usergeneratedcontent.net/\x61\x76\x69\x66\x62\x69\x67\x74\x6e/5/06/" + zeroHash + ".\x61\x76\x69\x66"
	if gotURL != want {
		t.Fatalf("avif thumb proxy path: got %q want %q", gotURL, want)
	}
}

func TestViewerThumbProxyTNFirstWhenGalleryQueryAndLocalExists(t *testing.T) {
	v, galleryRoot := newTestViewer(t, "")
	var gotURL string
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		gotURL = rawurl
		return []byte("tn"), nil
	}
	if err := os.MkdirAll(filepath.Join(galleryRoot, "4252697"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(galleryRoot, "4252697", sampleHash+".webp"), []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/thumb/"+sampleHash+"?format=webp&gallery=4252697", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "tn" {
		t.Fatalf("TN-first thumb: code %d body %q", rec.Code, rec.Body.String())
	}
	if gotURL == "" || !strings.Contains(gotURL, "tn.") {
		t.Errorf("hitomi work page should try TN first even when local exists: got %q", gotURL)
	}
}

func TestSafeThumbnailCachePathUsesVariantAndStaysInsideThumbCache(t *testing.T) {
	cacheRoot := filepath.Join("cache", "images")
	got, err := safeThumbnailCachePath(cacheRoot, sampleHash, "webp", "bigtn")
	if err != nil || got != filepath.Join(cacheRoot, "thumbs", sampleHash+".webp.bigtn") {
		t.Errorf("thumb cache path: got %q err %v", got, err)
	}
	if _, err := safeThumbnailCachePath(cacheRoot, sampleHash, "../bad", "bigtn"); err == nil {
		t.Error("unsafe thumb cache format should be rejected")
	}
}

func TestViewerThumbProxyUsesFullCacheBeforeTN(t *testing.T) {
	v, _ := newTestViewer(t, `var galleryinfo = {"id":"1","files":[]}`)
	if err := os.MkdirAll(v.cacheRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v.cacheRoot, sampleHash+".webp"), []byte("cache bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/thumb/"+sampleHash+"?format=webp&gallery=1", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "cache bytes" || rec.Header().Get("x-cache") != "cache" {
		t.Errorf("thumbProxy should serve full cache before remote TN: code %d body %q header %q", rec.Code, rec.Body.String(), rec.Header().Get("x-cache"))
	}
}

func TestViewerThumbProxyPersistsTNThumbnailCacheAndServesCacheHit(t *testing.T) {
	v, _ := newTestViewer(t, `var galleryinfo = {"id":"1","files":[]}`)
	calls := 0
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls++
		return []byte("tn"), nil
	}

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/thumb/"+sampleHash+"?format=webp&gallery=1&variant=bigtn", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "tn" || rec.Header().Get("x-cache") != "tn" {
		t.Fatalf("remote TN thumb: code %d body %q header %q", rec.Code, rec.Body.String(), rec.Header().Get("x-cache"))
	}
	cacheFile := filepath.Join(v.cacheRoot, "thumbs", sampleHash+".webp.bigtn")
	data, err := os.ReadFile(cacheFile)
	if err != nil || string(data) != "tn" {
		t.Fatalf("persistent TN thumb cache: err %v data %q", err, string(data))
	}

	rec = httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/thumb/"+sampleHash+"?format=webp&gallery=1&variant=bigtn", nil))
	if calls != 1 {
		t.Errorf("TN cache hit should not call network: calls %d", calls)
	}
	if rec.Code != http.StatusOK || rec.Body.String() != "tn" || rec.Header().Get("x-cache") != "tn-cache" {
		t.Errorf("TN cache hit: code %d body %q header %q", rec.Code, rec.Body.String(), rec.Header().Get("x-cache"))
	}
}

func TestViewerThumbProxyFallsBackToLocalWhenTNMissingAndLocalExists(t *testing.T) {
	v, galleryRoot := newTestViewer(t, "")
	var gotURL string
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		gotURL = rawurl
		if strings.Contains(rawurl, "tn.") {
			return nil, errors.New("tn missing")
		}
		return []byte("cdn"), nil
	}
	if err := os.MkdirAll(filepath.Join(galleryRoot, "4252697"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(galleryRoot, "4252697", sampleHash+".webp"), []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/thumb/"+sampleHash+"?format=webp&gallery=4252697&variant=smalltn", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "local" {
		t.Fatalf("TN missing local fallback: code %d body %q", rec.Code, rec.Body.String())
	}
	wantTN := "https://\x62\x74\x6e.gold-usergeneratedcontent.net/\x77\x65\x62\x70smalltn/9/b8/" + sampleHash + ".\x77\x65\x62\x70"
	if gotURL != wantTN {
		t.Errorf("TN smalltn path: got %q want %q", gotURL, wantTN)
	}
}

func TestViewerGalleryFilesIncludeLocalAndHitomiFlags(t *testing.T) {
	galleryJSON := `var galleryinfo = {"id":"4252697","title":"Kemo","language":"japanese","type":"doujinshi","scene_indexes":[0,1],"files":[{"hash":"` + sampleHash + `","name":"001.jpg","hasavif":1},{"hash":"58e06942c116cdf2bdd416345856f889b4dcd68d76f361f923c34ab84d432688","name":"002.jpg","hasavif":0}]}`
	v, galleryRoot := newTestViewer(t, galleryJSON)
	if err := os.MkdirAll(filepath.Join(galleryRoot, "4252697"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(galleryRoot, "4252697", sampleHash+".webp"), []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/gallery/4252697", nil))
	if rec.Code != 200 {
		t.Fatalf("gallery: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	gallery := out["gallery"].(map[string]any)
	files := gallery["files"].([]any)
	if len(files) != 2 {
		t.Fatalf("files: got %v", files)
	}
	if files[0].(map[string]any)["local"] != true || files[1].(map[string]any)["local"] != false {
		t.Errorf("local flags: got %v", files)
	}
	if files[0].(map[string]any)["hasavif"] != float64(1) || files[1].(map[string]any)["hasavif"] != float64(0) {
		t.Errorf("hasavif flags: got %v", files)
	}
	if scene := gallery["scene_indexes"].([]any); len(scene) != 2 || scene[0].(float64) != 0 || scene[1].(float64) != 1 {
		t.Errorf("scene_indexes: got %v", gallery)
	}
}

func TestViewerThumbProxyFallsBackToFullImageWhenTNMissing(t *testing.T) {
	v, _ := newTestViewer(t, "")
	var urls []string
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		urls = append(urls, rawurl)
		if len(urls) == 1 {
			return nil, errors.New("tn missing")
		}
		return []byte("full"), nil
	}

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/thumb/"+sampleHash+"?format=webp&gallery=4252697", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "full" {
		t.Fatalf("fallback thumb: code %d body %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("content-type") != "image/webp" || rec.Header().Get("cross-origin-resource-policy") != "same-origin" {
		t.Fatalf("fallback thumb headers: %q %q", rec.Header().Get("content-type"), rec.Header().Get("cross-origin-resource-policy"))
	}
	if len(urls) != 2 || !strings.Contains(urls[0], "tn.") || strings.Contains(urls[1], "tn.") {
		t.Fatalf("fallback fetch order: got %v", urls)
	}
}

func TestViewerSearchHandlerReturnsIdsOnlyForLazyCards(t *testing.T) {
	v, _ := newTestViewer(t, "")
	key := hashTerm("furry")
	leaf := beNode(1, [][]byte{key}, [][2]int64{{1000, 10}}, make([]uint64, btreeOrder+1))
	data := append([]byte{0, 0, 0, 3}, beIDs([]uint32{10, 20, 30})...)
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		switch {
		case strings.HasSuffix(rawurl, "/version"):
			return []byte("1793467632"), nil
		case strings.HasSuffix(rawurl, ".index"):
			return leaf, nil
		case rng[0] == 1000:
			return data, nil
		default:
			return nil, nil
		}
	}

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/search?query=furry&lang=all&page=1&limit=2", nil))
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	ids := out["ids"].([]any)
	results := out["results"].([]any)
	if out["total"] != float64(3) || len(ids) != 2 || ids[0] != float64(10) || ids[1] != float64(20) {
		t.Errorf("search ids: got %v", out)
	}
	if len(results) != 0 {
		t.Errorf("search initial response should not block on card metadata: got %v", out)
	}
}

func TestSearchHandlerIncludesMeasurementShape(t *testing.T) {
	v, _ := newTestViewer(t, "")
	key := hashTerm("furry")
	leaf := beNode(1, [][]byte{key}, [][2]int64{{1000, 10}}, make([]uint64, btreeOrder+1))
	data := append([]byte{0, 0, 0, 3}, beIDs([]uint32{10, 20, 30})...)
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		switch {
		case strings.HasSuffix(rawurl, "/version"):
			return []byte("1793467632"), nil
		case strings.HasSuffix(rawurl, ".index"):
			return leaf, nil
		case rng[0] == 1000:
			return data, nil
		default:
			return nil, nil
		}
	}

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/search?query=furry&lang=all&page=1&limit=2", nil))
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	timing, ok := out["timing"].(map[string]any)
	if !ok {
		t.Fatalf("timing field missing: got %v", out)
	}
	for _, key := range []string{"search_ms", "record_ms", "json_ms"} {
		if _, ok := timing[key]; !ok {
			t.Errorf("timing field %q missing: got %v", key, timing)
		}
	}
	header := rec.Header().Get("x-search-timing")
	if !strings.Contains(header, "search=") || !strings.Contains(header, "record=") || !strings.Contains(header, "json=") {
		t.Errorf("timing header missing phase fields: got %q", header)
	}
	metrics, ok := out["metrics"].(map[string]any)
	if !ok || len(metrics) == 0 {
		t.Fatalf("metrics field missing: got %v", out)
	}
	versionMetric, ok := metrics["version"].(map[string]any)
	if !ok || versionMetric["calls"].(float64) < 1 {
		t.Errorf("version metric not recorded: got %v", metrics)
	}
	indexMetric, ok := metrics["index"].(map[string]any)
	if !ok || indexMetric["calls"].(float64) < 1 {
		t.Errorf("index metric not recorded: got %v", metrics)
	}
}

func TestViewerSearchCardHandlerReturnsResult(t *testing.T) {
	galleryJSON := `var galleryinfo = {"id":"1","title":"Kemo","language":"english","type":"doujinshi","tags":[],"artists":[],"files":[{"hash":"` + sampleHash + `","name":"001.png"}]}`
	v, _ := newTestViewer(t, galleryJSON)

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/search-card/1", nil))
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	wantFormat := imageFormat(v.cfg)
	wantThumb := "/api/thumb/" + sampleHash + "?format=" + wantFormat + "&gallery=1&variant=bigtn"
	if out["id"] != float64(1) || out["title"] != "Kemo" || out["thumbnail"] != wantThumb {
		t.Errorf("search card: got %v", out)
	}
	if rec.Header().Get("x-cache") != "miss" {
		t.Errorf("search card first x-cache: got %q", rec.Header().Get("x-cache"))
	}

	rec = httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/search-card/1", nil))
	if rec.Header().Get("x-cache") != "cache" {
		t.Errorf("search card second x-cache should show cache hit: got %q", rec.Header().Get("x-cache"))
	}
}

func TestEmbeddedUIUsesLocalThumbProxy(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "/api/thumb/") || !strings.Contains(string(page), "thumbURL") {
		t.Errorf("ui should use local /api/thumb proxy: got %q", string(page))
	}
}

func TestSearchResultUsesCoverOnlyThumbnail(t *testing.T) {
	galleryJSON := `var galleryinfo = {"id":"1","files":[{"hash":"` + sampleHash + `","name":"001.png"},` +
		`{"hash":"58e06942c116cdf2bdd416345856f889b4dcd68d76f361f923c34ab84d432688","name":"002.jpg"}]}`
	v, _ := newTestViewer(t, galleryJSON)
	out := v.searchResult(1)
	wantFormat := imageFormat(v.cfg)
	wantThumb := "/api/thumb/" + sampleHash + "?format=" + wantFormat + "&gallery=1&variant=bigtn"
	if out["thumbnail"] != wantThumb {
		t.Errorf("thumbnail: got %v want %s", out["thumbnail"], wantThumb)
	}
	if _, ok := out["pages"]; ok {
		t.Errorf("search result should not include mini page thumbnails")
	}
}

func TestEmbeddedUIReaderStripUsesFullCacheOrPlaceholder(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if strings.Contains(s, `if (!entry.local && i !== 0) return;`) {
		t.Errorf("ui reader strip must not hide non-local entries: got %q", s)
	}
	if !strings.Contains(s, `function formatForEntry(entry) {`) || !strings.Contains(s, `entry.hasavif ? "avif" : (current.format || "webp")`) {
		t.Errorf("ui should use hitomi TN format rules for entry source: got %q", s)
	}
	if !strings.Contains(s, `function stripThumbSource(entry, galleryId = current.id) {`) || !strings.Contains(s, `function updateThumbCache(key, objectUrl) {`) {
		t.Errorf("ui strip should prefer cached full objectURL and update placeholders from full cache: got %q", s)
	}
	if !strings.Contains(s, `stripThumbSource(entry, current.id)`) || !strings.Contains(s, `data-key='`) || !strings.Contains(s, `data-hash='`) {
		t.Errorf("ui strip should render placeholders keyed to full cache entries: got %q", s)
	}
	if !strings.Contains(s, `const items = stripItems();`) || !strings.Contains(s, `const group = stripGroupPage(items);`) {
		t.Errorf("ui reader strip should use hitomi work page scene items and group of 50 entries: got %q", s)
	}
	if !strings.Contains(s, `current.sceneIndexes.slice(0, current.sceneIndexes.length - 1).map`) || !strings.Contains(s, `sceneIndex + 1`) {
		t.Errorf("ui reader strip should render hitomi scene_indexes pages when present: got %q", s)
	}
	if !strings.Contains(s, `item.badge > 1`) {
		t.Errorf("ui should render hitomi scene badge count for scenes with multiple images: got %q", s)
	}
	if !strings.Contains(s, "const marker = entry.local ? \" title='local'\" : \"\";") {
		t.Errorf("ui should keep local as app feature marker, not filter: got %q", s)
	}
}

func TestEmbeddedUIStripControlsUseSelectableGroup(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `id="strip-controls"`) || !strings.Contains(s, `id="strip-group"`) || !strings.Contains(s, `id="strip-group-status"`) {
		t.Errorf("ui should expose strip group controls: got %q", s)
	}
	if !strings.Contains(s, `let stripGroup = null;`) || !strings.Contains(s, `function stripGroupTotal(items) {`) || !strings.Contains(s, `function setStripGroup(group) {`) {
		t.Errorf("ui should track a selectable strip group independently from current page: got %q", s)
	}
	if !strings.Contains(s, `const group = stripGroupPage(items);`) || !strings.Contains(s, `renderStripControls(items, group);`) {
		t.Errorf("renderStrip should render the selectable group and update controls: got %q", s)
	}
	if strings.Contains(s, `function stripGroupPage(items) {`) && strings.Index(s, `if (stripGroup && stripGroup >= 1 && stripGroup <= total) return stripGroup;`) < 0 {
		t.Errorf("stripGroupPage should prefer the user-selected group when it is valid: got %q", s)
	}
	if !strings.Contains(s, `document.getElementById("strip-prev-group").addEventListener`) || !strings.Contains(s, `document.getElementById("strip-next-group").addEventListener`) {
		t.Errorf("ui should wire strip group controls to user actions: got %q", s)
	}
	if !strings.Contains(s, `async function openGallery(id, page, second) {
  const gen = ++openGeneration;
  stripGroup = null;`) {
		t.Errorf("openGallery should reset the selected strip group for a new gallery target: got %q", s)
	}
}

func TestEmbeddedUIHasNewTabQueryPolicy(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `const newTabMode = new URLSearchParams(window.location.search).get("newtab") === "1";`) || !strings.Contains(s, `function targetAttr() {`) || !strings.Contains(s, `target='_blank'`) {
		t.Errorf("ui should support a query-param new-tab policy while preserving same-tab hash routes: got %q", s)
	}
	renderStart := strings.Index(s, "function renderSearchCard(r) {")
	renderBody := s[renderStart:strings.Index(s, "\nasync function loadSearchCardCover(r, searchGen = searchGeneration) {")]
	if !strings.Contains(renderBody, `targetAttr()`) {
		t.Errorf("search card links should use the new-tab query policy: got %q", renderBody)
	}
	listStart := strings.Index(s, "async function showLists(showGen, openGen) {")
	listBody := s[listStart:strings.Index(s, "\nfunction renderStripLoading(id) {")]
	if !strings.Contains(listBody, `targetAttr()`) || !strings.Contains(listBody, `targetAttr()`) {
		t.Errorf("saved search, favorites, and unread links should use the new-tab query policy: got %q", listBody)
	}
	stripStart := strings.Index(s, "function renderStrip() {")
	stripBody := s[stripStart:strings.Index(s, "\nfunction highlightStrip() {")]
	if !strings.Contains(stripBody, `targetAttr()`) {
		t.Errorf("strip links should use the new-tab query policy: got %q", stripBody)
	}
	if !strings.Contains(s, `window.addEventListener("hashchange"`) {
		t.Errorf("same-tab hashchange routing must remain available when new-tab mode is not requested: got %q", s)
	}
}

func TestEmbeddedUIIndependentScrollPanels(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `body { font-family: system-ui, sans-serif; margin: 1rem; background: #14161a; color: #e8eaed; height: calc(100vh - 2rem); overflow: hidden; display: flex; flex-direction: column; }`) {
		t.Errorf("body should constrain the viewport so panels can scroll independently: got %q", s)
	}
	if !strings.Contains(s, `flex: 1; min-height: 0;`) {
		t.Errorf(".cols should take the remaining viewport height without allowing inner panels to expand the page scroll: got %q", s)
	}
	if !strings.Contains(s, `.side { width: 30rem; height: 100%; min-height: 0; overflow-y: auto; }`) || !strings.Contains(s, `.main { flex: 1; min-width: 20rem; height: 100%; min-height: 0; overflow-y: auto; }`) {
		t.Errorf(".side and .main should fill the viewport height and scroll independently: got %q", s)
	}
}

func TestEmbeddedUIShowsSearchExecutingStatus(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `id="search-status"`) || !strings.Contains(s, `検索実行中`) {
		t.Errorf("ui should show search executing status: got %q", s)
	}
	if !strings.Contains(s, `submit.disabled = true`) || !strings.Contains(s, `loadSearchCards`) {
		t.Errorf("ui should mark search executing and load cards lazily: got %q", s)
	}
	if !strings.Contains(s, `submit.dataset.original`) {
		t.Errorf("ui should restore search button after concurrent searches: got %q", s)
	}
	if !strings.Contains(s, "const li = results.querySelector(\"[data-search-id='\" + ids[i] + \"']\");") {
		t.Errorf("ui lazy search card lookup must use a valid selector: got %q", s)
	}
}

func TestEmbeddedUIPrefetchesFullImagesInBackground(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `id="prefetch-status"`) {
		t.Errorf("ui should show background prefetch status: got %q", s)
	}
	if !strings.Contains(s, `let prefetchAbort = null;`) || !strings.Contains(s, `const PREFETCH_CONCURRENCY = 4;`) {
		t.Errorf("ui should use cancellable bounded prefetch concurrency: got %q", s)
	}
	if !strings.Contains(s, `function ensureBackgroundPrefetch(displayed) {`) || !strings.Contains(s, `AbortController`) || !strings.Contains(s, "fetch(fullSrc, { cache: \"no-store\", signal })") {
		t.Errorf("ui should prefetch full images background through abortable no-store entry source fetch: got %q", s)
	}
	if !strings.Contains(s, `r.headers.get("x-cache")`) || !strings.Contains(s, `function updatePrefetchCounts(status, entry, galleryId = current.id) {`) || !strings.Contains(s, `tnDone`) {
		t.Errorf("ui should count x-cache results as full, tn, or error separately: got %q", s)
	}
	if !strings.Contains(s, `displayPassByEntry`) || !strings.Contains(s, `sourceStatusByEntry`) || !strings.Contains(s, `sourceTypeByEntry`) || !strings.Contains(s, `activeObjectUrlKeys`) {
		t.Errorf("ui should keep actual display pass separate from backend/source status: got %q", s)
	}
	if !strings.Contains(s, `actual pixels `) || !strings.Contains(s, `local direct `) || !strings.Contains(s, `objectURL cache `) || !strings.Contains(s, `no full `) || !strings.Contains(s, `TN display fallback disabled`) {
		t.Errorf("prefetch status should show actual pixels, local direct display, objectURL cache, no full, and TN display fallback disabled, not TN display: got %q", s)
	}
	if !strings.Contains(s, `counts.noFullDone`) {
		t.Errorf("ui should count no_full separately from network errors: got %q", s)
	}
	if !strings.Contains(s, `displayedPages`) || !strings.Contains(s, `loadFullImage(img, item.entry, signal, priorityGalleryId, priorityShowGen, priorityOpenGen, priorityRenderGen)`) {
		t.Errorf("ui should load displayed reader items through current priority without double-counting them: got %q", s)
	}
	if !strings.Contains(s, `Math.max(items.length - currentIdx, currentIdx)`) || !strings.Contains(s, `currentIdx + distance`) {
		t.Errorf("ui should order background prefetch nearest to the current reader page first when the background queue is initialized: got %q", s)
	}
	if !strings.Contains(s, `prefetchQueueKeys`) || !strings.Contains(s, `backgroundGalleryId !== current.id`) || !strings.Contains(s, `startBackgroundWorkers(signal)`) {
		t.Errorf("ui should keep a stable background queue and only recompute it when the gallery target changes: got %q", s)
	}
	if !strings.Contains(s, `function entrySource(entry, galleryId = current.id) {`) || !strings.Contains(s, `entry.local_formats`) {
		t.Errorf("ui should use entry local_formats and remote TN/full format for background prefetch: got %q", s)
	}
	if !strings.Contains(s, `prefetchedEntries`) || !strings.Contains(s, `countsByGallery`) {
		t.Errorf("ui should keep already-prefetched entries and gallery counts instead of resetting them on page movement: got %q", s)
	}
	if strings.Contains(s, `await ensureBackgroundPrefetch`) {
		t.Errorf("showPage should start background prefetch without blocking current page load: got %q", s)
	}
	showStart := strings.Index(s, "async function showPage() {")
	if showStart < 0 {
		t.Fatalf("showPage function not found")
	}
	showBody := s[showStart:strings.Index(s, "\nfunction renderStrip() {")]
	if !strings.Contains(showBody, `renderReader();`) || !strings.Contains(showBody, `ensureBackgroundPrefetch(displayed);`) {
		t.Errorf("showPage should render the current reader view and start full-image prefetch without blocking: got %q", showBody)
	}
}

func TestEmbeddedUIProtectsAsyncReaderWrites(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `let openGeneration = 0;`) || !strings.Contains(s, `let showGeneration = 0;`) || !strings.Contains(s, `let renderGeneration = 0;`) {
		t.Errorf("ui should guard async reader renders with generations: got %q", s)
	}
	openStart := strings.Index(s, "async function openGallery(id, page, second) {")
	if openStart < 0 {
		t.Fatalf("openGallery function not found")
	}
	openBody := s[openStart:strings.Index(s, "\nasync function showLists(showGen, openGen) {")]
	if !strings.Contains(openBody, `const gen = ++openGeneration;`) || !strings.Contains(openBody, `if (gen !== openGeneration) return;`) || !strings.Contains(openBody, `await showLists(showGeneration, gen)`) {
		t.Errorf("openGallery should ignore stale async gallery loads: got %q", openBody)
	}
	showStart := strings.Index(s, "async function showPage() {")
	if showStart < 0 {
		t.Fatalf("showPage function not found")
	}
	showBody := s[showStart:strings.Index(s, "\nfunction renderStrip() {")]
	if !strings.Contains(showBody, `const showGen = ++showGeneration;`) || !strings.Contains(showBody, `if (showGen !== showGeneration || openGen !== openGeneration) return;`) {
		t.Errorf("showPage should ignore stale async reader updates: got %q", showBody)
	}
	if !strings.Contains(showBody, `renderStrip();`) || !strings.Contains(showBody, `highlightStrip();`) {
		t.Errorf("showPage should render strip immediately before waiting for async read updates: got %q", showBody)
	}
	if strings.Contains(showBody, `await post("/api/read"`) || strings.Contains(showBody, `await showLists()`) {
		t.Errorf("showPage display path should not block on post read or showLists: got %q", showBody)
	}
	if !strings.Contains(s, `function runBackgroundReadAndLists(showGen, openGen) {`) || !strings.Contains(s, `showLists(showGen, openGen)`) {
		t.Errorf("showPage should move read and lists updates to a generation-guarded background helper: got %q", s)
	}
	loadStart := strings.Index(s, "async function loadFullImage(img, entry, signal, galleryId = current.id, loadShowGen = showGeneration, loadOpenGen = openGeneration, loadRender = renderGeneration) {")
	if loadStart < 0 {
		t.Fatalf("loadFullImage function not found")
	}
	loadBody := s[loadStart:strings.Index(s, "\nfunction renderReader() {")]
	if !strings.Contains(loadBody, `const targetRender = Number(img.dataset.render);`) || !strings.Contains(loadBody, `loadRender !== renderGeneration`) || !strings.Contains(loadBody, `Number(img.dataset.render) === targetRender`) {
		t.Errorf("loadFullImage should not write stale renders: got %q", loadBody)
	}
	renderStart := strings.Index(s, "function renderReader() {")
	if renderStart < 0 {
		t.Fatalf("renderReader function not found")
	}
	renderBody := s[renderStart:strings.Index(s, "\nasync function runPriority(displayed, signal) {")]
	if !strings.Contains(renderBody, `renderGeneration++;`) || !strings.Contains(renderBody, `data-render='`) {
		t.Errorf("renderReader should mark the current render generation: got %q", renderBody)
	}
}

func TestEmbeddedUIStaleAsyncWritesDoNotPolluteStatusOrStrip(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	updateStart := strings.Index(s, "function updatePrefetchCounts(status, entry, galleryId = current.id) {")
	if updateStart < 0 {
		t.Fatalf("updatePrefetchCounts function not found")
	}
	updateEnd := strings.Index(s, "\nfunction entrySource(entry, galleryId = current.id) {")
	if updateEnd < 0 {
		t.Fatalf("updatePrefetchCounts body end not found")
	}
	updateBody := s[updateStart:updateEnd]
	if !strings.Contains(updateBody, `galleryId !== current.id`) || !strings.Contains(updateBody, `backgroundGalleryId !== galleryId`) {
		t.Errorf("updatePrefetchCounts should not write status for stale gallery/background targets: got %q", updateBody)
	}
	prefetchStart := strings.Index(s, "async function prefetchPageImage(entry, signal, galleryId = current.id) {")
	if prefetchStart < 0 {
		t.Fatalf("prefetchPageImage function not found")
	}
	prefetchBody := s[prefetchStart:strings.Index(s, "\nfunction revokeFullObjectUrls() {")]
	if strings.Index(prefetchBody, `galleryId !== current.id || backgroundGalleryId !== galleryId`) >= strings.Index(prefetchBody, `storeFullCache(entry,`) {
		t.Errorf("prefetchPageImage should guard stale background writes before storing status or object URL: got %q", prefetchBody)
	}
	storeStart := strings.Index(s, "function storeFullCache(entry, status, objectUrl, galleryId = current.id) {")
	if storeStart < 0 {
		t.Fatalf("storeFullCache function not found")
	}
	storeBody := s[storeStart:strings.Index(s, "\nasync function prefetchPageImage(entry, signal, galleryId = current.id) {")]
	if !strings.Contains(storeBody, `galleryId === current.id && backgroundGalleryId === galleryId`) {
		t.Errorf("storeFullCache should update strip DOM only for the current background target: got %q", storeBody)
	}
	loadStart := strings.Index(s, "async function loadFullImage(img, entry, signal, galleryId = current.id, loadShowGen = showGeneration, loadOpenGen = openGeneration, loadRender = renderGeneration) {")
	if loadStart < 0 {
		t.Fatalf("loadFullImage function not found")
	}
	loadBody := s[loadStart:strings.Index(s, "\nfunction renderReader() {")]
	loadGuard := strings.Index(loadBody, `if (loadShowGen !== showGeneration || loadOpenGen !== openGeneration || loadRender !== renderGeneration) return;`)
	noFullStore := strings.Index(loadBody, `storeFullCache(entry, "no_full", null, galleryId);`)
	if loadGuard < 0 || noFullStore < 0 || loadGuard >= noFullStore {
		t.Errorf("loadFullImage should return before stale status or no_full store writes: got %q", loadBody)
	}
	if !strings.Contains(loadBody, `if (signal.aborted) return;`) {
		t.Errorf("loadFullImage should stop stale background work when aborted: got %q", loadBody)
	}
}

func TestEmbeddedUIMovesGalleryImmediatelyAndCachesGalleryData(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `const galleryCache = new Map();`) || !strings.Contains(s, `const searchCardCache = new Map();`) {
		t.Errorf("ui should cache opened gallery data and search card metadata for immediate navigation: got %q", s)
	}
	if !strings.Contains(s, `function renderSearchCard(r) {`) || !strings.Contains(s, `searchCardCache.set(r.id, r);`) || !strings.Contains(s, `function showGalleryMetaFromCache(id) {`) {
		t.Errorf("renderSearchCard should cache card metadata and showGalleryMetaFromCache should use it: got %q", s)
	}
	openStart := strings.Index(s, "async function openGallery(id, page, second) {")
	if openStart < 0 {
		t.Fatalf("openGallery function not found")
	}
	openBody := s[openStart:strings.Index(s, "\nasync function showLists(showGen, openGen) {")]
	if !strings.Contains(openBody, `if (prefetchAbort) prefetchAbort.abort();`) || !strings.Contains(openBody, `if (backgroundSignal) backgroundSignal.abort();`) {
		t.Errorf("openGallery should abort old prefetch immediately before the gallery API await: got %q", openBody)
	}
	if !strings.Contains(openBody, `history.replaceState(null, "", route(id, page, second));`) {
		t.Errorf("openGallery should update the gallery route immediately before waiting for gallery data: got %q", openBody)
	}
	if !strings.Contains(openBody, `showGalleryMetaFromCache(id);`) || !strings.Contains(openBody, `const cached = galleryCache.get(id);`) || !strings.Contains(openBody, `if (cached) {`) || !strings.Contains(openBody, `galleryCache.set(id, data);`) {
		t.Errorf("openGallery should use cached search card metadata and cached gallery data for immediate page movement: got %q", openBody)
	}
	if !strings.Contains(s, `function showGalleryMetaFromCache(id) {`) || !strings.Contains(s, `<span class='meta'>loading</span>`) || !strings.Contains(s, `setPrefetchStatus("gallery read load: " + id);`) {
		t.Errorf("showGalleryMetaFromCache should show loading status before async gallery data arrives: got %q", s)
	}
}

func TestViewerSearchCardIncludesFullGalleryDataForImmediateOpen(t *testing.T) {
	galleryJSON := `var galleryinfo = {"id":"1","title":"Kemo","language":"english","type":"doujinshi","tags":[],"artists":[],"scene_indexes":[0,1],"files":[{"hash":"` + sampleHash + `","name":"001.png","hasavif":1}]}`
	v, _ := newTestViewer(t, galleryJSON)

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/search-card/1", nil))
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	wantFormat := imageFormat(v.cfg)
	if out["format"] != wantFormat || out["scene_indexes"] == nil || out["files"] == nil || out["read_state"] == nil {
		t.Errorf("search card should include immediate open gallery data: got %v", out)
	}
	if _, ok := out["pages"]; ok {
		t.Errorf("search card should not include mini page thumbnails: got %v", out)
	}
	files := out["files"].([]any)
	if len(files) != 1 {
		t.Fatalf("files: got %v", out)
	}
	entry := files[0].(map[string]any)
	if entry["hash"] != sampleHash || entry["hasavif"] != float64(1) || entry["local_formats"] == nil {
		t.Errorf("search card entry should include immediate open fields: got %v", entry)
	}
	readState := out["read_state"].(map[string]any)
	if readState["gallery_id"] != float64(1) || readState["favorite"] != float64(0) {
		t.Errorf("search card read_state: got %v", readState)
	}
}

func TestEmbeddedUIShowsImmediateReaderLoadingForUncachedGallery(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `function galleryDataFromCard(card) {`) || !strings.Contains(s, `card.files`) {
		t.Errorf("ui should use full search card data for immediate gallery open: got %q", s)
	}
	if !strings.Contains(s, `function renderGalleryLoading(id, page, second) {`) || !strings.Contains(s, `data-reader-loading`) || !strings.Contains(s, `読込中`) {
		t.Errorf("ui should render immediate reader loading for uncached gallery data: got %q", s)
	}
	openStart := strings.Index(s, "async function openGallery(id, page, second) {")
	if openStart < 0 {
		t.Fatalf("openGallery function not found")
	}
	openBody := s[openStart:strings.Index(s, "\nasync function showLists(showGen, openGen) {")]
	if !strings.Contains(openBody, `const card = searchCardCache.get(id);`) || !strings.Contains(openBody, `const fullCardData = galleryDataFromCard(card);`) || !strings.Contains(openBody, `if (fullCardData) {`) {
		t.Errorf("openGallery should use full cached search card data immediately: got %q", openBody)
	}
	metaStart := strings.Index(openBody, `showGalleryMetaFromCache(id);`)
	loadingStart := strings.Index(openBody, `renderGalleryLoading(id, page, second);`)
	awaitStart := strings.Index(openBody, `await api("/api/gallery/" + id);`)
	if metaStart < 0 || loadingStart < 0 || awaitStart < 0 || metaStart > loadingStart || loadingStart > awaitStart {
		t.Errorf("openGallery should show loading UI before waiting for uncached gallery data: got %q", openBody)
	}
	if !strings.Contains(openBody, `renderStripLoading(id);`) || !strings.Contains(s, `data-strip-loading`) {
		t.Errorf("openGallery should immediately replace old strip thumbnails with loading placeholder: got %q", openBody)
	}
	stripStart := strings.Index(openBody, `renderGalleryLoading(id, page, second);`)
	stripLoadingStart := strings.Index(openBody, `renderStripLoading(id);`)
	if stripStart < 0 || stripLoadingStart < 0 || stripLoadingStart > awaitStart {
		t.Errorf("openGallery should clear old strip thumbnails before waiting for uncached gallery data: got %q", openBody)
	}
}

func TestEmbeddedUIPreservesSearchStateAcrossReload(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `const SEARCH_STORAGE_KEY = "hitomi_hub_last_search";`) || !strings.Contains(s, `let searchGeneration = 0;`) {
		t.Errorf("ui should persist search state and guard async search writes: got %q", s)
	}
	if !strings.Contains(s, `function parseSearchHash(hash) {`) || !strings.Contains(s, `function searchRoute(query, page) {`) || !strings.Contains(s, `function saveSearchState(state) {`) || !strings.Contains(s, `function restoreSearchState() {`) {
		t.Errorf("ui should parse, save, and restore search state: got %q", s)
	}
	if !strings.Contains(s, `localStorage.setItem(SEARCH_STORAGE_KEY, JSON.stringify(state))`) || !strings.Contains(s, `localStorage.getItem(SEARCH_STORAGE_KEY)`) {
		t.Errorf("ui should store search state in localStorage: got %q", s)
	}
	runStart := strings.Index(s, "async function runSearch() {")
	if runStart < 0 {
		t.Fatalf("runSearch function not found")
	}
	runBody := s[runStart:strings.Index(s, "document.getElementById(\"search\")")]
	if !strings.Contains(runBody, `const searchGen = ++searchGeneration;`) || !strings.Contains(runBody, `if (searchGen !== searchGeneration) return;`) {
		t.Errorf("runSearch should ignore stale async search results: got %q", runBody)
	}
	if !strings.Contains(runBody, `saveSearchState({ query, lang, page, ids, total: data.total });`) || !strings.Contains(runBody, `history.replaceState(null, "", searchRoute(query, page))`) {
		t.Errorf("runSearch should save search state and route across reload: got %q", runBody)
	}
	loadCardStart := strings.Index(s, "async function loadSearchCards(ids, searchGen = searchGeneration) {")
	if loadCardStart < 0 {
		t.Fatalf("loadSearchCards function not found")
	}
	loadCardBody := s[loadCardStart:strings.Index(s, "async function runSearch() {")]
	if !strings.Contains(loadCardBody, `if (searchGen !== searchGeneration) return;`) || !strings.Contains(loadCardBody, `const li = results.querySelector("[data-search-id='" + ids[i] + "']");`) {
		t.Errorf("loadSearchCards should guard stale async card writes while keeping lazy card selectors: got %q", loadCardBody)
	}
	initStart := strings.Index(s, "showLists();\nconst initialSearch = parseSearchHash(window.location.hash);")
	if initStart < 0 {
		t.Fatalf("initial search restore not found")
	}
	initBody := s[initStart:]
	if !strings.Contains(initBody, `restoreSearchState();`) || !strings.Contains(initBody, `runSearch();`) {
		t.Errorf("page load should restore search state or start search: got %q", initBody)
	}
}

func TestEmbeddedUIHasReaderSpreadAndFullscreen(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `id="view-mode"`) || !strings.Contains(s, `value="double"`) || !strings.Contains(s, `value="fullscreen"`) {
		t.Errorf("ui should expose reader view modes: got %q", s)
	}
	if !strings.Contains(s, `function displayMode() {`) || !strings.Contains(s, `function readerItems() {`) || !strings.Contains(s, `function parseReaderHash(hash) {`) {
		t.Errorf("ui should have reader spread display mode and hash parser: got %q", s)
	}
	if !strings.Contains(s, `function isAlreadyDoublePage(entry) {`) || !strings.Contains(s, `Number(entry.width) > Number(entry.height)`) {
		t.Errorf("ui should classify already landscape/spread entries from entry width and height: got %q", s)
	}
	if !strings.Contains(s, `function displayModeFor(firstEntry) {`) || !strings.Contains(s, `function readerClass(firstEntry = null) {`) || !strings.Contains(s, `return "reader single landscape"`) {
		t.Errorf("ui should render already landscape entries as single landscape: got %q", s)
	}
	if !strings.Contains(s, `if (fullscreenState !== "off") return "fullscreen" + (doubleActive() ? " double" : "");`) || !strings.Contains(s, `if (fullscreenState !== "off") return "reader fullscreen" + (doubleActive() ? " double" : "");`) {
		t.Errorf("fullscreen state should drive fullscreen display without using fullscreen as layout mode: got %q", s)
	}
	if !strings.Contains(s, `firstEntry && firstEntry.entry ? firstEntry.entry : firstEntry`) {
		t.Errorf("readerClass should accept reader items or entries so renderReader can pass the first reader item: got %q", s)
	}
	if !strings.Contains(s, `readerClass(items[0])`) {
		t.Errorf("renderReader should pass the first reader item to readerClass so already landscape entries are not paired: got %q", s)
	}
	if !strings.Contains(s, `if (doubleActive() && first.entry && !isAlreadyDoublePage(first.entry))`) {
		t.Errorf("ui reader items should include second page only when double is active and the first entry is not already a landscape spread: got %q", s)
	}
	if !strings.Contains(s, `function route(id, page, second) {`) {
		t.Errorf("ui route should support hitomi-like #2-3 spread hashes: got %q", s)
	}
	if !strings.Contains(s, `document.addEventListener("fullscreenchange"`) || !strings.Contains(s, `window.addEventListener("keydown"`) || !strings.Contains(s, `e.key === "f"`) || !strings.Contains(s, `setFullscreenFromGesture()`) || !strings.Contains(s, `requestFullscreen`) {
		t.Errorf("ui should support fullscreen state, keyboard d/s/f, gesture-bound fullscreen, and requestFullscreen: got %q", s)
	}
	if !strings.Contains(s, `window.innerWidth < 720`) {
		t.Errorf("ui should fall back to single when the normal page size is too narrow: got %q", s)
	}
	if !strings.Contains(s, `function doubleActive() { return viewMode !== "single" && window.innerWidth >= 720; }`) {
		t.Errorf("ui should allow double view outside single mode, including fullscreen: got %q", s)
	}
	if !strings.Contains(s, `return "reader fullscreen" + (doubleActive() ? " double" : "")`) {
		t.Errorf("ui should apply reader fullscreen double class when fullscreen double is active: got %q", s)
	}
	if !strings.Contains(s, `if (doubleActive() && first.entry && !isAlreadyDoublePage(first.entry))`) {
		t.Errorf("ui reader items should include second page when double is active, except already landscape entries: got %q", s)
	}
	if !strings.Contains(s, `function startFullPrefetch(displayed = readerItems()) {`) || !strings.Contains(s, `ensureBackgroundPrefetch(displayed);`) {
		t.Errorf("setViewMode should start full prefetch through a defined helper using the current reader items: got %q", s)
	}
	setStart := strings.Index(s, "function setViewMode(mode) {")
	if setStart < 0 {
		t.Fatalf("setViewMode function not found")
	}
	setBody := s[setStart:strings.Index(s, "\nfunction parseReaderHash(hash) {")]
	if !strings.Contains(setBody, `renderReader();`) || !strings.Contains(setBody, `startFullPrefetch();`) {
		t.Errorf("setViewMode should render the active reader view and start full-image prefetch: got %q", setBody)
	}
	if !strings.Contains(s, `renderReader();`) || !strings.Contains(s, `startFullPrefetch();`) {
		t.Errorf("ui should render reader items then load full images through background prefetch: got %q", s)
	}
}

func TestEmbeddedUIShowsPlaceholderThumbUntilFullLoaded(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `function displayPlaceholder(entry) { return ""; }`) {
		t.Errorf("ui should use empty display placeholders so reader display does not add extra TN requests: got %q", s)
	}
	if !strings.Contains(s, `gap: 0;`) || !strings.Contains(s, `--reader-page-width: min(45vw, 500px);`) || !strings.Contains(s, `--reader-page-height: min(78vh, 700px);`) || !strings.Contains(s, `width: min(var(--reader-page-width), 100%);`) || !strings.Contains(s, `object-fit: contain;`) {
		t.Errorf("ui should fix reader display size without gap between pages: got %q", s)
	}
	if !strings.Contains(s, `async function loadFullImage(img, entry, signal, galleryId = current.id, loadShowGen = showGeneration, loadOpenGen = openGeneration, loadRender = renderGeneration) {`) || !strings.Contains(s, `fetch(fullSrc, { cache: "no-store", signal })`) || !strings.Contains(s, `r.headers.get("x-cache")`) || !strings.Contains(s, `URL.createObjectURL(blob)`) || !strings.Contains(s, `img.src = objectUrl`) {
		t.Errorf("ui should replace placeholder display from a no-store fetched full image bytes, while counting TN separately: got %q", s)
	}
	if !strings.Contains(s, `let fullObjectUrls = new Set();`) || !strings.Contains(s, `function revokeFullObjectUrls() {`) || !strings.Contains(s, `revokeFullObjectUrls();`) {
		t.Errorf("ui should keep a bounded object URL cache and revoke stored object URLs when the cache is cleared: got %q", s)
	}
	if !strings.Contains(s, `const fullCache = new Map();`) || !strings.Contains(s, `const prefetchedEntries = new Map();`) || !strings.Contains(s, `function cachedFullSrc(entry, galleryId = current.id) {`) || !strings.Contains(s, `function storeFullCache(entry, status, objectUrl, galleryId = current.id) {`) {
		t.Errorf("ui should store full-image cache entries instead of revoking every display cache on re-render: got %q", s)
	}
	cachedStart := strings.Index(s, "function cachedFullSrc(entry, galleryId = current.id) {")
	if cachedStart < 0 {
		t.Fatalf("cachedFullSrc function not found")
	}
	cachedBody := s[cachedStart:strings.Index(s, "\nfunction getGalleryCounts(galleryId) {")]
	if !strings.Contains(cachedBody, `statusIsViewerDisplaySource`) || !strings.Contains(cachedBody, `cached && cached.objectUrl`) || !strings.Contains(cachedBody, `return cached.objectUrl`) {
		t.Errorf("reader display should use full objectURL only, not TN, TN-cache, or no_full fallback: got %q", cachedBody)
	}
	if !strings.Contains(s, `localDirectSrc`) || !strings.Contains(s, `displayViewerSource`) || !strings.Contains(s, `entrySourceType`) {
		t.Errorf("ui should keep direct local viewer source separate from objectURL display source: got %q", s)
	}
	if !strings.Contains(s, `attachDisplayLoadHandlers`) || !strings.Contains(s, `recordImageDisplayPassFromImg`) {
		t.Errorf("ui should record actual pixels from DOM naturalWidth, not from status text: got %q", s)
	}
	loadStart := strings.Index(s, "async function loadFullImage(img, entry, signal, galleryId = current.id, loadShowGen = showGeneration, loadOpenGen = openGeneration, loadRender = renderGeneration) {")
	loadBody := s[loadStart:strings.Index(s, "\nfunction renderReader() {")]
	if !strings.Contains(loadBody, `if (r.status === 404 && cache === "no_full") {`) || !strings.Contains(loadBody, `storeFullCache(entry, "no_full", null, galleryId);`) {
		t.Errorf("loadFullImage should mark missing full storage as no_full, not error: got %q", loadBody)
	}
	if !strings.Contains(loadBody, `if (!r.ok) {`) || !strings.Contains(loadBody, `updatePrefetchCounts("error", entry, galleryId);`) {
		t.Errorf("loadFullImage should still count true fetch failures as error, not no_full: got %q", loadBody)
	}
	if !strings.Contains(loadBody, `if (cache === "tn") {`) || !strings.Contains(loadBody, `tn display fallback disabled`) || !strings.Contains(loadBody, `storeFullCache(entry, "tn", null, galleryId);`) {
		t.Errorf("loadFullImage should not store TN bytes as full display object URL: got %q", loadBody)
	}
	if !strings.Contains(loadBody, `if (activeObjectUrlKeys.has(key))`) || !strings.Contains(loadBody, `img.src = fullCache.get(key).objectUrl`) {
		t.Errorf("loadFullImage should use existing active full objectURL without an extra network fetch: got %q", loadBody)
	}
	if !strings.Contains(loadBody, `if (entry.local_formats && entry.local_formats.length)`) || !strings.Contains(loadBody, `img.src = localDirectSrc(entry, galleryId)`) {
		t.Errorf("loadFullImage should display already-downloaded local entries through direct /files src without blob fetch overhead: got %q", loadBody)
	}
	renderStart := strings.Index(s, "function renderReader() {")
	if renderStart < 0 {
		t.Fatalf("renderReader function not found")
	}
	renderBody := s[renderStart:strings.Index(s, "\nasync function runPriority(displayed, signal) {")]
	if strings.Contains(renderBody, `revokeFullObjectUrls();`) {
		t.Errorf("renderReader should not revoke the full cache on every render: got %q", renderBody)
	}
	if !strings.Contains(renderBody, `displayViewerSource(entry, current.id)`) || !strings.Contains(renderBody, `data-viewer-display='1'`) || !strings.Contains(renderBody, `data-source-type='`) {
		t.Errorf("renderReader should prefer active objectURL and direct local sources, mark viewer-display images, and not use TN fallback: got %q", renderBody)
	}
	if !strings.Contains(renderBody, `attachDisplayLoadHandlers(container);`) {
		t.Errorf("renderReader should attach DOM load handlers for actual pixel display pass: got %q", renderBody)
	}
	if !strings.Contains(s, `function renderReader() {`) || !strings.Contains(s, `function ensureBackgroundPrefetch(displayed) {`) || !strings.Contains(s, `loadFullImage(img, item.entry, signal, priorityGalleryId, priorityShowGen, priorityOpenGen, priorityRenderGen)`) {
		t.Errorf("ui should render placeholder items and load full images through current priority and background prefetch: got %q", s)
	}
	if !strings.Contains(s, `async function runPriority(displayed, signal) {`) || !strings.Contains(s, `await currentPriorityDone;`) {
		t.Errorf("ui background prefetch should wait for current displayed priority before background requests: got %q", s)
	}
	if strings.Contains(s, `await ensureBackgroundPrefetch`) {
		t.Errorf("showPage should not block on background prefetch: got %q", s)
	}
}

func TestEmbeddedUISearchCardUsesCoverOnlyThumbnail(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if strings.Contains(s, "class='mini'") {
		t.Errorf("search card should render only cover thumbnail: got %q", s)
	}
	if !strings.Contains(s, "<img class='search-card-cover'") || !strings.Contains(s, "r.thumbnail") {
		t.Errorf("search card should use cover thumbnail source: got %q", s)
	}
	if !strings.Contains(s, `function searchCardCoverSource(card) {`) || !strings.Contains(s, `function loadSearchCardCover(r, searchGen = searchGeneration) {`) {
		t.Errorf("search card cover should be loaded lazily after current priority, using full cache if available: got %q", s)
	}
	if !strings.Contains(s, `src=''`) || !strings.Contains(s, `await currentPriorityDone;`) {
		t.Errorf("search card should render metadata-first placeholder before loading low priority TN cover: got %q", s)
	}
	loadCardStart := strings.Index(s, "async function loadSearchCards(ids, searchGen = searchGeneration) {")
	if loadCardStart < 0 {
		t.Fatalf("loadSearchCards function not found")
	}
	loadCardBody := s[loadCardStart:strings.Index(s, "async function runSearch() {")]
	if !strings.Contains(loadCardBody, `await loadSearchCardCover(card, searchGen);`) {
		t.Errorf("loadSearchCards should load card covers after current priority: got %q", loadCardBody)
	}
}

func TestEmbeddedUISearchTagsAreaIsClamped(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, "--search-card-height: 130px") {
		t.Errorf("search card should define --search-card-height: got %q", s)
	}
	if !strings.Contains(s, "max-height: var(--search-card-height)") {
		t.Errorf("search tags area should be limited to thumbnail height: got %q", s)
	}
	if !strings.Contains(s, "-webkit-line-clamp: 9") {
		t.Errorf("search tags area should use line-clamp: got %q", s)
	}

	renderStart := strings.Index(s, "function renderSearchCard(r) {")
	if renderStart < 0 {
		t.Fatalf("renderSearchCard function not found")
	}
	body := s[renderStart:strings.Index(s, "async function loadSearchCardCover(")]
	if !strings.Contains(body, "tagsTitle") || !strings.Contains(body, "<div class='tags meta'\" + tagsTitle") {
		t.Errorf("search card tags area should expose full tags via title attribute: got %q", body)
	}
	if !strings.Contains(body, "escapeHTML(tags)") {
		t.Errorf("search card tags should be escaped before display: got %q", body)
	}
}

func TestEmbeddedUIPageSwitchDisplayStatusSeparation(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)

	if !strings.Contains(s, `const displayPassByEntry = new Map();`) || !strings.Contains(s, `const sourceStatusByEntry = new Map();`) || !strings.Contains(s, `const sourceTypeByEntry = new Map();`) || !strings.Contains(s, `const activeObjectUrlKeys = new Set();`) {
		t.Errorf("ui should separate displayPass, sourceStatus, sourceType, and active objectURL cache: got %q", s)
	}

	statusStart := strings.Index(s, "function renderPrefetchStatus() {")
	if statusStart < 0 {
		t.Fatalf("renderPrefetchStatus function not found")
	}
	statusBody := s[statusStart:strings.Index(s, "\nfunction stripItems() {")]
	if !strings.Contains(statusBody, `actual pixels `) || !strings.Contains(statusBody, `local direct `) || !strings.Contains(statusBody, `objectURL cache `) || !strings.Contains(statusBody, `no full `) || !strings.Contains(statusBody, `TN display fallback disabled`) {
		t.Errorf("renderPrefetchStatus should report actual pixels, local direct display, objectURL cache, no full, and TN display fallback disabled separately: got %q", statusBody)
	}
	if !strings.Contains(statusBody, `displayPassByEntry.get(key)`) || !strings.Contains(statusBody, `sourceTypeByEntry.get(key)`) || !strings.Contains(statusBody, `activeObjectUrlKeys.has(key)`) {
		t.Errorf("renderPrefetchStatus should count display truth and active objectURL cache, not status text alone: got %q", statusBody)
	}

	loadStart := strings.Index(s, "async function loadFullImage(img, entry, signal, galleryId = current.id, loadShowGen = showGeneration, loadOpenGen !== openGeneration, loadRender !== renderGeneration) {")
	if loadStart < 0 {
		loadStart = strings.Index(s, "async function loadFullImage(img, entry, signal, galleryId = current.id, loadShowGen = showGeneration, loadOpenGen = openGeneration, loadRender = renderGeneration) {")
	}
	if loadStart < 0 {
		t.Fatalf("loadFullImage function not found")
	}
	loadBody := s[loadStart:strings.Index(s, "\nfunction renderReader() {")]
	if strings.Index(loadBody, `if (loadShowGen !== showGeneration || loadOpenGen !== openGeneration || loadRender !== renderGeneration) return;`) >= strings.Index(loadBody, `setSourceStatus(entry`) {
		t.Errorf("loadFullImage should guard stale display/status writes before recording display pass or source status: got %q", loadBody)
	}
	if !strings.Contains(loadBody, `recordImageDisplayPassFromImg`) || !strings.Contains(loadBody, `setSourceStatus`) {
		t.Errorf("loadFullImage should record display pass separately from source status: got %q", loadBody)
	}

	renderStart := strings.Index(s, "function renderReader() {")
	renderBody := s[renderStart:strings.Index(s, "\nasync function runPriority(displayed, signal) {")]
	if !strings.Contains(renderBody, `attachDisplayLoadHandlers(container);`) || !strings.Contains(renderBody, `data-viewer-display='1'`) {
		t.Errorf("renderReader should mark viewer display images and attach actual pixel load handlers: got %q", renderBody)
	}

	stripStart := strings.Index(s, "function renderStrip() {")
	stripBody := s[stripStart:strings.Index(s, "\nfunction highlightStrip() {")]
	if !strings.Contains(stripBody, `data-viewer-display='1'`) || !strings.Contains(stripBody, `attachDisplayLoadHandlers`) {
		t.Errorf("renderStrip should mark viewer display images and attach actual pixel load handlers: got %q", stripBody)
	}

	showStart := strings.Index(s, "async function showPage() {")
	if showStart < 0 {
		t.Fatalf("showPage function not found")
	}
	showBody := s[showStart:strings.Index(s, "\nfunction renderStrip() {")]
	if strings.Contains(showBody, `await ensureBackgroundPrefetch`) {
		t.Errorf("showPage should render and attach display handlers without blocking on background prefetch: got %q", showBody)
	}
}

func TestEmbeddedUIHasSpreadDirectionToggle(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `let spreadDirection = "rtl";`) || !strings.Contains(s, `const SPREAD_DIRECTION_STORAGE_KEY = "hitomi_hub_spread_direction";`) {
		t.Errorf("ui should default RTL spread display and persist spread direction: got %q", s)
	}
	if !strings.Contains(s, `function setSpreadDirection(dir) {`) || !strings.Contains(s, `function loadSpreadDirection() {`) {
		t.Errorf("ui should support RTL/LTR spread direction toggle: got %q", s)
	}
	dirStart := strings.Index(s, "function setSpreadDirection(dir) {")
	if dirStart < 0 {
		t.Fatalf("setSpreadDirection function not found")
	}
	dirBody := s[dirStart:strings.Index(s, "\nfunction galleryDataFromCard(card) {")]
	if !strings.Contains(dirBody, `startFullPrefetch();`) {
		t.Errorf("setSpreadDirection should preserve full image display by starting background prefetch after render: got %q", dirBody)
	}

	resizeStart := strings.Index(s, "window.addEventListener(\"resize\", () => {")
	if resizeStart < 0 {
		t.Fatalf("resize handler not found")
	}
	resizeBody := s[resizeStart:strings.Index(s, "\nwindow.addEventListener(\"keydown\", async (e) => {")]
	if !strings.Contains(resizeBody, `startFullPrefetch();`) {
		t.Errorf("resize should preserve full image display by starting background prefetch after render: got %q", resizeBody)
	}
	if !strings.Contains(s, `id="spread-direction"`) || !strings.Contains(s, `e.key === "r"`) {
		t.Errorf("ui should expose spread direction button and key r: got %q", s)
	}

	renderStart := strings.Index(s, "function renderReader() {")
	if renderStart < 0 {
		t.Fatalf("renderReader function not found")
	}
	renderBody := s[renderStart:strings.Index(s, "\nasync function runPriority(displayed, signal) {")]
	if !strings.Contains(renderBody, `if (spreadDirection === "rtl" && items.length > 1) parts.reverse();`) {
		t.Errorf("renderReader should reverse display order for RTL without changing logical pair order: got %q", renderBody)
	}

	keyStart := strings.Index(s, "window.addEventListener(\"keydown\", async (e) => {")
	if keyStart < 0 {
		t.Fatalf("keydown handler not found")
	}
	keyBody := s[keyStart:strings.Index(s, "\ndocument.getElementById(\"strip-prev-group\")")]
	if !strings.Contains(keyBody, `if (e.key === "r") setSpreadDirection(spreadDirection === "rtl" ? "ltr" : "rtl");`) {
		t.Errorf("keydown should toggle RTL/LTR with r: got %q", keyBody)
	}
	if !strings.Contains(s, `function route(id, page, second) {`) || !strings.Contains(s, `function parseReaderHash(hash) {`) || !strings.Contains(s, `"/page/" + page + "-" + second`) || !strings.Contains(s, `page + "-" + second`) {
		t.Errorf("route/hash logical order must remain #2-3-like page-second: got %q", s)
	}
}

func TestEmbeddedUIHasViewModeButtonsAndState(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `id="view-mode-buttons"`) || !strings.Contains(s, `data-mode="single"`) || !strings.Contains(s, `data-mode="double"`) || !strings.Contains(s, `data-mode="fullscreen"`) {
		t.Errorf("ui should expose view mode buttons: got %q", s)
	}
	if !strings.Contains(s, `id="view-state"`) || !strings.Contains(s, `function renderViewState() {`) {
		t.Errorf("ui should expose view state row: got %q", s)
	}
	if !strings.Contains(s, `modeButton.blur();`) || !strings.Contains(s, `e.key === "f"`) {
		t.Errorf("view mode buttons should preserve keyboard mode by clearing button focus and keep f toggle: got %q", s)
	}
	if !strings.Contains(s, `function setFullscreenFromGesture() {`) || !strings.Contains(s, `if (e.key === "f") {`) || !strings.Contains(s, `setFullscreenFromGesture();`) {
		t.Errorf("f should toggle fullscreen through fullscreenState without losing last layout mode: got %q", s)
	}
	if !strings.Contains(s, `last layout: " + lastViewMode`) || !strings.Contains(s, `fullscreen " + fullscreenState`) {
		t.Errorf("view state should show last layout mode and fullscreenState: got %q", s)
	}

	setStart := strings.Index(s, "function setViewMode(mode) {")
	if setStart < 0 {
		t.Fatalf("setViewMode function not found")
	}
	setBody := s[setStart:strings.Index(s, "\nfunction parseReaderHash(hash) {")]
	if !strings.Contains(setBody, `renderViewState();`) {
		t.Errorf("setViewMode should render view state: got %q", setBody)
	}

	showStart := strings.Index(s, "async function showPage() {")
	if showStart < 0 {
		t.Fatalf("showPage function not found")
	}
	showBody := s[showStart:strings.Index(s, "\nfunction renderStrip() {")]
	if !strings.Contains(showBody, `renderViewState();`) {
		t.Errorf("showPage should render view state for current page indicator: got %q", showBody)
	}

	if !strings.Contains(s, `window.addEventListener("resize"`) || !strings.Contains(s, `renderViewState();`) {
		t.Errorf("ui should update view state on resize because doubleActive is evaluated at render time: got %q", s)
	}
}

func TestEmbeddedUISeparatesFullscreenStateAndRestoreLastViewMode(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `let fullscreenState = "off";`) || !strings.Contains(s, `let lastViewMode = "double";`) {
		t.Errorf("ui should keep fullscreen presentation state and last layout mode separately: got %q", s)
	}
	if !strings.Contains(s, `function fullscreenTarget() {`) || !strings.Contains(s, `document.getElementById("reader")`) || !strings.Contains(s, `document.documentElement`) {
		t.Errorf("fullscreen target should prefer reader element so UI chrome can be separated: got %q", s)
	}
	if !strings.Contains(s, `document.body.classList.toggle("fullscreen", fullscreenState !== "off");`) || !strings.Contains(s, `document.body.classList.toggle("pseudo-fullscreen", fullscreenState === "fallback");`) {
		t.Errorf("fullscreen presentation should apply fullscreen class and explicit CSS fallback class separately: got %q", s)
	}
	if !strings.Contains(s, `function requestFullscreenWithFallback() {`) || !strings.Contains(s, `const api = target.requestFullscreen || target.webkitRequestFullscreen;`) {
		t.Errorf("ui should implement fullscreen fallback order requestFullscreen, webkitRequestFullscreen, CSS fallback: got %q", s)
	}
	if !strings.Contains(s, `function setFullscreenFromGesture() {`) || !strings.Contains(s, `function enterFullscreenFromGesture() {`) {
		t.Errorf("fullscreen should be applied from user gesture without relying on load/restore fullscreen state: got %q", s)
	}
	if !strings.Contains(s, `viewMode = lastViewMode || viewMode;`) || !strings.Contains(s, `lastViewMode = viewMode || lastViewMode || "double";`) {
		t.Errorf("fullscreen exit should restore last layout mode: got %q", s)
	}
	if !strings.Contains(s, `document.addEventListener("fullscreenerror", () => {`) || !strings.Contains(s, `window.addEventListener("resize"`) {
		t.Errorf("ui should re-evaluate reader render/prefetch on fullscreenerror and resize: got %q", s)
	}
	if !strings.Contains(s, `body.pseudo-fullscreen .side { display: none; }`) || !strings.Contains(s, `body.pseudo-fullscreen #reader { position: fixed; inset: 0;`) {
		t.Errorf("ui should expose explicit CSS-only pseudo fullscreen fallback for reader display: got %q", s)
	}
	if !strings.Contains(s, `setFullscreenFromGesture();`) || !strings.Contains(s, `fullscreenerror`) {
		t.Errorf("ui should expose gesture-bound fullscreen toggle and fullscreenerror handling: got %q", s)
	}
}

func TestEmbeddedUIHasSearchPrevNextButtons(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `id="search-prev"`) || !strings.Contains(s, `id="search-next"`) || !strings.Contains(s, `id="search-page-status"`) {
		t.Errorf("ui should expose search page prev/next buttons and status: got %q", s)
	}
	if !strings.Contains(s, `function searchTotalPages(total) {`) || !strings.Contains(s, `Math.max(1, Math.ceil(Number(total || 0) / 25))`) {
		t.Errorf("ui should calculate search totalPages from total and 25 page size: got %q", s)
	}
	if !strings.Contains(s, `function renderSearchPageStatus() {`) {
		t.Errorf("ui should have renderSearchPageStatus: got %q", s)
	}
	setStart := strings.Index(s, "async function setSearchPage(delta) {")
	if setStart < 0 {
		t.Fatalf("setSearchPage function not found")
	}
	setBody := s[setStart:strings.Index(s, "\nasync function loadSearchCards(ids, searchGen = searchGeneration) {")]
	if !strings.Contains(setBody, `if (!total || target < 1 || target > totalPages) return;`) || !strings.Contains(setBody, `await runSearch();`) {
		t.Errorf("setSearchPage should clamp page and run search once: got %q", setBody)
	}
	if strings.Contains(setBody, `location.hash =`) {
		t.Errorf("setSearchPage should not double-trigger search through hashchange and direct runSearch: got %q", setBody)
	}

	runStart := strings.Index(s, "async function runSearch() {")
	if runStart < 0 {
		t.Fatalf("runSearch function not found")
	}
	runBody := s[runStart:strings.Index(s, "document.getElementById(\"search\")")]
	if !strings.Contains(runBody, `currentSearchTotal = Number(data.total || 0);`) || !strings.Contains(runBody, `renderSearchPageStatus();`) {
		t.Errorf("runSearch should store total and render page status without changing search-status semantics: got %q", runBody)
	}

	restoreStart := strings.Index(s, "function restoreSearchState() {")
	if restoreStart < 0 {
		t.Fatalf("restoreSearchState function not found")
	}
	restoreBody := s[restoreStart:strings.Index(s, "\nasync function loadSearchCards(ids, searchGen = searchGeneration) {")]
	if !strings.Contains(restoreBody, `currentSearchTotal = Number(state.total || 0);`) || !strings.Contains(restoreBody, `renderSearchPageStatus();`) {
		t.Errorf("restoreSearchState should render search page status from saved total: got %q", restoreBody)
	}
}

func TestEmbeddedUIHasSearchMeasurementStatus(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `id="perf-status"`) || !strings.Contains(s, `function renderPerfStatus() {`) || !strings.Contains(s, `function recordPerfCache(value) {`) {
		t.Errorf("ui should expose search measurement status: got %q", s)
	}
	if !strings.Contains(s, `performance.now()`) || !strings.Contains(s, `data._x_cache = r.headers.get("x-cache")`) || !strings.Contains(s, `data._x_search_timing = r.headers.get("x-search-timing")`) {
		t.Errorf("ui should measure search timing with performance.now and expose cache/timing response headers: got %q", s)
	}

	runStart := strings.Index(s, "async function runSearch() {")
	if runStart < 0 {
		t.Fatalf("runSearch function not found")
	}
	runBody := s[runStart:strings.Index(s, "document.getElementById(\"search\")")]
	if !strings.Contains(runBody, `const searchStart = performance.now();`) || !strings.Contains(runBody, `perfStatus.searchMs = performance.now() - searchStart;`) || !strings.Contains(runBody, `recordPerfCache(data._x_cache);`) || !strings.Contains(runBody, `renderPerfStatus();`) {
		t.Errorf("runSearch should measure /api/search timing without changing search-status semantics: got %q", runBody)
	}

	loadStart := strings.Index(s, "async function loadSearchCards(ids, searchGen = searchGeneration) {")
	if loadStart < 0 {
		t.Fatalf("loadSearchCards function not found")
	}
	loadBody := s[loadStart:strings.Index(s, "\nasync function runSearch() {")]
	if !strings.Contains(loadBody, `const cardStart = performance.now();`) || !strings.Contains(loadBody, `perfStatus.cardTotalMs += performance.now() - cardStart;`) || !strings.Contains(loadBody, `recordPerfCache(card._x_cache);`) || !strings.Contains(loadBody, `renderPerfStatus();`) {
		t.Errorf("loadSearchCards should measure each /api/search-card timing without changing search-status semantics: got %q", loadBody)
	}
	if !strings.Contains(s, `Object.keys(perfStatus.cacheDistribution)`) || !strings.Contains(s, `perfStatus.cardsLoaded`) || !strings.Contains(s, `perfStatus.serverTiming`) {
		t.Errorf("perf status should show cache distribution, card count, and server timing: got %q", s)
	}
}

func TestEmbeddedUIHasSameTitleJapaneseAnchor(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `const titleGroups = new Map();`) || !strings.Contains(s, `const TITLE_GROUPS_STORAGE_KEY = "hitomi_hub_title_groups";`) {
		t.Errorf("ui should keep same-title Japanese group cache separately from search storage: got %q", s)
	}
	if !strings.Contains(s, `function normalizeTitleKey(r) {`) || !strings.Contains(s, `String(r.title || r.japanese_title || r.id`) || !strings.Contains(s, `function loadTitleGroups() {`) || !strings.Contains(s, `function saveTitleGroups() {`) || !strings.Contains(s, `function setSearchTitleGroup(r) {`) || !strings.Contains(s, `function sameTitleJapaneseAnchor(r) {`) {
		t.Errorf("ui should expose same-title group helpers: got %q", s)
	}
	if !strings.Contains(s, `Object.fromEntries(titleGroups)`) || !strings.Contains(s, `localStorage.setItem(TITLE_GROUPS_STORAGE_KEY`) {
		t.Errorf("same-title group cache should persist in localStorage separately from SEARCH_STORAGE_KEY: got %q", s)
	}

	renderStart := strings.Index(s, "function renderSearchCard(r) {")
	if renderStart < 0 {
		t.Fatalf("renderSearchCard function not found")
	}
	renderBody := s[renderStart:strings.Index(s, "async function loadSearchCardCover(")]
	if !strings.Contains(renderBody, `setSearchTitleGroup(r);`) || !strings.Contains(renderBody, `sameTitleJapaneseAnchor(r)`) || !strings.Contains(renderBody, `anchor + `) {
		t.Errorf("renderSearchCard should append same-title Japanese anchor without default hide: got %q", renderBody)
	}
	if !strings.Contains(s, `同名の日本語版`) {
		t.Errorf("same-title Japanese anchor text should be exposed: got %q", s)
	}
	if !strings.Contains(s, `loadTitleGroups();`) {
		t.Errorf("ui should load same-title group cache before search restore: got %q", s)
	}
}

func TestEmbeddedUIShowsGalleryViewerPositionOnly(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `id="reader-position"`) {
		t.Errorf("ui should expose gallery/viewer position display: got %q", s)
	}
	if !strings.Contains(s, `function renderPosition() {`) {
		t.Errorf("ui should have renderPosition for gallery viewer position: got %q", s)
	}
	if strings.Contains(s, `function renderSearchPosition`) || strings.Contains(s, `#search-position`) {
		t.Errorf("#11 search result side should be out of scope: got %q", s)
	}
	if !strings.Contains(s, `function renderLoadingPosition(id, pageLabel, pairLabel) {`) {
		t.Errorf("ui should show a loading position without claiming gallery count: got %q", s)
	}
	if !strings.Contains(s, `const page = second ? current.page + "-" + second : String(current.page);`) || !strings.Contains(s, `el.textContent = "position: " + page + " / " + (current.count || "-");`) {
		t.Errorf("renderPosition should show current page/pair and current count: got %q", s)
	}

	setStart := strings.Index(s, "function setGalleryMeta(data) {")
	if setStart < 0 {
		t.Fatalf("setGalleryMeta function not found")
	}
	setBody := s[setStart:strings.Index(s, "\nfunction fileFormatFromName(name) {")]
	if !strings.Contains(setBody, `renderPosition();`) {
		t.Errorf("setGalleryMeta should render current gallery position: got %q", setBody)
	}

	showStart := strings.Index(s, "async function showPage() {")
	if showStart < 0 {
		t.Fatalf("showPage function not found")
	}
	showBody := s[showStart:strings.Index(s, "\nfunction renderStrip() {")]
	if !strings.Contains(showBody, `renderPosition();`) {
		t.Errorf("showPage should render current gallery position: got %q", showBody)
	}

	viewStart := strings.Index(s, "function setViewMode(mode) {")
	if viewStart < 0 {
		t.Fatalf("setViewMode function not found")
	}
	viewBody := s[viewStart:strings.Index(s, "\nfunction parseReaderHash(hash) {")]
	if !strings.Contains(viewBody, `renderPosition();`) {
		t.Errorf("setViewMode should render current gallery position: got %q", viewBody)
	}

	loadingStart := strings.Index(s, "function renderGalleryLoading(id, page, second) {")
	if loadingStart < 0 {
		t.Fatalf("renderGalleryLoading function not found")
	}
	loadingBody := s[loadingStart:strings.Index(s, "\nfunction updatePrefetchCounts(status, entry, galleryId = current.id) {")]
	if !strings.Contains(loadingBody, `renderLoadingPosition(id, pageLabel, pairLabel);`) {
		t.Errorf("renderGalleryLoading should render loading position: got %q", loadingBody)
	}
}

func TestEmbeddedUIUpdatesTabTitleForCurrentWork(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if !strings.Contains(s, `function updateTabTitle(title) {`) || !strings.Contains(s, `document.title = String(display) + " - hitomi hub viewer";`) {
		t.Errorf("ui should update document title from current work title: got %q", s)
	}
	setStart := strings.Index(s, "function setGalleryMeta(data) {")
	if setStart < 0 {
		t.Fatalf("setGalleryMeta function not found")
	}
	setBody := s[setStart:strings.Index(s, "\nfunction fileFormatFromName(name) {")]
	if !strings.Contains(setBody, `updateTabTitle(g.japanese_title || g.title || current.id);`) {
		t.Errorf("setGalleryMeta should set tab title from gallery title data: got %q", setBody)
	}

	cacheStart := strings.Index(s, "function showGalleryMetaFromCache(id) {")
	if cacheStart < 0 {
		t.Fatalf("showGalleryMetaFromCache function not found")
	}
	cacheBody := s[cacheStart:strings.Index(s, "\nfunction searchCardCoverSource(card) {")]
	if !strings.Contains(cacheBody, `updateTabTitle((card && card.japanese_title) || title || id);`) {
		t.Errorf("showGalleryMetaFromCache should set tab title from cached card title: got %q", cacheBody)
	}

	loadingStart := strings.Index(s, "function renderGalleryLoading(id, page, second) {")
	if loadingStart < 0 {
		t.Fatalf("renderGalleryLoading function not found")
	}
	loadingBody := s[loadingStart:strings.Index(s, "\nfunction updatePrefetchCounts(status, entry, galleryId = current.id) {")]
	if !strings.Contains(loadingBody, `updateTabTitle((card && card.japanese_title) || title || id);`) {
		t.Errorf("renderGalleryLoading should set tab title from cached card title while loading: got %q", loadingBody)
	}
}

func TestViewerAcquisitionCopiesCacheFileIntoTitleFolder(t *testing.T) {
	db := openTestDB(t)
	root := t.TempDir()
	v := newViewer(db, defaultConfigForTest(), root)

	if err := upsertGallery(db, &GalleryInfo{ID: 1, Title: "Kemo", Language: "japanese"}, galleryDir(root, 1)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(v.cacheRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(v.cacheRoot, sampleHash+".png")
	png := []byte("\x89PNG\r\n\x1a\n")
	if err := os.WriteFile(cachePath, png, 0o644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("POST", "/api/cache/acquire",
		strings.NewReader(`{"gallery_id":1,"page":2,"hash":"`+sampleHash+`","format":"png","overwrite":false}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("code: got %d body %q", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["status"] != "copied" || out["scan_status"] != "not_evaluated" || out["title"] != "Kemo" {
		t.Errorf("acquisition: got %v", out)
	}

	dest := filepath.Join(v.acquisitionRoot, "Kemo", "002.png")
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("dest: %s", err)
	}
	if string(data) != string(png) {
		t.Errorf("dest bytes: got %q", data)
	}
	if filepath.Base(dest) != "002.png" {
		t.Errorf("page filename should use page number, not hash: got %s", dest)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Errorf("cache source should remain: %s", err)
	}

	rel, err := filepath.Rel(root, dest)
	if err != nil {
		t.Fatal(err)
	}
	var path, status, source string
	if err := db.QueryRow("SELECT path, scan_status, scan_source FROM files WHERE path = ?", rel).Scan(&path, &status, &source); err != nil {
		t.Fatal(err)
	}
	if path != rel || status != "not_evaluated" || source != "acquisition" {
		t.Errorf("recorded file: got path=%q status=%q source=%q", path, status, source)
	}
}

func TestViewerAcquisitionExistingTitleFolderRequiresOverwrite(t *testing.T) {
	db := openTestDB(t)
	root := t.TempDir()
	v := newViewer(db, defaultConfigForTest(), root)

	if err := upsertGallery(db, &GalleryInfo{ID: 1, Title: "Kemo", Language: "japanese"}, galleryDir(root, 1)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(v.cacheRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(v.cacheRoot, sampleHash+".png")
	png := []byte("\x89PNG\r\n\x1a\n")
	if err := os.WriteFile(cachePath, png, 0o644); err != nil {
		t.Fatal(err)
	}
	titleDir := filepath.Join(v.acquisitionRoot, "Kemo")
	if err := os.MkdirAll(titleDir, 0o755); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("POST", "/api/cache/acquire",
		strings.NewReader(`{"gallery_id":1,"page":2,"hash":"`+sampleHash+`","format":"png","overwrite":false}`)))
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || out["status"] != "confirm" {
		t.Errorf("overwrite false should require confirmation: code %d body %q", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(titleDir, "002.png")); err == nil {
		t.Error("existing title folder should not be written before confirmation")
	}

	rec = httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("POST", "/api/cache/acquire",
		strings.NewReader(`{"gallery_id":1,"page":2,"hash":"`+sampleHash+`","format":"png","overwrite":true}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("overwrite code: got %d body %q", rec.Code, rec.Body.String())
	}
	out = map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["status"] != "copied" {
		t.Errorf("overwrite: got %v", out)
	}
	data, err := os.ReadFile(filepath.Join(titleDir, "002.png"))
	if err != nil {
		t.Fatalf("overwrite dest: %s", err)
	}
	if string(data) != string(png) {
		t.Errorf("overwrite dest bytes: got %q", data)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Errorf("cache source should remain: %s", err)
	}
}

func TestViewerAcquisitionRejectsPathOutsideAcquisitionRoot(t *testing.T) {
	db := openTestDB(t)
	root := t.TempDir()
	v := newViewer(db, defaultConfigForTest(), root)

	if err := upsertGallery(db, &GalleryInfo{ID: 1, Title: "..", Language: "japanese"}, galleryDir(root, 1)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(v.cacheRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(v.cacheRoot, sampleHash+".png")
	png := []byte("\x89PNG\r\n\x1a\n")
	if err := os.WriteFile(cachePath, png, 0o644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("POST", "/api/cache/acquire",
		strings.NewReader(`{"gallery_id":1,"page":1,"hash":"`+sampleHash+`","format":"png","overwrite":true}`)))
	if rec.Code != http.StatusForbidden {
		t.Errorf("path outside root: code %d body %q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "path outside acquisition dir") {
		t.Errorf("path outside root body: got %q", rec.Body.String())
	}
}

func TestEmbeddedUIHasAcquisitionHandler(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)

	if !strings.Contains(s, `id="acquisition-controls"`) ||
		!strings.Contains(s, `id="acquisition-start"`) ||
		!strings.Contains(s, `id="acquisition-abort"`) ||
		!strings.Contains(s, `id="acquisition-status"`) ||
		!strings.Contains(s, `id="acquisition-results"`) {
		t.Errorf("ui should expose acquisition button, abort, progress, and per-page results: got %q", s)
	}
	if strings.Contains(s, `download button`) || strings.Contains(s, `id="download"`) {
		t.Errorf("acquisition UI should not use download: got %q", s)
	}

	if !strings.Contains(s, `function renderAcquisitionStatus() {`) ||
		!strings.Contains(s, `function appendAcquisitionResult(r) {`) ||
		!strings.Contains(s, `async function runAcquisition() {`) {
		t.Errorf("ui should have acquisition handler helpers: got %q", s)
	}
	if !strings.Contains(s, `acquisition progress `) || !strings.Contains(s, `acquisitionDone`) || !strings.Contains(s, `acquisitionAbort`) {
		t.Errorf("ui should track acquisition progress and abort: got %q", s)
	}
	if !strings.Contains(s, `window.confirm(`) || !strings.Contains(s, `await post("/api/cache/acquire"`) {
		t.Errorf("ui should show overwrite confirmation through the acquisition endpoint: got %q", s)
	}
	if !strings.Contains(s, `document.getElementById("acquisition-start").addEventListener("click", runAcquisition);`) ||
		!strings.Contains(s, `document.getElementById("acquisition-abort").addEventListener("click"`) {
		t.Errorf("acquisition buttons should be wired to acquisition handlers: got %q", s)
	}

	runStart := strings.Index(s, "async function runAcquisition() {")
	if runStart < 0 {
		t.Fatalf("runAcquisition function not found")
	}
	runBody := s[runStart:strings.Index(s, "\ndocument.getElementById(\"acquisition-start\")")]
	if !strings.Contains(runBody, `if (acquisitionAbort) break;`) ||
		!strings.Contains(runBody, `result.status === "confirm"`) ||
		!strings.Contains(runBody, `appendAcquisitionResult(`) {
		t.Errorf("runAcquisition should abort, handle confirmation, and append per-page results: got %q", runBody)
	}
}

func TestViewerIndexServesEmbeddedPage(t *testing.T) {
	v, _ := newTestViewer(t, `var galleryinfo = {"id":"1","files":[]}`)
	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "hitomi hub viewer") {
		t.Errorf("index: code %d body %q", rec.Code, rec.Body.String())
	}
}

