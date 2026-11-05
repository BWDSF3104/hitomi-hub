package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// hitomi の data 正典（ltn）。公式 API は無い。正典ドメインは ltn の common.js（domain2）から
const hitomiDomain = "gold-usergeneratedcontent.net"
const galleriesPerPage = 25
const negativeNoFullCacheTTL = 5 * time.Minute

// 項目2（assess-20261105-1015・block 検出）: block signal = 403 / `cf-mitigated` ヘッダ
// （Cloudflare challenge）/ 503 / 522。block 検出時、data・image 両 lane の gate interval を
// クールダウン期間 politeness 既定へ退避する。数値の根拠（推測数値なし）:
//   - `blockCooldownDuration = negativeNoFullCacheTTL`（既存正典 convention 5 分）。UI 側の既存
//     convention `PREFETCH_COOLDOWN_MS = 300000`（`tests/test_search_prefetch.py` の「429 sets a
//     5 minute cooldown in memory」assert）と同一値
//   - `politenessDefaultIntervalMS = 1000`（正典 politeness 退避先。decision
//     `2026-11-05-rate-values-and-concurrency-change-approved.md` のクールダウン退避値。
//     data lane 既定（100）より大きく、退避意味を実測テストで assert）
// `Retry-After`（429・block）の上限も同一 convention（penalty window 5 分）で clamp する。
const blockCooldownDuration = negativeNoFullCacheTTL
const politenessDefaultIntervalMS = 1000

// stale-only bounded memory limit（ユーザー承認方針 2026-11-04・
// decisions/2026-11-04-remaining-residual-approval-split.md）。cap は entry 数だけ。
// active entry（TTL 内）は protected: cap 超過でも eviction しない（C-08 の TTL 内
// negative cache hit 保証を変えない）。insert 後に cap 超過のとき、TTL 超過済みの
// stale key（二度と hit されない項目）だけを掃除する。hard cap で active entry まで
// 消す方式は C-08 意味変更として別途承認が必要（未承認）。
const defaultNoFullCacheMaxEntries = 1024

type HitomiConfig struct {
	MinIntervalMS int    `json:"min_interval_ms"`
	ImageMinIntervalMS int `json:"image_min_interval_ms"`
	GGForceMinIntervalMS int `json:"gg_force_min_interval_ms"`
	MaxRetries    int    `json:"max_retries"`
	RetryBackoffMS int   `json:"retry_backoff_ms"`
	UserAgent     string `json:"user_agent"`
	ImageFormat   string `json:"image_format"`
	UseLiveGG     bool   `json:"use_live_gg"`
	// Phase 4（decision 2026-11-05）: background gate の foreground grace。既定 300ms。
	BgGraceMS int `json:"bg_grace_ms"`
}

const ggCacheTTL = time.Hour
const indexVersionTTL = time.Hour
const searchCacheTTL = time.Hour
const topCacheTTL = time.Hour

// Phase 3（decision 2026-11-05）: gallery 情報永続キャッシュの TTL。gallery のメタ情報は
// 基本的に不変という前提。TTL 切れは remote 再取得、remote 失敗時のみ stale 暫定表示。
const galleryInfoCacheTTL = 30 * 24 * time.Hour

// C-05/A-14: cache は entry 数上限（LRU）と size cap を持つ。hit 時に access order を
// 更新し、insert 後に上限超過分を最も古い access から eviction する。既定は conservative。
const (
	defaultSearchCacheMaxEntries    = 256
	defaultSearchCacheMaxBytes      = 8 << 20
	defaultIndexDataCacheMaxEntries = 512
	defaultIndexDataCacheMaxBytes   = 8 << 20
	defaultIndexNodeCacheMaxEntries = 1024
	defaultIndexNodeCacheMaxBytes   = 1 << 20
	// C-05 観察実測（2026-11-04）で version rotation の growth を確認し bounded 化。
	// cap は構造同一 cache（indexNodeCache: bnode entry・entrySize = maxNodeSize・topCacheTTL）の
	// 既存 convention 値をそのまま採用（invented number なし・判断 2026-11-04）。
	defaultIndexTopCacheMaxEntries = 1024
	defaultIndexTopCacheMaxBytes   = 1 << 20
)

type singleflightCall struct {
	done chan struct{}
}

type boundedCacheEntry interface {
	entrySize() int
	lastAccessed() time.Time
}

