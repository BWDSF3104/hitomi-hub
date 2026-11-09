package main

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// N-12 回帰テスト（decision 2026-11-09-n12-group-registry-effective-lane-exception.md・ユーザー承認 2026-11-09）。
// 正典流儀: mock fetch（外部 API なし）・C-10 interval inject・elapsed 上限付き判定・network 中に mutex 保持しない。
// 固定テスト（join 数・calls 1・joiner slot 0・未来予約禁止・公平枠・grace・key 境界）は変更しない。

func claimCount(c *Client, key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if claim := c.groupClaims[key]; claim != nil {
		return claim.fgCount
	}
	return 0
}

func waitForClaim(t *testing.T, c *Client, key string, want int) bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if claimCount(c, key) >= want {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// group effective lane = 参加者最高優先度（key 単位）。fg joiner の group は dispatch 時刻のみ fg slot
// 時刻（last+interval）へ先行（承認済み限定例外）。上界（3×interval＋grace）・共有 last 連鎖・interval 算術不変。
// joiner gate slot 消費 0（claim 記録は slot を消費しない）。
func TestN12GroupClaimEffectiveLaneDispatchesAtFgSlotTime(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.testGateIntervalMS = 100
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return []byte{}, nil
	}
	key := "n12-effective-key"

	// fg record（grace 未満たしの窓）: 正典 bg なら grace 300 ms 待ち。claim がある group は fg slot 時刻で dispatch。
	if err := c.gateLane(context.Background(), c.cfg.MinIntervalMS, &c.dataGate, false); err != nil {
		t.Fatal(err)
	}
	lastAtRecord := c.dataGate.last
	c.recordGroupClaim(key, true) // fg joiner 参加
	// joiner slot 0: claim 記録は共有 last 連鎖・fgPending・bg slot を動かさない
	if !c.dataGate.last.Equal(lastAtRecord) {
		t.Errorf("claim recording must not consume gate slot (joiner slot 0): last %v", c.dataGate.last)
	}
	if c.dataGate.fgPending != 0 || !c.dataGate.lastBg.IsZero() {
		t.Error("joiner must not enter gate (joiner gate slot 0)")
	}

	start := time.Now()
	if err := c.gateLane(c.withGroupKey(context.Background(), key), c.cfg.MinIntervalMS, &c.dataGate, true); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	// 改善は gate 成分のみ（実測 209 ms/240 ms 相当）: dispatch 時刻 last+interval（100 ms）。grace（300 ms）は尊重しない（限定例外）。
	if elapsed < 60*time.Millisecond || elapsed > 180*time.Millisecond {
		t.Errorf("claim-based dispatch must happen at fg slot time last+interval (100ms), not grace (300ms): elapsed %s", elapsed)
	}
	// 共有 last 連鎖は interval 分だけ1回進む（FIFO 算術不変・同一 group 1 slot）
	if !c.dataGate.last.Equal(lastAtRecord.Add(100 * time.Millisecond)) {
		t.Errorf("shared last chain must advance by one interval exactly once: last %v", c.dataGate.last)
	}
	if !c.dataGate.lastBg.IsZero() {
		t.Error("bg slot must not be reserved on effective fg dispatch (gate 正典値不変)")
	}
	c.releaseGroupClaim(key, true)
}

// key 単位の group 参照（候補3・key 台帳）。同一 lane の無関係 group（他 key・claim なし）は正典 bg のまま
// （per-lane storage では claim が漏れる。承認済み「key 単位の group」意味の回帰防止）。
func TestN12GroupClaimIsPerKeyNotPerLane(t *testing.T) {
	c := &Client{cfg: defaultHitomiConfig()}
	c.testGateIntervalMS = 100
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		return []byte{}, nil
	}
	keyA := "n12-key-a"
	keyB := "n12-key-b"

	if err := c.gateLane(context.Background(), c.cfg.MinIntervalMS, &c.dataGate, false); err != nil {
		t.Fatal(err)
	}
	c.recordGroupClaim(keyA, true)
	startA := time.Now()
	if err := c.gateLane(c.withGroupKey(context.Background(), keyA), c.cfg.MinIntervalMS, &c.dataGate, true); err != nil {
		t.Fatal(err)
	}
	if time.Since(startA) > 160*time.Millisecond {
		t.Errorf("keyA（fg joiner の group）must dispatch at fg slot time: elapsed %s", time.Since(startA))
	}

	// keyB（claim なし）→ 正典 bg のまま（grace を尊重）
	startB := time.Now()
	if err := c.gateLane(c.withGroupKey(context.Background(), keyB), c.cfg.MinIntervalMS, &c.dataGate, true); err != nil {
		t.Fatal(err)
	}
	elapsedB := time.Since(startB)
	if elapsedB < 200*time.Millisecond || elapsedB > 450*time.Millisecond {
		t.Errorf("keyB must respect grace, claim must not leak per-lane: elapsed %s", elapsedB)
	}
	if c.dataGate.lastBg.IsZero() {
		t.Error("keyB must consume a bg slot as before (正典条件不変)")
	}
	c.releaseGroupClaim(keyA, true)
}

