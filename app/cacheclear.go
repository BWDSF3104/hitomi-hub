package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// 手動キャッシュクリア（decision docs/ai/decisions/2026-11-09-manual-cache-clear-button-approved-design.md）。
// R-15 の「件数・作成順・status を filesystem scan で数えない」条は無変更。ここでの filesystem 列挙は
// 削除対象の列挙のみで、件数判定には使わない。UI の削除前表示は DB 行だけ（walk preview を置かない）。
// 完成フォルダ（`Galleries`・`acquisition`）と `scan_cache`（`hitomi.db`）は対象外
// （docs/ai/decisions/2026-11-09-completion-folders-no-delete-rule.md）。

// 削除列挙の上限（bounded 流儀 C-05/A-14）。正典既定は cap 上限 maxCacheDiskMaxEntries。
// test inject は `cacheClearLimit` field（testGateIntervalMS と同じ口）。超過分は削除せず truncated として報告。

var cacheClearScopes = []string{"full", "thumbs", "galleryinfo", "memory"}

func isCacheClearScope(scope string) bool {
	for _, allowed := range cacheClearScopes {
		if scope == allowed {
			return true
		}
	}
	return false
}

type cacheClearCounts struct {
	Full           int `json:"full"`
	Thumbs         int `json:"thumbs"`
	GalleryInfo    int `json:"galleryinfo"`
	UnlistedFull   int `json:"unlisted_full"`
	UnlistedThumbs int `json:"unlisted_thumbs"`
	Memory         int `json:"memory"`
}

type cacheClearResult struct {
	Deleted        cacheClearCounts `json:"deleted"`
	RowsDeleted    int              `json:"rows_deleted"`
	SkippedInflight int             `json:"skipped_inflight"`
	Failed         int              `json:"failed"`
	Truncated      bool             `json:"truncated"`
}

// cacheFileNameKind は cache 命名の file を kind に写像する（削除列挙用・件数判定源ではない）。
// full = `<hash>.<format>`、thumbs = `<hash>.<format>.<variant>`。正典 helper（validHash・normalizeFormat・
// TNVariantSuffix）を使い、命名に一致しない file（temp・別内容）は対象にしない。
func cacheFileNameKind(name string) (string, bool) {
	parts := strings.Split(name, ".")
	if len(parts) == 2 && validHash(parts[0]) && normalizeFormat(parts[1]) != "" {
		return "full", true
	}
	if len(parts) == 3 && validHash(parts[0]) && normalizeFormat(parts[1]) != "" && TNVariantSuffix(parts[2]) == parts[2] {
		return "thumbs", true
	}
	return "", false
}

// cacheClearRootGuard は完成フォルダへ向く解決を拒否する。完成フォルダ不削除条の UI 操作版。
func (v *viewer) cacheClearRootGuard() error {
	targets := []string{v.cacheRoot, v.infoCacheRoot}
	forbidden := []string{v.galleryRoot, v.acquisitionRoot}
	for _, target := range targets {
		if target == "" {
			return errors.New("cache root is not resolved")
		}
		if !withinRoot(v.root, target) {
			return fmt.Errorf("cache root %q is outside root", target)
		}
		for _, folder := range forbidden {
			if folder == "" {
				continue
			}
			if target == folder || withinRoot(folder, target) {
				return fmt.Errorf("cache root %q resolves into completion folder %q", target, folder)
			}
		}
	}
	return nil
}

func (v *viewer) clearCache(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var body struct {
		Confirm bool     `json:"confirm"`
		Scope   []string `json:"scope"`
	}
	if err := decoder.Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !body.Confirm {
		writeJSONError(w, http.StatusBadRequest, "confirm must be true")
		return
	}
	if len(body.Scope) == 0 {
		writeJSONError(w, http.StatusBadRequest, "scope must list cache layers")
		return
	}
	scopes := map[string]bool{}
	for _, scope := range body.Scope {
		if !isCacheClearScope(scope) {
			writeJSONError(w, http.StatusBadRequest, "scope must be one of: full, thumbs, galleryinfo, memory")
			return
		}
		scopes[scope] = true
	}
	if err := v.cacheClearRootGuard(); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := v.clearCaches(scopes)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "cache clear failed: "+err.Error())
		return
	}
	jsonOut(w, result)
}

// clearCaches は prune と同一 mutex（cacheEntriesMu）で直列化する。network を含まない。
func (v *viewer) clearCaches(scopes map[string]bool) (*cacheClearResult, error) {
	v.cacheEntriesMu.Lock()
	defer v.cacheEntriesMu.Unlock()

	result := &cacheClearResult{Deleted: cacheClearCounts{}}

	known := map[string]bool{}
	if scopes["full"] || scopes["thumbs"] {
		if err := v.clearCacheRowsLocked(scopes, known, result); err != nil {
			return nil, err
		}
		if err := v.clearUnlistedLocked(scopes, known, result); err != nil {
			return nil, err
		}
	}
	if scopes["galleryinfo"] {
		result.Deleted.GalleryInfo = v.clearInfoDiskForClear()
	}
	if scopes["memory"] {
		v.infoCacheMu.Lock()
		result.Deleted.Memory = len(v.infoCache)
		v.infoCache = map[int]*infoCacheEntry{}
		v.infoCacheBytes = 0
		v.infoCacheMu.Unlock()
		if v.client != nil {
			result.Deleted.Memory += v.client.ClearMemoryCaches()
		}
	}
	return result, nil
}

