# 2026-11-06: 外部指摘の承認判断（R-05・R-08・R-11・R-13・R-15・R-20）

**Status**: 設計判断確定（実装未開始）
**Date**: 2026-11-06

## 前提

- 実装・ファイル変更（コード・UI・schema）はしていない。このファイルは設計判断の記録。
- 実装開始の合意は別途必要。

## R-11: 検索語を分けて 1 語ずつ取得する做法

- 判定: 正典準拠。元サイトは 1 リクエストで複数語をまとめて投げるのではなく、語ごとの data を取って共通部分を作る（`get_galleryids_for_query` の集合演算を `app/searchhtml.go` が mirror）。
- 集合が元サイトと同じである条件:
  1. 順序の起点を変えない（最初の語の dataset、または plain 語で order dataset を使う分岐）
  2. `language:` 語の pre-scan（C-04）の扱いを変えない
  3. cache key（`area|tag|language`、order を含む）を変えない
- この条件を満たせば、並行化は結果（集合と順序）を変えず、タイミングだけ変わる。条件を崩すと順序・集合が変わる。
- ブラウザ側の取得の並行度（逐次か同時か）は未観測。正典 JS で確認したのは集合演算の単位（語ごと）まで。
- 補足（2026-11-06 質問）: 語ごとに分けて 1 語ずつ取得するのは、AND 検索そのもの。元サイトには「複数語を 1 リクエストで AND する」経路がない（nozomi dataset は 1 tag 分、index key は 1 語分）。AND は「語ごとの data を取って共通部分」で組み立てる（`app/searchhtml.go` の intersection・`or` は group 内 union・`-term` は exclusion）。R-11 の指摘は意味（集合）の話ではなく、取得の逐次・同時の話だけ。
- 決定: 逐次のまま。体感速度は R-10（同じ検索条件の最終結果を少し覚えて、ページ送りの再計算を減らす）で回収する。
- 速度の実感（2026-11-06・機構から）。実測は未実施。
  - cold（cache なし）で語数 N の AND 検索は、語ごとに取得するため概ね N 回のゲート通過 + N 回の network を重ねる。data lane の正典値は 100 ms（`docs/ai/decisions/2026-11-05-search-speedup-recommended-approved.md`）。語数が 3 なら 0.3 s の gate + network 3 回、5 なら 0.5 s + network 5 回。plain term は index 降下で depth ぶんの追加呼出しもあり得る。
  - warm（同一語の dataset が 1 時間 TTL の `searchCache` にある）は network 0 回だが、呼出每次に共通部分の map を作り直す（R-10）。
  - どの部分が支配かは実測で分ける。P3 #10（`docs/ai/backlog-items/p3-10-search-speed-measurement.md`）で「gate wait と network を分離して測る」が採用済み。この実測が終わるまで、並行化は採用しない。
  - 実測で cold latency が予算（例: 1.5 s）を超える場合の候補: data lane の interval は守ったまま、in-flight の直列化だけを外す案。これは A-05 の FIFO・grace 意味を変えるため、実測後に別の承認が要る。
- 間隔短縮の方向（2026-11-06 10:43・ユーザー承認）。値は未確定。
  - 今の正典値: data lane `min_interval_ms` 100 ms（`docs/ai/decisions/2026-11-05-search-speedup-recommended-approved.md`）。image lane 60 ms（`docs/ai/decisions/2026-11-06-tn-image-lane-60ms-approved.md`）。設定の下限 guard は 50 ms（`app/main.go:149-176`・`minHitomiRateFloorMS`）。
  - 1000 ms は別物。block クールダウン中の politeness 退避先（`app/hitomi.go:30-35`・`politenessDefaultIntervalMS`）で、語ごとの通常 interval ではない。ここを短縮すると「作法」の意味が変わる。
  - 候補: 50 ms（下限 guard 内・2026-11-05 実測で 429 なしを観測した範囲。単発実測のみで、持続呼出しと累計規模は未測）。25 ms（下限 guard 変更が必要。block 挙動未測）。現状 100 ms 維持。
  - 推奨: 50 ms へ。ただし P3 #10 の実測（gate wait と network を分離）で、interval 短縮が効く部分と network 支配の部分を分けてから正典値の変更を承認する。短縮効果は語数ぶんだけ（3 語で 300 ms → 150 ms）。network が支配なら効果は小さい。

## R-15: 先読み範囲と保存容量

