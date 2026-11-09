# boundedCachePut の eviction が O(n) 走査

**Status**: 完了
**Date**: 2026-11-09
**Next Action**: 調査委譲（assess-20261109-1320）

## 意図 / 背景

- 指摘: `boundedCachePut` の eviction が上限超過のたびに全 entry を線形走査して最古を探す。上限 256〜1024 なので致命的ではないが、cache 満杯の定常状態では put のたびに走査が走る。`container/list` による真 LRU か heap で定数/対数時間にできる、という性能項目（新規）。
- 確認事項: (1) 現行で eviction が線形走査か、(2)「最古」の意味＝作成順か last access 順か、正典 C-05/R-15 の削除順定義と一致するか、(3) 定常状態（cap 1024・put N 回）の累计 µs を mock 実測、(4) 対応案比較と推奨、(5) TTL 切れ entry の扱い（hit 時 delete しない現行流儀）との整合。
- 正典参照（読み取り）: `docs/ai/decisions` 経由の C-05 削除順定義は `docs/ai/known-issues/archive/2026-11-04-c05-non-target-caches-unbounded.md`（hit で access order 更新・`lastAccessed()` 最小を淘汰）。viewer/memory 削除順の status 優先→LRU は `docs/ai/tasks/2026-11-06-r15-viewer-prefetch-window-and-disk-lru.md`（disk は作成順、memory は保存済み status 優先→LRU）。本項目の helper は後者（viewer の disk/memory）ではなく C-05 の server memory cache helper。

## 調査結果 / 変更内容

### 有効性: 有効（確定・実測済み）

指摘の機構（ eviction 時の全 entry 線形走査 = O(n) ）は現行コードで確定。ただし実運用での影響は小（下記「優先順位」）。

### 現在の実装と判断根拠

- `app/hitomi.go:105-123` の `boundedCachePut`。insert 後 `len(cache) > maxEntries || *totalBytes > maxBytes` で eviction ループ。ループ内で `for k, e := range cache` の全 entry 線形走査し `lastAccessed()` 最小の entry を淘汰（`app/hitomi.go:114-119`）。
- **「最古」の意味 = last access 順（LRU by lastAccessed）**。insert 順ではない。正典 C-05 定義（`archive/2026-11-04-c05-non-target-caches-unbounded.md`・`hitomi.go:72-73` 正典 comment「hit 時に access order を更新し、insert 後に上限超過分を最も古い access から eviction」）と一致。よって現行線形走査は正典削除順（last access 優先）を正しく実装している。
  - 帰結: 「単純 LRU（作成順＝insertion order、hit で move-to-front しない）」に変更すると正典 last access 順意味を変更する → **承認案件**。意味を保持する LRU は「access 順 LRU（hit で move-to-front）」。
- hit path（`searchhtml.go:129,444`・`searchindex.go:241,283,316`・`viewer.go:1856`）は hit 時に `lastAccess = time.Now()` を更新する（正典 hit LRU touch）。TTL 内のみ hit 返却。TTL 切れ entry は map から消えず、refetch で上書き。eviction 走査は TTL 切れ entry も `lastAccessed()` で淘汰対象にえる（正典流儀と整合）。

### 実測（定常状態・cache 満杯・mock・外部 API なし・server 起動なし・2026-11-09）

scratch: `docs/ai/tmp/assess-20261109-1320/n21_bench_test.go`（`go test -overlay` 注入）。goos=windows goarch=amd64 cpu=i7-8700K。`-benchtime=200000x`。entry は `searchCacheEntry`（ids=5 → entrySize=40）、bytes cap 8 MiB（entry 数駆動）。意味一致テスト: eviction 対象・上書き（同一 key）・bytes cap 駆動で現行 == heap == list（単調 ts・tie なし）。

put のみの定常状態（per put = 1 eviction）:

