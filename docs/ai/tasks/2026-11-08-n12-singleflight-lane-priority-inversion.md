# N-12: singleflight で leader の lane により優先度が逆転する

**Status**: 完了
**実行固有ID**: assess-20261108-1332
**Date**: 2026-11-08
**Next Action**: 実装（案2）- group registry 案（候補3）ユーザー承認 2026-11-09: gateLane key 参照＋info join guard 追加・pin 1件更新承認済み・委譲準備

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

## 2026-11-09 再調査（assess-20261109-1320）

### 現在の実装（機構照合・確定・前批と同一）

join 構造は前批（assess-20261108-1332）記録と同一。正典値変更・join 構造変更は前批以降の入った commit（settings 系・Type filter・cache clear）にもなし（`git log --since 2026-11-08` 照合。joiner が gate に入らない・joiner の `background` 引数は join 後に使用されない・leader 単独が `getWithGate` を呼ぶ、という機構は全 helper で維持）。現行行番号（確認時点）:

- `fetchTNOnce`（`app/viewer.go:1520-1556`）: join block `1527-1538`（`call.done` / `ctx.Done()` select）。leader fetch `1544`（`getImage(..., background)`）。join observer hook（`1528-1530`）追加済・join 意味は同一。
- `fetchFullImageOnce`（`app/viewer.go:1558-1594`）: join block `1565-1576`（同 select）。leader fetch `1582`（`StorageURL`）。
- `resolveInfoOnce`（`app/viewer.go:1882-`）: join block `1887-1894`（`<-call.done`・ctx guard なし・既存流儀）。leader fetch `1900`（`client.Gallery(id, background)`）。
- `SearchNozomiParts`（`app/searchhtml.go:114-197`）: join `139-150`。leader fetch `155`（`state.background`）。joiner の `state.background` は join 後も不使用。
- `EnsureGG`（`app/gg.go:118-201`）: join `150-157`。leader fetch `163` で常に `false`（foreground）→ 逆転余地なし（前批どおり）。
- gate 算術（`app/hitomi.go:495-564`）は呼出側の `background` だけでレーンを決める。joiner は `getWithGate`（`574-589`）自体を呼ばない → joiner の priority は算術に入らない（前批どおり）。

### gate 正典値の現行実測（現行コード値・前批正典値と一致）

- data lane 90 ms（`app/hitomi.go:319`・commit `a9f7786` R-11b。`config.example.json` 同値）。image lane 60 ms（同一 line。decision `2026-11-06-tn-image-lane-60ms-approved.md`）。grace 300 ms（`defaultBgGraceMS`・`app/hitomi.go:467`）。下限 guard 50・politeness 退避先 1000・block cooldown（`503-508`・max 適用）は非触。
- FIFO・共有 `last` 連鎖（`517-531`）、foreground 未来予約、background「今この瞬間のみ予約・未来予約禁止」（`534-544`）、公平枠 3×interval（`537`）— 正典 `2026-11-05-phase4-two-lane-gate-implementation.md` のまま。2026-11-09 の settings 系 commit は gate 値・gate semantics 非変更（`git diff a9f7786..HEAD -- app/hitomi.go` に interval・BgGraceMS の値変更なし）。

### 再実測（正典流儀 mock harness・9 run・外部 API 呼出しなし）

scratch replica（`docs/ai/tmp/assess-20261109-1320/r12/`・module `assess202611081332r12`・`go test -count=1 -timeout 180s .` → `ok 5.759 s`）は現行 `gateLane`（`app/hitomi.go:495-564`）の条件式をそのまま再現（block cooldown は scenario で未発動・replica 非モデル）。正典値 data 90 ms・image 60 ms・grace 300 ms。結果 `docs/ai/tmp/assess-20261109-1320/r12/results.json`:

