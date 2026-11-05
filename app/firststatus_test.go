package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// jump-to-first-status-page（decision 2026-11-05）。test の external API 呼出し方針は
// docs/ai/decisions/2026-11-01-hitomi-hub-tests-may-call-external-api.md（fetch は inject fixture）。

func firstStatusTestViewer(t *testing.T, dataset []uint32, infos map[string]string) (*viewer, string, *int) {
	t.Helper()
	db := openTestDB(t)
	root := t.TempDir()
	galleryRoot := filepath.Join(root, "Galleries")
	if err := os.MkdirAll(galleryRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	v := newViewer(db, defaultConfigForTest(), root)
	v.client.testGateIntervalMS = 1
	fetches := 0
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		if strings.Contains(rawurl, ".nozomi") {
			return beIDs(dataset), nil
		}
		if strings.Contains(rawurl, "/galleries/") {
			if text, ok := infos[rawurl[strings.Index(rawurl, "/galleries/")+len("/galleries/"):]]; ok {
				return []byte(text), nil
			}
			return nil, errors.New("no fixture galleryinfo for " + rawurl)
		}
		return nil, errors.New("unexpected fetch " + rawurl)
	}
	return v, galleryRoot, &fetches
}

func firstStatusInfoFixture(galleryID string, hashes []string) string {
	parts := []string{}
	for _, h := range hashes {
		parts = append(parts, `{"hash":"`+h+`","name":"001.png"}`)
	}
	return `var galleryinfo = {"id":"` + galleryID + `","title":"T","files":[` + strings.Join(parts, ",") + `]}`
}