- 決定: 案 B。全体先読みを維持し、永続保存（disk cache）に容量上限を入れる。利便性と操作性を最優先。
- 上限の効かせ方（この判断で決めた形）: gallery 単位の古いものから捨てる。現在閲覧中の gallery と、列（strip）で表示済みの分は捨てない。
- 補足（2026-11-06）: ブラウザ内（memory 側）の原寸画像参照の上限は 2000 では足りない。4000 くらいへ上げる。または簡単に設定変更できる形にする。
- 削除順序（memory 側）: 保存済みステータス（完成フォルダにファイルがある）のものを優先的に捨てる。LRU（最後に使った順）は 2 番目の基準。理由は、保存済みの分は disk から再表示でき、memory から消しても再取得が不要なため。
- disk 側の削除順序: disk cache の項目はすべて保存済みなので「保存済み優先」で区別できない。gallery 単位の古い順を維持する。この項目の「保存済み優先」は memory 側の話として理解（解釈が違えば確認が必要）。
- 変更可能にする形（設計点）: `app/viewer.html:208` の `MAX_FULL_CACHE_ENTRIES = 2000` は定数直書きで、テスト（`app/viewer_test.go:816`）が 2000 を固定している。設定 JSON（`viewer` 節）から page へ渡す形にすると設定変更が可能。メモリ使用量が増える（参照が保持するバイト）。
- 現方針で維持: 縮小画像を列の表示にのみ使う、縮小画像のデータを原寸画像の保存として扱わない、永続先（既定 `cache/images`）を作業フォルダへ移さない。
- 残る設計点: disk 側の容量上限値（実測後に決定）、上限到達時にユーザーへどう知らせるか（表示）、memory 側の設定名（4000 を既定へ上げるか設定化するかの確定）。
- 保存先の事実（読み取り・2026-11-06）。アプリの書き込みは完成フォルダ内に閉じる（`docs/ai/architecture.md`）。
  - `Galleries/<gallery_id>/`（config `gallery_dir`）— download 成果物。CLI `hub.exe -download` が書く。
  - `acquisition/<title>/`（config `acquisition_dir`）— 画面の「取得」ボタンが書く。R-05 の名前問題はここ。
  - `cache/images/<hash>/<format>`（config `cache_dir`、既定 `cache/images`）— 閲覧で取得した原寸画像の永続 cache。
  - `cache/images/thumbs/`、`cache/galleryinfo/`、`hitomi.db`。
- 画面からの操作方法: 「保存」ボタンはない。保存が起きる操作は (a)「取得」ボタン（acquisition へ）、(b) 閲覧中の画像取得（cache へ）。既存ファイルを使う表示（local-direct）は保存操作ではない。保存元は `showSourceStatus` の表示（local・cache・tn-cache・no_full）で分かる（`docs/ai/decisions/2026-11-01-tn-display-and-source-status.md`）。削除操作は画面にない（正典 `kilo-safe-remove`）。
- LRU が効く範囲: cache 層（`cache/images`、`cache/images/thumbs`）のみ。`Galleries`（保存済みファイル）と `hitomi.db` は削除対象にしない。
- 「保存済み優先で捨てる」の実現形: memory から捨てる対象は、disk に同じ内容がある項目を優先（完成フォルダのファイルまたは `cache/images` に同じ sha256 がある項目。DB の `files` で判定できる）。この解釈で合意している。disk 側の項目はすべて保存済みなので優先順位で区別できず、gallery 単位の古い順を維持。
- 決定の不足（2026-11-06・R-15）。実施前に埋める必要がある点:
  1. disk 上限の単位（件数か byte か）。memory は件数（4000 件）。disk は byte 推奨（画像の大きさがばらつく）。
  2. disk 側の「使った順」の判定手段。mtime は読書で更新されない（LRU の順にならない）。明示的な last access の記録（DB 列または内存 LRU を disk へ反映）が必要。
  3. disk 側の優先削除の形: 完成フォルダ（Galleries・acquisition）に同じ内容がある cache 項目を優先（冗長コピー）。判定は `files` の sha256 lookup（`schema.sql:83` の `idx_files_sha256`）。memory 側は client が持つ `local_formats`（card data）で判定できる（`app/viewer.html:295-300`）。
  4. pinned の定義: 「現在閲覧中の gallery」だけか、列表示済みの全 gallery か。列は現在 gallery だけ。他の gallery の表示済み分を捨ててよいか。
  5. TN 層の扱い: `cache/images/thumbs` に上限がない（R-15 の disk 上限に含まれるか、TN は別枠か）。TN を原寸画像の保存として数えない現方針は維持（`app/viewer.go:331`）。
  6. 削除後の再表示の操作感: cache から消えた gallery を開くと remote 再取得（gate 消費・R-07）。この待ちを予算化する。
  7. 知らせ方: どの面（`prefetch-status` か `#perf-status` か）に出すか未定。
  8. client cap の設定化: 設定名（例 `viewer.max_object_urls`）、0 は既定、下限 guard の要否。テスト `app/viewer_test.go:816` が 2000 を固定しているため更新が必要。client 側の `prefetchedEntries`・`countsByGallery` は cap の対象外（C-05 の残存点）で、4000 へ上げると map 全体が増える。
  9. 確認テスト: canonical build・served 画面確認、cold hygiene（操作前後の before/after）が必要（`docs/ai/rules/confirmation-test-rule.md`）。合格条件: full image の順序（local → 永続 cache → remote）、TN 表示禁止、`x-cache` 判定、generation guard を弱めない。