| Run | 案 | 同一 key の外部呼出し | 同一 key の gate slot | joiner 壁時間 | leader 壁時間 | 前批（assess-20261108-1332） |
|---|---|---|---|---|---|---|
| A data 90 ms・network 0（bg leader + fg joiner） | 案3 現状（join） | 1 | 1 | 390 ms | 390 ms | 390 / 390（一致） |
| A | 案1（joiner 自分の lane） | 2 | 2 | 180 ms | 480 ms | 一致 |
| A | 案2（group effective lane = 参加者最高優先度） | 1 | 1 | 180 ms（前批 181・jitter） | 180 ms | 一致（±1 ms） |
| B image 60 ms・network 679 ms（Phase 4 基線 TN median） | 案3 | 1 | 1 | 1039 ms | 1039 ms | 一致 |
| B | 案1 | 2 | 2 | 799 ms（前批 800） | 1099 ms（前批 1100） | 一致（±1 ms） |
| B | 案2 | 1 | 1 | 800 ms（gate 120＋network） | 800 ms | 一致 |
| C fg leader + bg joiner（逆方向） | 案3 | 1 | 1 | 180 ms | 180 ms | 一致（逆転なし） |
| D 上界（正典公平枠） | 案3 | 1 | 1 | 380 ms（前批 379） | 390 ms | 一致（±1 ms） |
| E canceled waiter | 案3 | 1 | 1 | 0 ms（即 return） | 300 ms（1 call） | 一致（leader 非 cancel） |

数値は前批と jitter（±1 ms）内で一致。機構（joiner は gate slot 消費 0・joiner priority は算術に入らない・案2 の改善は gate 成分 209 ms（data）/ 240 ms（image）・network latency は解消しない）は変更なし。

### 混合優先度が現行 UI で成立する経路（server singleflight と client priority の対応関係）

- 伝達経路: handler の `priority` query param（例 `app/viewer.go:1597`・`1311-1314`）→ `background bool` 引数 → singleflight helper → leader だけ gate 算術に入る。joiner の `background` は join 後も gate に参加しない（joiner は `getWithGate` を呼ばない）。
- client 側 marker（`prefetchMarkerUrl`・`app/search.html:872-874`）が `priority=background` を付けるのは search page の prefetch 経路のみ（`/api/search`・`/api/search-card/`・cover（TN））。表示要求は marker なし（normal）。
- 現行で mixed priority が成立する経路（確定）: TN（prefetch bg → display join・image lane）、nozomi data（prefetch bg → display join・data lane）、info（search-card prefetch（bg marker）→ `searchResult(id, background)`（`app/viewer.go:1332-1334`）→ `info(id, background)` → cold 時 `resolveInfoOnce`（bg leader）→ viewer 表示要求（marker なし）が同一 gallery id key で join）。
- 追加確認（前批からの差分・full image）: `app/viewer.html` の background prefetch（`startFullPrefetch`・`loadFullImage`）は `priority` param を付けない（`fetch(fullSrc)`・marker なし）。よって full image singleflight（`fetchFullImageOnce`）は現行 UI 経路で bg marker 呼出しが存在せず、leader は always foreground → full image の逆転は機構上有効だが現行経路では到達しない（full join は fg→fg 継承）。実害は TN・nozomi・info の 3 key に限定される。

### 正典照合（現状維持確認）

- R-13（案 A 承認・join・同一内容 1 回・key 単位・network 中に mutex 非保持）は現行実装どおり。案1 は「同一内容 1 回」を失う（再実測 2 call・2 slot）＋ join 数・calls 1 の固定アサート（`app/viewer_test.go` join tests・`app/hitomi_test.go`「フォア待機中に background は slot を予約しない・未来予約禁止・公平枠」）を壊す — 現状維持（固定テストは現行 pass）。案1 は採用しない。
- 案2（group effective lane = 参加者最高優先度）の正典例外（「background は未来予約しない」への例外）は承認案件のまま。前批以降の decision（2026-11-08/09 の settings 系・Type filter・cache clear）は gate semantics・priority 意味に一切触れておらず、この例外の承認記録なし → 保留状態は変更なし。案3（現状維持）は正典ルール変更なし。
- 正典文書同士の矛盾: 本再調査で新規矛盾なし。data lane 90 ms（`2026-11-06-review-findings-approval-decisions.md` R-11 が `2026-11-05-search-speedup-recommended-approved.md` 100 ms を supersede）を引き続き採用（前批どおり）。
- TN 正典（reader 表示源禁止・display fallback 禁止・TN バイト full cache 保存禁止・`x-cache` source 判定・generation guard）は三案共通で非触（現行 `fetchTNOnce` joiner の `x-cache: tn` 意味維持）。

### 有効性: 有効（機構確定・再実測で閉じた）

