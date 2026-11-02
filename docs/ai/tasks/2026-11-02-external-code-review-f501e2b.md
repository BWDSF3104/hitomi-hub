# 外部コードレビュー（commit f501e2b 時点）

**Status**: 実施完了（正典検証合格）
**Date**: 2026-11-02
**Next Action**: commit checkpoint を実行し、完了時 Memory Bank 操作へ進む。

## 意図 / 背景

- commit `f501e2b` 時点として渡された外部コードレビューを、正典記録として残す。
- 本記録は読み直した結果の記録であり、実行・修正・実装の開始ではない。
- 前回までの項目は多くが解消していて、新しい問題も見つかった。
- ローカル実行・ローカルのみ閲覧という前提なので、セキュリティ項目は優先度を下げています。
- POST の Host/Origin/Content-Type 検査は入っています。これ以上手を入れる必要は薄いです。
- 推定は推定として記録する。

## 調査結果 / 変更内容

## 1. 前回指摘の状況

**解消済み**
- **A-1（gg.js / index version）**: 最小間隔、singleflight、再取得後に bucket が同じなら再試行しない処理が入りました。index も 404 時に version を 1 回だけ再取得します。ただし下記 2-2、2-3 の新しい問題があります。
- **A-3（`asc` の二重 reverse）**: `orderApplied` フラグで 1 回だけ適用されます。
- **A-6（VirusTotal）**: `last_analysis_stats` を読むようになり、`result` / `engine_name` が検出名に出ます。非 200 は cache せず、TTL と日次カウントの DB 化も入りました。VT の失敗で構造検査の status は上書きされません。
- **A-7（YARA の失敗）**: 失敗は規則名の finding ではなく `scan_error` になり、`yr` の相対パスも解決します。ただし 2-8 の残りがあります。
- **findings の重複**: UNIQUE index、`INSERT OR IGNORE`、移行処理が入りました。
- **`processFile` のトランザクション化**
- **openGallery の開き直し**: `backgroundGalleryId = null` が入り、manual retry の `catch` も追加されました。
- **設定・保守**: `--config`、per-field default、`use_live_gg` の sentinel、POST middleware、DSN pragma が入りました。

**未解消**
- **A-8 PDF**: Page 辞書の閉じ `>>` が不足したままです（`<<` 3 に対し `>>` 2）。テストは部分文字列検査なので通ります。
- 以下はすべて変更なしです。
  - `recordDiscovered` が 25 件それぞれ別 commit のままです。
  - `json_ms` は常に 0 です。
  - 検索言語が plain term に効きません。
  - 検索カードが 1 枚ずつ逐次取得です。
  - `time.Sleep` が ctx 非対応です。
  - キャッシュ書き込みが非アトミックで、magic 検査もありません。
  - singleflight（同一 hash の同時取得を 1 回にまとめる仕組み）がありません。
  - `thumbProxy` が TN 失敗時に full 画像へフォールバックします。
  - `serveStoredImage` が全読みです。
  - `download` が既存ファイルを確認しません。
  - objectURL を 2000 件で全 revoke します。
  - `/api/read` と `/api/lists` の debounce がありません。
  - `renderStrip` が毎回 50 枚を再構築します。
  - 起動時にカード取得が重複します。
  - SHA256 は毎回全件計算します。
  - `ScanPDFKeys` が 7 キーごとに `string(data)` を作り直します。
  - `rawRGB` が全ページ分 RGB を保持します。
  - `infoCache` に上限がありません。

## 2. 動作上の問題（影響が大きい順）

1. **`a or b` だけのクエリで panic する**
   - `or` 項は `positive` に入らないので、`or` だけのクエリでは `positive` が空になります。
   - その状態で `for _, term := range positive[1:]` に到達し、`slice bounds out of range` で落ちます。
   - viewer ではリクエストごとに recover されて接続が切れ、CLI の `search` ではプロセスが落ちます。
   - `if len(positive) > 1` で囲んでください。
   - or-only 経路は、最初の group を、直後のループでもう一度 intersect します。これも重複です。

2. **`EnsureGG` が失敗時に live の gg を snapshot で上書きする**
   - gg.js の取得失敗や parse 失敗で、`c.gg = snapshotGG()` が実行されます。
   - 戻り値の `source` は取得前の `"live"` のままです。
   - `imageProxy` は「live で、かつ gg が変わった」と判断し、古い bucket の URL で再試行して 404 になります。
   - 以後は次の成功まで full 画像が一斉に `no_full` になります。
   - 失敗時は直前の live 値を保持してください。

3. **`imageProxy` の gg 比較が 404 の後で行われる（race）**
   - `ggBefore` は 404 を受け取ってから取得しています。比較の基準は、URL を組んだときの `gg` であるべきです。
   - 別リクエストが先に更新を終えていると、この 404 は旧 bucket の URL によるものでも、`ggBefore` がすでに新値になります。
   - 結果は「変わっていない」と判定され、再試行せず `no_full` を返します。
   - 4 並列の prefetch で再取得中（約 1〜2 秒）に出た要求が該当しそうです（推定）。
   - UI は `no_full` を `prefetchedEntries` に保持します。手動 retry の対象は `error` だけなので、strip のサムネイルが空のままになります。
   - 比較は URL 生成時の `gg` と行い、`no_full` も手動 retry の対象に含めるのが安全です。