## 確定（2026-11-06 10:52・R-11 と R-15）

大前提: ユーザーに関知させる必要はない。不便が起きることは許容しない。通知 UI は置かない。これ以上判断を求めない。

### R-11（interval）

- data lane `min_interval_ms` を **90 ms** に決定（`docs/ai/decisions/2026-11-05-search-speedup-recommended-approved.md` の 100 ms の点のみ supersede）。
- 下限 guard（50 ms）・image lane（60 ms）・politeness 退避先（1000 ms）は変更しない。
- 効果: 語数ぶん（3 語で 300 ms → 270 ms）。network が支配なら効果は小さい。
- 実装時に更新が必要: `defaultHitomiConfig()`（`app/hitomi.go:237-240`）と、100 ms を期待する test（`app/hitomi_test.go:1507-1528`）。実装開始指示待ち。

### R-15（上限・削除順）

- 上限は**件数**。容量（byte）計算は使わない。
  - memory（ブラウザ内の画像参照）: **4000 件**。設定化（例 `viewer.max_object_urls`、0 は既定、下限 1000）。テスト `app/viewer_test.go:816` の 2000 固定を更新。
  - disk（`cache/images`・`cache/images/thumbs`）: **8000 件**ずつ。
- 削除順:
  - disk: **作成順**（古いファイルから）。last access 記録は作らない。sha256 で完成フォルダとの同一内容 lookup はしない。現在閲覧中の gallery の項目は捨ない。
  - memory: 既存の entry status（保存済み＝local）を優先して捨てる。次に最後に使った順。現在表示中の項目は捨ない。status は書き込み時に記録済みの値を見る（追加 lookup なし）。
- 件数・作成順・status を filesystem scan ではなく DB の 1 表（例 `cache_entries`: hash、format、kind、created_at、source_status）で持つ。追加コストは書き込み・削除時の 1 行だけ。filesystem scan で数えない。
- 通知 UI: 置かない（関知させない）。
- 不便の許容: 保存済みの項目を memory から捨てた場合は disk から再表示できる。disk で作成順の項目を捨てることは、一度見て二度と使わない分を消すこととして許容（再取得を前提にしない）。現在閲覧中と表示中は捨てない。

## R-20: 読み取り要求の要求元検査

- 元の推奨: 案 A（読み取り要求にも、自分自身の端末からの要求だけを認める検査を追加）。
- 一時撤回（2026-11-06 10:32 UTC）→ 取り消し。決定（2026-11-06 10:38 UTC）: 案 A を承認。読み取り要求にも、自分自身の端末からの要求だけを認める検査を追加する（既存の書き込み検査と同じ流儀）。実施開始指示待ち。
- LAN 構成へ変更する場合は、この項目は「除外」でなくなる。その時点で下記の判断が必要になる。
- 将来、同一私有ネットワーク内のアクセスを認める構成にする場合:
  - 「自分自身の端末」判定を、設定で決めた許可ホストまたは私有ネットワーク範囲へ置き換える。同じ判定を書き込み要求と読み取り要求の両方に適用する。
  - 単に localhost 判定を外して要求元のホスト名を信任する形は、DNS 切り替えによるなりすましを許可するため不可。
  - 許可をホスト名で受けると、ホストヘッダ偽装の影響が残る。許可判定は IP 範囲（私有アドレス）または設定したホスト名に限定する。
  - LAN で共有すると、既読・お気に入り・保存検索の書き込み口も同じ信頼境界に入る。認証を入れるか、信頼できる局域网として扱うかの判断が必要。
  - 現状の「ローカル専用前提でこの項目を扱わない」方針は、LAN 構成にした時点でその方針から移動する。

## R-05・R-08・R-13

- R-05: 案 A 承認。保存先フォルダ名を「作品名 + 作品番号」へ。予約名・末尾の点・末尾の空白・長さ上限を処理。既存フォルダは再命名せず、新規保存分から適用。
- R-08: 案 A 承認。ファイルチェックを 1 gallery 1 回へ減らし、表示に必要な情報（保存済み枚数、代表画像の材料）は残す。
- R-13: 案 A 承認。縮小画像・作品情報・nozomi 分割データに「同じ内容なら 1 回」を追加（既存の流儀に揃える）。
