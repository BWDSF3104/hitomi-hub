package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// Phase 3（decision 2026-11-05・リモートタスク 2026-11-05-hitomi-hub-search-phase3-gallery-info-persistent-cache）:
// gallery 情報の永続キャッシュ。置き場は完成フォルダ内 `cache/galleryinfo/<id>.json`（1 gallery 1 ファイル）、
// 保存内容は parse 済み `GalleryInfo` の JSON + 取得時刻 `fetched_at`。参照順は memory → disk → remote。
// 画像バイト経路（imageProxy・TN 経路・full cache）とは分離し、ゲート間隔・リトライ方針は変更しない。

// persistedGalleryInfo は disk 層の JSON 形。
type persistedGalleryInfo struct {
	GalleryInfo
	FetchedAt time.Time `json:"fetched_at"`
}

func (v *viewer) infoCachePath(id int) string {
	return filepath.Join(v.infoCacheRoot, strconv.Itoa(id)+".json")
}

// readInfoDisk は disk 層を読む（infoDiskMu を内部で保持）。ファイル不在は nil。
// 内容が壊れている（JSON parse 不可・id 不一致）場合はミス扱い、そのファイルを除去して nil を返す。
// 壊れた内容を根拠にエラーを返さない。
func (v *viewer) readInfoDisk(id int) *persistedGalleryInfo {
	v.infoDiskMu.Lock()
	defer v.infoDiskMu.Unlock()

	path := v.infoCachePath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entry persistedGalleryInfo
	if err := json.Unmarshal(data, &entry); err != nil || entry.ID != id {
		v.removeInfoDiskLocked(path)
		return nil
	}
	return &entry
}

// writeInfoDisk は remote 成功時の永続書き込み。atomic 書き込みの既存流儀（A-07a 同型:
// temp へ書いて close 後に rename・temp は infoCacheRoot 内・失敗時 temp cleanup）。
// 書き込み後の上限超過分を、取得時刻古い順に間引く（exec 時: 書き込み時・scan は超過時のみ）。
func (v *viewer) writeInfoDisk(id int, info *GalleryInfo) error {
	v.infoDiskMu.Lock()
	defer v.infoDiskMu.Unlock()

	entry := persistedGalleryInfo{GalleryInfo: *info, FetchedAt: time.Now()}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	path := v.infoCachePath(id)
	var oldSize int64
	if st, err := os.Stat(path); err == nil {
		oldSize = st.Size()
	}
	if err := v.writeInfoCacheFileAtomically(path, data); err != nil {
		return err
	}
	if !v.infoDiskSynced {
		v.syncInfoDiskLocked()
	} else {
		if oldSize == 0 {
			v.infoDiskCount++
		}
		v.infoDiskBytes += int64(len(data)) - oldSize
	}
	v.pruneInfoDiskLocked()
	return nil
}

// writeInfoCacheFileAtomically は temp+rename の atomic 書き込み（A-07a 流儀）。existing target は
// 更新（TTL 再取得後の永続更新）、OS temp dir・scratch/・output/ は使わない。
func (v *viewer) writeInfoCacheFileAtomically(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".galleryinfo-*.tmp")
	if err != nil {
		return err
	}
	cleanupTemp := func() {
		_ = os.Remove(tmp.Name())
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanupTemp()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanupTemp()
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		cleanupTemp()
		return err
	}
	return nil
}

// syncInfoDiskLocked must be called with infoDiskMu held. Accounting baseline scan（再起動横断の
// 上限判定用。呼び出しは書き込み時・読み取りでは scan しない）。
func (v *viewer) syncInfoDiskLocked() {
	v.infoDiskCount = 0
	v.infoDiskBytes = 0
	v.infoDiskSynced = true
	entries, err := os.ReadDir(v.infoCacheRoot)
	if err != nil {
		return
	}
	for _, de := range entries {
		if de.IsDir() || filepath.Ext(de.Name()) != ".json" {
			continue
		}
		if st, err := de.Info(); err == nil {
			v.infoDiskCount++
			v.infoDiskBytes += st.Size()
		}
	}
}

// pruneInfoDiskLocked must be called with infoDiskMu held. 上限（件数/容量の先に到達した方）
// 超過時のみ scan し、取得時刻（JSON field 正・parse 不可は mtime fallback）古い順に evict する。
// evict 中に壊れたファイルを見つけた場合は同じミス扱いで除去する（scan はこのとき必要分だけ）。
func (v *viewer) pruneInfoDiskLocked() {
	maxEntries, maxBytes := boundedMax(v.infoDiskMaxEntries, v.infoDiskMaxBytes, defaultInfoDiskMaxEntries, defaultInfoDiskMaxBytes)
	if v.infoDiskCount <= maxEntries && v.infoDiskBytes <= int64(maxBytes) {
		return
	}

	type diskEntry struct {
		path      string
		size      int64
		fetchedAt time.Time
	}
	var kept []diskEntry
	v.infoDiskCount = 0
	v.infoDiskBytes = 0
	entries, err := os.ReadDir(v.infoCacheRoot)
	if err != nil {
		return
	}
	for _, de := range entries {
		if de.IsDir() || filepath.Ext(de.Name()) != ".json" {
			continue
		}
		full := filepath.Join(v.infoCacheRoot, de.Name())
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		var entry persistedGalleryInfo
		fetchedAt := time.Time{}
		if err := json.Unmarshal(data, &entry); err != nil {
			// 壊れたファイルはミス扱い・除去
			_ = os.Remove(full)
			continue
		}
		if entry.FetchedAt.IsZero() {
			if st, err := de.Info(); err == nil {
				fetchedAt = st.ModTime()
			} else {
				fetchedAt = time.Now()
			}
		} else {
			fetchedAt = entry.FetchedAt
		}
		st, err := de.Info()
		if err != nil {
			continue
		}
		kept = append(kept, diskEntry{path: full, size: st.Size(), fetchedAt: fetchedAt})
		v.infoDiskCount++
		v.infoDiskBytes += st.Size()
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].fetchedAt.Before(kept[j].fetchedAt) })
	for _, e := range kept {
		if v.infoDiskCount <= maxEntries && v.infoDiskBytes <= int64(maxBytes) {
			break
		}
		if err := os.Remove(e.path); err != nil {
			continue
		}
		v.infoDiskCount--
		v.infoDiskBytes -= e.size
	}
}

func (v *viewer) removeInfoDiskLocked(path string) {
	if v.infoDiskSynced {
		if st, err := os.Stat(path); err == nil {
			v.infoDiskCount--
			v.infoDiskBytes -= st.Size()
		}
	}
	_ = os.Remove(path)
}

// recordInfoDiskStaleHit は stale 暫定表示の判別口（metrics/ログ要求に対する採用方針 2026-11-05）。
// stale hit は fetch metrics（kind `gallery`・network 呼出し基準）に現れないため、カウンターと
// stderr ログで判別する。
func (v *viewer) recordInfoDiskStaleHit(id int, fetchedAt time.Time) {
	v.infoDiskMu.Lock()
	v.infoDiskStaleHits++
	v.infoDiskMu.Unlock()
	log.Printf("galleryinfo stale served id=%d age=%s", id, time.Since(fetchedAt).Round(time.Second))
}
