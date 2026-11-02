package main

import (
	"context"
	"strings"
	"testing"
)

func TestRunSavedQueriesTracksNewGalleries(t *testing.T) {
	db := openTestDB(t)
	if err := addQuery(db, "yuri", "female:yuri", 0); err != nil {
		t.Fatal(err)
	}
	if err := upsertGallery(db, &GalleryInfo{ID: 100, Language: "english"}, "Galleries/100"); err != nil {
		t.Fatal(err)
	}
	page := beIDs([]uint32{100, 200, 300})
	c := &Client{cfg: defaultHitomiConfig(), fetch: func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		if strings.Contains(rawurl, ".nozomi") {
			return page, nil
		}
		return nil, nil
	}}
	results, err := runSavedQueries(db, c, t.TempDir(), "english", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results %v", results)
	}
	got := results[0]
	if got["error"] != nil {
		t.Fatalf("query error: %v", got["error"])
	}
	newIDs := got["new"].([]int)
	if len(newIDs) != 3 || newIDs[0] != 100 || newIDs[1] != 200 || newIDs[2] != 300 {
		t.Fatalf("first run new ids %v", newIDs)
	}
	if got["seen"] != 3 {
		t.Fatalf("seen %v", got["seen"])
	}

	results, err = runSavedQueries(db, c, t.TempDir(), "english", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results %v", results)
	}
	got = results[0]
	if got["error"] != nil {
		t.Fatalf("query error: %v", got["error"])
	}
	secondIDs := got["new"].([]int)
	if len(secondIDs) != 0 {
		t.Fatalf("second run should have no new ids: %v", secondIDs)
	}
	if got["seen"] != 3 {
		t.Fatalf("second seen %v", got["seen"])
	}
	queries, err := listRows(db, "queries")
	if err != nil {
		t.Fatal(err)
	}
	if queries[0]["last_run_at"] == "" {
		t.Error("last_run_at not set by check")
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM galleries").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("galleries rows: got %d", count)
	}
}

func TestRunSavedQueriesUsesIndexForPlainQuery(t *testing.T) {
	db := openTestDB(t)
	if err := addQuery(db, "kemo", "kemo", 0); err != nil {
		t.Fatal(err)
	}
	key := hashTerm("kemo")
	leaf := beNode(1, [][]byte{key}, [][2]int64{{1000, 12}}, make([]uint64, btreeOrder+1))
	data := append([]byte{0, 0, 0, 2}, beIDs([]uint32{4252697, 4294614})...)
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
	results, err := runSavedQueries(db, c, t.TempDir(), "english", 1)
	if err != nil {
		t.Fatal(err)
	}
	newIDs := results[0]["new"].([]int)
	if len(newIDs) != 1 || newIDs[0] != 4252697 {
		t.Fatalf("new ids %v", newIDs)
	}
}
