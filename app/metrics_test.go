package main

import "testing"

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