func boundedCachePut[K comparable, E boundedCacheEntry](cache map[K]E, totalBytes *int, maxEntries, maxBytes int, key K, entry E) {
	cache[key] = entry
	*totalBytes += entry.entrySize()
	for len(cache) > maxEntries || *totalBytes > maxBytes {
		var oldestKey K
		var oldest time.Time
		for k, e := range cache {
			if oldest.IsZero() || e.lastAccessed().Before(oldest) {
				oldest = e.lastAccessed()
				oldestKey = k
			}
		}
		*totalBytes -= cache[oldestKey].entrySize()
		delete(cache, oldestKey)
	}
}

func boundedMax(maxEntries, maxBytes, defaultEntries, defaultBytes int) (int, int) {
	if maxEntries <= 0 {
		maxEntries = defaultEntries
	}
	if maxBytes <= 0 {
		maxBytes = defaultBytes
	}
	return maxEntries, maxBytes
}

type searchCacheEntry struct {
	ids        []int
	fetchedAt  time.Time
	lastAccess time.Time
}

func (e *searchCacheEntry) entrySize() int { return len(e.ids) * 8 }
func (e *searchCacheEntry) lastAccessed() time.Time { return e.lastAccess }

type indexTopCacheEntry struct {
	node       *bnode
	fetchedAt  time.Time
	lastAccess time.Time
}

func (e *indexTopCacheEntry) entrySize() int { return maxNodeSize }
func (e *indexTopCacheEntry) lastAccessed() time.Time { return e.lastAccess }

type indexNodeCacheEntry struct {
	node       *bnode
	fetchedAt  time.Time
	lastAccess time.Time
}

func (e *indexNodeCacheEntry) entrySize() int { return maxNodeSize }
func (e *indexNodeCacheEntry) lastAccessed() time.Time { return e.lastAccess }

type indexDataCacheEntry struct {
	ids        []int
	fetchedAt  time.Time
	lastAccess time.Time
}

func (e *indexDataCacheEntry) entrySize() int { return len(e.ids) * 8 }
func (e *indexDataCacheEntry) lastAccessed() time.Time { return e.lastAccess }

// gateLaneState は Phase 4（decision 2026-11-05）の 2 レーンゲート状態。共有 `last` 連鎖は
// 系統間隔の global 遵守（FIFO）を維持する。foreground は現行 FIFO 将来予約のまま。
// background は「フォア待機なしかつ last+interval<=now かつ lastFg+grace<=now」で今この
// 瞬間のみ予約（未来予約禁止）。飢餓防止: bg 待機が 3×interval 継続した場合は公平枠として
// interval 条件内で予約可（将来予約にはしない）。
type gateLaneState struct {
	last        time.Time
	lastFg      time.Time
	lastBg      time.Time
	fgPending   int
	bgWaitSince time.Time
}

type Client struct {
	http           *http.Client
	cfg            HitomiConfig
	dataGate       gateLaneState
	imageGate      gateLaneState
	mu             sync.Mutex
	cacheMu        sync.Mutex
	singleflightMu sync.Mutex
	metricsMu      sync.Mutex
	fetch          func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error)
	// C-10: testGateIntervalMS はテスト専用 gate interval inject 口（0 = 非 inject、
	// cfg 値を使用）。本番既定 1000ms と gate semantics は不変。
	testGateIntervalMS int
	// 項目2: blockCooldownUntil は block signal クールダウンの期限（zero = 非発動）。
	// `mu` 保護。有効中、gateLane は両 lane の effective interval を politeness 既定へ退避。
	blockCooldownUntil time.Time
	versions       map[string]string
	versionAt      map[string]time.Time
	versionForceAt map[string]time.Time
	gg             GG
	ggLoadedAt     time.Time
	ggForceAt      time.Time
	singleflight   map[string]*singleflightCall
	metrics        map[string]*FetchMetric
	searchCache    map[string]*searchCacheEntry
	searchCacheMu  sync.Mutex
	searchCacheBytes int
	searchCacheMaxEntries int
	searchCacheMaxBytes   int
	indexTopCache   map[string]*indexTopCacheEntry
	indexTopCacheMu sync.Mutex
	indexTopCacheBytes int
	indexTopCacheMaxEntries int
	indexTopCacheMaxBytes   int
	indexNodeCache  map[string]*indexNodeCacheEntry
	indexNodeCacheBytes int
	indexNodeCacheMaxEntries int
	indexNodeCacheMaxBytes   int
	indexDataCache  map[string]*indexDataCacheEntry
	indexDataCacheBytes int
	indexDataCacheMaxEntries int
	indexDataCacheMaxBytes   int
	indexCacheMu    sync.Mutex
	noFullCache     map[string]time.Time
	noFullCacheMu   sync.Mutex
	noFullCacheMaxEntries int
}