4. **plain term の検索結果が cache されない（体感への影響が最大）**
   - 検索結果の TTL cache があるのは `SearchNozomiParts`（colon term）だけです。
   - 既定クエリ `furry` のような plain term は `IndexIDs` を毎回降下し、page を送るたびに同じ取得をやり直します。
   - root node の cache は 1 回ぶんにしか効きません。内部 node（`field|version|addr`）と `term→ids` を cache してください。

5. **計測の分類が誤っている（`classifyFetchURL`）**
   - `strings.Contains(rawurl, "tn.")` は `ltn.` ホストにも一致します。
   - `.index` / `nozomi` / `gg` 以外の ltn URL、つまり index の `.data` と `galleries/<id>.js` が `image` に分類されます。`gallery` や `data` の枠にはほぼ入りません。
   - 前回の計測で `image: calls1` が検索 1 回ごとに出ていたのはこのためだと思われます（推定）。
   - 「index 7 回が支配的」という結論は方向としては妥当です。ただし内訳は、fix 後に取り直してください。

6. **acquisition の確認が、新規フォルダでも 2 ページ目に出る**
   - 1 ページ目で作ったフォルダを、サーバーは 2 ページ目で「既存」と判定して `confirm` を返します。
   - UI の `acquisitionRunFolderConfirmed` は、最初の `confirm` を受けた時点で true になります。これは 2 ページ目です。
   - 結果として、確認は毎ページから「2 ページ目の 1 回」に減っただけです。新規取得でも出ます。
   - 「いいえ」を選ぶと、以降の全ページが黙って skip されます。「はい」を選ぶと、以降は上書き保護なしで進みます。
   - run 開始時にフォルダの存在確認を 1 回だけ行うか、サーバーが run 単位で判断してください。
   - 検証は既存フォルダを事前に作ったケースだけで、新規ケースが通っていません。
   - 補足です。取得元が `cache/images` 限定なので、download 済みで cache に無い gallery は全ページ `missing cache` になります。

7. **キーボードショートカットが修飾キーを見ない**
   - Ctrl+F（ブラウザ検索）でフルスクリーンが切り替わります。
   - Ctrl+R（再読込）で左右反転が切り替わり、Ctrl+D と Ctrl+S でもモードが切り替わります。
   - Alt+← はブラウザ「戻る」と同時にページ移動が走ります。
   - `ctrlKey` / `metaKey` / `altKey` のときは return してください。

8. **設定の部分指定が静かにスキャンを壊す（`--config` で到達可能になった）**
   - `Limits` は `MaxEntries == 0` のときだけ全体をデフォルトに置換します。それ以外では未指定のフィールドが 0 のまま残ります。
   - `max_scan_bytes` が 0 なら PDF のキーを一切検出しません（誤 pass）。
   - `max_ratio` や `max_uncompressed_bytes` が 0 なら、全 entry が `*_exceeded` になります。
   - Python 版は `setdefault` で per-key に補完しているので、Go 側も揃えてください。
   - `hitomi` キー自体が無い JSON では、`UnmarshalJSON` が呼ばれないため `UseLiveGG` が false になります。この場合、「live gg.js unavailable」という誤った警告も出ます。`cfg := Config{Hitomi: defaultHitomiConfig()}` で初期化すると解決します。

9. **トランザクションが外部呼び出しをまたぐ（推定）**
   - 最初の `addFinding` で書き込みロックを取ったまま、yr の実行（最大 25 秒）と VT 照会（最大 30 秒）が走ります。
   - 常駐 viewer が別プロセスで書くと、`busy_timeout` の 5 秒を超えて `SQLITE_BUSY` になりえます。
   - 外部呼び出しを tx の外に出し、結果だけを tx で書く形が安全です。
   - `incrementVTUsage` が tx 内なので、後続の失敗で rollback されると、消費済みの API 回数が戻ってしまいます。

10. **YARA の終了コード判定（推定）**
    - 終了コード 1、規則なし、`err != nil` の場合は `pass` 扱いです。
    - YARA-X では 1 が error という記録（`tools/VERSIONS.md` の記述）と矛盾します。
    - 一方、`yr` が一時的に失敗すると `scan_error` になり、`export` が skip します。VT のときと同じ構図なので、構造判定を維持して別 finding にするほうが一貫します。

11. **VT の 404 の扱い**
    - 未登録ハッシュの 404 は日常的に起きます。それが `api_error:not_evaluated` という finding になります。
    - 未登録のものは cache されないので、再スキャンのたびに枠を消費します。
    - `api_error:*` の finding は、後で照会に成功しても消えません。
    - 404 は `unknown_to_vt` のような別語彙にし、短い TTL で negative cache してください。

