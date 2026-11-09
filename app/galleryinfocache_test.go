package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Phase 3 mock tests（外部 API を呼ばない・既存 `client.fetch` hook 方式）。
// 合格条件 2 の固定項目: 永続 hit で remote 呼出なし / TTL 切れで remote 再取得・永続更新 /
// 壊れた永続ファイルは除去して再取得 / 上限超過で取得時刻古い順 evict。

func galleryJSONForID(id int, title string) []byte {
	return []byte("var galleryinfo = {\"id\":\"" + strconv.Itoa(id) + "\",\"title\":\"" + title + "\",\"files\":[]}")
}

func writeInfoDiskFileForTest(t *testing.T, v *viewer, id int, fetchedAt time.Time) string {
	t.Helper()
	if err := os.MkdirAll(v.infoCacheRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	path := v.infoCachePath(id)
	data, err := json.Marshal(persistedGalleryInfo{
		GalleryInfo: GalleryInfo{ID: id, Title: "pre", Files: []FileEntry{}},
		FetchedAt:   fetchedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// 1. 永続キャッシュが存在する ID は remote を呼ばずに情報が返る（再起動横断・ゲート非消費）。
func TestViewerInfoDiskPersistentHitDoesNotCallRemote(t *testing.T) {
	v, _ := newTestViewer(t, "")
	fetches := 0
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		id, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rawurl, "https://ltn."+hitomiDomain+"/galleries/"), ".js"))
		if err != nil {
			return nil, err
		}
		return galleryJSONForID(id, "T"+strconv.Itoa(id)), nil
	}

	info, err := v.info(7, false)
	if err != nil {
		t.Fatalf("info 7: %s", err)
	}
	if info.Title != "T7" || fetches != 1 {
		t.Fatalf("first call should fetch once and persist: fetches %d title %q", fetches, info.Title)
	}
	if _, err := os.Stat(v.infoCachePath(7)); err != nil {
		t.Fatalf("persistent file must exist: %s", err)
	}

	// server 再起動相当: 同じ root（disk 層は残る）で new viewer。memory は空。
	v2 := newViewer(v.db, defaultConfigForTest(), v.root)
	v2.client.fetch = v.client.fetch
	got, err := v2.info(7, false)
	if err != nil {
		t.Fatalf("info 7 after restart: %s", err)
	}
	if fetches != 1 {
		t.Fatalf("persistent hit must not call remote: fetches %d", fetches)
	}
	if got.Title != "T7" {
		t.Errorf("persistent hit content: got %q", got.Title)
	}
	v2.infoCacheMu.Lock()
	loaded, ok := v2.infoCache[7]
	v2.infoCacheMu.Unlock()
	if !ok || loaded.info.ID != 7 {
		t.Error("persistent hit must load memory layer")
	}
}

// 2. TTL 切れでは remote で取り直し、永続側が更新される。
func TestViewerInfoDiskTTLExpiredRefetchesAndUpdatesPersistentFile(t *testing.T) {
	v, _ := newTestViewer(t, "")
	fetches := 0
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		id, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rawurl, "https://ltn."+hitomiDomain+"/galleries/"), ".js"))
		if err != nil {
			return nil, err
		}
		return galleryJSONForID(id, "T"+strconv.Itoa(id)), nil
	}

	staleAt := time.Now().Add(-31 * 24 * time.Hour)
	path := writeInfoDiskFileForTest(t, v, 8, staleAt)

	info, err := v.info(8, false)
	if err != nil {
		t.Fatalf("info 8: %s", err)
	}
	if fetches != 1 {
		t.Fatalf("TTL-expired entry must refetch: fetches %d", fetches)
	}
	if info.Title != "T8" {
		t.Errorf("refetch content: got %q", info.Title)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entry persistedGalleryInfo
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("persisted file must remain valid JSON: %s", err)
	}
	if time.Since(entry.FetchedAt) > 10*time.Second {
		t.Errorf("persisted fetched_at must be updated on refetch: got %s", entry.FetchedAt)
	}

	// 更新後の永続は TTL 内 → memory を空にするとの remote なしで返る。
	v.infoCacheMu.Lock()
	v.infoCache = map[int]*infoCacheEntry{}
	v.infoCacheMu.Unlock()
	if _, err := v.info(8, false); err != nil {
		t.Fatal(err)
	}
	if fetches != 1 {
		t.Errorf("updated persistent entry must serve without remote: fetches %d", fetches)
	}
}