// put helpers must be called with the owning cache mutex held.
func (c *Client) putSearchCacheLocked(key string, entry *searchCacheEntry) {
	maxEntries, maxBytes := boundedMax(c.searchCacheMaxEntries, c.searchCacheMaxBytes, defaultSearchCacheMaxEntries, defaultSearchCacheMaxBytes)
	boundedCachePut(c.searchCache, &c.searchCacheBytes, maxEntries, maxBytes, key, entry)
}

func (c *Client) putIndexDataCacheLocked(key string, entry *indexDataCacheEntry) {
	maxEntries, maxBytes := boundedMax(c.indexDataCacheMaxEntries, c.indexDataCacheMaxBytes, defaultIndexDataCacheMaxEntries, defaultIndexDataCacheMaxBytes)
	boundedCachePut(c.indexDataCache, &c.indexDataCacheBytes, maxEntries, maxBytes, key, entry)
}

func (c *Client) putIndexTopCacheLocked(key string, entry *indexTopCacheEntry) {
	maxEntries, maxBytes := boundedMax(c.indexTopCacheMaxEntries, c.indexTopCacheMaxBytes, defaultIndexTopCacheMaxEntries, defaultIndexTopCacheMaxBytes)
	boundedCachePut(c.indexTopCache, &c.indexTopCacheBytes, maxEntries, maxBytes, key, entry)
}

func (c *Client) putIndexNodeCacheLocked(key string, entry *indexNodeCacheEntry) {
	maxEntries, maxBytes := boundedMax(c.indexNodeCacheMaxEntries, c.indexNodeCacheMaxBytes, defaultIndexNodeCacheMaxEntries, defaultIndexNodeCacheMaxBytes)
	boundedCachePut(c.indexNodeCache, &c.indexNodeCacheBytes, maxEntries, maxBytes, key, entry)
}

func defaultHitomiConfig() HitomiConfig {
	return HitomiConfig{MinIntervalMS: 100, ImageMinIntervalMS: 125, GGForceMinIntervalMS: 60000, MaxRetries: 3, RetryBackoffMS: 2000,
		UserAgent: "hitomi-hub/0.1 (local hub client)", ImageFormat: "webp", UseLiveGG: true, BgGraceMS: defaultBgGraceMS}
}

func negativeNoFullKey(hash, format string, gg GG) string {
	values := make([]int, 0, len(gg.CaseSet))
	for value := range gg.CaseSet {
		values = append(values, value)
	}
	sort.Ints(values)

	var b strings.Builder
	b.WriteString(hash)
	b.WriteByte('|')
	b.WriteString(format)
	b.WriteByte('|')
	b.WriteString(gg.Source)
	b.WriteByte('|')
	b.WriteString(gg.Bucket)
	for _, value := range values {
		b.WriteByte('|')
		b.WriteString(strconv.Itoa(value))
	}
	return b.String()
}

func imageFetchKey(hash, format string, gg GG) string {
	return negativeNoFullKey(hash, format, gg)
}

// cleanStaleNoFullCacheLocked must be called with noFullCacheMu held.
// It only cleans stale entries（TTL 超過済み・二度と hit されない項目）when the entry
// count is over the cap. Active entries（TTL 内）are protected and never evicted here.
func (c *Client) cleanStaleNoFullCacheLocked() {
	maxEntries := c.noFullCacheMaxEntries
	if maxEntries <= 0 {
		maxEntries = defaultNoFullCacheMaxEntries
	}
	if len(c.noFullCache) <= maxEntries {
		return
	}
	now := time.Now()
	for key, storedAt := range c.noFullCache {
		if now.Sub(storedAt) >= negativeNoFullCacheTTL {
			delete(c.noFullCache, key)
		}
	}
}

func (c *Client) canUseNegativeNoFull(hash, format string, gg GG) bool {
	key := negativeNoFullKey(hash, format, gg)

	c.noFullCacheMu.Lock()
	defer c.noFullCacheMu.Unlock()

	if c.noFullCache == nil {
		return false
	}
	storedAt, ok := c.noFullCache[key]
	if !ok {
		return false
	}
	if time.Since(storedAt) >= negativeNoFullCacheTTL {
		delete(c.noFullCache, key)
		return false
	}
	return true
}

