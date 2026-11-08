# N-12: singleflight で leader の lane により優先度が逆転する

**Status**: 進行中
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 調査委譲（assess-20261108-1332）。記録のみ。実装開始指示待ち

## 意図 / 背景

- 項目原文（読み取り指摘・委譲前確度：未検証）: singleflight で優先度が逆転する。TN・full・info・nozomi はいずれも leader の lane で待つ。leader が background だと、後から来た foreground の要求がフォア待機中は譲る background lane の待ちに巻き込まれる。
- 記録: docs/ai/tasks/2026-11-08-claude-review-20261108-findings.md の N-12 欄。個別タスク記録への追記は子（委譲時）のみ。
- 委譲停止中（指示 2026-11-08 05:08Z）。親は調査実務を行わない。Status と Next Action は親が正典 `task_manager` で管理。

## 調査結果 / 変更内容

### 現在の実装（確度: 確定・コード照合）

- join 経路は joiner が gate に入らない（gate slot を消費しない）。joiner の待ち時間は leader の gate 予約＋leader の network で決まり、joiner 自身の `background` 引数は gate に参加しない。
  - `fetchTNOnce`（`app/viewer.go:985-1026`）: join block `app/viewer.go:997-1008`（ctx select）。leader fetch は `app/viewer.go:1014` で `background` を `getImage` へ渡す。joiner の `background` は join 後も使用されない。
  - `fetchFullImageOnce`（`app/viewer.go:1028-`）: join block `app/viewer.go:1035-1046`。leader fetch `app/viewer.go:1052`。
  - `resolveInfoOnce`（`app/viewer.go:1352-`）: join block `app/viewer.go:1357-1364`（`<-call.done`）。leader fetch `app/viewer.go:1370`（`client.Gallery(id, background)`）。
  - `SearchNozomiParts`（`app/searchhtml.go:114-197`）: join `app/searchhtml.go:135-149`。leader fetch `app/searchhtml.go:155` で `state.background` を使う。joiner の `state.background` は join 後も使用されない。
  - `EnsureGG`（`app/gg.go:118-201`）: join `app/gg.go:150-157`。leader fetch は `app/gg.go:163` で常に `false`（foreground）→ この箇所は逆転余地なし（leader が常にフォア）。
- gate 算術（`app/hitomi.go:455-524`）は呼出側の `background` だけでレーンを決める。foreground は FIFO 未来予約、background は「今この瞬間のみ予約・フォア待機中は譲る・3×interval で公平枠・grace 300 ms」。joiner は `getWithGate`（`app/hitomi.go:534-549`）自体を呼ばないので、joiner の priority は算術に入らない。
- metrics（`app/metrics.go:56-65`）は joiner を計上しない（join で call 数が減る既存意味）。`recordMarkerRequest` は handler 側 per request（`app/metrics.go:69-74`）で join と無関係。
- 混合優先度が同一 key で成立する条件（実在経路）: search page の prefetch は `priority=background`（`app/search.html:788`）、表示要求は normal。同一 TN key・同一 gallery id・同一 cacheKey に prefetch（bg）→ 表示（normal）の順で到達し得る。

### 有効性: 有効（実測で閉じた）

逆転は機構として確定、かつ mock 実測で数値確認。joiner は自分の priority を持たず leader の lane を継承する（leader が background のとき、joiner は「フォア待機中は譲る」待ちに巻き込まれる）。

実測（scratch replica `docs/ai/tmp/assess-20261108-1332/r12/`・`go test -v -count=1 -timeout 180s .` → `ok assess202611081332r12 5.745s`・外部 API 呼出しなし・gateLane は正典条件式をそのまま再現・正典値 data lane 90 ms（R-11）/ image lane 60 ms（TN）/ grace 300 ms）:

| Run | 案 | 同一 key の外部呼出し | 同一 key の gate slot | joiner 壁時間 | leader 壁時間 |
|---|---|---|---|---|---|
| A data 90 ms・network 0（bg leader + fg joiner） | 案3 現状（join） | 1 | 1 | 390 ms | 390 ms |
| A | 案1（joiner 自分の lane で dispatch） | 2 | 2 | 180 ms | 480 ms |
| A | 案2（join 維持・group effective lane = 参加者最高優先度） | 1 | 1 | 181 ms | 181 ms |
| B image 60 ms・network 679 ms（Phase 4 基線 TN median） | 案3 | 1 | 1 | 1039 ms | 1039 ms |
| B | 案1 | 2 | 2 | 800 ms | 1100 ms |
| B | 案2 | 1 | 1 | 800 ms | 800 ms |
| C fg leader + bg joiner（逆方向） | 案3 | 1 | 1 | 180 ms | 180 ms |