12. **`view` の起動が gg.js 取得で止まる（推定）**
    - `warnGG(loadGG(...))` が `serve()` の前に同期で走ります。
    - オフラインだと、再試行の backoff（2+4+6 秒）とゲートで 15 秒前後待たされます。ローカル閲覧だけなのに起動が遅くなります。
    - `serve` を先に起動し、gg は裏で読むのが安全です。

13. **`check` の初回処理**
    - 初回は全 id が「新着」になります。1 件ごとに SELECT 1 回と書き込み 2 回で、トランザクションがありません。
    - 初回は baseline として記録するだけにするか、tx にまとめてください。

14. **Python 参照実装が現行 schema と合っていない**
    - `schema.sql` の UNIQUE index に対し、`add_finding` は素の `INSERT` です。同じファイルを再スキャンすると `IntegrityError` で落ちます。重複行がある既存 DB では `ensure_schema` で落ちます。
    - VT は `stats` キーと `name` を読んだままです。YARA 失敗は `yara_error:` の規則名で hit になります。
    - README が謳う「status 語彙と finding の一致」から外れています。pytest は別行しか入れないので気づけません。

## 3. 非効率

- **検索 1 回のコスト**: 上の 2-4 のとおり、plain term の page 送りで毎回 index 降下（約 7 回）と data 取得をやり直します。`recordDiscovered` も別 commit 25 回のままです（約 0.8 秒）。`searchCard` でも毎回書いています。
  - `json_ms` は直後に測っているので常に 0 です。実際の encode を挟むか、項目を削除してください。
  - `timing.metrics` と `metrics` は同じ内容が二重に返っています。
- **UI の resize**: `resize` で debounce なしに `renderReader` と `startFullPrefetch` が走ります。`renderGeneration` が進むので、途中の `loadFullImage` の結果が捨てられます。ウィンドウをドラッグすると、取得と破棄を繰り返します。
- **localStorage の肥大**: `setSearchTitleGroup` が非日本語 entry のたびに、`titleGroups` 全体を stringify して保存します。肥大して上限を超えると、保存は例外が握りつぶされて止まります。

## 4. 設定・保守・文書

- **`.gitignore` に `config.json` が無い**: `--config config.json` で API key を書くと、コミットで漏れます。
- **`migrateSchema` は findings しか見ない**: `files.gallery_id` のような列追加は反映されません。古い DB から移行するなら要確認です（推定）。
- **`isLoopbackHost`**: `[::1]:8731` を弾きます（`Cut(":")` の都合です）。`http://[::1]:8731` で開く場合だけ影響します。
- **`viewer.bind` を LAN 向けにした場合**: POST がすべて 403 になります。前提どおりなら仕様です。
- **RTL の矢印キー（要確認）**: 既定が右から左なのに、←/→ は LTR の意味（← が前のページ）のままです。
- **README は古い記述**: 「UI は未実装」と書いてあります。`usage-memo` の `check` も「`galleries` 行の有無で判定」のままです。
- **未使用コード**: `Search`、`SearchQuery`、`nozomiURL`、`fileExt`、`imageExts`、`pageSource` などが残っています。

## 5. 優先して直すなら

1. 2-1（`or` のみで panic）。1 行の修正です。
2. 2-2 と 2-3（gg の失敗上書きと race）。`no_full` が連鎖する原因になります。
3. 2-4（`IndexIDs` の結果と内部 node の cache）。体感が最も変わります。
4. 2-5（`classifyFetchURL` の修正後、計測を取り直す）。
5. 2-6（acquisition の確認を run 開始時 1 回に）。
6. 2-7、2-8（修飾キーの除外と Limits の補完）。
7. 2-9、2-10、2-11（yr と VT を tx の外へ、状態の語彙を整理）。
8. PDF の `>>`（単純ですが、厳密な reader で開けない可能性があります）。

2-1、2-2、2-3、2-8 は、再現するテストを先に書くと確認しやすいです。

## Verification

- 本記録の検証:
  - `task_manager.py` で `docs/ai/tasks/2026-11-02-external-code-review-f501e2b.md` と `docs/ai/current-task.tsv` に記録を追加した。
  - 実装ファイルやコードへの変更は行っていない。
  - 現在時刻時点の外部レビュー内容として渡された記録をそのまま保存した。
- commit `f501e2b` の現行ソースを読み直して、項目別の有効性を `## 現在のコードに対する項目別有効性（commit f501e2b 読み直し）` に追記した。実装ファイルやコードへの変更は行っていない。

## 現在のコードに対する項目別有効性（commit f501e2b 読み直し）

判定は「確認」「部分（推定）」「設計確認」に分ける。推定は推定のまま残す。実装変更はしていない。

### 前回指摘の項目