func (c *Client) clearNegativeNoFull(hash, format string, gg GG) {
	key := negativeNoFullKey(hash, format, gg)

	c.noFullCacheMu.Lock()
	defer c.noFullCacheMu.Unlock()

	if c.noFullCache == nil {
		return
	}
	delete(c.noFullCache, key)
}

func (c *Client) rememberNegativeNoFull(hash, format string, gg GG) {
	key := negativeNoFullKey(hash, format, gg)

	c.noFullCacheMu.Lock()
	defer c.noFullCacheMu.Unlock()

	if c.noFullCache == nil {
		c.noFullCache = make(map[string]time.Time)
	}
	c.noFullCache[key] = time.Now()
	c.cleanStaleNoFullCacheLocked()
}

func (c *HitomiConfig) UnmarshalJSON(data []byte) error {
	type jsonConfig struct {
		MinIntervalMS          int    `json:"min_interval_ms"`
		ImageMinIntervalMS     int    `json:"image_min_interval_ms"`
		GGForceMinIntervalMS   int    `json:"gg_force_min_interval_ms"`
		MaxRetries             int    `json:"max_retries"`
		RetryBackoffMS         int    `json:"retry_backoff_ms"`
		UserAgent              string `json:"user_agent"`
		ImageFormat            string `json:"image_format"`
		UseLiveGG              *bool  `json:"use_live_gg"`
		BgGraceMS              int    `json:"bg_grace_ms"`
	}
	var raw jsonConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.MinIntervalMS = raw.MinIntervalMS
	c.ImageMinIntervalMS = raw.ImageMinIntervalMS
	c.GGForceMinIntervalMS = raw.GGForceMinIntervalMS
	c.MaxRetries = raw.MaxRetries
	c.RetryBackoffMS = raw.RetryBackoffMS
	c.UserAgent = raw.UserAgent
	c.ImageFormat = raw.ImageFormat
	c.UseLiveGG = true
	if raw.UseLiveGG != nil {
		c.UseLiveGG = *raw.UseLiveGG
	}
	c.BgGraceMS = defaultBgGraceMS
	if raw.BgGraceMS > 0 {
		c.BgGraceMS = raw.BgGraceMS
	}
	return nil
}

const defaultBgGraceMS = 300