- 上界（正典公平枠）: joiner 壁時間 379 ms（`3×interval` 270 ms ＋ grace 300 ms の範囲内・run D）。
- canceled waiter（run E）: joiner 即 return（0 ms）で leader に影響なし・leader 1 call（gate 300 ms）で完了。三案共通で壊さない。
- 案1 の副作用（実測）: joiner が自分で slot を取ると共有 `last` 連鎖が前に進むため、leader（bg）の待ちが増える（390 → 480 ms、image 360 → 420 ms）。joiner は早く受け取るが「同一内容 1 回」が消える（2 call）。
- 案2 は gate 成分だけ先行させる（image: 360 ms → 120 ms）。leader の network latency（679 ms）は解消しない（run B の joiner 壁 800 ms = gate 120 + network 679）。
- 逆方向（run C）は現状でも逆転しない：join は leader の fg lane を継承し、joiner は自分の bg lane 待ち（300 ms 超）より早く（180 ms）受け取る。

### 今の挙動 → 変更後の挙動（案2 採用時）

- 今: prefetch（background）が同一 key を dispatch した瞬間、後から来た表示要求（normal/foreground）は join するだけで、leader の background lane 予約（フォア待機yield＋grace）を待つ。表示待ちの実測 390 ms（data）／1039 ms（image・network 含む）。
- 変更後（案2）: join は維持、呼出し 1 回も維持。group の effective lane = 参加者の最高優先度なので、fg joiner が来た時点で leader は fg FIFO slot（`last + interval`）で dispatch できる。表示待ち 181 ms（data）／800 ms（image、network 支配）。
- 変化しないもの: 同一 key 1 回、joiner の gate slot 消費（0）、FIFO 算術（共有 `last` 連鎖）、grace 300 ms、3×interval 公平枠、canceled waiter 非伝播、metrics kind 分類、`recordMarkerRequest` per request、TN 表示源・TN display fallback 禁止・`x-cache` 判定・generation guard、R-13 の fetch group / write group 分離（D-05）。

### 正典照合（破らないこと）

- `docs/ai/decisions/2026-11-05-phase4-two-lane-gate-implementation.md`（FIFO 共有 last 連鎖・background は今この瞬間のみ予約＝未来予約禁止・3×interval 公平枠・grace 300 ms・フォア待機中は譲る）。案2 は fg joiner のために background dispatch を fg 未来予約の時刻へ Advance する。FIFO 算術と interval 遵守は不变だが、「background は未来予約しない」の点だけ正典ルール変更（承認が必要）として分離する。案3（現状維持）は正典ルール変更なし。
- `docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md` R-11（interval 値・フォア遅延の意味）と R-13（案 A 承認・join・network 中に mutex を保持しない・key 単位）、D-05（fetch group と write group 分離維持）。参照: `docs/ai/tasks/2026-11-06-r13-singleflight-tn-info-nozomi.md`（R-13 本記録）。案2 は R-13 の承認済み案 A（join・1 回）と衝突しない。案1 は「同一内容 1 回」の承認済み意味を失う（実測 2 call）→ R-13 意味変更として別途承認が必要。
- TN 正典（`docs/ai/decisions/2026-11-01-root-and-tn-display-correction-policy.md`・`docs/ai/decisions/2026-11-01-fast-nonblocking-viewer-policy.md`）: 三案とも TN を reader 表示源にしない・TN display fallback 禁止・TN バイトを full cache に保存しない・`x-cache` source 判定（joiner は `tn` 保持）・generation guard を変えない。
- 正典文書同士の状態記述の矛盾（日付優先）: `docs/ai/decisions/2026-11-05-search-speedup-recommended-approved.md` の data lane 100 ms は `2026-11-06-review-findings-approval-decisions.md`（R-11・90 ms）で点 supersede。実測は新しい正典値（90 ms）と image lane 60 ms（`2026-11-06-tn-image-lane-60ms-approved.md`）を使用。採用文書: 2026-11-06 側（日付優先）。
- R-13 参照の gate 意味（joiner は gate lane を消費しない）は案2で維持、案1で失う。

### 既存テスト照合（変更しない）

- `app/viewer_test.go:4371`（TN: calls 1・join 4・joiner x-cache `tn`・marker per request 5・canceled joiner `no_tn` 502・leader 1 call）、`:4460`（info: calls 1・join 4・shared pointer）、`:4518`（nozomi: calls 1・join 4・per-call copy）、key 境界（variant/cacheKey 別 group → 2 calls）。
- `app/hitomi_test.go:959`（フォア待機中に background は slot を予約しない・未来予約禁止・公平枠）。
- 既存テストは join 数・gate slot 消費（joiner 0・leader 1）を固定しているが、混合優先度（leader bg・joiner fg）は固定していない。案2 は join 数と joiner slot 消費（0）を変えないので既存テストは通過。案1 は join 数（4）と calls（1）の固定アサートを壊す → 既存テスト変更が必要（この項目の制約外）。