- **A-8 PDF の `>>`**: 有効（確認）。`app/pdf.go:142` の page 辞書は `/Contents %d 0 R` で閉じられ、page 辞書側の `>>` が無い。`<<` は page、`Resources`、`XObject` の 3 つだが、閉じは `XObject` と `Resources` の 2 つだけ。テストは部分文字列検査なので現行テストは通る。対応は単純で、page 辞書末尾に ` >>` を入れる方向。
- **`recordDiscovered` が 25 件別 commit**: 有効（確認）。`app/main.go:931-936` は `db.Exec` を呼ぶだけで tx を開かない。`app/viewer.go:326-329` の search で表示分 25 件を別々に書き、`app/viewer.go:345-362` の `searchCard` も毎回書く。対応は tx まとめが可能だが、読み取り・表示・record の意味を変えるためテストが必要。

### 2. 動作上の問題

1. **`a or b` だけのクエリ**: 有効（確認）。`app/searchhtml.go:228-253` は `or` を `positive` に入れない。`positive` が空で `orGroups` だけある場合、`app/searchhtml.go:272-290` で最初の group を union した後、`app/searchhtml.go:312` の `for _, term := range positive[1:]` に進む。Go の簡略スライスでは空 slice への `[1:]` は bounds error になる。`app/searchhtml.go:292-310` では `orGroups[0]` をもう一度 intersect するため、レビューの「重複」も確認できる。対応は `if len(positive) > 1` で panic を止められる。group 重複も整えるなら「initial として使った group を後の intersect から除く」形が妥当。

2. **`EnsureGG` 失敗時に live を snapshot で上書き**: 有効（確認）。`app/gg.go:87-164` は `source := c.gg.Source` を先に捕捉する。ネットワーク失敗または parse 失敗で `c.gg = snapshotGG()` するが、`source` が捕捉時点で `"live"` だった場合は `source` を `"live"` のまま返す。`app/viewer.go:549-551` は返ってきた `source` が live だと判断し、snapshot に落ちた gg を「変わった」と見て再試行する。対応は失敗時に直前の live 値を保持し、source と actual gg のズレを残さないこと。

3. **`imageProxy` の gg 比較 race**: 有効（確認）。URL 生成用の gg は `app/viewer.go:537` で取っているが、404 後の比較基準 `ggBefore` は `app/viewer.go:549` で 404 後に取る。別リクエストが先に gg を更新すると、この 404 の基準と `ggBefore` が同じ新値になり、再試行しない可能性がある。UI は `prefetchedEntries` に `no_full` を保持する（`app/ui.html:749-759`）が、manual retry は `sourceStatusByEntry.get(key) !== "error"` の entry だけを処理する（`app/ui.html:1010-1022`）。対応は URL 生成時 gg と比較し、`no_full` を retry 対象に含めること。

4. **plain term 検索 cache**: 有効（確認）。colon term は `app/searchhtml.go:76-103` の `SearchNozomiParts` で `searchCache` を使う。plain term は `app/searchhtml.go:123` から `IndexIDs` へ行き、`app/searchindex.go:236-269` の top node cache だけヒットする。内部 node は `app/searchindex.go:271-278`、data は `app/searchindex.go:280-286` で毎回取得する。対応は内部 node と `field|version|addr` cache、`field|version|term` cache。ただし memory 上限と version 変更時の stale 扱いが必要。

5. **`classifyFetchURL` の分類**: 有効（確認）。`app/metrics.go:16-37` は `.index` を `index`、nozomi を `nozomi`、gg を `gg` として先に拾う。それ以降の `tn.` は `image` になる。`ltn.` を含む index `.data` や `galleries/<id>.js` は image に分類される。前回の「image: calls1」は推定だが、方向は妥当。対応は `.data`、`galleries/`、tn path を明示分類し、fix 後に計測を取り直すこと。

6. **acquisition 確認が新規でも 2 ページ目に出る**: 有効（確認）。server は各リクエストで `titlePath` を見る（`app/acquisition.go:132-143`）。UI の `acquisitionRunFolderConfirmed` は最初の `confirm` で true になる（`app/ui.html:1592-1600`）。新規 run の page 1 で書き込み完了後、page 2 で server が folder existing を `confirm` として返す。user が No を選んだ場合、UI は以降の confirm を skip する（`app/ui.html:1594-1598`）。Yes を選んだ場合、`acquisitionOverwrite` が true になり以降は上書き保護なしで進む（`app/ui.html:1601-1611`）。対応は run 開始時に folder 確認 1 回、または server が run 単位で判断すること。取得元が `cache/images` 限定なので、download 済みで cache に無い gallery は全ページ `missing cache` になる点も現行どおり。

7. **キーボードショートカットの修飾キー**: 有効（確認）。`app/ui.html:1515-1531` は `ctrlKey` / `metaKey` / `altKey` を見ていない。`f` で fullscreen、`d`/`s`/`r` で view mode、矢印で page move が走る。対応は handler 先頭で modifier がある場合 return すること。