- 逆転は機構として確定、実測数値は前批と一致（joiner 壁 data 390 ms・image 1039 ms・上界 380 ms）。
- 範囲の縮小（今回の確定点）: 現行 UI で mixed priority が到達するのは TN・nozomi・info（search page prefetch が bg leader になる cold 窓）のみ。full image singleflight は marker なし経路のため現行 UI で逆転不成立。UI に bg marker 付き full prefetch がない限り、full image 分の指摘は「機構上有効・経路なし」。
- 推奨は前批どおり案2（join・1 回維持・group priority inheritance・joiner gate 待ちだけ先行）。正典例外の承認が要る。案3（現状維持）は追加コストなしで上界 380 ms。
- 実装規模（推定）: 小〜中（前批と同一。`app/hitomi.go` gateLane に group priority 参照、viewer/searchhtml の join 側 priority 記録、新規テスト 2〜3・既存テスト非触）。

### 画面確認

- UI 項目でない（画面表示源・`x-cache` 判定・generation guard を変えない）。本再調査はコード変更なし・案2 は未承認のため変更後画面は該当なし（省略）。前批の Drive 保存なし・未確認記録のまま。

### Verification（本再調査）

- `go vet .`（`app`）clean。`go test -count=1 -timeout 300s .`（`app`・既存テスト変更なし）: `ok hitomi-hub/app 26.140s`。経過 26.140 s（R-13 join 固定テスト・gate 固定テスト pass 含む）。
- scratch 再実測: `go test -count=1 -timeout 180s .`（module `assess202611081332r12`・`docs/ai/tmp/assess-20261109-1320/r12/`）→ `ok 5.759s`・9 run（前批と同一条件）。外部 API 呼出しなし。数値 `results.json`・出力 `rerun_output.txt`。
- `python -m pytest tests/ -v`: 非該当（Python・本項目非触。docs/ai のみの変更は検証不要の正典条による。Go 項目の実測として上記を実行）。
- 変更したのは個別タスク記録のみ。ソース・既存テスト非触。`Set-Content` パイプライン不使用。`current-task.tsv`・Status・Next Action 非変更（親管理）。

## 2026-11-09 実装段階（assess-20261109-1320 子）

### 実装方式（確定・承認範囲内の分）

- 方式（正典流儀・mock harness 流儀と同一機構）: key 単位 call に per-key claim（参加者 priority 記録＋wake）。`gateLane` の 1 箇所参照は ctx 経由（join helper が joiner priority を call の claim に記録し、leader fetch の ctx へ `context.WithValue` で伝達。`gateLane` は ctx の claim から effective lane を決め、fg 参加者がいる group だけ fg FIFO slot（`last + interval`）で dispatch。joiner arrival の再評価（wake select）も `gateLane` 内（同一機構）。
- `gateLane` の signature（`ctx, intervalMS, st, background` 4 引数）は保持必須: 既存固定テスト `app/hitomi_test.go:959`（フォア待機中に background は slot を予約しない・未来予約禁止・公平枠）等が `gateLane(ctx, intervalMS, st, background)` を直接呼出（4 args pin・実測 pass）。claim を引数追加で渡す方案は既存テスト変更になり不可。
- per-key 正確性: `st` は lane 共有状態（dataGate に nozomi・info が共有、imageGate に TN・full が共有）なので、claim を `gateLaneState`（lane 単位）に置くと同一 lane の無関係 group（他 key）の bg dispatch へ priority claim が漏れる。これは承認済み意味「group effective lane = 参加者最高優先度（key 単位の group）」を壊す（未承認 group の意味変更）ので不可。claim は ctx 経由でなければならない。
- claim 型・ctx key・accessor は join helper 側ファイル（`app/viewer.go`）に置き、`app/hitomi.go` の変更は `gateLane` の参照 1 箇所（＋wake select）に限定する。
- 承認範囲内で配線可能（確定）: `fetchTNOnce`（`app/viewer.go:1524`・ctx guard `1536-1541`）・`fetchFullImageOnce`（`app/viewer.go:1562`・ctx guard `1574-1579`）は ctx があり、joiner priority 記録・canceled joiner の claim 解放まで実装可。`SearchNozomiParts`（`app/searchhtml.go:114`）は join block に ctx guard がない（R-13 既存流儀・固定テスト pin）ので canceled joiner が存在せず claim 解放は該当なし。ただし leader fetch の `context.Background()`（`app/searchhtml.go:155`）は helper が制御するので claim 伝達（priority 記録）は可。

### 段階3（持ち帰り・実装前の壁）