// clearCacheRowsLocked は DB 行由来の file を削除し、成功行だけ DELETE する（prune と同じ流儀:
// 削除失敗の行は保持）。行 key は (hash, format, kind, variant)。
func (v *viewer) clearCacheRowsLocked(scopes map[string]bool, known map[string]bool, result *cacheClearResult) error {
	for _, kind := range []string{"full", "thumbs"} {
		if !scopes[kind] {
			continue
		}
		rows, err := v.db.Query(`SELECT hash, format, variant FROM cache_entries WHERE kind = ?1`, kind)
		if err != nil {
			return err
		}
		type cacheRow struct {
			hash    string
			format  string
			variant string
			path    string
		}
		victims := []cacheRow{}
		for rows.Next() {
			var row cacheRow
			if err := rows.Scan(&row.hash, &row.format, &row.variant); err != nil {
				rows.Close()
				return err
			}
			path, err := cacheEntryPath(v, kind, row.hash, row.format, row.variant)
			if err != nil {
				continue // path を組めない行は保持
			}
			known[path] = true
			victims = append(victims, cacheRow{hash: row.hash, format: row.format, variant: row.variant, path: path})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()

		for _, victim := range victims {
			if v.cacheWriteInFlight(victim.path) {
				result.SkippedInflight++
				continue // write 中の path は触らない（行も保持）
			}
			removed, err := v.removeCacheFile(victim.path)
			if err != nil {
				result.Failed++
				continue // 削除失敗の行は保持
			}
			if removed {
				if kind == "full" {
					result.Deleted.Full++
				} else {
					result.Deleted.Thumbs++
				}
			}
			if _, err := v.db.Exec(`DELETE FROM cache_entries WHERE hash = ?1 AND format = ?2 AND kind = ?3 AND variant = ?4`,
				victim.hash, victim.format, kind, victim.variant); err != nil {
				return err
			}
			result.RowsDeleted++
		}
	}
	return nil
}

// clearUnlistedLocked は「リスト化前」（DB 行がない cache file）を削除する。件数判定ではなく削除列挙
// （decision 2026-11-09-manual-cache-clear-button-approved-design.md）。cache 命名に一致しない file と
// write 中の temp は対象にしない。列挙件数上限超過分は削除せず truncated として報告。
func (v *viewer) clearUnlistedLocked(scopes map[string]bool, known map[string]bool, result *cacheClearResult) error {
	dirs := []string{}
	if scopes["full"] {
		dirs = append(dirs, v.cacheRoot)
	}
	if scopes["thumbs"] {
		dirs = append(dirs, filepath.Join(v.cacheRoot, "thumbs"))
	}
	remaining := v.cacheClearLimit
	if remaining <= 0 {
		remaining = maxCacheDiskMaxEntries
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue // temp（write 中）は skip
			}
			kind, ok := cacheFileNameKind(entry.Name())
			if !ok || !scopes[kind] {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			if known[path] {
				continue // DB 行由来は既に処理済み
			}
			if remaining <= 0 {
				result.Truncated = true
				continue
			}
			remaining--
			if v.cacheWriteInFlight(path) {
				result.SkippedInflight++
				continue
			}
			removed, err := v.removeCacheFile(path)
			if err != nil {
				result.Failed++
				continue
			}
			if removed {
				if kind == "full" {
					result.Deleted.UnlistedFull++
				} else {
					result.Deleted.UnlistedThumbs++
				}
			}
		}
	}
	return nil
}

// cacheWriteInFlight は singleflight で write 中の key 判定（R-16 seam の cacheWriteMu を使う）。
func (v *viewer) cacheWriteInFlight(path string) bool {
	v.cacheWriteMu.Lock()
	defer v.cacheWriteMu.Unlock()
	return v.cacheWriteGroup[path] != nil
}

// removeCacheFile は削除（test seam `cacheRemoveHook` は nil のとき非適用・A-07a seam 流儀）。
// 返す bool は「実際に file を除去したか」。行だけ残って file が不在（ErrNotExist）は除去数に数えない。
func (v *viewer) removeCacheFile(path string) (bool, error) {
	if v.cacheRemoveHook != nil {
		if err := v.cacheRemoveHook(path); err != nil {
			return false, err
		}
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// clearInfoDiskForClear は galleryinfo disk 層の JSON を削除し、counter を 0 へ戻す。
func (v *viewer) clearInfoDiskForClear() int {
	v.infoDiskMu.Lock()
	defer v.infoDiskMu.Unlock()

	entries, err := os.ReadDir(v.infoCacheRoot)
	if err != nil {
		if os.IsNotExist(err) {
			v.infoDiskCount = 0
			v.infoDiskBytes = 0
			v.infoDiskSynced = true
			return 0
		}
		return 0
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if base := strings.TrimSuffix(entry.Name(), ".json"); base == "" || strings.Trim(base, "0123456789") != "" {
			continue // `<gallery_id>.json` 以外の file は対象にしない
		}
		if err := os.Remove(filepath.Join(v.infoCacheRoot, entry.Name())); err != nil {
			continue
		}
		removed++
	}
	v.infoDiskCount = 0
	v.infoDiskBytes = 0
	v.infoDiskSynced = true
	return removed
}

// cacheEntryCounts は settings panel の read-only 表示用（DB 行だけ。walk は使わない）。
func (v *viewer) cacheEntryCounts() map[string]int {
	counts := map[string]int{"full": 0, "thumbs": 0}
	rows, err := v.db.Query(`SELECT kind, COUNT(1) FROM cache_entries GROUP BY kind`)
	if err != nil {
		return counts
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			return counts
		}
		counts[kind] = count
	}
	return counts
}