| 機構 | cap 1024 (ns/put) | cap 256 (ns/put) | cap1024・N=1000 累计 |
|---|---|---|---|
| 現行 線形走査 | 15,991 | 3,901 | **15,991 µs** |
| heap（indexed・access 順） | 433.6 | 382.2 | 434 µs |
| container/list（access 順 LRU） | 270.2 | — | 270 µs |

効果（cap1024・N=1000・put の eviction 部分）: 線形→heap ≈ **15,557 µs 削減（約 37 倍）**、線形→list ≈ **15,721 µs（約 59 倍）**。

mixed workload（hit 90% + put 10%・実運用 shape に近い、per iteration平均）: 線形 1,622 ns/op、heap 243.1 ns/op、list 135.9 ns/op。

### 対応案の比較と推奨

- **案A: container/list access 順 LRU（推奨）**。正典 last access 順を保持したまま eviction O(1)（hit で move-to-front）。おまけに tie 決定化（正典既知 flaky `archive/2026-11-04-test-search-cache-size-cap-flaky-lru-tie.md` の map 反復順 tie 問題を物理順で解消）。
  - ただし現行 hit path は `lastAccess` field を直接更新するだけで list 物理順を更新しない。既存テストは hit を経由せず `lastAccess` を直接固定値へ set して eviction 対象を決める（`hitomi_test.go`・`searchhtml_test.go`・`searchindex_test.go`・`viewer_test.go` の LRU 固定）。drop-in の physical LRU はこの field 直 set と非互換（list 順が lastAccessed とズレる）。意味保持には hit path・テストの更新経路を helper 経由に揃える変更が必要。
- **案B: indexed min-heap（lastAccessed キー）**。O(log n)。正典 last access 順を保持。hit path で heap key 更新が必要（lazy heap で hit path 非変更も可能だが read-heavy で stale 再堆化により O(n log n) へ退化し保証がない → 非推奨）。tie は非決定（線形と同一、テストは tie 回避済）。
- **案C: 単純 insertion-order LRU**（list・hit で move-to-front しない）。O(1) だが**正典 last access 順を insertion 順へ変更** → 承認案件（本項目の推奨外。意味変更案として分離）。
- 推奨: 意味変更を避けたいなら案A（access 順 physical LRU）または案B（indexed heap）。実運用で put は cache miss 時・politeness gate（100〜1000 ms）に律速されるため eviction scan（~16 µs/put）は network fetch に対して無視でき、実測効果は小。実装する価値は micro-optimization 程度。

### 副作用・影響範囲・代替案

- 正典 full image rules・imageProxy 順序・永続 cache path・TN rules・UI full 判定・API rate policy は無接触（helper 内部のみ）。UI 変更なし → Playwright 実機確認該当なし。
- テスト影響: eviction 対象を `lastAccess` 値で固定する既存テスト（6 箇所）は、構造（list/heap）が hit path 経由で更新されない限り field 直 set と非互換。案A/B 実施時はこれらのテストを helper 経由更新へ寄せるか、eviction が `lastAccessed()` 値ベースの構造（例: heap）を使う必要がある。assertion 弱化なしで済ませるには更新経路の統一が条件。
- TTL 切れ entry: hit path は TTL 内のみ返却、TTL 切れは map に残り eviction で last access 順に消える（現行流儀・hit 時 delete しない）と全案で整合。

## 実装＋再実測（2026-11-09・実作業・実行固有ID assess-20261109-1320）

### 採用判断（段階1・記録の推奨案＝案A・ユーザー同意済み 2026-11-09）

- access 順 LRU（意味一致）を `container/list` の物理 access order 記録機構で実装。正典意味は不変:
  hit（access 時）で access order 更新（`recordAccess` → `list.MoveToFront`）、eviction は access 順
  （`list.Back()` 取得 O(1)・全 entry 線形走査を置換）。insertion 順 LRU 化（意味変更側）は実施しない。