func insertStatusTestFileRow(t *testing.T, db *sql.DB, galleryID int, hash string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO files (path, gallery_id, sha256, size, scan_status, scan_source, scan_time)
		VALUES (?, ?, ?, 1, 'pass', 'test', ?)`, "Galleries/"+strconv.Itoa(galleryID)+"/"+hash+".webp", galleryID, hash, nowISO())
	if err != nil {
		t.Fatal(err)
	}
}

func TestFirstStatusRankToPageConversion(t *testing.T) {
	dataset := []uint32{}
	for id := 100; id < 130; id++ {
		dataset = append(dataset, uint32(id))
	}
	v, _, _ := firstStatusTestViewer(t, dataset, map[string]string{})
	db := v.db
	if err := recordDiscovered(db, 127, galleryDir(v.root, 127)); err != nil {
		t.Fatal(err)
	}
	if err := setViewed(db, 127); err != nil {
		t.Fatal(err)
	}

	res, err := v.firstStatus("female:yuri", "all", statusMaskDefault)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Hit {
		t.Fatal("hit expected at rank 27")
	}
	if res.Rank != 27 || res.Page != 2 {
		t.Errorf("rank→page: rank %d page %d (want 27/2)", res.Rank, res.Page)
	}
	if res.ID != 127 || res.Total != 30 {
		t.Errorf("hit id/total: %d/%d", res.ID, res.Total)
	}
	if res.Flags["viewed"] != 1 || res.Flags["read"] != 0 || res.Flags["saved"] != 0 || res.Flags["favorite"] != 0 {
		t.Errorf("hit flags: %v", res.Flags)
	}
}

func TestFirstStatusStatusMaskMembership(t *testing.T) {
	dataset := []uint32{201, 202, 203}
	v, _, _ := firstStatusTestViewer(t, dataset, map[string]string{})
	db := v.db
	for _, id := range []int{201, 202} {
		if err := recordDiscovered(db, id, galleryDir(v.root, id)); err != nil {
			t.Fatal(err)
		}
	}
	if err := setViewed(db, 201); err != nil {
		t.Fatal(err)
	}
	if err := setFavorite(db, 202, true); err != nil {
		t.Fatal(err)
	}

	// mask = favorite のみのとき viewed candidate（rank 0）は hit にならない
	res, err := v.firstStatus("female:yuri", "all", statusBitFavorite)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Hit || res.ID != 202 || res.Rank != 1 || res.Page != 1 {
		t.Fatalf("favorite-only mask: %v", res)
	}
	if res.Flags["favorite"] != 1 || res.Flags["viewed"] != 0 {
		t.Errorf("favorite-only flags: %v", res.Flags)
	}

	// mask = viewed|favorite（複数選択）のときは rank 順の最初の一 hit
	res, err = v.firstStatus("female:yuri", "all", statusBitViewed|statusBitFavorite)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Hit || res.ID != 201 {
		t.Fatalf("multi mask: %v", res)
	}

	// CLI/API の status 名 parse（単一/複数・既定=いずれか）
	if mask, err := parseStatusMask("favorite"); err != nil || mask != statusBitFavorite {
		t.Errorf("parse favorite: %v %v", mask, err)
	}
	if mask, err := parseStatusMask("viewed,favorite"); err != nil || mask != statusBitViewed|statusBitFavorite {
		t.Errorf("parse viewed,favorite: %v %v", mask, err)
	}
	if mask, err := parseStatusMask(""); err != nil || mask != statusMaskDefault {
		t.Errorf("default mask: %v %v", mask, err)
	}
	if _, err := parseStatusMask("bogus"); err == nil {
		t.Error("unknown status name must error")
	}
}

func TestFirstStatusSavedStrictPartial(t *testing.T) {
	hashA := "1111111111111111111111111111111111111111111111111111111111111111"
	hashB := "2222222222222222222222222222222222222222222222222222222222222222"
	hashC := "3333333333333333333333333333333333333333333333333333333333333333"
	hashD := "4444444444444444444444444444444444444444444444444444444444444444"
	infos := map[string]string{
		"202.js": firstStatusInfoFixture("202", []string{hashA, hashB}),
		"205.js": firstStatusInfoFixture("205", []string{hashC, hashD}),
	}
	v, galleryRoot, fetches := firstStatusTestViewer(t, []uint32{201, 202, 203, 204, 205}, infos)
	db := v.db
	for _, id := range []int{202, 205} {
		if err := recordDiscovered(db, id, galleryDir(v.root, id)); err != nil {
			t.Fatal(err)
		}
	}
	// files rows = saved pre-filter（scan 記録由来）。未 scan の filesystem 単独ファイルは
	// candidate に出ない（限界・タスク記録に記載）。
	insertStatusTestFileRow(t, db, 202, hashA)
	if err := setViewed(db, 202); err != nil {
		t.Fatal(err)
	}
	// 202: partial save（disk に hashA のみ・galleryinfo 2 entry）→ strict saved=0
	if err := os.MkdirAll(filepath.Join(galleryRoot, "202"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(galleryRoot, "202", hashA+".webp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 205: full save（全 entry local）
	if err := os.MkdirAll(filepath.Join(galleryRoot, "205"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{hashC, hashD} {
		if err := os.WriteFile(filepath.Join(galleryRoot, "205", h+".webp"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		insertStatusTestFileRow(t, db, 205, h)
	}

	// mask = saved: partial（rank 1）は strict 判定で skip、full save の rank 4 が hit。
	// walk 中の info fetch は hit candidate 分だけ（202・205 の 2 件・lazy）。
	res, err := v.firstStatus("female:yuri", "all", statusBitSaved)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Hit || res.ID != 205 || res.Rank != 4 || res.Flags["saved"] != 1 {
		t.Fatalf("saved-only strict: %v", res)
	}
	if *fetches != 3 {
		t.Errorf("lazy saved check must fetch info only for hit candidates: fetches %d (want 3=1 dataset+2 info)", *fetches)
	}

	// mask = saved|viewed: rank 1（partial だが viewed）が hit。flags は strict 結果（saved=0・viewed=1）
	before := *fetches
	res, err = v.firstStatus("female:yuri", "all", statusBitSaved|statusBitViewed)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Hit || res.ID != 202 || res.Rank != 1 || res.Flags["viewed"] != 1 || res.Flags["saved"] != 0 {
		t.Fatalf("strict/partial distinction: %v", res)
	}
	// cache 共有（card 描画と同一源 v.info・searchCache）: 2 度目の呼出で fetch が増えない
	if *fetches != before {
		t.Errorf("second jump call must not refetch (shared caches): %d → %d", before, *fetches)
	}
}

func TestFirstStatusRandomOrderError(t *testing.T) {
	v, _, fetches := firstStatusTestViewer(t, []uint32{10, 20}, map[string]string{})
	_, err := v.firstStatus("female:yuri orderby:random", "all", statusMaskDefault)
	if err == nil || err.Error() != "random order is not supported" {
		t.Fatalf("random error: %v", err)
	}
	if *fetches != 0 {
		t.Errorf("random must error before candidate set and dataset fetch: fetches %d", *fetches)
	}
}

func TestFirstStatusNoHitNull(t *testing.T) {
	v, _, _ := firstStatusTestViewer(t, []uint32{301, 302}, map[string]string{})
	res, err := v.firstStatus("female:yuri", "all", statusMaskDefault)
	if err != nil {
		t.Fatal(err)
	}
	if res.Hit {
		t.Fatalf("no hit must be null: %v", res)
	}
	out := firstStatusJSON("female:yuri", "all", statusMaskDefault, res)
	if out["hit"] != nil {
		t.Errorf("no hit JSON must null hit: %v", out)
	}
	if out["total"] != 2 {
		t.Errorf("no hit total: %v", out["total"])
	}
}

func TestFirstStatusSharedSearchCacheNoRefetch(t *testing.T) {
	// 単一 cache 経路（統合条件 1）: SearchHTML（既存 route）は dataset を 1 回 fetch して
	// cache してから slice。jump は同一 cacheKey を共有し、2 度目の呼出で fetch が増えない。
	v, _, fetches := firstStatusTestViewer(t, []uint32{401, 402, 403, 404}, map[string]string{})
	db := v.db
	if err := recordDiscovered(db, 402, galleryDir(v.root, 402)); err != nil {
		t.Fatal(err)
	}
	if err := setFavorite(db, 402, true); err != nil {
		t.Fatal(err)
	}

	if _, _, err := v.client.SearchHTML("female:yuri", "all", 1, 25, false); err != nil {
		t.Fatal(err)
	}
	if *fetches != 1 {
		t.Fatalf("first search fetches dataset once: %d", *fetches)
	}
	before := *fetches
	res, err := v.firstStatus("female:yuri", "all", statusBitFavorite)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Hit || res.ID != 402 {
		t.Fatalf("hit: %v", res)
	}
	if *fetches != before {
		t.Errorf("jump must share searchCache (no additional dataset fetch): %d → %d", before, *fetches)
	}
	before = *fetches
	res, err = v.firstStatus("female:yuri", "all", statusBitFavorite)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Hit || res.ID != 402 {
		t.Fatalf("second hit: %v", res)
	}
	if *fetches != before {
		t.Errorf("second jump call must not refetch: %d → %d", before, *fetches)
	}
}

func TestSearchFirstStatusHandlerShapeAndReadonly(t *testing.T) {
	// API は foreground・read-only。DB 書込み（galleries 行）が増えないこと。
	v, _, _ := firstStatusTestViewer(t, []uint32{501, 502}, map[string]string{})
	db := v.db
	if err := recordDiscovered(db, 502, galleryDir(v.root, 502)); err != nil {
		t.Fatal(err)
	}
	if err := setViewed(db, 502); err != nil {
		t.Fatal(err)
	}
	var galleriesBefore int
	if err := db.QueryRow("SELECT COUNT(1) FROM galleries").Scan(&galleriesBefore); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/search-first-status?query=female%3Ayuri&lang=all", nil))
	if rec.Code != 200 {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	hit, ok := out["hit"].(map[string]any)
	if !ok || hit["page"] != float64(1) || hit["rank"] != float64(1) || hit["gallery_id"] != float64(502) {
		t.Errorf("API hit shape: %v", out)
	}
	if out["total"] != float64(2) {
		t.Errorf("API total: %v", out["total"])
	}
	flags, ok := hit["flags"].(map[string]any)
	if !ok || flags["viewed"] != float64(1) {
		t.Errorf("API flags: %v", hit["flags"])
	}
	var galleriesAfter int
	if err := db.QueryRow("SELECT COUNT(1) FROM galleries").Scan(&galleriesAfter); err != nil {
		t.Fatal(err)
	}
	if galleriesAfter != galleriesBefore {
		t.Errorf("jump endpoint must be read-only: galleries %d → %d", galleriesBefore, galleriesAfter)
	}

	// random → 400 error（decision 2026-11-05 random 無効）
	rec = httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/search-first-status?query=female%3Ayuri+orderby%3Arandom", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("random status: %d", rec.Code)
	}
	var errOut map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &errOut); err != nil {
		t.Fatal(err)
	}
	if errOut["error"] != "random order is not supported" {
		t.Errorf("random error body: %v", errOut)
	}

	// hit なし → hit null（status 行「該当なし」の UI 源）
	rec = httptest.NewRecorder()
	v.handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/search-first-status?query=female%3Ayuri&status=favorite", nil))
	if rec.Code != 200 {
		t.Fatalf("no-hit status: %d", rec.Code)
	}
	out = map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["hit"] != nil || out["total"] != float64(2) {
		t.Errorf("no-hit JSON: %v", out)
	}
}