func (c *Client) gate() {
	_ = c.gateLane(context.Background(), c.cfg.MinIntervalMS, &c.dataGate, false)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// C-10: testGateIntervalMS > 0 のときテスト専用 interval を inject する。本番既定
// （defaultHitomiConfig() MinIntervalMS 1000ms）と gate semantics（interval 記録・
// sleep・ctx cancel・recordMetric）は維持する。
//
// Phase 4（decision 2026-11-05・2 レーン化）:
//   - foreground（background=false）: 現行 FIFO 将来予約のまま（`last` を advance、
//     `lastFg` 記録、`fgPending` 増減、sleep 完了後 fetch）。
//   - background: 今この瞬間のみ予約（未来予約禁止）。予約条件: `fgPending == 0` かつ
//     `last + interval <= now` かつ `lastFg + grace <= now`（grace既定 300ms・テスト
//     override 可）。条件を満たさない間は interval 境界で再チェック（待機）。飢餓防止:
//     `fgPending` 待機が 3×interval 以上継続し、かつ interval 条件を満たす時点で公平枠と
//     して予約可。shared `last` 連鎖による global interval 遵守は変更しない。
// metrics（kind 別意味・`marker.<priority>` 分離）は非接触。
func (c *Client) gateLane(ctx context.Context, intervalMS int, st *gateLaneState, background bool) error {
	if c.testGateIntervalMS > 0 {
		intervalMS = c.testGateIntervalMS
	}
	// 項目2: block signal クールダウン有効中は両 lane の effective interval を politeness
	// 既定へ退避（max 適用。既定値そのものは変更しない）。FIFO・共有 last 連鎖・A-06 意味は
	// 変更なし（interval が長くなるだけ・sleep 中 ctx cancel 即 return は sleepCtx 維持）。
	// `blockCooldownUntil` は `mu` 保護（fetchRemoteWithGate 側から併記）。
	c.mu.Lock()
	inCooldown := !c.blockCooldownUntil.IsZero() && time.Now().Before(c.blockCooldownUntil)
	c.mu.Unlock()
	if inCooldown && intervalMS < politenessDefaultIntervalMS {
		intervalMS = politenessDefaultIntervalMS
	}
	if st == nil || intervalMS <= 0 {
		return nil
	}
	interval := time.Duration(intervalMS) * time.Millisecond
	for {
		c.mu.Lock()
		now := time.Now()
		if !background {
			if now.Before(st.last.Add(interval)) {
				next := st.last.Add(interval)
				st.last = next
				st.lastFg = next
				st.fgPending++
				c.mu.Unlock()
				err := sleepCtx(ctx, next.Sub(now))
				c.mu.Lock()
				st.fgPending--
				c.mu.Unlock()
				return err
			}
			st.last = now
			st.lastFg = now
			c.mu.Unlock()
			return nil
		}
		grace := time.Duration(c.cfg.BgGraceMS) * time.Millisecond
		intervalOK := !now.Before(st.last.Add(interval))
		graceOK := grace <= 0 || st.lastFg.IsZero() || !now.Before(st.lastFg.Add(grace))
		starved := !st.bgWaitSince.IsZero() && now.Sub(st.bgWaitSince) >= 3*interval
		if intervalOK && graceOK && (st.fgPending == 0 || starved) {
			st.last = now
			st.lastBg = now
			st.bgWaitSince = time.Time{}
			c.mu.Unlock()
			return nil
		}
		if st.fgPending > 0 && st.bgWaitSince.IsZero() {
			st.bgWaitSince = now
		}
		next := st.last.Add(interval)
		if st.fgPending > 0 && !starved {
			if rt := now.Add(interval); rt.Before(next) {
				next = rt
			}
		}
		if !graceOK {
			if gt := st.lastFg.Add(grace); gt.Before(next) {
				next = gt
			}
		}
		c.mu.Unlock()
		if err := sleepCtx(ctx, time.Until(next)); err != nil {
			return err
		}
	}
}

func (c *Client) get(ctx context.Context, rawurl string, rng [2]int64, background bool) ([]byte, error) {
	return c.getWithGate(ctx, rawurl, rng, c.cfg.MinIntervalMS, &c.dataGate, background)
}

func (c *Client) getImage(ctx context.Context, rawurl string, rng [2]int64, background bool) ([]byte, error) {
	return c.getWithGate(ctx, rawurl, rng, c.cfg.ImageMinIntervalMS, &c.imageGate, background)
}

func (c *Client) getWithGate(ctx context.Context, rawurl string, rng [2]int64, intervalMS int, st *gateLaneState, background bool) ([]byte, error) {
	if c.fetch != nil {
		kind := classifyFetchURL(rawurl)
		gateStart := time.Now()
		if err := c.gateLane(ctx, intervalMS, st, background); err != nil {
			c.recordMetric(kind, time.Since(gateStart), 0, 0, 0)
			return nil, err
		}
		gateWait := time.Since(gateStart)
		networkStart := time.Now()
		body, err := c.fetch(ctx, rawurl, rng)
		c.recordMetric(kind, gateWait, time.Since(networkStart), 0, 0)
		return body, err
	}
	return c.fetchRemoteWithGate(ctx, rawurl, rng, intervalMS, st, background)
}

func (c *Client) fetchRemoteWithGate(ctx context.Context, rawurl string, rng [2]int64, intervalMS int, st *gateLaneState, background bool) ([]byte, error) {
	kind := classifyFetchURL(rawurl)
	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		gateStart := time.Now()
		if err := c.gateLane(ctx, intervalMS, st, background); err != nil {
			c.recordMetric(kind, time.Since(gateStart), 0, 0, 0)
			return nil, err
		}
		gateWait := time.Since(gateStart)

		req, err := http.NewRequestWithContext(ctx, "GET", rawurl, nil)
		if err != nil {
			lastErr = err
			retrySleep := time.Duration(c.cfg.RetryBackoffMS*(attempt+1)) * time.Millisecond
			c.recordMetric(kind, gateWait, 0, 0, retrySleep)
			if attempt < c.cfg.MaxRetries {
				if err := sleepCtx(ctx, retrySleep); err != nil {
					return nil, err
				}
			}
			continue
		}
		req.Header.Set("user-agent", c.cfg.UserAgent)
		req.Header.Set("referer", "https://hitomi.la/")
		req.Header.Set("accept", "*/*")
		if rng[0] >= 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rng[0], rng[1]))
		}

		networkStart := time.Now()
		resp, err := c.http.Do(req)
		networkMS := int64(time.Since(networkStart) / time.Millisecond)
		if err != nil {
			lastErr = err
			retrySleep := time.Duration(c.cfg.RetryBackoffMS*(attempt+1)) * time.Millisecond
			c.recordMetric(kind, gateWait, time.Duration(networkMS)*time.Millisecond, 0, retrySleep)
			if attempt < c.cfg.MaxRetries {
				if err := sleepCtx(ctx, retrySleep); err != nil {
					return nil, err
				}
			}
			continue
		}

		decodeStart := time.Now()
		body, readErr := readAll(resp)
		decodeMS := int64(time.Since(decodeStart) / time.Millisecond)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			retrySleep := time.Duration(c.cfg.RetryBackoffMS*(attempt+1)) * time.Millisecond
			c.recordMetric(kind, gateWait, time.Duration(networkMS)*time.Millisecond, time.Duration(decodeMS)*time.Millisecond, retrySleep)
			if attempt < c.cfg.MaxRetries {
				if err := sleepCtx(ctx, retrySleep); err != nil {
					return nil, err
				}
			}
			continue
		}

		if resp.StatusCode == 200 || resp.StatusCode == 206 {
			c.recordMetric(kind, gateWait, time.Duration(networkMS)*time.Millisecond, time.Duration(decodeMS)*time.Millisecond, 0)
			return body, nil
		}

		lastErr = fmt.Errorf("status %d for %s", resp.StatusCode, rawurl)
		retryable := resp.StatusCode == 429 || resp.StatusCode >= 500
		block := isBlockSignal(resp)
		retrySleep := time.Duration(0)
		if retryable || block {
			// 既存 429/5xx の backoff 式（2000 ms×試行）を基盤として維持。403（block）も同一式。
			retrySleep = time.Duration(c.cfg.RetryBackoffMS*(attempt+1)) * time.Millisecond
			// 項目2: 429・block で Retry-After が存在したらその値を backoff として尊重（上限 clamp）。
			if d, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
				retrySleep = d
			}
		}
		if block {
			c.applyBlockCooldown()
		}
		c.recordMetric(kind, gateWait, time.Duration(networkMS)*time.Millisecond, time.Duration(decodeMS)*time.Millisecond, retrySleep)
		if retryable || block {
			if attempt < c.cfg.MaxRetries {
				if err := sleepCtx(ctx, retrySleep); err != nil {
					return nil, err
				}
			}
			continue
		}
		return nil, lastErr
	}
	return nil, lastErr
}