// canceled joiner（TN）: 即 return（0 ms）・leader 非影響（既存意味）。canceled joiner は参加を取り消す
// （priority claim を解放）。同一 key 1 call・join・joiner slot 0 維持。
func TestN12TNCanceledJoinerReturnsImmediatelyAndReleasesClaim(t *testing.T) {
	v, _ := newTestViewer(t, `var galleryinfo = {"id":"1","files":[]}`)
	c := v.client
	c.cfg.UseLiveGG = false
	c.gg = snapshotGG()
	c.testGateIntervalMS = 1 // behavior focus（joiner は join で gate に入らない）

	key := tnFetchKey(sampleHash, "webp", "bigtn", snapshotGG())
	release := make(chan struct{})
	joined := make(chan string, 1)
	var calls atomic.Int32
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls.Add(1)
		<-release
		return []byte("tn"), nil
	}
	v.tnFetchJoinObserver = func(k string) { joined <- k }

	leaderErr := make(chan error, 1)
	leaderBody := make(chan []byte, 1)
	go func() {
		body, err := v.fetchTNOnce(context.Background(), sampleHash, "webp", "bigtn", snapshotGG(), true) // bg leader（指摘の条件）
		leaderBody <- body
		leaderErr <- err
	}()
	waitForCounter(t, &calls, 1) // group register は fetch 前（joiner は join path を通る）

	joinCtx, cancel := context.WithCancel(context.Background())
	joinErr := make(chan error, 1)
	joinElapsed := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		_, err := v.fetchTNOnce(joinCtx, sampleHash, "webp", "bigtn", snapshotGG(), false) // fg joiner（表示要求）
		joinElapsed <- time.Since(start)
		joinErr <- err
	}()

	if got := <-joined; got != key {
		t.Fatalf("join observer key: got %q want %q", got, key)
	}
	if !waitForClaim(t, c, key, 1) {
		t.Fatal("fg joiner priority claim not recorded in group registry")
	}
	cancel()
	if jerr := <-joinErr; !errors.Is(jerr, context.Canceled) {
		t.Fatalf("canceled joiner error: got %v", jerr)
	}
	if elapsed := <-joinElapsed; elapsed >= 50*time.Millisecond {
		t.Errorf("canceled joiner must return immediately (0 ms): elapsed %s", elapsed)
	}
	if claimCount(c, key) != 0 {
		t.Errorf("canceled joiner must release its priority claim: fgCount %d", claimCount(c, key))
	}

	close(release)
	if err := <-leaderErr; err != nil {
		t.Fatalf("leader must complete unaffected by joiner cancellation: %v", err)
	}
	if string(<-leaderBody) != "tn" {
		t.Error("leader result changed")
	}
	if calls.Load() != 1 {
		t.Errorf("同一 key 1 回（join・canceled joiner refetch なし）: calls %d", calls.Load())
	}
	if claimCount(c, key) != 0 {
		t.Error("claim must be gone after group completion（台帳の group 消滅）")
	}
}