// 3. 壊れた永続ファイルはミス扱いになり、除去されて remote で取り直せる。
func TestViewerInfoDiskCorruptedFileRemovedAndRefetched(t *testing.T) {
	v, _ := newTestViewer(t, "")
	fetches := 0
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		id, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rawurl, "https://ltn."+hitomiDomain+"/galleries/"), ".js"))
		if err != nil {
			return nil, err
		}
		return galleryJSONForID(id, "T"+strconv.Itoa(id)), nil
	}

	if err := os.MkdirAll(v.infoCacheRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	path := v.infoCachePath(9)
	if err := os.WriteFile(path, []byte(`not json {`), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := v.info(9, false)
	if err != nil {
		t.Fatalf("corrupted persistent file must not return error: %s", err)
	}
	if fetches != 1 {
		t.Fatalf("corrupted entry must refetch: fetches %d", fetches)
	}
	if info.Title != "T9" {
		t.Errorf("refetch content: got %q", info.Title)
	}

	// 壊れた内容のファイルは除去され、remote 成功で valid JSON に更新されている。
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("refetched entry must persist valid JSON: %s", err)
	}
	var entry persistedGalleryInfo
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("persisted file still corrupted: %s", err)
	}
	if entry.ID != 9 {
		t.Errorf("persisted id mismatch: got %d", entry.ID)
	}

	v.infoCacheMu.Lock()
	v.infoCache = map[int]*infoCacheEntry{}
	v.infoCacheMu.Unlock()
	if _, err := v.info(9, false); err != nil {
		t.Fatal(err)
	}
	if fetches != 1 {
		t.Errorf("repaired persistent entry must serve without remote: fetches %d", fetches)
	}

	// id 不一致（parse できるが内容が別 gallery）も壊れた扱いで除去・再取得。
	mismatchData, err := json.Marshal(persistedGalleryInfo{GalleryInfo: GalleryInfo{ID: 999, Title: "wrong", Files: []FileEntry{}}, FetchedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(v.infoCachePath(10), mismatchData, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := v.info(10, false); err != nil {
		t.Fatalf("id-mismatch file must not return error: %s", err)
	}
	if fetches != 2 {
		t.Fatalf("id-mismatch entry must refetch: fetches %d", fetches)
	}
	if _, err := os.Stat(v.infoCachePath(10)); err != nil {
		t.Errorf("id-mismatch file must be replaced by valid entry: %s", err)
	}
}

// 4. 上限超過時に古いもの（取得時刻順）から evict される。テストは insert 時刻を固定して決定化
// （known-issues archived の LRU tie 決定化の教訓）。
func TestViewerInfoDiskEvictsOldestByFetchedAtWhenOverEntryCap(t *testing.T) {
	v, _ := newTestViewer(t, "")
	fetches := 0
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		id, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rawurl, "https://ltn."+hitomiDomain+"/galleries/"), ".js"))
		if err != nil {
			return nil, err
		}
		return galleryJSONForID(id, "T"+strconv.Itoa(id)), nil
	}
	v.infoDiskMaxEntries = 3
	v.infoDiskMaxBytes = 1 << 20

	base := time.Now().Add(-40 * 24 * time.Hour)
	writeInfoDiskFileForTest(t, v, 1, base)
	writeInfoDiskFileForTest(t, v, 2, base.Add(10*time.Second))
	writeInfoDiskFileForTest(t, v, 3, base.Add(20*time.Second))

	if _, err := v.info(4, false); err != nil {
		t.Fatalf("info 4: %s", err)
	}
	if fetches != 1 {
		t.Fatalf("evict test setup must fetch only for new id: fetches %d", fetches)
	}
	if _, err := os.Stat(v.infoCachePath(1)); err == nil {
		t.Error("oldest entry (by fetched_at) must be evicted when over entry cap")
	}
	for _, id := range []int{2, 3, 4} {
		if _, err := os.Stat(v.infoCachePath(id)); err != nil {
			t.Errorf("entry %d must remain: %s", id, err)
		}
	}
	// hysteresis（案1・ユーザー同意 2026-11-09）: evict は target（上限の 90%・端数切捨て）まで。
	// cap 3 では 10% が 0 件なので target == cap。scan 頻度 1/501 への減少は scratch harness
	// 実測（docs/ai/tmp/assess-20261109-1320/）で確認。
	if target := v.infoDiskMaxEntries - v.infoDiskMaxEntries/10; v.infoDiskCount != target {
		t.Errorf("disk count must stay at hysteresis target after evict: count %d target %d", v.infoDiskCount, target)
	}
	if v.infoDiskBytes > int64(v.infoDiskMaxBytes) {
		t.Errorf("disk bytes must stay below cap: bytes %d", v.infoDiskBytes)
	}

	// evict された id は disk hit が消えたので remote 再取得（count/bytes は scan 実測へ同期済み）。
	v.infoCacheMu.Lock()
	v.infoCache = map[int]*infoCacheEntry{}
	v.infoCacheMu.Unlock()
	if _, err := v.info(1, false); err != nil {
		t.Fatal(err)
	}
	if fetches != 2 {
		t.Errorf("evicted id must refetch from remote: fetches %d", fetches)
	}
	if v.infoDiskCount != 3 || v.infoDiskBytes > int64(v.infoDiskMaxBytes) {
		t.Errorf("disk must stay bounded after refetch insert: count %d bytes %d", v.infoDiskCount, v.infoDiskBytes)
	}
}

// 5. stale 暫定表示の採用（採用推奨方針の実装): remote 失敗時だけ期限切れ永続 entry を暫定表示、
// counter とログで stale と判別可。永続なしの remote 失敗はエラー。
func TestViewerInfoDiskStaleEntryUsedOnlyWhenRemoteFails(t *testing.T) {
	v, _ := newTestViewer(t, "")
	fetches := 0
	v.client.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		fetches++
		return nil, errors.New("remote failure")
	}

	// 永続なし + remote 失敗 → エラー（stale 表示なし）。
	if _, err := v.info(11, false); err == nil {
		t.Fatal("remote failure without persistent entry must return error")
	}

	// 期限切れ永続 + remote 失敗 → stale 暫定表示・counter で判別。
	writeInfoDiskFileForTest(t, v, 12, time.Now().Add(-31*24*time.Hour))
	info, err := v.info(12, false)
	if err != nil {
		t.Fatalf("stale entry must serve tentatively on remote failure: %s", err)
	}
	if info.ID != 12 || info.Title != "pre" {
		t.Errorf("stale content: got %v", info)
	}
	v.infoDiskMu.Lock()
	hits := v.infoDiskStaleHits
	v.infoDiskMu.Unlock()
	if hits != 1 {
		t.Errorf("stale hit counter must identify stale use: hits %d", hits)
	}
	if fetches != 2 {
		t.Errorf("stale path requires the failed remote attempt only: fetches %d", fetches)
	}
}

func boundedMaxInt(value, def int) int {
	if value <= 0 {
		return def
	}
	return value
}
