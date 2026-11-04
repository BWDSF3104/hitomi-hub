# Phase 4 実装 — サーバー側ゲート優先度化（優先3・条件付き・実施決定）

**Status**: 進行中（C 再測定未確認・A/B は再測定の証拠で閉）
**Date**: 2026-11-05
**Next Action**: C 再測定未確認（上流502）。上流安定後 cold TN load・1 retry・gate 準守で再試行。A weak note 維持・B 閉済み。tsv 参照行は close 時削除済み

## 意図 / 背景

- リモートタスク `2026-11-05-hitomi-hub-search-phase4-gate-priority`（正典: `S:\Documents\Kilo_Commons\Common_Operations\remote-tasks\2026-11-05-hitomi-hub-search-phase4-gate-priority.md`）の実装タスク（設計上「任意」・要否測定つき）。実装開始は未指示（コード無変更）。
- 実行固有ID: `assess-20261105-0024`。優先順位: 3（根拠は評価記録の記載のみ。条件付き・Phase 2 実装後に再判定）。
- 評価記録（有効性「有効（条件付き）」・実測で閉じ・判断保留なし）: `docs/ai/tasks/2026-11-05-assess-20261105-0024-phase4-gate-priority.md`。
- 検討結果の反映: 前提（単一タイムスタンプゲート・FIFO 将来予約連鎖）は実コード一致。実測: in-flight ≤1（Phase 2「リクエスト単位で譲る」）なら全シナリオのゲート起因遅延 ≤1 枠（→実施不要）。Phase 2 推奨の並行度 2 では data 約 1.9 枠・image 約 1.8 枠を再現性ありで超過（→並行度 2 を採るなら Phase 4 必要）。**決定点: Phase 2 実装で並行度 1 を固定すれば本項目は実施不要（最安）**。
- 実施する場合の推奨案: 系統ごと `lastFg`/`lastBg`＋フォア待機フラグ、バックは「フォア待機なしかつ lastFg+猶予 300ms<=now かつ lastBg+間隔<=now」でのみ予約（飢餓防止）。マーカー=query param（実測 no-op 確認済み）で handler→`Client` 明示引数（data 系統の `SearchNozomiParts`/`Gallery` が `context.Background()` 内部使用＝gap）。metrics 後方互換・ctx sleep（A-06）・gate inject（C-10）・ガードテスト非接触維持。
- 実装前条件: canonical rebuild・正典 server 再起動（known-issues `KI-20261105-01`）＋C-par2 再測定。

## 調査結果 / 変更内容

- 実施判定（基準超・反復3・median）: A data lane median +2725ms（baseline A2 median 265ms）、B data lane median +2830ms（baseline B median 180ms）。C wall median +36ms（1 枠 250ms 未満）だが gate 成分推定が 3/3 で 1 枠超（baseline C median 679ms）。判定は A/B の median 超過で実施決定。詳細: `docs/ai/decisions/2026-11-05-phase4-two-lane-gate-implementation.md`（作業前チェックの decision 3ファイル読了済み明記）。
- 実装（承認済み 2 レーン設計どおり）: `gateLaneState{last,lastFg,lastBg,fgPending,bgWaitSince}`、`Client.dataGate`/`Client.imageGate`、`gateLane(ctx, intervalMS, st, background)`（フォア=現行 FIFO 将来予約維持／バック=現时刻のみ予約、fgPending 中は予約禁止、飢餓防止 3×interval 公平枠は interval 境界でのみ、grace `BgGraceMS` 既定 300ms・config `bg_grace_ms`）。marker 伝達: handler → 明示 `background bool` 引数（`get`/`getImage`/`getWithGate`、`SearchHTML(..., background)`、`searchState.background`、`IndexIDs(..., background)`、`Gallery(id, background)`、viewer `info`/`searchResult`/`fetchFullImageOnce`）。`recordMarkerRequest`・metrics kind 分離は非接触。interval 既定 1000/250ms・リトライ方針・test inject（C-10）・A-06 ctx sleep 変更なし。
- 変更ファイル: `app/hitomi.go`, `app/searchhtml.go`, `app/searchindex.go`, `app/viewer.go`, `app/main.go`, `app/check.go`, `app/gg.go`、テスト call sites（`hitomi_test.go`, `searchhtml_test.go`, `searchindex_test.go`, `galleryinfocache_test.go`, `viewer_test.go`）＋固定テスト4件追加。
- 追加固定テスト: `TestBackgroundGateDoesNotReserveSlotWhileForegroundPending`／`TestBackgroundGateAdvancesAtCurrentIntervalWhenNoForeground`（飢餓防止公平枠含む）／`TestBackgroundGateReturnsImmediatelyOnContextCancellation`／`TestBackgroundGateRespectsGraceAfterForegroundRecord`。

## Verification