- `resolveInfoOnce`（`app/viewer.go:1886`）の joiner priority 記録が `gateLane` に届く経路が承認範囲内に存在しない:
  - helper に ctx がない（join は `<-call.done` の ctx guard なし流儀・「既存 guard test の signature を変えないため info は ctx を持たない」が記録済み。既存 info join 固定テストが signature を pin）。
  - leader fetch `v.client.Gallery(id, background)`（`app/viewer.go:1900`）は `Client.Gallery`（`app/hitomi.go:899-900`）内部で `context.Background()` を生成して `c.get` へ渡すため、gate 呼出の ctx は helper 側から attach できない。
  - `gateLane` は per-key の key/URL を持たない（ctx・intervalMS・st・background の 4 引数のみ）なので、per-key claim を参照できる carriers は ctx・per-lane storage のみ。per-lane storage は上記の通り group（key 単位）意味を壊す。
- 解決案はすべて承認範囲超（いずれかの変更を要る）: (a) `resolveInfoOnce`/`Gallery` 系に ctx を通す signature 変更（既存 guard test 流儀・comment と衝突）、(b) claim lookup を `gateLane`（承認 1 箇所）ではなく `getWithGate` 側へ移す、(c) lane 単位の claim（未承認 group の意味変更）。
- 因此、この項目の info 分は実装不能。実装を開始せず、decision エントリも未作成（例外承認自体は前批どおり。実装形確定待ち）。作業ツリーは委譲前状態（HEAD `e3e7daf`・clean・source 変更なし）のまま。合格不能のため `task_manager end` は行わず進行中 Maintained。

### 画面確認（本段階）

- UI 項目でないため served 画面確認は該当なし（既存ルールによる・実施なし）。

### Verification（本段階・実装前確定のみ）

- `go vet .`（`app`）: clean。`go test -count=1 -timeout 300s .`（`app`・既存テスト変更なし・source 変更なし・HEAD `e3e7daf`）: `ok hitomi-hub/app 27.536s`。経過 27.536 s。TN join 固定テスト・info join 固定テスト・nozomi join 固定テスト・gate 固定テスト（`hitomi_test.go:959`）変更なしで通過。
- `python -m pytest tests/ -v`（HEAD・変更なし）: `64 passed in 26.01s`。
- harness 再実測: 実施せず（実装形が info 分で確定できず、「実装形」の再実測が成立しない。前批の機構数値 data 209 ms／image 240 ms・同一 key 1 回・1 slot は機構として有効なまま。本段階では推翻していない）。
- 変更したのは個別タスク記録のみ（本節＋Next Action の task_manager 更新）。ソース・既存テスト非触。`Set-Content` パイプライン不使用。

### 実装（assess-20261109-1320 子・2026-11-09 承認形どおり・実施済み）

- decision（実装前作成）: `docs/ai/decisions/2026-11-09-n12-group-registry-effective-lane-exception.md`（正典例外＋実装形）。作業前チェックの3 decision（`2026-11-01-root-and-tn-display-correction-policy.md`・`2026-11-01-fast-nonblocking-viewer-policy.md`・`2026-11-01-ggset-generation-and-drift.md`）を読了。
- `app/hitomi.go`: `groupClaim`（fgCount・wake）＋`groupCtxKeyType`/`groupCtxKey`＋`withGroupKey`/`groupKeyFromCtx`＋`recordGroupClaim`/`releaseGroupClaim`/`deleteGroupClaim`（`mu` 保護・network 中に mutex 非保持）。`Client` に `groupClaims map[string]*groupClaim`。`gateLane` は ctx の key から registry を参照し、fg claim 決定分岐は plain sleepCtx（決定確定後 wake なし＝二重予約禁止・1 slot。初版の `continue` 二重 advance を修正）。bg path の joiner arrival 再評価は `sleepWake`/`claimWake` select。`Client.Gallery`（2 args pin）は `galleryCtx(ctx,...)` の wrapper へ。
- `app/viewer.go`: `fetchTNOnce`・`fetchFullImageOnce`・`resolveInfoOnce`（ctx 追加）で joiner 参加時 claim 記録・canceled joiner 即 return＋claim 解放。info join guard 追加（TN と同じ「即 return」）。pin 維持流儀: `v.info(id, background)`（galleryinfocache_test・viewer_test 2 args pin）は `infoCtx` の wrapper、handler 2 箇所は `v.infoCtx(r.Context(), ...)`。`searchResult(id, background)`（`app/viewer_test.go:1691` pin）は ctx なし＝guard 該当ルートなし。
- `app/searchhtml.go`: `SearchNozomiParts` join block に claim 記録（ctx guard なし＝既存 pin・canceled joiner 該当なし）、leader fetch の ctx を `c.withGroupKey(context.Background(), cacheKey)` へ、group 完了3経路で `deleteGroupClaim`。
- 固定テスト更新（承認済み1件のみ）: `app/hitomi_test.go` `TestBackgroundGateDoesNotReserveSlotWhileForegroundPending`（委譲時 959 行）の 4 args pin → key 参照（`c.withGroupKey(ctx, "n12-pin-key")`）。他のテスト・`gateLane` 4 args signature 非触。
- 新規回帰テスト `app/n12_group_registry_test.go`（5件: effective lane・per-key isolation（claim を lane 共有状態に置かない意味）・TN canceled joiner 解放・info join guard・nozomi claim＋group 完了時台帳消去）＋`claimCount`/`waitForClaim`。
- 修正バグ: 型/変数同名 `groupKeyCtxKey`（vet error）→ `groupCtxKeyType`/`groupCtxKey`。