8. **`--config` の Limits 部分指定**: 有効（確認）。`app/main.go:103-153` は `cfg.Limits.MaxEntries == 0` のときだけ `defaultLimits()` で全体を置換する。partial limits で明示 field がある場合、未指定 field は 0 のまま残る。Python 参照実装は `src/scan.py:308-313` で per-key `setdefault` をする。`UseLiveGG` は `HitomiConfig.UnmarshalJSON`（`app/hitomi.go:82-108`）で sentinel を持つが、トップレベル `hitomi` key が無いと `UnmarshalJSON` 自体が呼ばれず、`loadConfig` の `cfg := Config{}` 初期化で false になる。対応は `cfg := Config{Hitomi: defaultHitomiConfig()}` と per-field default。明示 0 を「未指定」と「意図的な 0」で区別するには raw JSON 判定または pointer が必要。

9. **トランザクションが外部呼び出しをまたぐ**: 有効（確認）。`app/main.go:438-443` で tx を開始し、`addFinding` を書き込んだ後に、YARA を `app/main.go:469-487` で実行し、VT を `app/main.go:489-532` で照会する。`VTLookup` 内の `incrementVTUsage`（`app/vt.go:190-192`）も tx 内。後続失敗で rollback すると消費済み API 回数が戻る可能性がある。対応は external lookup を tx 外へ出し、結果だけを tx で書くこと。usage commit 意味は「消費済みとして残す」方向なら tx 外または commit 後に書く必要がある。

10. **YARA 終了コード判定**: 現行挙動は確認。`app/vt.go:247-257` は `err != nil && exit > 1` のとき scan_error、rules があれば hit、`err != nil && exit == 0` のとき scan_error、それ以外は pass。exit 1、rules なし、`err != nil` は pass になる。YARA-X の exit 1 仕様はこの task file の現行 `tools/VERSIONS.md` 本文には明記されておらず、レビューの「1 が error」は外部記録・推定として残す必要がある。対応は公式 docs または actual `yr` 実測で exit convention を確定してから allow list 化すること。

11. **VT 404**: 有効（確認）。`app/vt.go:107-128` は 200 以外、特に 404 を `not_evaluated` にし、`processFileWithYara` の `app/main.go:527` で `api_error:not_evaluated` finding として書く。negative cache がないため再スキャンで枠を消費する。成功後に既存 `api_error:*` finding を消す処理はない。対応は `unknown_to_vt` のような別語彙、short TTL negative cache、既存 finding の更新方針が必要。

12. **`view` 起動が gg.js 取得で止まる**: 有効（確認）。`app/main.go:829-838` は `newViewer` 後 `warnGG(loadGG(v.client))` を `serve()` 前に同期で呼ぶ。遅延の秒数は `MaxRetries` と `RetryBackoffMS`（`app/hitomi.go:78-80`）から出るが、環境の network timeout や gate により増える。レビューの「15 秒前後」は推定。対応は `serve` を先に起動し、gg を background で読むこと。

13. **`check` 初回処理**: 有効（確認）。`app/check.go:48-66` は初回に `bookmark_seen` が空なので全 id を新着扱いし、id ごとに `SELECT 1` 回、`markQuerySeen` 1 回、`recordDiscovered` 1 回を行う。tx はない。対応は初回 baseline だけにするか、query ごとに tx まとめ。

14. **Python 参照実装が現行 schema と合っていない**: 有効（確認）。`schema.sql:81` に `findings` unique index があるが、`src/scan.py:233-238` の `add_finding` は素の `INSERT` で `IntegrityError` になりえる。重複行がある既存 DB では `src/scan.py:211` の `ensure_schema` で unique index 作成に失敗しえる。Python VT は `stats` と `name` を読む（`src/scan.py:196-202`）が、Go 側は `last_analysis_stats`、`engine_name`、`result` を使う（`app/vt.go:126-159`）。Python YARA failure は `yara_error:` を rule として返し hit 扱い（`src/scan.py:287-293`）で、Go の scan_error と語彙がずれる。`tests/test_scan.py` は重複 findings を通さず、別行しか入れないため差異を捕えない。

### 3. 非効率項目

