# boundedCachePut の eviction が O(n) 走査

**Status**: 進行中
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

## Verification

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