- 機構: `boundedCacheEntry` に `recordAccess(t)`・`accessOrder()` を追加。entry 型（正典 6 型）に
  `cacheOrderNode`（list 参照・自 elem）を embed。`boundedCachePut` は miss 時 order の list 参照のみ
  共有し新 entry を order へ登録、hit LRU touch（正典 hit 時 access 更新）は `recordAccess` 経由。
  `boundedMax`・cap・TTL・上書き accounting（R-01）・単 entry cap 超即 eviction は不変。
- scope 解釈（記録の案A = 「hit path・テスト更新経路を helper 経由」＝ユーザー同意済みの推奨の実体）:
  access 記録は hit path の call site（`app/searchhtml.go` 2・`app/searchindex.go` 3・`app/viewer.go` 1 の
  1 行置換）と entry 型の実装（`app/hitomi.go` 5 型＋`app/viewer.go` の `infoCacheEntry`）を含む。
  `negativeNoFullKey` memo（access 記録・淘汰なし・同意済み C-08 分離）は非触。実 DB・config・完成
  フォルダ・cache 実データ・gate・TN・full 判定・`x-cache`・DB 1 表・`recordCacheEntry`・created 順正典
  は非接触。委譲文の「`app/hitomi.go` のみ」の記述は記録の案A（hit path 経由）と不一致 → 正典（記録）の
  案A のとおり実施し、テスト変更は許可された `app/viewer_test.go` の pin 2件のみとした（他のテストは as-is
  合格: pin 順 == insertion 順で物理順と一致）。
- 副作用（記録どおりの改善）: LRU tie 決定化が map 反復順依存から物理順ベースへ（既知 issue archive
  `2026-11-04-test-search-cache-size-cap-flaky-lru-tie.md` の原因が機構側で解消）。機構を外れた direct
  field 書き（テスト pin 流儀・helper 経由 map 書換）は物理順を動かさない（value と分離・記録済み）。
  helper 経由 direct map 書換 entry の cap 超過には無限ループ防止 guard（正典書込経路は put helper のみで非発動）。

### 試行（修正・再検証）

- 試行1: 初版 `boundedCachePut` の miss 分岐が `*node = *e.accessOrder()` で既存 entry の elem ポインタまで
  コピー → 新 entry が order へ未登録（`MoveToFront` が既存 entry の要素を動かす）→ 正典 LRU テスト 6件
  FAIL（eviction 不发生・memory cap 超）。原因分析: miss 時は list 参照のみ共有し新 elem を `PushFront`
  すべき。修正（上書き分岐は既存 elem 維持・miss 分岐は list 参照のみ）→ 再検証。
- 試行2: 正典 Go suite 全通過（下記）。修正は同ファイル内のみ。試行回数は 2（上限 3 内）。

### 正典テストの re-anchor（合格条件が要求・記録）

- `app/viewer_test.go` pin 2件のみ変更（許可範囲）:
  - `TestViewerInfoCacheEvictsLRUEntriesWhenOverEntryLimit`（975）: pin を記録機構経由
    （`entry.recordAccess(...)`）へ。access 順意味で物理 order を決定化。assertion 弱化なし・合格値同一。
  - `TestCacheClearResetsMemoryCachesWithoutChangingCacheHeaders`（5695）: warm memory entry の direct map
    書換を put helper（`putInfoCacheLocked`）経由へ（order 登録・memory 件数意味不変）。
- 他のテスト（`searchhtml_test.go`・`searchindex_test.go`・`hitomi_test.go` の pin 6箇所）は as-is・無変更
  （pin 順 == insertion 順で物理順と一致するため合格。機構一致は harness で再実測）。

## Verification

### 実装分（2026-11-09・実作業）