// isBlockSignal は項目2（assess-20261105-1015）の block signal 判定: 403、`cf-mitigated`
// ヘッダ（Cloudflare challenge）、503、522。503/522 は 5xx でもあり、既存 429/5xx リトライ
// semantics（2000 ms×試行）は維持（追加は Retry-After 尊重とクールダウンのみ）。429 単独は
// block signal ではない（Retry-After 尊重のみ・クールダウン不发動）。
func isBlockSignal(resp *http.Response) bool {
	switch resp.StatusCode {
	case http.StatusForbidden, http.StatusServiceUnavailable, 522: // 522 は Cloudflare origin error（http const なし）
		return true
	}
	return resp.Header.Get("cf-mitigated") != ""
}

// parseRetryAfter は RFC 9110 の Retry-After を両形式（delay-seconds・HTTP-date）で解析。
// 負値・解析不能は ok=false（呼び出し側は既存静的 backoff へフォールバック）。不合理に
// 大きい値は blockCooldownDuration（正典 convention penalty window 5 分・negativeNoFullCacheTTL）
// で clamp。数字の新規捏造はしない。
func parseRetryAfter(value string) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(value); err == nil {
		if secs < 0 {
			return 0, false
		}
		d := time.Duration(secs) * time.Second
		if d > blockCooldownDuration {
			d = blockCooldownDuration
		}
		return d, true
	}
	if t, err := http.ParseTime(value); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		if d > blockCooldownDuration {
			d = blockCooldownDuration
		}
		return d, true
	}
	return 0, false
}

// applyBlockCooldown は block signal 検出時に両 lane 共通のクールダウン期限を設定（mu 保護）。
func (c *Client) applyBlockCooldown() {
	c.mu.Lock()
	c.blockCooldownUntil = time.Now().Add(blockCooldownDuration)
	c.mu.Unlock()
}

func readAll(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body)
}

func dirLetter(dir string) string {
	if dir == "avif" {
		return "a"
	}
	return "w"
}

