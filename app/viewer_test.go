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

func newTestViewer(t *testing.T, galleryJSON string) (*viewer, string) {
	t.Helper()
	db := openTestDB(t)
	root := t.TempDir()
	galleryRoot := filepath.Join(root, "Galleries")
	if err := os.MkdirAll(galleryRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	v := newViewer(db, defaultConfigForTest(), root)
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
}

func TestViewerImageProxyFallsBackToTnBigtnWhenStorageMissing(t *testing.T) {
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
	if rec.Code != http.StatusOK || rec.Body.String() != "tn" {
		t.Fatalf("image TN fallback: code %d body %q", rec.Code, rec.Body.String())
	}
	if len(urls) != 2 || strings.Contains(urls[0], "tn.") || !strings.Contains(urls[1], "tn.") {
		t.Fatalf("image fallback fetch order: got %v", urls)
	}
	if rec.Header().Get("content-type") != "image/webp" || rec.Header().Get("cross-origin-resource-policy") != "same-origin" {
		t.Fatalf("image TN fallback headers: %q %q", rec.Header().Get("content-type"), rec.Header().Get("cross-origin-resource-policy"))
	}
}

func TestViewerImageProxyFallsBackToSmalltnVariantWhenBigtnTNMissing(t *testing.T) {
	v, _ := newTestViewer(t, "")
	var urls []string
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		urls = append(urls, rawurl)
		if strings.Contains(rawurl, "tn.") {
			if strings.Contains(rawurl, "/webpsmalltn/") {
				return []byte("tn-small"), nil
			}
			return nil, errors.New("tn missing")
		}
		return nil, errors.New("storage missing")
	}
	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/image/"+sampleHash, nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "tn-small" {
		t.Fatalf("image TN variant fallback: code %d body %q", rec.Code, rec.Body.String())
	}
	if len(urls) != 4 || strings.Contains(urls[0], "tn.") || !strings.Contains(urls[1], "/webpbigtn/") || !strings.Contains(urls[2], "/webpsmallbigtn/") || !strings.Contains(urls[3], "/webpsmalltn/") {
		t.Fatalf("image TN variant order: got %v", urls)
	}
}

func TestViewerImageProxyFallsBackToAvifSmalltnVariantForHasavif(t *testing.T) {
	v, _ := newTestViewer(t, "")
	var urls []string
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		urls = append(urls, rawurl)
		if strings.Contains(rawurl, "tn.") {
			if strings.Contains(rawurl, "/avifsmalltn/") {
				return []byte("tn-small"), nil
			}
			return nil, errors.New("tn missing")
		}
		return nil, errors.New("storage missing")
	}
	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/image/"+sampleHash+"?format=avif", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "tn-small" {
		t.Fatalf("image avif TN variant fallback: code %d body %q", rec.Code, rec.Body.String())
	}
	if len(urls) != 4 || strings.Contains(urls[0], "tn.") || !strings.Contains(urls[1], "/avifbigtn/") || !strings.Contains(urls[2], "/avifsmallbigtn/") || !strings.Contains(urls[3], "/avifsmalltn/") {
		t.Fatalf("image avif TN variant order: got %v", urls)
	}
	if rec.Header().Get("content-type") != "image/avif" || rec.Header().Get("cross-origin-resource-policy") != "same-origin" {
		t.Fatalf("image avif TN fallback headers: %q %q", rec.Header().Get("content-type"), rec.Header().Get("cross-origin-resource-policy"))
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

func TestViewerImageProxyDoesNotPersistTNFallback(t *testing.T) {
	v, _ := newTestViewer(t, "")
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		if strings.Contains(rawurl, "tn.") {
			return []byte("tn"), nil
		}
		return nil, errors.New("storage missing")
	}

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/image/"+sampleHash, nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "tn" || rec.Header().Get("x-cache") != "tn" {
		t.Fatalf("TN fallback: code %d body %q header %q", rec.Code, rec.Body.String(), rec.Header().Get("x-cache"))
	}
	cacheFile := filepath.Join(v.cacheRoot, sampleHash+".webp")
	if _, err := os.Stat(cacheFile); err == nil {
		t.Errorf("TN fallback should not persist as full-image cache: %s", cacheFile)
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

func TestEmbeddedUIThumbUsesHitomiTNVariants(t *testing.T) {
	page, err := uiPage.ReadFile("ui.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	if strings.Contains(s, `if (!entry.local && i !== 0) return;`) {
		t.Errorf("ui reader strip must not hide non-local hitomi TN entries: got %q", s)
	}
	if !strings.Contains(s, `function formatForEntry(entry) {`) || !strings.Contains(s, `entry.hasavif ? "avif" : (current.format || "webp")`) {
		t.Errorf("ui should use hitomi TN format rules: got %q", s)
	}
	if !strings.Contains(s, `return window.devicePixelRatio === 1 ? "smallsmalltn" : "smalltn";`) {
		t.Errorf("ui reader strip should use hitomi work page TN variants: got %q", s)
	}
	if !strings.Contains(s, `thumbURL(entry.hash, formatForEntry(entry), current.id, variant)`) {
		t.Errorf("ui reader strip should pass TN variant and gallery id: got %q", s)
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
	if !strings.Contains(s, `let prefetchAbort = null;`) || !strings.Contains(s, `const PREFETCH_CONCURRENCY = 2;`) {
		t.Errorf("ui should use cancellable bounded prefetch concurrency: got %q", s)
	}
	if !strings.Contains(s, `function startFullPrefetch() {`) || !strings.Contains(s, `AbortController`) || !strings.Contains(s, "fetch(entrySource(entry), { signal })") {
		t.Errorf("ui should prefetch full images background through abortable entry source fetch: got %q", s)
	}
	if !strings.Contains(s, `function entrySource(entry) {`) || !strings.Contains(s, `entry.local_formats`) {
		t.Errorf("ui should use entry local_formats and remote TN/full format for background prefetch: got %q", s)
	}
	if strings.Contains(s, `await startFullPrefetch`) {
		t.Errorf("showPage should start background prefetch without blocking current page load: got %q", s)
	}
	showStart := strings.Index(s, "async function showPage() {")
	if showStart < 0 {
		t.Fatalf("showPage function not found")
	}
	showBody := s[showStart:]
	if !strings.Contains(showBody, `img.src = data.src;`) || !strings.Contains(showBody, `startFullPrefetch();`) {
		t.Errorf("showPage should start full-image prefetch immediately after current image src: got %q", showBody)
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
	if !strings.Contains(s, "<img class='cover'") || !strings.Contains(s, "r.thumbnail") {
		t.Errorf("search card should use cover thumbnail: got %q", s)
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