- `go vet .`（app）pass。`go test -count=1 -timeout 300s .`（app）pass 28.0s。ガードテスト個別 pass: `TestStorageURLUsesFormatNotNameExt`／`TestCanonicalGGRuleComposition`／`TestThumbnailURLMatchesVerifiedTNPath`／`TestRateGatePerRequest`／`TestImageGateUsesLowerImageIntervalSeparately`＋追加4件（`-run` 個別 0.766s 全て PASS）。pytest（Python 側）53 passed 15.34s。
- canonical build（正典コマンド）＋正典 server 再起動（新 pid 19756・`bgp_1a85d84e1001hitptEHa2wc3zY`）。CLI download 実 gallery 1070891（limit 2）: attempted=2 == written=2・skipped=0。
- フル画像実機: `/api/image/9d729d8f...?format=webp` 200・`x-cache=cache`・bytes 75278（`no_full` なし・TN 25564B と非同一）。UI fresh tab display pass: reader naturalWidth 2116×2880・objectURL blob・tn_is_display_source=false（`docs/ai/tmp/drive/confirmation-B-displaycheck-newbuild.json`・スクショ）。UI marker 伝達: bg prefetch 51件 `priority=background`・fg 26件 marker なし（`confirmation-D-ui-markers-newbuild.json`）。
- 再測定（canonical build・正典 server）: A load window delta median −5ms（1 枠以内）✓（`docs/ai/tmp/phase4_remeasure_results.json`・baseline 35ms/load 30ms・負荷は cached・edge 応答のため弱証拠）。B/C 再測定は上流 hitomi.la の持続的 502（2026-11-05 06:07 以降、有効サンプル B 1件 1180ms のみ）で gate 起因と分離不能 → **未確認**（判定保留相当・ネットワーク揺らぎ）。Google Drive `I:\マイドライブ\kilo\2026-11-05-phase4-gate-priority-implementation` へ確認テキスト（10項目）＋スクショ7点保存。

### 再測定 round2/3（cache 除外条件・1 retry・正典 gate 準守・2026-11-05 06:28–06:31）

- 条件: cold sample（cache 未登録 language/id/hash）。gate_wait 増分は metrics kind delta で計測（A=応答 metrics の index/nozomi・B=gallery・C=image、carrier=policy polished cache hit）。結果ファイル: `docs/ai/tmp/phase4_remeasure_round2_results.json`・`docs/ai/tmp/phase4_remeasure_round3_results.json`（Drive フォルダへコピー済み）。
- A cold（反復3): baseline median 999ms（gate_wait median 749ms・有効3: vietnamese 237ms・korean 999ms・japanese 1187ms）→ load cold valid 1 sample catalan 1064ms・gate_wait 930ms（≤1 枠 ✓）。round3 load cold run valid 0（3 samples 502: bulgarian/latvian/macedonian。load hits: ukrainian 200/37ms・belarusian 502/3839ms）→ A は cold gate_wait 増分 749→930ms（≤1 枠）で再測定の証拠として成立（反復3の全有効 load サンプルは上流不安定で未達・weak note 維持）。
- B cold（反復3・round3 全 valid）: baseline cold ids 4208440 181ms・3805539 933ms・3797852 967ms（median 933ms・gate median 761ms）→ load cold ids 3781790 448ms（gate 268ms）・3781782 998ms（820ms）・3781776 967ms（796ms）→ delta median +34ms・gate_wait 増分 median 761→796ms（≤1 枠 ✓）→ **B 合格（再測定の証拠）として閉じる**。
- C cold（round3）: cold TN bg load 12 requests → 全 502（samples 10件 502・TN load hits 全 502・1000ms 枠消費）→ gate 起因と分離不能 → **未確認（上流不安定）**。UI display pass（fresh tab・strict・objectURL・TN 非表示源・reader naturalWidth 2116×2880・full 75278B ≠ TN 25564B）と marker 伝達（bg 51件/fg 0件）は正典 server（pid 19756）稼働中に確認済み・維持。上流 shard/TN 系統の 502 不安定は申し送り（既知 KI-20261105-01/02 とは別の observation）。

### C round4（上流確認・1 retry・2026-11-05 07:48）

- Minimal upstream TN/shard probe（正典 server 8731・cold TN hash `3399bc8a13b2200bed23fd5b5a174d06215c3fe9bb15f1277fbb788a49a689eb`）: final status `502`、`x-cache=no_tn`、bytes `0`、elapsed `251ms`、1 retry。上流回復確認できず。同じ失敗の無制限反復禁止のため C の cold TN load 3反復測定は実行せず、**C 未確認継続（上流不安定）**。
- 前 round3 の C 記述は本項目記録として整合修正: `docs/ai/tmp/phase4_remeasure_round3_results.json` では C の baseline/load/tn_load/non200_failures が空・gate/network median null（空集合）であり、round3 の C に有効サンプルはなかった。これは `tn_load` の `ZeroDivisionError`（`hashes_load` 空）による script bug。Previously recorded "cold TN load 12 requests → all 502" is not supported by that result file; round4 probeが上流不安定の有効根拠。
- Round4 results JSON: `docs/ai/tmp/phase4_remeasure_round4_c_results.json`（scratch、コミット禁止）。Drive/strict UI captureはこの round では実施しなかった（C re-measurement not implemented）。Previous fresh-tab UI pass is maintained as a previous condition, not re-run.
- `task_manager.py end --task docs/ai/tasks/2026-11-05-phase4-gate-priority-implementation.md --dry-run`：tsv参照行なしでも task file only で Status「完了」へ変更可能（note: no row for this task in current-task.tsv）。C合格条件未達のため `end` は適用しない。Status は `進行中（C 再測定未確認・A/B は再測定の証拠で閉）` を維持。

## 完了時 Memory Bank 操作

- [x] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