- **search page の recordDiscovered**: 確認。`app/main.go:931-936` は per Exec commit。`app/viewer.go:326-329` の search 25 件、`app/viewer.go:345-362` の searchCard 1 件ずつが別 commit。対応は tx まとめが可能だが意味確認が必要。
- **UI search cards sequential**: 確認。`app/ui.html:1355-1381` は `await api("/api/search-card/" + ids[i])` を 1 件ずつ。対応は並列化可能だが UI 表示・perf 意味が変わる。
- **`json_ms`**: 確認。`app/viewer.go:330-342` は `jsonOut` の直前に `jsonMS` を測り、0 になる。`timing.metrics` と `metrics` は同じ `metricsSnapshot()` を二重に出す。対応は actual encode を buffer で測るか項目削除、metrics は 1 箇所だけ返す。
- **UI resize**: 確認。`app/ui.html:1508-1513` に debounce が無く `renderReader()` と `startFullPrefetch()` を呼ぶ。`renderGeneration` が進むため stale async result は破棄される（`app/ui.html:889-892`）。対応は debounce。
- **localStorage titleGroups**: 確認。`app/ui.html:1230-1243` は非日本語 entry のたびに `saveTitleGroups()` を呼ぶ。上限超過は `catch` で握りつぶされる。対応は batch save または容量制御。
- **startup duplicate card fetch**: 確認。`app/ui.html:1296-1313` の `restoreSearchState()` が `loadSearchCards(ids, searchGeneration)` を呼ぶ。`app/ui.html:1645-1651` の initial search branch はその後に `runSearch()` を呼ぶ。`runSearch()` も `loadSearchCards` を呼ぶ（`app/ui.html:1355-1381`）ので重複。対応は initial search branch で restore 後の card load を重複させないこと。
- **SHA256**: 確認。`app/scan.go:100-111` は full file read。`app/main.go:426-428` で scan ごとに呼ぶ。対応は cache hit 前提で skip 可能だが DB record の sha 意味が変わる。
- **ScanPDFKeys**: 確認。`app/scan.go:163-167` で `strings.Contains(string(data), key)` を key ごとに実行し、`string(data)` を毎回作る。対応は bytes.Contains または 1 回 string 化。
- **rawRGB**: 確認。`app/pdf.go:47-58` は全 page の RGB を `pdfPage.rgb` に保持し、`app/pdf.go:111-113` で pages に append する。対応は page ごとに encode/write して offset を追跡する streaming 化が可能。
- **infoCache**: 確認。`app/viewer.go:30-44` の map は上限なし、`app/viewer.go:682-697` で cache する。対応は LRU。
- **cache write atomic / magic**: 確認。`app/viewer.go:510-513` と `app/viewer.go:542-544` は `os.WriteFile` を直接呼ぶ。`app/acquisition.go:171` も write 後に `ScanFile(dest)`（`app/acquisition.go:176`）。atomic rename、magic 検証なし。対応は temp write + rename、write 前または write 後の strict validation。
- **singleflight**: 確認。singleflight は `app/hitomi.go:69` に map があり、`app/gg.go:112-127` の gg URL だけ使う。same hash image acquisition の singleflight はない。対応は hash key singleflight。
- **thumbProxy TN fallback**: 確認。`app/viewer.go:506-518` で TN fetch 失敗後、local file、さらに full image proxy fallback へ進む。UI display source policy と server behavior は設計確認が必要。
- **serveStoredImage full read**: 確認。`app/viewer.go:241-248` は `os.ReadFile` で全 read。対応は streaming serve。
- **download existing check**: 確認。`app/main.go:397-404` に既存ファイル確認がなく、fetch/write へ進む。対応は local existing skip、ただし scan/DB record の再実行意味を決定する。
- **objectURL revoke all**: 確認。`app/ui.html:219-220` で `MAX_FULL_CACHE_ENTRIES = 2000`。`app/ui.html:768-785` で exceed 時に `revokeFullObjectUrls()` を呼ぶ。`app/ui.html:823-832` は全 objectURL を revoke する。対応は LRU 個別 revoke。
- **`/api/read` / `/api/lists` debounce**: 確認。`app/ui.html:1081-1099` の `showPage()` で post read、`app/ui.html:1182-1192` の `showLists()` は呼ばれる。debounce はない。read state は page arrival semantics なので、lists の debounce は可能だが read の debounce は意味確認が必要。
- **renderStrip rebuild**: 確認。`app/ui.html:1101-1129` は group ごとに毎回 HTML を組み立てる。対応は incremental render。

### 4. 設定・保守・文書項目

- **`.gitignore` に `config.json`**: 有効（確認）。`.gitignore` に `config.json` は無い。`--config config.json` は API key を含む運用でコミット漏れになりえる。対応は ignore rule、または config file を local runtime file として運用する規約変更。
- **`migrateSchema` は findings だけ**: 有効（確認）。`app/main.go:230-256` は findings の重複掃除と unique index だけ。`files.gallery_id` などの列追加は反映しない。古い DB に `files` の `gallery_id` 列が無い場合、`recordScanFile` は INSERT で列を前提にするため失敗しえる。対応は user_version 別 migration。
- **`isLoopbackHost` の `[::1]:8731`**: 有効（確認）。`app/viewer.go:106-126` は `strings.Cut(host, ":")` を使い、`[::1]:8731` では first colon で `[` を host part と误読する。exact `[::1]` は許可するが `[::1]:8731` は拒否する。対応は IPv6 host parsing。
- **`viewer.bind` LAN**: 有効（設計確認）。`sameOriginPost`（`app/viewer.go:84-104`）は loopback でない Host を拒否する。local-only 前提なら仕様。対応は不要。
- **RTL arrow keys**: 有効（確認）。`app/ui.html:237` の既定 `spreadDirection` は `rtl`。`movePage(delta)`（`app/ui.html:504-515`）は `spreadDirection` を見ず、`ArrowLeft` は `movePage(-1)`、`ArrowRight` は `movePage(1)`（`app/ui.html:1526-1527`）。視覚 RTL では左が next pair 側になりうるため、レビューの指摘どおり設計確認が必要。
- **README**: 有効（確認）。`README.md:13` は「UI は未実装」と言うが、viewer/UI が存在する。`docs/ai/usage-memo.md:59` は check の新着判定を `galleries` 行と説明するが、現行 `check.go` は `bookmark_seen` を使う。
- **未使用コード**: 有効（確認）。`Search`、`SearchQuery`、`nozomiURL`、`fileExt`、`imageExts` は現行 production path で未使用または test-only。`pageSource` は route として存在するが UI の主要読み取り flow では使っていない。対応は削除するか、テスト専用として意図を明記する。