- harness（正典流儀・mock・外部 API なし・server 起動なし・scratch `docs/ai/tmp/assess-20261109-1320/`・
  `go test -overlay`）: `node` 流儀 9 checks の Go 版として 9 checks passed（前批 9 checks 流儀）:
  1. 定常状態 eviction 対象 == oracle（access 順・単調 ts・tie なし・N=5000）
  2. access 時更新（機構 recordAccess）で eviction 順が insertion 順でなく access 順
  3. 上書き accounting == oracle（R-01・正味 8）
  4. bytes cap 駆動・単 entry cap 超即 eviction == oracle
  5. TTL 切れ entry は map に残り access 順で eviction（hit 時 delete しない流儀）== oracle
  6. tie（同一 instant）淘汰対象の物理順決定化（200 round 安定）
  7. per-put eviction 定数（線形走査なし・cap 非依存経路）
  8. helper 経由 direct map 書換 entry で panic/無限ループなし（guard）
  9. clear 後 order 機構は entry と共に消え次の insert が正しく動く（stale order 残留なし）
  経過 1.943 s（check 内 0.18 s）。
- 改善数値実測（定常状態・cache 満杯・`-benchtime=200000x`・goos=windows goarch=amd64 cpu=i7-8700K):
  - put のみ（1 put = 1 eviction・cap1024）: **380.1 ns/op**（変更前 15,991 ns/put → 約 42 倍・
    N=1000 累计 15,991 µs → 380 µs・約 15,611 µs 削減）
  - mixed（hit 90%＋put 10%・per iteration 平均）: **127.4 ns/op**（変更前 1,622 ns/op → 約 12.7 倍）
  - 経過 1.268 s。
- 正典検証（変更後）: `go vet .` clean、`go test -count=1 -timeout 300s .`（app）→ ok **26.279 s**（既存
  LRU/TTL/singleflight/上書き accounting 回帰全通過・合格条件（DB 1 表・cap・「filesystem scan で数えない」・
  memory bytes cap）不変）。`python -m pytest tests/ -v` → **64 passed in 22.12 s**（正典 build fresh（app で
  `go build -o output\hitomi-hub\hub.exe .` 再 build 後・freshness 通過）。
- 画面確認: **該当なし**（UI 項目でない・helper 内部のみ）→ 実機画面操作・Drive 保存なし（未検証扱いなし）。
- 制約遵守: 実 DB・config・完成フォルダ・cache 実データ非接触（テストは scratch）。本項目分のみ単独 commit。

- 意味一致テスト（overlay 注入）: `go test -overlay="S:/Documents/Kilo_Commons/hitomi-hub/docs/ai/tmp/assess-20261109-1320/overlay.json" -v -run "TestN21" .`（app）→ `TestN21EvictTargetMatch` PASS、`TestN21OverwriteAndBytesCapMatch` PASS（eviction 対象・上書き・bytes cap 駆動で現行==heap==list・単調 ts・tie なし）。elapsed ~1.2 s。
- benchmark: `go test -overlay=".../overlay.json" -bench "BenchmarkN21" -benchtime=200000x -run "^$" .`（app）→ 上表の数値（elapsed 5.5 s）。
- 制約遵守: ソースコード未変更（正典 `boundedCachePut` 非触）。変更は本タスク記録と scratch `docs/ai/tmp/assess-20261109-1320/` のみ。実 DB・完成フォルダ非接触。実装ゲート: 本項目は調査のみ（実装開始指示なし）。実行固有ID `assess-20261109-1320`。

### 優先順位付けに効く要素

- 重要度: 低。実運用の put は cache miss 時で politeness gate（100〜1000 ms）に律速。eviction scan（cap1024 で ~16 µs/put）は network fetch に対して無視できる。定常状態でも「put ごと 1 回」の走査で、hit（多数）は eviction を駆動しない。
- 緊急度: 低。機能停止・誤 eviction・正典違反なし（現行は正典 last access 順を正しく実装）。
- リスク: 単純 insertion-order LRU 化は正典 last access 順意味の変更（承認案件）。案A/B は既存 LRU テストの field 直 set と非互換（更新経路統一が条件）。
- 依存: なし（helper 内部）。実装時は hit path・テスト 6 箇所の更新経路統一。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
