# 2026-11-09 — singleflight group priority inheritance（案2）: 「background は未来予約しない」への限定例外（ユーザー承認）

**Date**: 2026-11-09
**Status**: active（ユーザー承認 2026-11-09・前回委譲 assess-20261109-1320 の段階3持ち帰りに対する判断）
**Task**: `docs/ai/tasks/2026-11-08-n12-singleflight-lane-priority-inversion.md`（項目 #13・実行固有ID assess-20261109-1320）

## Decision（承認済み）

1. **正典例外（限定・supersede ではない）**: `docs/ai/decisions/2026-11-05-phase4-two-lane-gate-implementation.md` の background 条「今この瞬間のみ予約（未来予約禁止）」に対して、次の限定例外を追加する: group（key 単位の singleflight call）に foreground 参加者（joiner）がいる場合に限り、leader は fg FIFO slot 時刻（`last + interval`）で dispatch できる。FIFO・共有 `last` 連鎖・interval 算術（data 90 ms・image 60 ms）・grace 300 ms・3×interval 公平枠は不变。例外が働くのは group に fg joiner が参加している場合のみ。
2. **実装形（候補3・key 台帳）**: `Client` 内の group registry（key 単位の map・group→claim）＋既存 mutex（`mu`）で joiner priority claim を保持する。`gateLane` は ctx に載った key を参照して registry から effective lane を決める。`gateLane` の 4 args signature（`ctx, intervalMS, st, background`）は保持（既存 gate 固定テスト②③④の直接呼出 pin を壊さない）。固定テストの承認済み更新は 1 件のみ: `app/hitomi_test.go` の「フォア待機中に background は slot を予約しない・未来予約禁止・公平枠」（委譲時 959 行）を 4 args pin → key 参照へ更新。
3. **per-lane claim の拒否**: claim を `gateLaneState`（lane 単位）に置くと同一 lane の無関係 group（他 key）の bg dispatch へ priority claim が漏れ、承認済み意味「group effective lane = 参加者最高優先度（key 単位の group）」を壊す。carrier は key（ctx）のみ。
4. **join helper**: `fetchTNOnce`・`fetchFullImageOnce`・`resolveInfoOnce`・`SearchNozomiParts` で joiner 参加時の priority claim 記録、canceled joiner の claim 解放。info join の guard 追加を承認（挙動は TN と同じ「即 return」へ改善）。既存 info 流儀の pin（`v.info(id, background)`・`Client.Gallery(id, background)` の signature）は wrapper で維持し、guard と key 参照は core 側（`infoCtx`・`galleryCtx`・`resolveInfoOnce(ctx, ...)`）で追加する。`SearchNozomiParts` は join block に ctx guard がない既存流儀（pin）なので canceled joiner が存在せず、claim 解放は該当なし。
5. **変えないもの（合格条件）**: joiner の gate slot 消費 0、「同一内容 1 回」、network 中に mutex を保持しない、key 単位、R-13 案A、D-05（fetch group と write group 分離）、canceled waiter の既存意味（joiner 即 return・leader 非影響）、metrics kind 分類意味、`recordMarkerRequest` per request、TN 正典（reader 表示源禁止・display fallback 禁止・TN バイト full cache 保存禁止・`x-cache` source 判定・generation guard）、gate 正典値（politeness 1000・下限 guard 50・block cooldown）。

## 根拠（実測）

- 機構・数値は assess-20261108-1332 と assess-20261109-1320 の mock 実測（正典流儀・外部 API なし）で確定: bg leader＋fg joiner の joiner 壁 data 390 ms → 案2 で 180 ms（gate 成分改善 209 ms）、image 1039 ms → 800 ms（gate 成分改善 240 ms・network 679 ms は解消しない）。同一 key 1 call・1 slot・joiner slot 0 維持。canceled joiner 即 return（0 ms）・leader 非影響。上界（3×interval＋grace）非変更（実測 380 ms・正典上界 379–380 ms の範囲）。
- 実装後の決定論再実測（`docs/ai/tmp/assess-20261109-1320/r12b/`）で数値一致を確認（記録: 個別タスク記録）。

## 意味変更の境界

- ユーザーに見える変化（同意済み範囲）: cold 窓（search page prefetch が bg leader）での display 待ち短縮。表示源・`x-cache` 判定・generation guard・join・1 回・canceled waiter 意味は不変。
- metrics の `gate_wait_ms` 数値は leader の lane 归属で変わることがある（kind 分類意味は不变）。observability 上 joiner 待ちが metrics に現れない現状（R-13 記録済み）は不变。

関連: `docs/ai/decisions/2026-11-05-phase4-two-lane-gate-implementation.md`（FIFO・未来予約禁止・公平枠・grace — 本 entry はその background 条への限定例外のみ追加・文書自体は不変）、`docs/ai/decisions/2026-11-06-review-findings-approval-decisions.md`（R-13 案A・R-11）、`docs/ai/decisions/2026-11-06-tn-image-lane-60ms-approved.md`。