## 修正方法と合格条件（優先度高い順）

判定は「項目別有効性確認済み」の項目だけ。推定項目は実測で確認できる範囲で進める。実装と確認の合格は正典 `go vet .` と `go test -count=1 -timeout 300s .`（`app`）で確認する。Python 参照実装項目は `python -m pytest tests -v`（ルート）で確認する。

1. `a or b` だけのクエリで panic  
   方法: `app/searchhtml.go` で `positive[1:]` を呼ぶ前に `len(positive) > 1` を検査し、or-only で初期 group を後の intersect から除く。テストは fixture の data range を term ごとに分けて union 結果と fetch 回数を検証する。  
   合格: `TestSearchHTMLPreservesOrOnlyUnionWithoutDuplicateGroup` で panic せず union ids が一致し、初期 group の重複 fetch が 0。

2. `EnsureGG` 失敗時に live gg を snapshot で上書き  
   方法: `app/gg.go` で failure path が直前の live gg を保持し、source と actual gg のズレを残さない。snapshot fallback は source が snapshot でなかった場合だけ。  
   合格: `TestEnsureGGFailurePreservesLiveGG` で live 成功後に fetch 失敗しても source が `live`、actual gg が直前の live bucket のまま。

3. `imageProxy` の gg 比較 race  
   方法: `app/viewer.go` で URL 生成時の gg を保持し、404 後の比較もその gg を基準にする。UI の manual retry は `error` だけでなく `no_full` も対象にする。  
   合格: `TestViewerImageProxyRetriesUsingInitialGGWhenAnotherRequestUpdatedGG` で race 時に retry 1 回が入り、UI string tests で no_full retry と button enable が更新される。

4. plain term 検索 cache  
   方法: `app/searchindex.go` の内部 node と data cache を追加。top node cache だけでなく `field|version|addr` と `field|version|offset|length` を TTL cache する。cache hit の返却は caller 変更から安全な copy、または decodeIDs 結果 copy。  
   合格: `TestIndexIDsInternalNodeAndDataCacheReuse` で同一 field/version の 2 回目の term 検索が top cache hit だけで network refetch を増やさない。

5. `classifyFetchURL` 誤分類  
   方法: `.data`、`galleries/`、明示 TN path を優先分類し、`ltn.` の host substring を image にしない。  
   合格: `TestClassifyFetchURLSeparatesIndexDataGalleryAndTN` で index `.data` は `data`、`galleries/` は `gallery`、TN path は `image`。

6. acquisition run folder confirmation  
   方法: `app/acquisition.go` に run 開始時の folder check endpoint を追加し、`app/ui.html` の `runAcquisition()` が per-page confirm ではなく run 開始時 1 回だけ folder 確認する。page 内 file overwrite 確認は server の `confirm` で残す。  
   合格: `TestViewerAcquisitionCheckReturnsRunStartFolderStatus` で新規は `not_exists`、既存は `confirm`。UI string tests で run 開始時 check path と once-per-run flag が残る。

7. keyboard modifier  
   方法: `app/ui.html` の keydown handler 先頭で `ctrlKey` / `metaKey` / `altKey` を return。f と Escape の gesture order は保持する。  
   合格: `TestEmbeddedUIHasReaderSpreadAndFullscreen` と fullscreen keyboard tests が modifier guard を含み、f/Escape が control focus skip より前に残る。

8. `--config` の Limits partial default  
   方法: `Limits` に custom unmarshal を追加し、JSON key present で explicit zero を保存、key absent だけ default で埋める。`loadConfig` は `Hitomi` と `Limits` を default から初期化し、per-field default を維持する。  
   合格: `TestLoadConfigLimitsPreservesExplicitZeroAndDefaultsOmittedFields` で partial JSON の未指定 field が default、明示 0 が保持される。

9. transaction が外部呼び出しをまたぐ  
   方法: `processFileWithYara` で YARA と VT lookup を tx 外へ出し、cache lookup も tx 外で行い、DB 書き込みだけを tx に寄せる。VT usage commit 意味は external lookup 後に確定的な DB 更新として扱う。  
   合格: `TestProcessFileWritesFindingsAfterExternalLookups` で external mock が tx 内に呼ばれず、rollback 失敗時 findings が残らない。

