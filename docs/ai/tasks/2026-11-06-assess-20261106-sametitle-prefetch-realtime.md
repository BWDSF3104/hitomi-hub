# 同名日本語版リンク：先読み深化とリアルタイム付与の実現可能性検証

**Status**: 進行中
**Date**: 2026-11-06
**Next Action**: dynamic/実装 re-anchor待ち（枚数6は実測判定上限）

## 意図 / 背景

要望: 先読み（prefetch）を深化し、同名作品の日本語版リンクを、分かった時点でリアルタイムに card へ付与する。基本動作を壊さずに可能かどうかの検証。実装はしていない（実装開始指示なし・評価のみ）。

## 調査結果 / 変更内容

### 0. 先読みページ数の変数化

- `app/search.html` の `PREFETCH_AHEAD_PAGES` を推奨値 `3`（この検証の算術例 depth3）へ変更。
- `runPrefetch()` は `offset = 1..PREFETCH_AHEAD_PAGES` の ahead-page loop とし、current page の次の 1 page だけでなく、total pages に収まる範囲で最大 3 page 先読みする。
- `tests/search_prefetch_harness.mjs` と `tests/test_search_prefetch.py` を、固定 page+1 ではなく configured ahead-page count を見る形へ更新。
- 推奨値そのものは正典 decision に明示記録がない。この実装は算術例の `3` を採用したもので、判断項目は「5. 基本動作の guardian との衝突」に列挙。

### 1. データ取得コスト（追加リクエスト0で可能）

- `/api/search-card/{id}` の response は `title` / `japanese_title` / `language` を返す（`app/viewer.go:656`）。
- prefetch は既に page+1 の card meta を取得済み（`app/search.html:710`）で、`onCard` は内部配列へ push するだけで DOM へ書かない（`app/search.html:710-712`）。
- group cache への登録は `renderSearchCard()` 経由の `setSearchTitleGroup()` だけ（`app/search.html:284`）。prefetch 経路では呼ばれない。
- anchor は render 時点で一括計算（`app/search.html:236-241`）で、後から分かった knowledge は既表示 card へ反映されない。
- → 「分かった時点で付与」は、取得済み meta を DOM へ patch するだけで可能。追加 remote 呼出し 0・data/image lane gate 非接触。

### 2. 同名 group key の実測（key 順序変更は改善にならない）

正典 persistent cache 281 gallery（`output/hitomi-hub/cache/galleryinfo` + 確認用 cache）で測定（`docs/ai/tmp/assess-20261106-sametitle-feasibility/analyze_groups2.py`）:

- `japanese_title` 非空は `language=japanese` の 67/149 のみ。非日本語 132 件は全て空。
- current key（`title || japanese_title`・`app/search.html:209`）: groups=269 / multi=11 / cross-language=8 / anchor 対象 card=11（非日本語 card 132 件中 8.3%）
- decision 2026-11-01-p3-implementation-decisions が示す `japanese_title || title` へ変更: multi=8 / cross=4 / anchor=2 → 劣化
- union(title, jp) 追加橋渡し: multi=11 / cross=8（増なし）
- 実測上、group 成立は hitomi の `title` が言語横断で同一文字列の場合のみ。リアルタイム化は coverage を増やすのではなく既知 group の遡適用（可視性）を上げる。key 順序変更は不建议。

### 3. server 側 source は信用できない

- 正典 DB 実測: `galleries` 454 行中 title 埋まっているのは 14（NULL 440）。`recordDiscovered()` は title/lang を NULL で挿入（`app/main.go:1343-1349`）。
- `SearchHTML()` は id と total のみ（`app/searchhtml.go:462-468`）。nozomi/galleriesindex は title を返さない → 全 title 揃えは 1 gallery = 1 data lane slot。
- 既知分は persistent cache（`cache/galleryinfo`）に 281 件あるので、server 側回答は cache 済み分に限定（追加 remote なし）。client localStorage の方が既に広く、既存設計（`TITLE_GROUPS_STORAGE_KEY`）が正。

### 4. 先読み depth の rate budget（算術・正典値）

- 正典値: data 100 ms / image 60 ms（`app/hitomi.go:237-240`）。page あたり data = 1 search + 25 card meta = 26 slot = 2.6 s、TN = 25 × 60 ms = 1.5 s（image lane・data と並列）。
- depth 1 ≈ 2.6 s、depth 3 ≈ 7.8 s の background slot 占有。
- foreground ≤1 枠遅延保証は background が未来予約しない設計（decision `2026-11-05-phase4-two-lane-gate-implementation.md`、`app/hitomi.go:385-392`）なので depth 増加でも保たれる。block 時の持続窓挙動は未測（decision 2026-11-05-search-speedup・2026-11-06-tn の未確認点）→ 実測なしに「壊さない」とは言い切れない。

### 5. 基本動作の guardian との衝突（要 decision + test re-anchor）