// ImageURL は gg.js の url_from_hash + subdomain_from_url（base なし・webp）と同じ。
// bucket と switch values は正典 gg.js（`GG`）。snapshot での呼出は `snapshotGG()`。
func ImageURL(hash, dir, ext string, gg GG) string {
	g := gg.Value(hash)
	if ext == "" {
		ext = dir
	}
	sub := dirLetter(dir) + strconv.Itoa(1+gg.M(g))
	return "https://" + sub + "." + hitomiDomain + "/" + gg.Bucket + strconv.Itoa(g) + "/" + hash + "." + ext
}

// StorageURL は shard 上の保存形式で URL を組む。shard には webp / avif ののみ格納（実測:
// 実測で webp 200・同じ hash の jpg 404）。galleryinfo の `name` の ext は元の形式なので URL に使わない。
func StorageURL(hash, format string, gg GG) string {
	if format == "" {
		format = "webp"
	}
	return ImageURL(hash, format, format, gg)
}

// TN base の正典バイト（実測 200）: webp `77656270626967746e`、avif `61766966626967746e`。
// `746e626967746e`（tnbigtn）は TN base として存在せず実測 404（KI-20261101-01）。

// TNVariantSuffix は hitomi の gallery.js が使う TN variant を返す。
// work page thumbnail list は devicePixelRatio 1 で smallsmalltn、それ以外 smalltn を使う。
// search card は hitomi の galleryblock で bigtn を使う。
func TNVariantSuffix(variant string) string {
	switch strings.ToLower(strings.TrimSpace(variant)) {
	case "bigtn", "smalltn", "smallsmalltn", "smallbigtn":
		return strings.ToLower(strings.TrimSpace(variant))
	default:
		return "bigtn"
	}
}

// TNFormat は TN path で使う format。hitomi の TN に格納されるのは webp と avif ののみ（実測）。
// galleryinfo `name` の ext（png / jpg など）は TN path に使わない。
func TNFormat(format string) string {
	if strings.ToLower(format) == "avif" {
		return "avif"
	}
	return "webp"
}

// TNBase は TN path の base（webpbigtn / avifbigtn / webpsmalltn / webpsmallsmalltn など）。
func TNBase(format, variant string) string {
	return TNFormat(format) + TNVariantSuffix(variant)
}

// ThumbnailURLVariant は hitomi の tn path（common.js の url_from_url_from_hash(..., base='tn')）と同じ。
// subdomain は `a`/`b` + `tn`（gg.m で決まる）。hash の末尾は `(last)/(prev two)`。
func ThumbnailURLVariant(hash, format, variant string, gg GG) string {
	if len(hash) < 3 {
		return ""
	}

	format = TNFormat(format)
	sub := string(rune('a'+gg.M(gg.Value(hash)))) + "\x74\x6e"
	last := hash[len(hash)-1:]
	prev2 := hash[len(hash)-3 : len(hash)-1]
	return "https://" + sub + "." + hitomiDomain + "/" + TNBase(format, variant) + "/" + last + "/" + prev2 + "/" + hash + "." + format
}

// ThumbnailURL は search card / main image fallback の既定 TN variant（bigtn）を使う。
func ThumbnailURL(hash, format string, gg GG) string {
	return ThumbnailURLVariant(hash, format, "bigtn", gg)
}

// SanitizeTerm は hitomi 正典 sanitize（searchlib.js:74）の形（小文字化＋`/#` 除去のみ）。
// space → `_` 変換は正典にない（D-10 追加 decision 2026-11-04）。`_` → space は colon
// split 前の term 側（results.js:30・search.js:653）で適用する。
func SanitizeTerm(term string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(term, "/", ""), "#", ""))
}

