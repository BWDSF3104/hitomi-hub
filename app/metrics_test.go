package main

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestClassifyFetchURLSeparatesIndexDataGalleryAndTN(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://ltn.hitomi.la/galleriesindex/version", "version"},
		{"https://ltn.hitomi.la/galleriesindex/galleries.123.index", "index"},
		{"https://ltn.hitomi.la/galleriesindex/galleries.123.data", "data"},
		{"https://ltn.hitomi.la/galleries/1234567.html", "gallery"},
		{"https://hitomi.la/galleries/1234567.html", "gallery"},
		{"https://tn.hitomi.la/webpbigtn/abc.webp", "image"},
		{"https://webpbigtn.hitomi.la/abc.webp", "image"},
		{"https://ltn.hitomi.la/nozomi/abc", "nozomi"},
	}
	for _, tc := range cases {
		if got := classifyFetchURL(tc.url); got != tc.want {
			t.Fatalf("classify %q: got %q want %q", tc.url, got, tc.want)
		}
	}
}

// Phase 2 completion condition: the prefetch marker is recorded only (metrics kind separation)
// and marked requests must not change endpoint responses or cache behavior.
func TestBackgroundPrefetchMarkerRecordedSeparatelyInMetrics(t *testing.T) {
	galleryJSON := `var galleryinfo = {"id":"125","title":"kemo","language":"japanese","type":"doujinshi",` +
		`"files":[{"hash":"` + sampleHash + `","name":"001.jpg","width":1813,"height":2542}]}`
	v, _ := newTestViewer(t, galleryJSON)
	// R-16: TN 書き込み（shared core・magic 照合）で tn-cache hit が成立する webp magic fixture。
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		if classifyFetchURL(rawurl) == "image" {
			return webpMagicBytes("tn"), nil
		}
		return []byte(galleryJSON), nil
	}

	get := func(path string) (string, string) {
		rec := httptest.NewRecorder()
		v.handler().ServeHTTP(rec, loopbackGETRequest(path))
		return rec.Header().Get("x-cache"), rec.Body.String()
	}

	// cache state must come from the existing mechanism, not from the marker
	firstCache, _ := get("/api/search-card/125")
	plainCache, plainBody := get("/api/search-card/125")
	markedCache, markedBody := get("/api/search-card/125?priority=background")
	if firstCache != "miss" || plainCache != "cache" || markedCache != "cache" {
		t.Fatalf("search-card cache states: %q %q %q", firstCache, plainCache, markedCache)
	}
	if markedBody != plainBody {
		t.Errorf("marked search-card response must equal the unmarked response: got %q want %q", markedBody, plainBody)
	}

	tnPlainCache, tnPlainBody := get("/api/thumb/" + sampleHash + "?format=webp&gallery=125&variant=bigtn")
	tnMarkedCache, tnMarkedBody := get("/api/thumb/" + sampleHash + "?format=webp&gallery=125&variant=bigtn&priority=background")
	if tnPlainCache != "tn" || tnMarkedCache != "tn-cache" {
		t.Fatalf("tn cache states: %q %q", tnPlainCache, tnMarkedCache)
	}
	if tnMarkedBody != tnPlainBody {
		t.Errorf("marked tn response must equal the unmarked response: got %q want %q", tnMarkedBody, tnPlainBody)
	}

	metrics := v.client.metricsSnapshot()
	fetchCounts := map[string]int64{"gallery": 1, "image": 1}
	for kind, want := range fetchCounts {
		m, ok := metrics[kind].(map[string]int64)
		if !ok {
			t.Fatalf("fetch kind %q missing: got %v", kind, metrics)
		}
		if m["calls"] != want {
			t.Errorf("fetch kind %q calls: got %d want %d (marker must not move fetch-kind counts)", kind, m["calls"], want)
		}
	}
	marker, ok := metrics["marker.background"].(map[string]int64)
	if !ok {
		t.Fatalf("marker kind missing: got %v", metrics)
	}
	if marker["calls"] != 2 {
		t.Errorf("marker.background calls: got %d want 2 (only the two marked requests)", marker["calls"])
	}
}