// info join guard（ユーザー承認 2026-11-09・挙動は TN と同じ「即 return」へ改善）: canceled joiner 即 return・
// leader 非影響・同一 key 1 call・claim 解放。既存 info join 固定テスト（join・shared pointer・2 args pin）は非触。
func TestN12InfoJoinGuardCanceledJoinerImmediateReturnAndReleasesClaim(t *testing.T) {
	galleryJSON := `var galleryinfo = {"id":"4252697","title":"Kemo","language":"japanese","type":"doujinshi","files":[]}`
	v, _ := newTestViewer(t, galleryJSON)
	c := v.client
	c.testGateIntervalMS = 1

	key := strconv.Itoa(4252697)
	release := make(chan struct{})
	joined := make(chan int, 1)
	var calls atomic.Int32
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls.Add(1)
		<-release
		return []byte(galleryJSON), nil
	}
	v.infoFetchJoinObserver = func(id int) { joined <- id }

	leaderErr := make(chan error, 1)
	leaderInfo := make(chan *GalleryInfo, 1)
	go func() {
		info, err := v.info(4252697, true) // bg leader（既存 2 args pin・guard は core が ctx を受ける）
		leaderInfo <- info
		leaderErr <- err
	}()
	waitForCounter(t, &calls, 1) // group register は fetch 前（joiner は join path を通る）

	joinCtx, cancel := context.WithCancel(context.Background())
	joinErr := make(chan error, 1)
	joinElapsed := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		_, err := v.resolveInfoOnce(joinCtx, 4252697, false, nil) // fg joiner（表示要求）
		joinElapsed <- time.Since(start)
		joinErr <- err
	}()

	if got := <-joined; got != 4252697 {
		t.Fatalf("join observer id: got %d", got)
	}
	if !waitForClaim(t, c, key, 1) {
		t.Fatal("fg joiner priority claim not recorded in group registry")
	}
	cancel()
	if jerr := <-joinErr; !errors.Is(jerr, context.Canceled) {
		t.Fatalf("canceled info joiner must return immediately with ctx error（TN と同じ「即 return」）: got %v", jerr)
	}
	if elapsed := <-joinElapsed; elapsed >= 50*time.Millisecond {
		t.Errorf("canceled info joiner must return immediately (0 ms): elapsed %s", elapsed)
	}
	if claimCount(c, key) != 0 {
		t.Errorf("canceled joiner must release its priority claim: fgCount %d", claimCount(c, key))
	}

	close(release)
	if err := <-leaderErr; err != nil {
		t.Fatalf("leader must complete unaffected: %v", err)
	}
	info := <-leaderInfo
	if info == nil || info.Title != "Kemo" {
		t.Errorf("leader result changed: %v", info)
	}
	if calls.Load() != 1 {
		t.Errorf("同一 key 1 回: calls %d", calls.Load())
	}
	if claimCount(c, key) != 0 {
		t.Error("claim must be gone after group completion（台帳の group 消滅）")
	}
}

// nozomi: join block に ctx guard がない既存流儀（固定テスト pin）なので canceled joiner が成立せず、
// claim 解放は該当なし。joiner 参加時の claim 記録と group 完了時の台帳消去のみ確認する。
func TestN12NozomiJoinerClaimRecordedAndGoneAtGroupCompletion(t *testing.T) {
	v, _ := newTestViewer(t, "")
	c := v.client
	c.testGateIntervalMS = 1

	stateLeader := &searchState{area: "tag", tag: "female:yuri", language: "all", background: true}  // bg leader（指摘の条件）
	stateJoiner := &searchState{area: "tag", tag: "female:yuri", language: "all", background: false} // fg joiner（表示要求）
	cacheKey := "tag|female:yuri|all"

	release := make(chan struct{})
	joined := make(chan string, 1)
	var calls atomic.Int32
	c.fetch = func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
		calls.Add(1)
		<-release
		return beIDs([]uint32{11, 22, 33}), nil
	}
	c.searchGroupJoinObserver = func(k string) { joined <- k }

	leaderErr := make(chan error, 1)
	go func() {
		_, err := c.SearchNozomiParts(stateLeader)
		leaderErr <- err
	}()
	waitForCounter(t, &calls, 1) // group register は fetch 前（joiner は join path を通る）

	joinErr := make(chan error, 1)
	go func() {
		_, err := c.SearchNozomiParts(stateJoiner)
		joinErr <- err
	}()

	if got := <-joined; got != cacheKey {
		t.Fatalf("join observer key: got %q want %q", got, cacheKey)
	}
	if !waitForClaim(t, c, cacheKey, 1) {
		t.Fatal("fg joiner priority claim not recorded in group registry")
	}

	close(release)
	if err := <-leaderErr; err != nil {
		t.Fatalf("leader: %v", err)
	}
	if err := <-joinErr; err != nil {
		t.Fatalf("joiner: %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("同一 key 1 回: calls %d", calls.Load())
	}
	if claimCount(c, cacheKey) != 0 {
		t.Error("claim must be gone after group completion（台帳の group 消滅）")
	}
}