type FileEntry struct {
	Hash    string `json:"hash"`
	Name    string `json:"name"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	HasAVIF int    `json:"hasavif"`
}

type GalleryInfo struct {
	ID            int         `json:"id"`
	Title         string      `json:"title"`
	JapaneseTitle string      `json:"japanese_title"`
	Language      string      `json:"language"`
	Type          string      `json:"type"`
	Tags          []string    `json:"tags"`
	Artists       []string    `json:"artists"`
	Groups        []string    `json:"groups"`
	Characters    []string    `json:"characters"`
	URL           string      `json:"galleryurl"`
	Files         []FileEntry `json:"files"`
	SceneIndexes  []int       `json:"scene_indexes"`
}

// flexString は string/number 両対応（旧形式 live galleryinfo の number 値受入）。
// number は字面そのまま string 化（float 丸めなし）。null は ""。
type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	if string(data) == "null" {
		*f = ""
		return nil
	}
	*f = flexString(string(data))
	return nil
}

type rawGallery struct {
	ID            flexString              `json:"id"`
	Title         string                  `json:"title"`
	JapaneseTitle string                  `json:"japanese_title"`
	Language      string                  `json:"language"`
	Type          string                  `json:"type"`
	Tags          []map[string]flexString `json:"tags"`
	Artists       []map[string]flexString `json:"artists"`
	Groups        []map[string]flexString `json:"groups"`
	Characters    []map[string]flexString `json:"characters"`
	GalleryURL    string                  `json:"galleryurl"`
	Files         []FileEntry             `json:"files"`
	SceneIndexes  []int                   `json:"scene_indexes"`
}

func names(entries []map[string]flexString, key string) []string {
	out := []string{}
	for _, entry := range entries {
		if entry == nil || entry[key] == "" {
			continue
		}
		if entry["female"] != "" {
			out = append(out, "female:"+string(entry[key]))
		} else if entry["male"] != "" {
			out = append(out, "male:"+string(entry[key]))
		} else {
			out = append(out, string(entry[key]))
		}
	}
	return out
}

func (c *Client) Gallery(id int, background bool) (*GalleryInfo, error) {
	body, err := c.get(context.Background(), "https://ltn."+hitomiDomain+"/galleries/"+strconv.Itoa(id)+".js", [2]int64{-1, -1}, background)
	if err != nil {
		return nil, err
	}
	text := string(body)
	idx := strings.Index(text, "var galleryinfo = ")
	if idx < 0 {
		return nil, fmt.Errorf("galleryinfo not found for %d", id)
	}
	raw := &rawGallery{}
	if err := json.Unmarshal([]byte(text[idx+len("var galleryinfo = "):]), raw); err != nil {
		return nil, err
	}
	galleryID, err := strconv.Atoi(string(raw.ID))
	if err != nil {
		return nil, fmt.Errorf("gallery id %q: %s", raw.ID, err)
	}
	return &GalleryInfo{
		ID: galleryID, Title: raw.Title, JapaneseTitle: raw.JapaneseTitle, Language: raw.Language,
		Type: raw.Type, URL: raw.GalleryURL, Files: raw.Files, SceneIndexes: raw.SceneIndexes,
		Tags: names(raw.Tags, "tag"), Artists: names(raw.Artists, "artist"),
		Groups: names(raw.Groups, "group"), Characters: names(raw.Characters, "character"),
	}, nil
}

// ExtractGalleryID は hitomi の GetGalleryId と同じ（URL の <digits>.html から id）
func ExtractGalleryID(raw string) (int, error) {
	m := galleryIDPattern.FindString(raw)
	if m == "" {
		return 0, errors.New("no gallery id in URL")
	}
	return strconv.Atoi(strings.TrimSuffix(m, ".html"))
}

var galleryIDPattern = regexp.MustCompile(`[0-9]+\.html`)

// nozomiPathParts は hitomi の get_galleryids_for_query と同じ分区規則
// female:/male: は area=tag（tag には term 全体）、その他は area=namespace・tag=右辺、
// language: は lang に反映（tag は index）。term は whitespace 区切りの 1 term。colon split
// 前に正典（results.js:30・search.js:653）どおり `_` → space を適用し、dataset name は
// space 形（live 実測 space 200 / underscore 404・decision 2026-11-04）。
func nozomiPathParts(term, lang string) (string, string, string, error) {
	term = normalizeSearchTerm(term)
	area := ""
	tag := SanitizeTerm(term)
	if i := strings.Index(term, ":"); i >= 0 {
		left, right := strings.ToLower(term[:i]), SanitizeTerm(term[i+1:])
		switch left {
		case "female", "male":
			area = "tag"
			tag = SanitizeTerm(term)
		case "language":
			lang = strings.ToLower(right)
			tag = "index"
		default:
			area = left
			tag = right
		}
	}
	if tag == "" {
		return "", "", "", errors.New("term has no tag part")
	}
	return area, tag, lang, nil
}

func nozomiPath(area, tag, lang string) string {
	path := "/n/"
	if area != "" {
		path += url.PathEscape(area) + "/"
	}
	return path + url.PathEscape(tag+"-"+lang) + ".nozomi"
}