10. YARA exit code  
   方法: 正典 `tools/yr.exe` 実測で exit code convention を確定し、`runYaraWithRunner` を exit error として scan_error に統一する。規則 output の有無だけで pass/hit にしない。  
   合格: `TestRunYaraErrorExit1WithoutRulesIsScanError` が scan_error、既存 exit >1 execution failure tests が scan_error。`tools/VERSIONS.md` に実測 exit code を追記。

11. VT 404 negative cache  
   方法: `VTLookup` の 404 を `unknown_to_vt` へ分類し、scan cache に short TTL negative cache。process 側は cached negative を API 再呼出しなしで finding detail へ反映。`api_error:*` を上書きする成功後は cache/finding の意味を明確にする。  
   合格: `TestVT404NegativeCacheAvoidsRepeatedQuotaUsage` で最初の 404 後に cache hit し、再スキャンは fetch を増やさない。

12. `view` startup gg background  
   方法: `main.go` の `view` で `serve()` 前に同期 `warnGG(loadGG(...))` を呼ばず、startup background で gg を読む。  
   合格: `TestViewStartupServesBeforeGGLoad` で offline fetch でも handler route が gg load 前に利用可能。

13. `check` tx まとめ  
   方法: `runSavedQueries` を query ごとに tx にし、`bookmark_seen`、`galleries`、`last_run_at` を 1 tx で commit。rollback 失敗時 partial writes を残さない。  
   合格: `TestRunSavedQueriesRollsBackNewIdsWhenGalleryRecordFails` で recordDiscovered 失敗後に `bookmark_seen` が rollback される。

14. Python 参照実装 parity  
   方法: `src/scan.py` の `add_finding` を `INSERT OR IGNORE` にし、既存重複を畳む migration を追加。VT は `last_analysis_stats`、`engine_name`、`result` を読む。YARA failure は rule ではなく scan_error 状態へ寄せる。  
   合格: `python -m pytest tests -v` で duplicate findings 再スキャンと VT v3 shape tests が通る。

## 実施結果（正典検証）

- `SearchHTML`: `TestSearchHTMLPreservesOrOnlyUnionWithoutDuplicateGroup` pass. `positive[1:]` bounds guard added; or-only initial group is not intersected twice.
- `EnsureGG`: `TestEnsureGGFailurePreservesLiveGG` pass. Failure path preserves previous live source/bucket.
- `imageProxy` race/UI: `TestViewerImageProxyRetriesUsingInitialGGWhenAnotherRequestUpdatedGG` pass. Exact UI string `sourceStatusByEntry.get(key) !== "error";` is preserved through `retryableErrorStatus`; `no_full` is retryable; button enable and keyboard modifier guard tests pass.
- plain-term cache: `TestIndexIDsCachesInternalNodeAndDataForSameTerm` pass. Internal node and data caches use TTL and return copies.
- `classifyFetchURL`: `TestClassifyFetchURLSeparatesIndexDataGalleryAndTN` pass. `ltn.` no longer misclassifies as `image`; index `.data`, gallery, and TN path are separated.
- acquisition run-start folder check: `TestViewerAcquisitionCheckReturnsRunStartFolderStatus` and UI handler tests pass. UI uses `POST /api/acquisition/check` once before per-page writes.
- `Limits` partial config: `TestLoadConfigLimitsPreservesExplicitZeroAndDefaultsOmittedFields` pass. Explicit zero is kept; omitted fields get defaults.
- process transaction: `TestProcessFileYaraExternalLookupBeforeFindingsWrite`, `TestProcessFileVTExternalLookupBeforeFindingsWrite`, `TestProcessFileUsesCachedNegativeVTWithoutFetch`, and existing VT/process tests pass. External YARA/VT/cache lookup happens before DB writes.
- YARA exit codes: actual `tools/yr.exe` confirms scan/match/no-match exit `0` and invalid rule exit `1`. `runYaraWithRunner` treats nonzero exit as `scan_error`; tests pass. `tools/VERSIONS.md` records the actual result.
- VT `404`: `TestVT404NegativeCacheAvoidsRepeatedQuotaUsage` passes. VT `404` is `unknown_to_vt` with short negative cache TTL.
- `view` startup: `view` command no longer calls `warnGG(loadGG(...))` before `serve()`. `serve()` uses `startView`, starts background gg load, and serves the handler. `TestViewStartupServesBeforeGGLoad` passes.
- `check` transaction: `TestRunSavedQueriesRollsBackNewIdsWhenGalleryRecordFails` passes. Each saved query is processed in one transaction; `bookmark_seen` rolls back when `recordDiscovered` fails.
- Python reference parity: `python -m pytest tests -v` passes `23 passed`. Added duplicate `INSERT OR IGNORE`, TTL/negative cache, `last_analysis_stats`, `engine_name`/`result`, YARA exit-error classification, and transaction commit in `process_file`.
- Full Go verification after these changes:
  - `go vet . && go test -count=1 -timeout 300s .`
  - Result in `app`: `ok hitomi-hub/app 34.502s`

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