- 変更前 harness は「page+1 only」「one run covers at most one page worth」を固定していた。現在の実装で推奨値 `3` ahead-page loop を採用するため、正典 invariant（decision `2026-11-06-tn-image-lane-60ms-approved.md` の「page+1 限定」含む search page 無変更条）の明示 supersede が必要。harness は「configured ahead pages only, never beyond them」「at most configured ahead-page count」へ re-anchor 済み。
- `tests/search_prefetch_harness.mjs:29`（schedulerBlock に `setStatus(`/`renderSearchCard(`/`innerHTML` 不許可・"prefetch failures are not rendered on screen"）と `:31`（"cooldown is memory-only, never persisted"）。prefetch 経路から DOM へ anchor を書き、group cache へ保存すると、リテラルでは helper 経由なら通過できるが guard の意味が変わる。実装は schedulerBlock 内で helper 呼び出のみ（`innerHTML`/`setStatus(`/`renderSearchCard(`/`localStorage` リテラルを置かない）とし、check の意味を「失败は画面へ書かない・成功の anchor patch と group cache 更新は許可」へ re-anchor する必要。
- `app/viewer_test.go:3331-3347` は `renderSearchCard` が `sameTitleJapaneseAnchor(r)` と `anchor + ` を呼ぶことを固定 → render 時点 anchor は残す（リアルタイム patch は加算）。
- `app/viewer_test.go:2672-2682` card height contract（130px 固定・tags line-clamp 9・`.card .text` max-height calc(130px - 2rem)・overflow hidden）。anchor は `.card .text` の最後の子なので、tags が幅を使うと retroactive patch は clip される（現 render 時 anchor も同じ制約）。可視性の screen confirmation が必要（canonical build rule）。
- patch は `searchGeneration` guard と「表示中 card だけ」制約が必須（prefetch 分の id は現 page の DOM に無い）。既存 interrupt 条件（query input・card click・hidden tab）と marker（`priority=background`）、never-full-image 規則（harness:30）は非接触で維持可能。

### 6. ユーザー判定（2026-11-06 03:10 UTC）

- 先読み枚数 `6` は実測判定で確定できる上限。現在 `3` fixed 実装は変更しない。
- dynamicの既定 `3`、下限 `1`、上限 `6`。判断材料は実測1ページ分耗时とユーザー待機時間。
- リアルタイム付与: 成功時の card patch と group cache 更新を許可。失敗は画面へ出さない。schedulerBlock内は失敗処理だけ。成功patch/cache更新は helper 経由。
- key は現行 `title || japanese_title` を維持。
- foreground影響: `3`までは正典 foreground ≤1 枠保証で許容可能。`4~6`は block 持続窓と foreground影響の実測後に確定。
- 記録: `docs/ai/decisions/2026-11-06-sametitle-prefetch-judgments.md`

### 判定

- リアルタイム付与単体（depth 不変）: 追加 remote 0・rate/gate 非接触・image 経路非接触。衝突は「prefetch は画面へ書かない」guard の意味1件だけ → decision + re-anchor で解消可能。client 側 helper + patch で実装規模は小。
- 先読み深化: 技術的には可能だが「page+1 限定」の正典 invariant を明示的に supersede する必要。foreground ≤1 枠は保たれるが block exposure と background 総量が増える（未測）。per-page latency は改善しない（background 量が増えるだけ）。
- AGENTS.md「絶対に壊してはいけない条件」は非接触で維持可能（viewer imageProxy 順序、TN display fallback 禁止、TN バイトを full cache に保存しない、shard URL format、永続先）。

## Verification

- 変数化後の検証: `go vet .` clean、`go test -count=1 -timeout 300s .` pass `22.835s`、`python -m pytest tests/ -q` 53 passed `8.73s`。正典 exe を `go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .` で再生成し、canonical freshness test を通過。
- 推奨値 `3`・ahead-page loop 実装後の検証: `python -m pytest tests/ -q` 53 passed `9.11s`（canonical rebuild 後）、`go vet .` clean、`go test -count=1 -timeout 300s .` pass `22.837s`。canonical exe を同じ正典 build で再生成し、canonical freshness test を通過。
- baseline 検証: `go vet .`（app）clean、`go test -count=1 -timeout 300s .`（app）pass `24.054s`（変更なし・baseline）
- `python -m pytest tests/ -q` 53 passed `9.69s`（`tests/test_search_prefetch.py` の固定 behaviors 含む・baseline pass）
- 実測スクリプト（スクラッチ・コミット対象外）: `docs/ai/tmp/assess-20261106-sametitle-feasibility/analyze_groups2.py`、`analyze_union.py`
- 未確認点: anchor の可視性（card height contract で clip される事例）の screen confirmation。block 持続窓の挙動。translation entry で `japanese_title` が populated になる事例（本サンプル非日本語 132 件で 0 件）。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