### 代替案（比較した結果）

- 案1（joiner が自分の lane で dispatch・「同一 key pending」参照）: joiner は 180 ms／800 ms で受け取るが同一 key 二重 call（R-13 意味喪失）、既存テスト 2 件（join 数・calls 1）が壊れる、leader の待ちを増やす（実測 +90 ms）。採用しない。
- 案3（現状維持）: 正典ルール変更なし・既存テスト非触・上界は 3×interval＋grace（実測 379 ms）。追加コストなし。体感（display 待ち）は現状のまま。
- 案2（推奨）: 下記推奨欄。

### 推奨（案2・group priority inheritance）

- 単一 call 保証と join を維持したまま、group の effective lane を参加者最高優先度にする：fg joiner が group に参加した時点で、leader の dispatch 時刻を fg FIFO slot（`last + interval`）へ先行させる（joiner の gate 待ちだけ先行させる）。実測改善 209 ms（data）／240 ms（image の gate 成分）。network latency は解消しない。
- 実装形（承認後）: group に参加者の priority を記録（key 単位の call に `hasFG` 相当）、`gateLane` の effective lane を group 参照で決める、joiner arrival で leader の再評価（wake）。canceled joiner は参加を取り消す（priority claim を解放）。既存 helper 流儀（key 単位・mutex・network 中に mutex を保持しない・joiner は join wait）は維持。write group は非触（D-05）。
- 承認が要る論点（分離）: 「background は未来予約しない」の点への例外（group に fg joiner がいる場合のみ fg slot 時刻で dispatch）。正典ルール変更として承認を得てから実装する。metrics の `gate_wait_ms` 数値は leader の lane 归属で変わる（kind 分類意味は不変）。
- 実装規模（推定）: 小〜中（`app/hitomi.go` gateLane に group priority 参照 1 箇所、`app/viewer.go` 3 helper と `app/searchhtml.go` に joiner priority 記録、新規テスト 2〜3 件、既存テスト変更なし）。

### 副作用・影響範囲

- ユーザーに見える側（display 待ち）: search card cover・reader・gallery info の表示待ちが、prefetch 先行直後の冷条件で短くなる（案2）。表示源・`x-cache` 判定・generation guard は不変。
- politeness gate: 案2は interval 算術（FIFO・共有 last 連鎖）を変えない。案1は同一 key の slot を増やし、実測で leader 待ちを増やす（stampede 側）。
- metrics: joiner は計上されない（既存意味）。案2では leader の gate_wait 归属 lane が変わるため数値が変わる（意味・kind 分類は不変）。observability 上、joiner 待ちが metrics に現れない現状（R-13 記録済み）は三案共通。
- 依存: R-13（案 A・D-05）、Phase 4 2 レーン gate（FIFO・grace・priority）、P3 #10（gate/network 分離実測）で支配成分を分けてから優先度付けすべき。R-11 の interval 値は非触。

### 画面確認

- UI 項目でないため served 画面確認は不要として記録。案2 は display 待ちの体感を変える案なので既存ルールで確認できる範囲を試みたが、正典 source を変更できない（本項目は記録のみ）ため変更後の画面は**未確認**。cold hygiene 条件（共用 Playwright が並行 session のため per-run 条件が成立しない）で変更前表示待ちの before/after も成立せず → Drive 保存なし・未確認として記録。数値根拠は mock 実測（外部 API なし）。

## Verification

- `go vet .`（`app`）: clean。`go test -count=1 -timeout 300s .`（`app`・既存テスト変更なし・ソース変更なし）: `ok hitomi-hub/app 26.005s`。経過 26.005 s。
- scratch 実測: `go vet .` clean、`go test -v -count=1 -timeout 180s .`（module `assess202611081332r12`・`docs/ai/tmp/assess-20261108-1332/r12/n12_measure_test.go`）→ `ok assess202611081332r12 5.745s`（経過 5.745 s・1 test・9 run）。数値は `docs/ai/tmp/assess-20261108-1332/r12/results.json`。
- `python -m pytest tests/ -v`: 非触（この項目は Python・AGENTS.md 非変更）。既存 failure（`test_canonical_build_freshness_rule_exists`・R-10 報告済み KI 既存 failure）は本項目と無関係。
- 編集後の文件破損確認: 変更したのは個別タスク記録のみ。ソース・既存テスト非触。`Set-Content` パイプライン不使用。
- 画面確認: 未確認（上記）。Drive 保存なし。

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除（親が `task_manager` で管理・子は変更しない）
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当なし・本項目は指摘の有効性判定のみで issue を解決していない）
- [ ] `backlog.md` の完了項目を削除（該当なし・項目は未実施のまま）