### 決定論再実測（r12b・実装後の数値）

`docs/ai/tmp/assess-20261109-1320/r12b/`（正典流儀 mock・外部 API なし・`go test -count=1 -timeout 180s -v .` → `ok 4.380s`・9 run・`results.json`）。数値列: 同一 key の外部呼出し / 同一 key の gate slot / joiner 壁時間 / leader 壁時間（呼出し3は scenario setup のフォア窓呼出しを含む。key group 自体は 1 call・1 slot・joiner slot 0、固定テスト join tests の join 数・marker 数 pin 通過で確認）:

| Run | scheme | calls | slots | joiner | 合格条件 |
|---|---|---|---|---|---|
| A data 90 ms | current（前批 baseline） | 3 | 3 | 390 ms | 一致 |
| A | final（実装機構） | 3 | 3 | **180 ms**（gate 成分改善 209 ms） | ✓（180/181・jitter） |
| B image 60 ms・network 679 ms | current | 3 | 3 | 1040 ms | 一致（1039・±1） |
| B | final | 3 | 3 | **800 ms**（gate 成分改善 240 ms・network 非解消） | ✓ |
| C fg leader + bg joiner | current | 3 | 3 | 180 ms | ✓（逆転なし） |
| D 上界（3×interval＋grace） | current | 3 | 3 | 380 ms | ✓（上界非変更） |
| E canceled joiner | final（join helper） | 2（leader 1＋fg 1） | 2（leader 1・joiner 0） | **0 ms 即 return・claim 解放 true** | ✓（leader 300 ms・1 call 非影響。joiner 即 return を確定するため leader fetch を解放までブロック） |

- metrics: joiner 非計上（既存意味維持・kind 分類不変）。gate_wait 数値は leader の lane 归属で変わる（decision 記録済み）。

### Verification（本実装）

- `go vet .`（`app`）: clean。
- `go test -count=1 -timeout 300s .`（`app`）: `ok hitomi-hub/app 39.410s`（出力 `docs\ai\tmp\assess-20261109-1320\impl_go_test.txt`）。個別 PASS: 固定テスト① 0.20 s・effective lane 0.10 s・per key isolation 0.40 s・TN canceled joiner 0.11 s・info guard 0.18 s・nozomi claim 0.08 s。既存 join 固定テスト（viewer_test.go TN/info/nozomi）通過。
- `python -m pytest tests/ -v`: 66 passed in 25.94 s（fresh canonical build 後・freshness 2件 PASS 含む）。正典 build fresh 成功（`output\hitomi-hub\hub.exe`）。
- 実動作確認（viewer 変更・正典 build・外部 API 許可 decision 2026-11-01）:
  - CLI download（実在 gallery 4294614・scratch root）: `attempted 1 = written 1`（一致）✓。
  - viewer `/api/image/2d18a9a8fb2d971ec67f4287a13322738c04b8111e4cf4c8d73b2745a5942b89?format=webp`: **200・x-cache: miss・210784 bytes**（remote live gg hit・no_full なし）。同一ハッシュ `&gallery=4294614` では **200・x-cache: local・210784 bytes**（local full 経路）。byte 数は前批 cbz 実測（210784）と一致 ✓。
- 画面確認: 該当なし（UI 項目でない・表示源・`x-cache` 判定・generation guard 非変更。本実装は server 機構のみ）。Drive 保存なし。

## 完了時 Memory Bank 操作
