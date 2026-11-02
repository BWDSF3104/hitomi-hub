# 外部コードレビュー指摘記録

**Status**: 前回高項目の実装確認は完了。Strict UI actual physical confirmation は `openGallery` background reset について完了。実際物理画面 `3840x2160`、fresh tab、normal OS screen capture、actual UI click、direct server baseline match を確認し、スクリーンショットはタスク別 Google Drive フォルダへ保存した。manual full-image retry は `errorDone > 0` の再現条件が確認できるまで低優先度で保留。
**Date**: 2026-11-02
**Next Action**: manual full-image retry の actual UI `errorDone > 0` entry 再現条件が確認できるまで保留。確認できた場合は strict UI screenshot を `I:\マイドライブ\kilo\2026-11-02-external-code-review-findings` に追加する。

## 意図 / 背景

- 外部コードレビューによる指摘を正典記録として残す。
- 本記録の目的は指摘の保存であり、実装・修正の開始ではない。
- レビュー側は実行しておらず、推定を含む項目には推定と記載されている。
- 優先度が高い項目は、ユーザーの体感と記録の正しさに直結する A-1、A-2、A-4、A-5、A-6 とされた。

## 調査結果 / 変更内容

- `docs/ai/tasks/2026-11-02-external-code-review-findings.md` に外部レビュー指摘を記録した。
- 本タスクの開始で `docs/ai/tasks/2026-11-02-external-code-review-findings.md` と `docs/ai/current-task.tsv` に記録を追加した。
- 初期記録時点では実装ファイルやコードへの変更は行っていなかった。その後、ユーザーの指示で高項目を実装し、以下の変更を反映した。

### A. 動作上の不具合・誤判定

#### A-1. `gg.js` と index version が起動後 1 回しか読まれない
- `view` を常駐させる運用では、決定記録どおり bucket が 1 時間刻みで進むため、1 時間後から full 画像が一斉に `no_full`（404）になる可能性がある。
- `indexVersion` も永続キャッシュのため、index 更新後は検索が壊れる可能性がある。
- 対策候補:
  - 404 時に `gg.js` を再取得して 1 回だけ再試行する。
  - 最短間隔を付ける。
  - TTL による更新。
- `c.gg` と `c.versions` は mutex なしで読み書きされている可能性が指摘されている。

#### A-2. 検索クエリの正規化順序が逆
- `SearchHTML` が `normalizeSearchTerm(query)`（`_`→空白）を `strings.Fields` より先に実行している可能性が指摘されている。
- その結果 `female:big_breasts` が `female:big` と `breasts` に分裂する可能性がある。
- 対策候補:
  - 先に空白で分割し、term ごとに正規化する。

#### A-3. `orderbydirection:asc` が効かない
- `applyOrderForInitial` と最後の `applySearchOrder` で 2 回 reverse され、結果が元に戻っている可能性が指摘されている。
- 片方を削除する。両者は同一処理の重複とされた。

#### A-4. 既読が巻き戻る
- `setReadState` の upsert が `fully_read = excluded.fully_read` で上書きしている可能性が指摘されている。
- 読み完了後に 1 ページ開くだけで `fully_read` が 0 に戻る可能性がある。
- 対策候補:
  - `MAX(fully_read, excluded.fully_read)` を使用。
  - 明示的な解除だけ別操作にする。

#### A-5. scan の記録が黙って落ちる
- `galleryIDForPath` が id を取得できないと 0 を返し、そのまま INSERT している可能性が指摘されている。NULL ではない。
- export が作る `Galleries/123.cbz` は FK 違反になる可能性がある。`galleries` 行が無い id も同様。
- `processFile` が `recordFile` と `addFinding` のエラーを `_ =` で捨てており、DB に残らないのに summary だけ加算される可能性がある。
- 対策候補:
  - 0 を NULL とする。
  - 無ければ `recordDiscovered` を呼ぶ。
  - エラーをログに出す。

#### A-6. VirusTotal が「確認済み clean」を量産する
- `defaultFetch` が HTTP ステータスを見ていない可能性が指摘されている。
- 未登録ハッシュの 404、429、401 の JSON を `positives=0` として `pass` 扱いし、`scan_cache` に永続保存する可能性がある。
- `rate_limited`、`no_key`、`scan_error` のときも status が `pass` のままで、`scan_source` だけが `structural+virustotal` になる可能性がある。
- 日次上限は `RateState` がプロセス内のため、実行ごとにリセットされる可能性がある。
- 対策候補:
  - 200 以外は cache しない。
  - 「未照会」を別 status にする。
  - cache に TTL を付ける。
  - 日次カウントを DB に持つ。
- `ThreatName` には engine の category（`malicious`）が入り、検出名になっていない可能性が指摘されている。

#### A-7. YARA 実行失敗が `hit` になる
- `RunYara` が失敗を `"yara_error:..."` という規則名で返し、呼び出し側は findings として `hit` にしている可能性が指摘されている。
- `yr` のパスが相対（`tools/yr.exe`）で cwd がずれると、全 cbz が hit になる可能性がある。
- Python 版は終了コード 0/1 を許容するが、Go 版は `err != nil` なら即エラーとされる可能性が指摘されている。
- YARA-X が一致時に 1 を返す場合は取りこぼす可能性がある（推定）。
- 対策候補:
  - エラーは finding ではなく `scan_error` や警告として扱う。
  - 判定を揃える。

#### A-8. PDF 出力が仕様不正
- Page オブジェクトの辞書に閉じ `>>` がない可能性が指摘されている。
- テストは部分文字列検査だけなので通ってしまう可能性がある。
- 厳密なリーダーで開けない可能性がある。

#### A-9. `check`（新着追跡）の取りこぼし
- 「新着」を `galleries` 行の有無で判定している可能性が指摘されている。
- `/api/search`、`search-card`、`read` が閲覧しただけで行を作るため、検索画面に出た作品は以後の `check` で新着扱いされない可能性がある。
- 対策候補:
  - 専用の `seen` 管理にする。
  - `bookmark_queries` ごとの既知 id を持たせる。

#### A-10. データ競合
- `viewer.infoCache` が複数ハンドラから無保護で読み書きされ、`concurrent map writes` で落ちる可能性がある。
- `c.gg` と `c.versions` も同じ問題である可能性が指摘されている。

### B. 非効率

#### B-1. 検索・通信
- nozomi は毎回ファイル全体を取得しており、Range がない可能性が指摘されている。`type:doujinshi-japanese` は約 1.2MB とされた。
- `language:` 併用時は同じ tag を 2 回取得する可能性がある（`all` と `english`）。
- ページ送りのたびに再取得される可能性がある。
- 対策候補: `(area, tag, lang)` 単位で TTL キャッシュを持たせる。
- B-tree の root node と version を毎回取り直している可能性が指摘されている。メモリキャッシュで 1 検索あたり 1〜2 リクエスト減らせる可能性がある。
- 検索カードは 1 枚ずつ `galleries/<id>.js` を 1000ms ゲート越しに取得するため、25 件で 25 秒以上かかる可能性がある。
- `infoCache` はメモリのみで、再起動すると失われる可能性がある。
- カード取得ループが `loadSearchCardCover`（`currentPriorityDone` 待ち）を await するため、リーダーの画像 DL 中は次カードのメタ取得も止まる可能性がある。
- 対策候補:
  - 表示領域に入ったカードだけ取得する（IntersectionObserver）。
  - gallery info を SQLite に永続化する。
  - ゲートを ltn の静的取得用に緩める。
- `gate` と retry の `time.Sleep` が ctx 非対応である可能性が指摘されている。UI が abort しても、サーバー側は待機と再試行を続ける可能性がある。

#### B-2. 画像キャッシュ・ダウンロード
- キャッシュ書き込みが `os.WriteFile` 直書きであり、中断すると途中のファイルが永続的に配信される可能性がある。
- temp ファイルに書いて rename し、先頭の magic も検査すべきとされた。Cloudflare の HTML が 200 で来る場合があるため。
- 同一 hash の同時リクエストを 1 回にまとめる singleflight がない可能性が指摘されている。
- キャッシュに容量上限がなく、別 gallery でも同じ hash が重複保存される可能性がある。
- `thumbProxy` は TN が失敗すると `imageProxy` にフォールバックし、カバー 1 枚のために full 画像を DL してキャッシュする可能性がある。
- `download` は既存ファイルを確認せず毎回再取得する可能性があり、書き込みも非アトミックである可能性が指摘されている。
- `serveStoredImage` は `ReadFile` で全読みしている可能性が指摘されている。`http.ServeContent` なら 304 と Range に対応し、メモリも節約できる可能性がある。

#### B-3. ブラウザ側
- objectURL の上限 2000 件で全件 revoke するため、表示中の画像も壊れる可能性が指摘されている。
- full 画像 2000 枚をメモリに載せるのも過大である可能性が指摘されている。LRU で表示中を除外すべきとされた。
- strip の表示が「全ページ full DL 完了」に依存している。TN 禁止方針の帰結であり設計判断だが、帯域とメモリの最大の消費源とされた。
- サーバー側で縮小サムネイルを生成してキャッシュする案は、方針と衝突しない範囲で検討余地があるとされた。
- ページ移動のたびに `POST /api/read` と `GET /api/lists` が走っている可能性が指摘されている。決定記録は debounce と書かれているが、実装にはない可能性が指摘されている。
- `renderStrip()` が毎回 50 枚を innerHTML で再構築している可能性が指摘されている。`highlightStrip` だけで足りる場面が多いとされた。
- 起動時に `restoreSearchState()` と `runSearch()` が両方走り、カード取得が重複している可能性が指摘されている。

#### B-4. スキャン・DB・その他
- `scan` は毎回全ファイルを SHA256 している可能性が指摘されている。size と mtime が前回と同じなら skip できる可能性がある。
- `ScanPDFKeys` は 7 キーそれぞれで `string(data)` を作り直している可能性が指摘されている。2MB×7 コピーとされ、`bytes.Contains` にすべきとされた。
- `/AA` と `/JS` は 3 バイトしかなく、圧縮ストリーム 2MB 中に偶然現れる確率が各 10% 強あり、構造検査の誤検知源になる可能性が指摘されている。
- `pdf.go` の `rawRGB` は `img.At` をピクセルごとに呼び、全ページを RGB で保持している可能性が指摘されている。大きなページだと 1 枚 十数 MB、200 ページなら数 GB になる可能性がある。JPEG はそのまま DCTDecode で埋め込み、ページごとにファイルへ書き出すべきとされた。
- SQLite に `journal_mode=WAL` と `busy_timeout` がない可能性が指摘されている。viewer を常駐させたまま CLI（`download` や `check`）を走らせると `SQLITE_BUSY` になる可能性がある。
- スキーマ移行の仕組みがない可能性が指摘されている。`CREATE IF NOT EXISTS` のみで列追加が反映されず、`PRAGMA user_version` を使うべきとされた。

### C. 設定・保守

- ドキュメントの `--config` はコードに存在せず、指定するとフラグエラーで終了する可能性が指摘されている。実際は常に `config.example.json` を読む可能性が指摘されている。
- `MinIntervalMS == 0` のとき `cfg.Hitomi` 全体をデフォルトで上書きする可能性が指摘されている。`use_live_gg` を省略した config では bool のゼロ値で live gg が無効になり、黙って snapshot に落ちる可能性がある。
- 未使用コードとして `Search`、`SearchQuery`、`nozomiURL`、`fileExt`（テスト専用）、`Limits.MaxRequestsPerMinute`、`imageExts`、`pageSource` エンドポイントが指摘されている。

### D. 軽微なセキュリティ

- POST 系エンドポイントが Content-Type を見ておらず、Host や Origin も検査していない可能性が指摘されている。
- 別サイトからの `text/plain` の cross-origin POST と DNS rebinding で、お気に入りや保存検索を書き換えられる可能性がある。
- 対策候補: Host と Origin を `127.0.0.1:<port>` に限定する。

## Verification

- Strict UI actual physical confirmation:
  - Direct server baseline: `http://127.0.0.1:8731/api/search?query=furry&lang=all&page=1&limit=25` returned `200`, `total = 32882`, `ids.length = 25`, `X-Search-Timing` present, `content-length = 1071`.
  - Actual physical screen resolution was `3840x2160`.
  - Fresh tab `http://127.0.0.1:8731/?newtab=1` was used.
  - Screenshots were saved to `I:\マイドライブ\kilo\2026-11-02-external-code-review-findings\`.
  - `openGallery` background reset was confirmed by actual UI click on first result `4298786`, then the same first result link was clicked again. Post-click state showed `backgroundGalleryId = 4298786`, `backgroundSignal.signal.aborted = false`, `prefetch-status` reached `actual pixels 35/35`.
- 本記録の検証:
  - `task_manager.py add` で `docs/ai/tasks/2026-11-02-external-code-review-findings.md` と `docs/ai/current-task.tsv` に記録を追加した。
  - 実装ファイルの変更は行っていない。
- 現在のコードに対する有効性評価は静的読解のみ。実行検証、修正、commit は未実施。

## 現在のコードに対する有効性評価と対応方針（2026-11-02）

判定は現コードの静的読解に基づく。レビュー側の推定は「推定」と表記し、現コードで確認できた範囲と区別する。

### A-1. `gg.js` と index version が 1 回だけ読まれる
- 判定: 有効。
- 現コード:
  - `loadGG` は実行中に 1 回だけ `Client.LoadGG()` を呼び、`c.gg` を設定する。`app/main.go:711-713`、`app/gg.go:67-85`。
  - `GG()` は以後同じ `c.gg` を返す。`app/hitomi.go` の `ImageURL` / `StorageURL` / `ThumbnailURL` は `GG` を使う。`app/hitomi.go:181-197`、`app/hitomi.go:230-245`。
  - `indexVersion` は `c.versions` に field 単位で永続キャッシュし、更新 TTL がない。`app/searchindex.go:154-171`。
  - `app/hitomi.go:40-43` の `Client` で `versions` と `gg` は mutex 外で読み書きされている。`app/gg.go:68-85`。
- 対応: 可能。`c.gg` と `c.versions` を mutex で保護し、TTL 付きキャッシュ、または `404` 時に一度だけ `gg.js` を再取得する仕組みを追加できる。再取得は rate gate 内で行い、network 呼び出し中に mutex を保持しない設計が必要。

### A-2. 検索クエリの正規化順序が逆
- 判定: 有効。
- 現コード: `SearchHTML` は `normalizeSearchTerm(query)` を `strings.Fields` より先に実行している。`app/searchhtml.go:21-23`、`app/searchhtml.go:182-184`。
- 影響: `female:big_breasts` が `female:big` と `breasts` に分裂する可能性を確認できる。
- 対応: 可能。raw query を `strings.Fields` で分割し、各 term を個別に normalize する形に修正できる。order term / `or` term の解析も raw 文字列側で行う必要がある。

### A-3. `orderbydirection:asc` が効かない
- 判定: 有効。
- 現コード: positive term 1 件目の initial 結果に `applyOrderForInitial` を適用後、最終でも `applySearchOrder` を適用する。`app/searchhtml.go:242`、`app/searchhtml.go:294`、`app/searchhtml.go:299-308`。
- 影響: `asc` で reverse した結果が最終 reverse で元の順に戻る。`or` 経由の initial 経路では二重 reverse にならないため、経路によって挙動がずれる。
- 対応: 可能。duplicate reverse を 1 箇所だけ残す。既存 `rand` 挙動と intersection 後の順序維持を確認し、tests で確認する必要がある。

### A-4. 既読が巻き戻る
- 判定: 有効。
- 現コード: `setReadState` の upsert が `fully_read = excluded.fully_read` で上書きする。`app/library.go:25-36`。UI の読み完了は `complete=true` で呼ぶが、通常 page 移動は `complete=false` で呼ぶ。`app/ui.html:1451`、`app/ui.html:1062`、`app/viewer.go:541`。
- 影響: 読み完了後に 1 ページ開くと `fully_read` が 0 に戻る。
- 対応: 可能。通常 read は `MAX(fully_read, excluded.fully_read)` にし、明示的な解除だけ別操作にする設計にできる。CLI/UI で「解除」操作が未定義なら、その追加も必要。

### A-5. scan の記録が黙って落ちる
- 判定: 有効。
- 現コード:
  - `galleryIDForPath` は gallery dir 外、非数字、export ファイル名などで `0` を返す。`app/main.go:214-225`。
  - `files.gallery_id` は `galleries(gallery_id)` を参照し、schema 上 NULL は可能だが 0 は FK 違反になり得る。`schema.sql:11-22`。
  - `processFile` は `addFinding` と `recordFile` のエラーを `_ =` で捨て、summary だけ加算する。`app/main.go:198-212`、`app/main.go:326`、`app/main.go:330`、`app/main.go:336`、`app/main.go:365-371`。
- 対応: 可能。`galleryIDForPath` の無効値は NULL にし、既知 id が `galleries` に無い場合は `recordDiscovered` を呼ぶ。DB 書き込みエラーは stderr または summary の error 項目に出す。

### A-6. VirusTotal が「確認済み clean」を量産する
- 判定: 有効。
- 現コード:
  - `defaultFetch` は HTTP status を確認せず JSON decode 成功を返す。`app/vt.go:28-47`。
  - `VTLookup` は decode 成功後、`stats` が無く `positives=0` なら `status = "pass"` になる。`app/vt.go:80-93`。
  - `processFile` は `pass` / `hit` のとき `scan_cache` に永続保存する。`app/main.go:355-362`、`schema.sql:31-38`。
  - `rate_limited`、`no_key`、`scan_error` は cache に保存されないが、`VirusTotal.Enabled` なら `scan_source` が `structural+virustotal` になる。`app/main.go:365-371`。
  - 日次 count は `RateState` がプロセス内であり、実行ごとにリセットされる。`app/vt.go:21-24`。
- 対応: 可能。200/VT 正応答のみ cache する。`not_evaluated`、`rate_limited`、`no_key`、`scan_error` を別 status で DB に残す。cache TTL、日次 counter の DB 化、ThreatName に engine result を入れる修正が可能。

### A-7. YARA 実行失敗が `hit` になる
- 判定: 有効。推定部分あり。
- 現コード: `RunYara` は `cmd.Output()` の err を `"yara_error:..."` rule として返し、呼び出し側は findings に追加して status を `hit` にする。`app/vt.go:163-183`、`app/main.go:333-341`。
- 推定: `yr` の相対パスと cwd のずれで全 file が hit になる可能性は config 側で `tools/yr.exe` を使う運用では有効。`app/yara_test.go:21` で `tools/yr.exe` を参照している。
- 推定: YARA-X が match で exit code 1 を返す場合、Go の `err != nil` で error 扱いして取りこぼす可能性はコードから確認できるが、YARA-X の exit code 仕様自体は実行確認していない。
- 対応: 可能。実行失敗を finding にせず `scan_error` / warning として扱う。exit code は allow list（例: 0, match 時 1）で判定し、`yr` path は絶対 path または dataDir 基準で解決する。

### A-8. PDF 出力が仕様不正
- 判定: 有効。
- 現コード: Page オブジェクトの outer dictionary に閉じ `>>` がない。`app/pdf.go:140-143`。
- 現テスト: `export_test.go:168-171` は部分文字列検査のみで、strict parser で開けることを検証していない。
- 対応: 可能。Page object を正しい dictionary 構文にする。tests で strict parserまたは object 構造検証を追加する。

### A-9. `check` の新着判定の取りこぼし
- 判定: 有効。
- 現コード: `check` は `galleries` 行の有無で新着を判定する。`app/check.go:7-14`、`app/check.go:51-55`。
- 影響: `/api/search`、`/api/search-card/{id}`、`POST /api/read`、`POST /api/fav`、`POST /api/cache/acquire` が `recordDiscovered` で `galleries` 行を作るため、検索画面に出た gallery は `check` で新着扱いされなくなる。`app/viewer.go:261-263`、`app/viewer.go:290`、`app/viewer.go:537`、`app/viewer.go:562`、`app/acquisition.go:120-123`。
- 対応: 可能。`galleries` の有無ではなく、保存検索別または共有 `seen` 表で管理する。`bookmark_query_seen` のような dedicated table が適切。既存 `galleries` は discovered 記録として残せる。

### A-10. data race
- 判定: 有効。
- 現コード:
  - `viewer.infoCache` は mutex なしで複数 handler から read/write される。`app/viewer.go:28`、`app/viewer.go:286-289`、`app/viewer.go:593-603`。
  - `Client.versions` と `Client.gg` も mutex で保護されていない。`app/hitomi.go:40-43`、`app/searchindex.go:154-171`、`app/gg.go:68-85`。
  - `metrics` は mutex を使うが、上記 cache field と mutex domain が分離されていない。`app/hitomi.go:50-71`、`app/metrics.go:51-60`。
- 対応: 可能。cache field 用の mutex を追加し、network gate と分離する。`infoCache` は mutex-protected map または SQLite 永続 cache にする。

### B-1. 検索・通信の非効率
- nozomi full fetch:
  - `SearchNozomiParts` は full nozomi file を Range なしで取得する。`app/searchhtml.go:68-73`。
  - `language:` term は state の language を変え、同じ area/tag を前 term の lang と new lang で再取得する経路がある。`app/searchhtml.go:76-94`。
  - `(area, tag, lang)` 単位 TTL cache は可能。
- B-tree:
  - `IndexIDs` は term ごとに root node を fetch する。`app/searchindex.go:191-229`。
  - `indexVersion` は process 内で cache 済みだが、root node cache はない。`app/searchindex.go:154-171`。root node cache は可能。
- search card:
  - viewer search は cards を 1 枚ずつ `galleries/<id>.js` に equivalent な `v.info(id)` で取得し、1000ms gate を使う。`app/viewer.go:279-292`、`app/hitomi.go:50-73`。
  - UI の card load は sequential await で、`loadSearchCardCover` が `currentPriorityDone` を待つ。`app/ui.html:1259-1262`、`app/ui.html:1349-1367`。
  - `infoCache` は memory だけで、再起動時に失われる。`app/viewer.go:40`、`app/viewer.go:593-603`。
- ctx/gate/retry sleep:
  - gate と retry sleep は ctx を確認しない `time.Sleep` を使う。`app/hitomi.go:54-71`、`app/hitomi.go:94-166`。UI abort 後も server 側 sleep/再試行が継続し得る。
- 対応: IntersectionObserver、SQLite gallery info cache、static/ltn fetch の専用 relaxed gate、ctx-aware sleep は実装可能。

### B-2. 画像キャッシュ・ダウンロードの非効率・危険性
- atomic write:
  - cache write は `os.WriteFile` 直書き。`app/viewer.go:439-443`、`app/viewer.go:471-474`。download も direct write。`app/main.go:297-300`。
  - magic check は scanner にあるが、proxy download write path で applied していない。`app/scan.go:47-79`。
  - temp file + rename、magic/magic-prefix check は可能。
- singleflight / capacity:
  - cache path は hash+format なので同一 hash の image cache は shared だが、同時リクエストを 1 回にまとめる仕組みはない。`app/viewer.go:462-474`。
  - cache 容量上限はない。download で gallery dir に保存する同一 hash file は galleryごとに重複し得る。`app/main.go:291-300`。
- thumb fallback:
  - `thumbProxy` は TN fetch error 時に local full file、なければ `imageProxy` full image fallback へ行く。`app/viewer.go:431-449`。cover 1 枚のために full image を DL/cache し得る。
- download existing:
  - `download` は existing file を確認せず毎回 fetch/write する。`app/main.go:291-300`。
- serveStoredImage:
  - full `os.ReadFile` で全読みし、304/Range 非対応。`app/viewer.go:176-182`。
- 対応: singleflight、temp+rename、magic prefix check、既存ファイル skip、`http.ServeContent`、server-side thumbnail cache は可能。TN fallback policy と server-side thumbnail は design decision を要する。

### B-3. ブラウザ側
- objectURL:
  - `MAX_FULL_CACHE_ENTRIES = 2000` で超過時に `revokeFullObjectUrls()` が全 objectURL を revoke する。`app/ui.html:219`、`app/ui.html:782-784`、`app/ui.html:823-833`。表示中の img src が壊れ得る。LRU で表示中を除外する修正は可能。
- strip:
  - strip source は objectURL/local-direct only で、TN display fallback disabled。`app/ui.html:336-338`、`app/ui.html:347-351`、`app/ui.html:1109`、`app/ui.html:415`。
  - strip 表示が full DL/localに依存する。これは design decision。server-side thumbnail cache は方針衝突のない範囲で検討可能だが、viewer display source policy の変更または別 policy が必要。
- read/lists:
  - page 移動で `POST /api/read` と `GET /api/lists` を呼ぶ。debounce はない。`app/ui.html:1060-1065`、`app/ui.html:1094`、`app/ui.html:1176-1186`。
- render:
  - `renderStrip()` が 50 枚を innerHTML で再構築する。`app/ui.html:1096-1122`。`highlightStrip()` だけで足りる場面は可能だが、group change や strip item data change は再構築が必要。
- startup:
  - `restoreSearchState()` と `runSearch()` が両方走ることがある。`app/ui.html:1289-1307`、`app/ui.html:1630-1645`。duplicate card fetch を避ける設計に変更可能。

### B-4. scan・DB・その他
- scan SHA256:
  - `processFile` は毎回 `Sha256File(path)` を呼ぶ。`app/main.go:315-320`。size/mtime と stored row で skip する仕組みは可能。
- PDF key scan:
  - `ScanPDFKeys` は 7 keyそれぞれで `string(data)` を作る。`app/scan.go:152-168`。`bytes.Contains` に変換可能。
  - `/AA` / `/JS` は短く、compressed stream 内 false positive を生み得る。PDF object syntax boundariesや object stream range制限が必要だが、完全検知は難しい。
- PDF memory:
  - `rawRGB` は全 page を `img.At` 経由で RGB 保持する。`app/pdf.go:47-58`。JPEGはDCTDecodeで embed可能。WebP/PNGは decoder出力がDCTDecodeで直接 embedできる形式ではないため、format分岐が必要。
- SQLite:
  - `openDB` は `PRAGMA foreign_keys = ON` と `db.SetMaxOpenConns(1)` だけ。`journal_mode=WAL` と `busy_timeout` をコード側で設定していない。`app/main.go:154-165`。複数プロセスで `SQLITE_BUSY` 可能性は有効。
- schema migration:
  - `ensureSchema` は `CREATE TABLE IF NOT EXISTS` を流すだけで `PRAGMA user_version` migration がない。`app/main.go:167-174`、`schema.sql:1-68`。列追加は反映されない。migration frameworkは可能。

### C. 設定・保守
- `--config`:
  - `docs/ai/usage-memo.md` は `--config config.json` を例示しているが、`main.go` の flag set に `config` flag はない。`docs/ai/usage-memo.md:22-23`、`app/main.go:401-423`。unknown flagで exit し得る。
  - 実際は常時 `config.example.json` を読む。`app/main.go:428-431`。
  - docs修正または `--config` flag追加が可能。どちらを選ぶかは方針次第。
- `MinIntervalMS == 0`:
  - `loadConfig` は `cfg.Hitomi.MinIntervalMS == 0` で `cfg.Hitomi` 全体を `defaultHitomiConfig()` で上書きする。`app/main.go:121-126`。
  - `MinIntervalMS` が非 0 なら `Hitomi` は JSON の値を使い、`use_live_gg` 省略は Go bool zero value の false になる。`app/hitomi.go:23-31`、`app/hitomi.go:45-48`。
  - per-field default化または optional sentinelが必要。実装可能。
- unused code:
  - `Search`、`SearchQuery`、`nozomiURL`、`fileExt`、`imageExts`、`Limits.MaxRequestsPerMinute` は production path で未使用またはtest only。`app/hitomi.go:247-261`、`app/hitomi.go:403-480`、`app/main.go:36`、`app/scan.go:17`、`app/hitomi_test.go:130`、`app/searchhtml_test.go:124`。
  - `pageSource` endpoint は handler/routeに存在するがUIで呼ばれていない。`app/viewer.go:51`、`app/viewer.go:379`。APIとして残すか削除かは方針次第。

### D. 軽微な security
- POST endpoints:
  - `POST /api/read`、`POST /api/fav`、`POST /api/query`、`POST /api/cache/acquire` は handler側で Content-Type、Host、Originを制限していない。`app/viewer.go:44-57`、`app/viewer.go:523-591`。
  - cross-origin simple POSTや DNS rebindingで local viewerの書き込み操作が狙われ得る。
  - HostとOriginを loopback portに限定する middlewareは実装可能。same-origin requestのOriginを許し、cross-origin OriginとHost mismatchを拒否する設計にできる。

## 優先実装候補
- 優先度が高い項目: A-1、A-2、A-4、A-5、A-6。
- これらはユーザー体感、記録の正しさ、scan cacheの信頼性に直接影響する。
- 実装は未開始。実装指示があり、各項目の patch scopeを指定できる。

## 現在ユースケースで高い項目の修正方法と合格条件（2026-11-02）

判定は現コードの静的読解と現行ユースケースの優先順位に基づく。実施項目は A-1、A-2、A-4、A-5、A-6、A-9、A-10、SQLite WAL/busy_timeout/schema version。

### A-1. `gg.js` / index version の TTL と force refresh
- 修正方法:
  - `Client` に cache 用 mutex を追加し、`gg` と `versions` を mutex で保護する。
  - `EnsureGG(ctx, force)` を追加し、live `gg.js` を TTL で更新する。既定 TTL は 1 時間。
  - `imageProxy` が storage 404 を受けたとき、`EnsureGG(ctx, true)` を 1 回だけ呼び、rate gate 内で再取得して 1 回だけ storage URL を再試行する。
  - `indexVersion` を mutex + TTL cache にする。
- 合格条件:
  - `UseLiveGG=false` のときは live fetch せず snapshot を返す。
  - cache が TTL 内なら network fetch を増やさない。
  - `imageProxy` の storage 404 で live `gg.js` を force refresh できる。
  - `tn.` を image proxy の fallback として fetch しない。
  - `go test -race -count=1 .` で `Client.gg` / `Client.versions` / `viewer.infoCache` の data race を検出しない。

### A-2. 検索 term 正規化順序
- 修正方法:
  - `SearchHTML` は raw query を `strings.Fields` で先に分割し、各 term を個別に `normalizeSearchTerm` する。
  - `female:big_breasts` は raw term の `_` を保ち、1 term として nozomi path を組む。
- 合格条件:
  - `female:big_breasts` が `female:big` と `breasts` に分裂しない。
  - URL が `/n/tag/female:big_breasts-<lang>.nozomi` になる。

### A-4. `read_state` の完全既読 preserve
- 修正方法:
  - `setReadState` の upsert を `MAX(fully_read, excluded.fully_read)` 相当にする。
  - 明示的な解除は `setReadStateWithClear` と CLI `--clear-complete` で行う。
  - UI/CLI の通常 page 移動は解除扱いにしない。
- 合格条件:
  - `complete=true` の後に通常 page 移動 `complete=false` しても `fully_read=1` が保たれる。
  - 明示的な clear 操作だけが `fully_read=0` にできる。
  - `read` CLI の出力は更新後の DB 値を反映する。

### A-5. scan record の NULL gallery、known gallery、DB error
- 修正方法:
  - `galleryIDForPath` は `<gallery_dir>/<id>/<file>` の `<file>` 形式のみ id を認める。export 直下 `<gallery_dir>/<id>.<format>` は `NULL` にする。
  - 有効な gallery id の場合、file insert 前に `recordDiscovered` で parent row を作る。
  - `processFile` は `addFinding`, `cacheStore`, `recordScanFile` の DB error を stderr に出す。
  - summary は DB write 成功後にのみ加算する。
- 合格条件:
  - `Galleries/123.cbz` は `files.gallery_id` に FK 違反になる `0` を入れない。
  - known id の file path では parent row が作られ、DB error で黙って落ちない。
  - DB write failure 時に summary だけ加算されない。

### A-6. VirusTotal status / cache TTL / daily DB / engine result name
- 修正方法:
  - `FetchFunc` は HTTP status を返す。
  - `VTLookup` は 200 の正応答だけを `pass`/`hit` として扱い、404/401/429/other を cache しない。
  - `scan_cache` lookup に TTL を付ける。cache に保存するのは `pass`/`hit` のときだけ。
  - 日次 request count を `vt_rate_state` table で UTC date に保存する。
  - `ThreatName` は engine result name を優先する。engine name が無いときだけ category を使う。
  - `scan_source` は cached pass/hit または live pass/hit のときだけ `structural+virustotal`。
- 合格条件:
  - HTTP status 非 200 で `scan_cache` に `pass` を保存しない。
  - 404/429/401/no key/fetch error は `not_evaluated`、`rate_limited`、`auth_error`、`no_key`、`scan_error` として扱い、source は structural のまま。
  - cache TTL 切れの pass は live lookup 経路に落ちる。
  - 同一 UTC date の daily request count は DB に残り、process 間でリセットされない。
  - engine result name が `ThreatName` に出る。

### A-9. `check` の saved query 別 seen tracking
- 修正方法:
  - `bookmark_seen` table を追加する。
  - `runSavedQueries` は `galleries` 行の有無ではなく、`bookmark_seen` で saved query 別新着を判定する。
  - `bookmark_queries` の `last_run_at` は更新する。
- 合格条件:
  - 同じ保存検索の 2 回目 `check` で、1 回目に表示済みの id は new にならない。
  - 保存検索別の `bookmark_seen` が `runSavedQueries` 後に残る。
  - `galleries` 行が存在しても、その query の `bookmark_seen` に無い id は first run で new として扱える。

### A-10. shared cache data race
- 修正方法:
  - `Client.gg`, `Client.versions`, `viewer.infoCache` を mutex で保護する。
  - network fetch 中は cache mutex を保持しない。
- 合格条件:
  - cgo/gcc が可用な環境では `go test -race -count=1 .` で data race を検出しない。
  - 本環境では cgo/gcc がなく race detector は実行不可のため、mutex lock と concurrent handler/cache test で保護構造を確認する。
  - `Viewer` の `infoCache` cache hit/cache miss 判定は mutex lock で行う。

### SQLite WAL / busy_timeout / schema version
- 修正方法:
  - `openDB` で `PRAGMA journal_mode = WAL` と `PRAGMA busy_timeout = 5000` を設定する。
  - `ensureSchema` は `PRAGMA user_version` を schema version (2) に設定する。
  - schema.sql に `bookmark_seen` と `vt_rate_state` を追加する。
- 合格条件:
  - `PRAGMA journal_mode` が `wal` を返す。
  - `PRAGMA busy_timeout` が positive を返す。
  - `PRAGMA user_version` が schema version 2 を返す。
  - `bookmark_seen`, `vt_rate_state` が ensureSchema 後に存在する。

## 実装内容（高項目）

- A-1:
  - `Client` に `cacheMu`, `versionAt`, `ggLoadedAt` を追加。
  - `EnsureGG(ctx, force)` は network fetch 中は mutex を保持せず、fetch 後に mutex lock して `c.gg` を更新。
  - `indexVersion` は `ensureIndexVersion(field, force)` で TTL cache を参照。
  - `imageProxy` は storage 404 で live `gg.js` を force refresh し、live と確認できたときだけ 1 回 retry。
  - `UseLiveGG=false` のときは live fetch せず snapshot を返す。
  - `tn.` 判定は `ltn.` を TN と誤認しない。
- A-2:
  - `SearchHTML` は raw query を先に `strings.Fields` で分割し、term ごとに `normalizeSearchTerm`。
  - `female:big_breasts` の `_` を保持し、search card と query term route で一致。
- A-4:
  - `setReadState` は `MAX(fully_read, excluded.fully_read)` で `fully_read` を preserve。
  - 明示 clear は `setReadStateWithClear`。
  - CLI `findCommand` の `--clear-complete` で explicit clear route を追加。
- A-5:
  - `galleryIDForPath` は export direct path や invalid numeric id で `sql.NullInt64` の `NULL` を返す。
  - `recordScanFile` は known gallery id で `recordDiscovered` を呼ぶ。
  - `processFile` は `addFinding`, `cacheStore`, `recordScanFile` の DB error を log し、summary は成功後にのみ加算。
- A-6:
  - `FetchFunc` は `[]byte`, `error`, HTTP status を返す。
  - `VTLookup` は HTTP status を扱い、non-200 を cache しない。
  - `scan_cache` lookup に `scanCacheTTL` を追加。
  - 同一 UTC date の daily request count は `vt_rate_state` table に残す。
  - `ThreatName` は engine result name を優先。
- A-9:
  - `bookmark_seen` table を追加。
  - `runSavedQueries` は query 別 seen table で新着判定し、`markQuerySeen` で記録。
- A-10:
  - `Client.gg`, `Client.versions`, `viewer.infoCache` を mutex で保護。
  - `EnsureGG` は cache mutex を保持したまま fetch しない。
  - `searchCard` の infoCache hit/miss 判定は `infoCacheMu` の内側で行う。
- SQLite:
  - `openDB` で `PRAGMA journal_mode = WAL` と `PRAGMA busy_timeout = 5000` を設定。
  - `ensureSchema` で `PRAGMA user_version = 2` を設定。
  - `bookmark_seen`, `vt_rate_state` を schema に追加。

## 実装確認（2026-11-02）

- `go vet .` passes.
- `go test -count=1 .` passes.
- `go test -race -count=1 .` is not executable in this environment because cgo/gcc is unavailable. The race condition is prevented by mutex locks, and concurrent handler/cache tests pass under `go test -count=1 .`.

## 簡易 UI 確認（2026-11-02）

- Local UI server を `S:\Documents\Kilo_Commons\hitomi-hub\scratch\ui_confirm` root で起動して simple UI confirmation を実施。Strict UI confirmation blocker とは別。
- Screenshots are saved under the current workspace root `Common_Operations`:
  - `S:\Documents\Kilo_Commons\Common_Operations\scratch\ui_confirm\01_search_female_big_breasts.png`
    - UI query input に `female:big_breasts` を入力し、search route/result へ反映。
    - 1回目の UI search で results 25件、total `32882` を確認。raw query が `female:big` + `breasts` に分裂せず `female:big_breasts` term route で results が表示された。
  - `S:\Documents\Kilo_Commons\Common_Operations\scratch\ui_confirm\02_read_state_preserved_after_next_page.png`
    - gallery `4297838` で UI click `読み完了` 後、`#next` page move で通常 read update を実施。
    - `/api/gallery/4297838` read state は `last_page 3`, `fully_read 1` を保持。
  - `S:\Documents\Kilo_Commons\Common_Operations\scratch\ui_confirm\03_manual_retry_ui_current_state.png`
    - manual retry UI は存在し、`retry failed` button は errorDone が 0 のため disabled。
    - strict UI confirmation blocker（actual UI `errorDone > 0` entry 再現）は解決していない。
- Simple UI/API re-request で `/api/search?query=female%3Abig_breasts` が一時的に `502` を返した。これは local test server の network fetch/rate condition で、1回目の UI search success と screenshot evidence から UI path と raw query preservation を確認した。

## Search bottleneck detection result

- P3 `#10` measurement layer は `app/metrics.go`, `app/hitomi.go`, `app/viewer.go`, `app/ui.html` に実装済み。検出可能:
  - server side: `search_ms`, `record_ms`, `json_ms`, `x-search-timing`
  - fetch kind metrics: `calls`, `gate_wait_ms`, `network_ms`, `decode_ms`, `retry_sleep_ms`
  - UI side: `/api/search` time, `/api/search-card/{id}` time, `x-cache` distribution, `#perf-status`
- Previous P3 measurements showed the search bottleneck was visible:
  - cache-less `furry` first run: `/api/search 12008ms`, server `search=9444;record=2559;json=0`, cards `24061ms`, cache `cache=1 miss=24`.
  - cache-less `furry` second run on same server: `/api/search 9656ms`, server `search=7227;record=2425;json=0`, cards `26659ms`, cache `cache=26 miss=24`.
- Current `female:big_breasts` UI confirmation showed results `25` and `total 32882`, but current direct API re-run returned `502` because remote fetch failed, so no new `perf-status` number was captured in this confirmation.
- Therefore the measurement path detects bottlenecks, but the current session did not produce a new bottleneck measurement for `female:big_breasts` due to remote fetch failure. The previously recorded P3 measurements remain valid evidence.

### Search bottleneck three measurements

- Fresh server was started from:
  - `S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_three_measurements_fresh`
  - port `8735`
- Query used for meaningful actual server measurements: `furry`, `lang=all`, `page=1`, `limit=25`.
- Browser fetch in Playwright returned stale `total: 0` for the UI fetch path, so these three measurements were recorded directly from the server API with `curl.exe`.
- Run 1:
  - `X-Search-Timing: search=8488;record=959;json=0`
  - total `32881`
  - fetch metrics at end of run:
    - `index`: calls7, gate_wait6027ms, network968ms, decode0ms
    - `image`: calls1, gate_wait862ms, network146ms, decode340ms
    - `gg`: calls1, network453ms
    - `version`: calls1, network136ms
- Run 2:
  - `X-Search-Timing: search=7224;record=818;json=0`
  - total `32881`
  - incremental fetch metrics for this run:
    - `index`: +7 calls, +5170ms gate_wait, +958ms network, +0ms decode
    - `image`: +1 call, +863ms gate_wait, +136ms network, +86ms decode
    - `gg`: no additional call
- Run 3:
  - `X-Search-Timing: search=7215;record=769;json=0`
  - total `32881`
  - incremental fetch metrics for this run:
    - `index`: +7 calls, +5169ms gate_wait, +963ms network, +0ms decode
    - `image`: +1 call, +862ms gate_wait, +136ms network, +78ms decode
    - `gg`: no additional call

- Detected bottleneck:
  - Each `SearchHTML` request performs 7 `index` fetch calls.
  - The dominant cost is `index gate_wait_ms`, about `5160ms` per run in runs 2 and 3.
  - `record_ms` is around `800ms`, much smaller than the index gate wait.
  - The bottleneck is the searchindex rate gate/wait, not raw network transfer.
- UI card measurement layer is implemented, but UI card timings were not captured in this three-measurement pass because Playwright browser fetch returned stale responses.

### Canonical `hub.exe` bottleneck three measurements

- Corrected canonical server was started from:
  - binary: `S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe`
  - fresh root: `S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_canonical_fresh`
  - address: `http://127.0.0.1:8731/`
- Query used for meaningful actual server measurements: `furry`, `lang=all`, `page=1`, `limit=25`.
- Measurements were taken directly from the canonical server API with `curl.exe`.
- Run 1:
  - `X-Search-Timing: search=8500;record=892;json=0`
  - total `32881`
  - fetch metrics at end of run:
    - `index`: calls7, gate_wait6012ms, network984ms, decode0ms
    - `image`: calls1, gate_wait859ms, network150ms, decode348ms
    - `gg`: calls1, gate_wait0ms, network454ms, decode0ms
    - `version`: calls1, gate_wait0ms, network140ms, decode0ms
- Run 2:
  - `X-Search-Timing: search=7230;record=878;json=0`
  - total `32881`
  - incremental fetch metrics for this run:
    - `index`: +7 calls, +4969ms gate_wait, +1168ms network, +0ms decode
    - `image`: +1 call, +858ms gate_wait, +140ms network, +88ms decode
    - `gg`: no additional call
    - `version`: no additional call
- Run 3:
  - `X-Search-Timing: search=7220;record=757;json=0`
  - total `32881`
  - incremental fetch metrics for this run:
    - `index`: +7 calls, +5155ms gate_wait, +980ms network, +0ms decode
    - `image`: +1 call, +859ms gate_wait, +140ms network, +78ms decode
    - `gg`: no additional call
    - `version`: no additional call
- Detected bottleneck:
  - Each `SearchHTML` request performs `7` `index` fetch calls.
  - The dominant cost is `index gate_wait_ms`, about `4969ms` to `5155ms` per run.
  - `record_ms` is about `757ms` to `892ms`, much smaller than the index gate wait.
  - The bottleneck is the searchindex rate gate/wait, not raw network transfer.
  - This result was confirmed on the canonical `hub.exe`, so the earlier non-canonical `go run` measurement path is superseded as a measurement source.

## 外部コードレビュー（2026-11-02 再読）

- 外部レビュー指摘として記録する。実装・実行はしていない。
- 前回の指摘は現在のコードを読み直して更新した。前回の記述は一部古かった。
- 推定項目は推定として明記する。

### 前回指摘の状況

- 解消済み:
  - A-2 検索 term 正規化順序: `strings.Fields` で先に分割し、term ごとに正規化している。`female:big_breasts` は 1 term のまま nozomi path になる。
  - A-4 既読の巻き戻り: 通常移動では `fully_read` が保たれ、解除は `--clear-complete` のみ。
  - A-5 scan 記録: export 直下のファイルは NULL になり、既知 id では `galleries` 行を先に作り、DB エラーはログに出る。
  - A-9 新着追跡: `bookmark_seen` で保存検索ごとに管理している。
  - A-10 データ競合: `cacheMu` と `infoCacheMu` で保護されている。
  - SQLite: `WAL`, `busy_timeout`, `user_version` が入っている。

- 部分的に解消:
  - A-1 `gg.js` と index version の更新:
    - TTL と storage 404 時の `gg.js` 強制再取得が入った。
    - ただし index version は 404 時に再取得する経路がなく、更新後の最大 1 時間は壊れたまま。
  - A-6 VirusTotal の status:
    - 非 200 を cache しなくなり、TTL と日次カウントの DB 化が入った。
    - `ThreatName` はまだ誤りの可能性が高い。実 API レスポンスで確認が必要。
    - `scan_error` と `not_evaluated` の扱いにも問題がある。

- 未解消:
  - A-3 `orderbydirection:asc`: `SearchHTML` が `applyOrderForInitial` を呼び、さらに最後に `applySearchOrder` を呼ぶため、`asc` は 2 回 reverse されて元に戻ると指摘されている。order term は最初のループで state に入るため、初回適用時点で `asc` が有効とされている。
  - A-7 YARA 実行失敗:
    - 失敗が依然 `yara_error:...` という規則名として返り、`hit` になる。
    - `err != nil` の分岐後にある `ExitCode() > 1` の判定は到達しないコード。
    - `yr` の相対パスが cwd 依存のまま。
  - A-8 PDF: Page 辞書は `<<` が 3 つに対し `>>` が 2 つで、閉じが 1 つ足りない。テストは部分文字列検査のため通る。

### 新規問題

1. 強制再取得の無制限な繰り返し（`imageProxy`）
   - 404 のたびに `EnsureGG(force=true)` が走る。
   - 再取得に最小間隔も singleflight もなく、bucket が変わっていなくても再試行する。
   - 古い gallery で本当に窓外の `no_full` を 4 並列 prefetch すると、ページ数ぶん `gg.js` を取りに行く。
   - `gg.js` 取得は一般データ用の 1000ms ゲート `c.last` を共有し、検索やカード取得まで巻き込んで遅くなる。
   - 対策候補:
     - 最後の再取得から N 秒以内ならしない。
     - 再取得後に bucket が同じなら再試行しない。

2. VT 側の失敗で構造検査の `pass` が壊れる（`processFile`）
   - VT の結果が `pass`/`hit` 以外のとき、`scan_error` なら status が `scan_error` に、それ以外は `not_evaluated` に上書きされる。
   - 構造検査で `pass` だった cbz が、VT のレート制限、no_key、ネットワーク失敗だけで `scan_error` や `not_evaluated` になる。
   - `export` は `scan_error` を skip するため、一時的な VT 障害で書き出し対象から落ちる。
   - 404 の未登録ハッシュは日常的に起き、そのたびに `virustotal` の finding が 1 行増える。
   - 対策候補:
     - 構造検査の status は維持する。
     - VT の状態は別 finding か別カラムに分ける。

3. VT の `ThreatName`（推定）
   - 実装は `last_analysis_results` の `entry["name"]` を読んでいる。
   - VT v3 の実レスポンスでは、検出名は `result`、エンジン名は `engine_name` である可能性が高いと記憶されている。
   - `name` は無いはずなので、実運用では常に category の `malicious` になる可能性がある。
   - テスト fixture が `name` を使っているため、このテストでは検出できない。
   - 実 API レスポンスで確認が必要。

4. 既存 cbz の再スキャンで findings が重複する
   - `findings` に UNIQUE 制約も既存確認がない。
   - `scan` 再実行や `download` ごとの `processFile` で同じ行が増える可能性がある。

5. acquisition が新規でも 2 ページ目で上書き確認を出す
   - 1 ページ目で title フォルダができると、2 ページ目の `overwrite=false` リクエストはサーバー側で「フォルダが存在する」と判定されて `confirm` を返す。
   - UI は `acquisitionOverwrite=false` で始まるため、新規取得でも 2 ページ目で「既存の同名フォルダがあります」と確認が出る。
   - テストは 1 ページ分しか見ていない。
   - 対策候補:
     - フォルダ存在確認を 1 回だけ、開始時にする。
     - 同一 run 内の取得は確認済みとして扱う。

6. 同じ gallery を `openGallery` で開き直すと画像が取得されない可能性（推定・実機確認が必要）
   - `openGallery` は先頭で `backgroundSignal.abort()` を呼ぶ。
   - 開く gallery が現在と同じ id だと `backgroundGalleryId === current.id` のままで、`ensureBackgroundPrefetch` は reset せず、中断済みの `backgroundSignal.signal` で `runPriority` を呼ぶ可能性がある。
   - その場合、未キャッシュ画像は fetch が abort され、背景ワーカーも終了したままになる可能性がある。
   - ストリップのクリック、お気に入りリンク、同 gallery の hash 遷移で再現しうる経路。
   - 対策候補: `openGallery` で abort した場合に `backgroundGalleryId` も `null` に戻す。

7. 検索の言語指定が plain term に効かない
   - `termIDs` の plain term は `IndexIDs("galleries", term)` で `lang` を使わない。
   - UI の `lang` に `japanese` を入れても、`furry` は全言語の結果になる。
   - colon term では `lang` が効くため挙動が不一致。
   - hitomi 本家との parity が意図なら仕様どおりの可能性はあるが、UI 上は誤解を生む。

8. `check` の初回が全件「新着」になる
   - 保存直後の初回実行で、検索結果の全 id が new になる。
   - 数千〜数万件规模では、それぞれ SELECT 1 回と書き込み 2 回がトランザクションなしで走る可能性がある。
   - 対策候補:
     - 初回は baseline として seen に入れるだけにする。
     - トランザクションにまとめる。

### 非効率

実測根拠のある項目:

- 検索 1 回で index 取得 7 回、gate 待ちが約 5.2 秒。
- 純粋な通信は約 1 秒。
- `record_ms` が 770〜2560ms。
- 25 件の `recordDiscovered` が 1 件ずつ commit されている。
- カード 25 枚で 24〜26 秒。

改善案:

- 検索結果を `(query, lang)` 単位で TTL キャッシュする。
- B-tree の上位 node をキャッシュする。
- `recordDiscovered` を 1 トランザクションにまとめる。
- `PRAGMA synchronous=NORMAL` を WAL 前提で検討する。
- `searchCard` での record 書き込みも見直す。
- ltn の静的ファイルのゲート分離を検討する。ただし API 方針との兼ね合いが必要。
- `json_ms` は常に 0 であり、実測計測ではなく常に 0 である。実際の encode 時間を測っていない。

前回から変わっていない非効率:

- キャッシュの書き込みは非アトミックで、magic 検査もない。
- singleflight がない。
- キャッシュ容量の上限がない。
- `thumbProxy` が TN 失敗時に full 画像を取りに行く。
- `serveStoredImage` は全読みで、304 や Range に対応しない。
- `download` は既存ファイルを確認せず毎回再取得する。
- `gate`/retry の `time.Sleep` は ctx 非対応。
- objectURL を 2000 件超で全件 revoke し、表示中の画像も壊す可能性がある。
- ページ移動のたびに `POST /api/read` と `GET /api/lists` が走る。
- `renderStrip()` が毎回 50 枚を再構築する。
- 起動時に `restoreSearchState()` と `runSearch()` でカード取得が重複する。
- `scan` は毎回全ファイルを SHA256 する。
- `ScanPDFKeys` は 7 キーそれぞれで `string(data)` を作り直す。
- PDF の `/AA`, `/JS` は 3 バイトで誤検知源になる。
- `pdf.go` の `rawRGB` はページごとに全 RGB を保持する。
- `infoCache` は上限なしで増える。

### 設定・保守・セキュリティ

- `--config` フラグが無いまま、`usage-memo.md` の記述と食い違っている。
- `MinIntervalMS == 0` のとき `cfg.Hitomi` 全体が上書きされる。
- `MinIntervalMS` が 0 でなければ、`use_live_gg` を省略すると `false` になり、黙って snapshot に落ちる可能性がある。
- `Search`, `SearchQuery`, `nozomiURL`, `fileExt`, `imageExts`, `pageSource` は本番経路では未使用。
- POST 系に Host/Origin 検査がない。
- スキーマは `CREATE IF NOT EXISTS` のみで、列追加のマイグレーションはないと推定されている。古い DB で `recordFile` が失敗する可能性がある。
- `pragma` を `db.Exec` で 1 回だけ設定しており、接続が張り直されると失われる可能性がある。`modernc` の DSN の `_pragma=` 指定に寄せるほうが安全。

### 優先して直す順

1. `imageProxy` の `gg.js` 無制限再取得
2. VT 失敗で構造検査の `pass` が壊れる問題
3. 同じ gallery を `openGallery` で開き直すと画像が取得されない可能性
4. A-3 `asc` の二重 reverse
5. A-7 YARA 失敗が `hit` になる問題
6. 検索結果のキャッシュ化と `recordDiscovered` のトランザクション化

`openGallery` の問題は静的読解だけの推定であり、ストリップのクリックで再現するか実機確認が必要。

## 項目別 現在有効性（設定・保守・セキュリティ・UI 非効率）

- 判定は静的読解のみ。ファイル変更なし。

### 1. `--config` フラグ欠如と usage-memo 矛盾

- **現在有効性**: 有効（確定）
- **根拠**:
  - `app/main.go:490-513` の flag set に `config` がない。
  - `app/main.go:761-765` の `valueFlags` にもない。
  - `scan --root . --config config.json` は `flag provided but not defined: -config` で exit(2)。
  - `docs/ai/usage-memo.md:23` のように `--config` をサブコマンド前へ置くと、`findCommand`（`app/main.go:768-785`）が `config.json` をサブコマンドとして誤読する。
  - 設定は常に `dataDir/config.example.json` 固定（`app/main.go:519`）。
- **適切な対応**:
  - `usage-memo.md` から `--config` の記載を削除し、`--data-dir` フォルダ内の正典ファイルのみと明記する。
  - または `--config` flag を追加し、`valueFlags` に登録し、`findCommand` との互換を守る。
  - どちらにするかは方針選択が必要。

### 2. `MinIntervalMS == 0` 上書き / `use_live_gg` 省略時の snapshot

- **現在有効性**: 有効（確定）
- **根拠**:
  - `app/main.go:121-123` が `MinIntervalMS == 0` のとき `cfg.Hitomi` を `defaultHitomiConfig()` で全体置換する。明示した `user_agent`, `image_format`, `max_retries` も消失し得る。
  - `MinIntervalMS != 0` の場合、`ImageMinIntervalMS` 以外の per-field 補完がない（`app/main.go:124-126`）。
  - `UseLiveGG` は bool で JSON 省略時 `false`（`app/hitomi.go:23-31`）。
  - `defaultHitomiConfig()` の既定は true（`app/hitomi.go:51-54`）で、`docs/ai/usage-memo.md:115` の「既定 true」と食い違う。
  - `EnsureGG` は `false` のとき live fetch せず snapshot を使う（`app/gg.go:74-84`）。
- **適切な対応**:
  - JSON の bool を sentinel で区別し、省略時のみ default を適用する。
  - または `*bool` にして省略と false を区別する。

### 3. Schema `CREATE IF NOT EXISTS` のみ / 列追加 migration なし

- **現在有効性**: 有効（設計ギャップ確定）
- **根拠**:
  - `schema.sql:1-80` が全面 `CREATE IF NOT EXISTS`。
  - `app/main.go:175-195` の `ensureSchema` は `user_version` を `schemaVersion = 2`（`app/main.go:154`）へ上げるだけ。version 別 ALTER 経路がない。
  - `bookmark_seen`, `vt_rate_state` は古い DB でも作られる。
  - v2 後の列追加は skip され反映されない。例として `verifyMissing` の `missing_at_verify`（`app/main.go:274`）で列欠如によりエラーし得る。
- **適切な対応**:
  - `user_version` に基づく段階 migration を追加する。
  - または「正典最小構成＝現 `schema.sql`、古い DB は再生成」と運用明文化する。

### 4. pragma を `db.Exec` 1 回で設定する脆さ

- **現在有効性**: 実用上は単一接続固定で持続するが脆い
- **根拠**:
  - `openDB` は `foreign_keys`, `journal_mode=WAL`, `busy_timeout` を 1 回 `db.Exec` する（`app/main.go:156-173`）。
  - `journal_mode=WAL` はファイル側永続。
  - `foreign_keys`, `busy_timeout` は per-connection。`SetMaxOpenConns(1)`（`app/main.go:171`）で単一接続固定されている限り持続するが、pool 化や接続張り直で失われる。
- **適切な対応**:
  - `modernc` の DSN に `_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)` を寄せる。

### 5. POST の Host/Origin 検査なし

- **現在有効性**: 有効（確定）
- **根拠**:
  - `app/viewer.go:46-62` の handler に middleware がない。
  - `POST /api/read`, `/api/fav`, `/api/query`（`app/viewer.go:546-614`）、`POST /api/cache/acquire`（`app/acquisition.go:75-210`）は Content-Type, Host, Origin を見ていない。
  - bind が可変（`app/main.go:133-135`）。
  - `text/plain` simple POST や DNS rebinding 経由で read_state, favorite, queries, acquisition を書き換えられる。
- **適切な対応**:
  - Host を `127.0.0.1:<port>` / localhost 限定。
  - Origin が存在する場合は same-origin のみ許可。
  - Content-Type を JSON に限定する middleware。

### 6. UI / 非効率項目

- `objectURL` 2000 超で全件 revoke: 有効（`app/ui.html:219,782-784,823-833`）。表示中の参照も壊れる。対応: LRU、active/表示中除外。
- ページ移動ごとに `POST /api/read` と `GET /api/lists`: 有効（`app/ui.html:1060-1071,1094,1176-1186`）。debounce 決定記録は未実装。対応: debounce。
- `renderStrip()` が毎回 50 枚を再構築: 有効（`app/ui.html:1096-1125`）。対応: group/data 変化時のみ更新。
- 起動時に `restoreSearchState()` と `runSearch()` でカード取得重複: 有効（`app/ui.html:1289-1308,1349-1376,1630-1645`）。対応: hash 一致で skip、searchCard cache 再利用。
- `scan` が毎回 full SHA256: 有効（`app/main.go:361`, `app/scan.go:100-111`）。対応: size+mtime 不变で skip。
- `ScanPDFKeys` が 7 キーごとに `string(data)` を作る: 有効（`app/scan.go:152-168`）。対応: `bytes.Contains`。
- PDF `/AA`, `/JS` が 3 バイト substring で誤検知源: 有効（`app/scan.go:37,164`）。圧縮 stream 偶然列で誤検知し、圧縮 key は false negative になり得る。対応: object dictionary 範囲限定、または known-issues 正典化＋YARA 役割分担。
- `pdf.go` の `rawRGB` がページごとに全 RGB を保持: 有効（`app/pdf.go:47-58,112`）。対応: ページ単位 file 書き出し、JPEG DCTDecode 直接 embed。
- `infoCache` 上限なし: 有効（`app/viewer.go:29,42,616-631`）。mutex 済みで race は解消、容量のみ。対応: LRU/cap or SQLite 永続。
- cache 書き込み non-atomic: 有効（`app/viewer.go:445-448,477-492,489-492`, `app/acquisition.go:171`, `app/main.go:343`）。対応: temp+rename。
- cache 書き込み magic 検査なし: 有効（proxy path で `DetectMagic` 未適用、`app/scan.go:47-80`）。対応: 書き込み前 magic prefix check。
- singleflight なし: 有効（`app/viewer.go:457-506`, `app/viewer.go:423-455`, UI prefetch 4 並列）。対応: per-key singleflight。
- cache 容量上限なし: 有効（`app/viewer.go:39`）。対応: quota+LRU or 明文化。
- `thumbProxy` TN 失敗時 full fetch: 有効（`app/viewer.go:451-454`）。TN display fallback 禁止方針と server 挙動が衝突。対応: 失敗時 `no_full` 返却、方針確認要。
- `serveStoredImage` 304/Range なし・全読み: 有効（`app/viewer.go:177-184`）。対応: `http.ServeContent`, ETag。
- `download` 既存ファイル確認なし: 有効（`app/main.go:337-346`）。対応: 既存 skip（`--force`）。
- `gate`/retry `time.Sleep` ctx 非対応: 有効（`app/hitomi.go:71,114,133,147,165`）。対応: `select ctx.Done()`。
- `recordDiscovered` tx なし: 有効（`app/viewer.go:262-264`, `app/main.go:594-598`, `app/check.go:59-64`）。実測 `record_ms 770〜2560ms`。対応: 1 tx batch。

### 7. 本スコープ優先度

- P2:
  - 検索結果 cache
  - B-tree node cache
  - `recordDiscovered` tx
  - ctx-aware sleep
  - cache atomic + singleflight
  - 起動時重複カード fetch
  - `thumbProxy` full fetch
  - PDF `/AA`, `/JS` 誤検知
  - `use_live_gg` default
  - POST Host/Origin
- P3:
  - `--config` docs
  - pragma DSN
  - schema migration
  - dead code
  - objectURL LRU
  - renderStrip incremental
  - showLists debounce
  - download skip
  - 304
  - cache capacity
  - infoCache cap
  - rawRGB
  - SHA skip

## 項目別 現在有効性（VirusTotal）

- 静的読解と公式ドキュメント参照による評価。ファイル変更なし、実装なし。

### VT HTTP status / 非 200 キャッシュ

- **現在有効性**: 解消済み。前回の「非 200 を cache しない」は現コードで有効。
- **根拠**:
  - `defaultFetch` が HTTP status を返す（`app/vt.go:30-49`）。decode 失敗時も `resp.StatusCode` を返す。
  - `VTLookup` は非 200 を cache しない。401 は `auth_error`, 429 は `rate_limited`, その他は `not_evaluated`（`app/vt.go:105-114`）。
  - cache 書き込みは `pass`/`hit` のときだけ（`app/main.go:426-431`）。
  - `scanCacheTTL` は 30 日（`app/main.go:197`）。
  - VT 日次カウントは DB 化済み（`app/vt.go:74-82`, `app/vt.go:156-168`, `schema.sql:73-76`）。
- **対応**: 追加対応不要。残る問題は `stats` キーと未分析 handling。

### VT 失敗で構造検査 `pass` が壊れる

- **現在有効性**: 有効。重大度: 中〜高。
- **根拠**:
  - `app/main.go:439-451` で `result.Status == "scan_error"` のとき `status = "scan_error"`, `result.Status != "skipped"` のとき `status = "not_evaluated"` に上書きする。
  - 構造検査で `pass` だった cbz が、VT のレート制限、no_key、ネットワーク失敗、DB 書き込み失敗だけで `scan_error` / `not_evaluated` になる。
  - `export` と `pdf` は `hit` / `scan_error` を skip する（`app/export.go:127`, `app/pdf.go:98`）。一時的な VT 障害で書き出し対象から落ちる。
  - `usage-memo.md:199-202` の語彙定義では `scan_error` は「解読不能」だが、VT 障害まで `scan_error` になるのは意味と矛盾。
  - Python 参照実装は上書きしない（`src/scan.py:392-393`）。parity 乖離。
- **適切な対応**:
  - 構造検査 status を維持する。`files` に `structural_status` / `vt_status` を分離、または findings に VT 状態のみ記録する。
  - `scan_error` はファイル解読不能の意味に限定し、VT 障害は `api_error` など別語彙にする。
  - export skip 条件を構造 `scan_error` だけにする。
- **リスク**: 意味の再定義なしに修正すると、既存 DB 行と usage-memo 語彙の再整理が必要。

### `findings` 重複

- **現在有効性**: 有効。重大度: 中。
- **根拠**:
  - `schema.sql:40-47` の `findings` に UNIQUE 制約がない。`id INTEGER PRIMARY KEY` だけ。
  - `addFinding` は `INSERT INTO findings ... VALUES`（`INSERT OR IGNORE` ではなく、既存行確認なし）（`app/main.go:227-231`）。
  - `scan` 再実行で構造 findings が毎回増える（`app/main.go:373-378`）。
  - `download` ごとの `processFile` でも同じ検知が増える。
  - cached pass の `cache_hit` 行が毎回増える（`app/main.go:418-421`）。
  - VT status 行、特に 404 未登録ハッシュで毎回増える（`app/main.go:446-450`）。
- **適切な対応**:
  - `UNIQUE(sha256, rule, detail)` を追加し、`INSERT OR IGNORE` にする。
  - 既存 DB に重複がある場合、制約追加前に重複行 cleanup が必要。
  - `user_version` 3 の migration で既存表へ制約追加するか、新規 DB で schema.sql に UNIQUE を入れる。

### VT `stats` key — 追加問題（重大）

- **現在有効性**: 実 API では無効。重大度: 高。
- **根拠**:
  - `app/vt.go:116-123` は `attrs["stats"]` を読む。正しくは File object の `last_analysis_stats`。
  - `stats` は Analysis object のサブフィールド名であり、File object の属性として公式リファレンスに列挙されていない。
  - フォールバックがないため、実 API では `positives = 0` になり、status が `pass` として scan cache に 30 日永続化される。
  - `last_analysis_results` が無い、分析未完了、`analysis-in-progress` でも positives = 0 で pass になる。意味的には `not_evaluated` であるべき。
- **適切な対応**:
  - `last_analysis_stats` を読む。必要なら `stats` と `last_analysis_stats` の両方を読む。
  - analysis status / last analysis date を見て未分析を `not_evaluated` にする。
  - 誤 clean の 30 日固定を短縮または status 別管理にする。

### VT `ThreatName` field

- **現在有効性**: 実 API では無効。重大度: 高。
- **根拠**:
  - `app/vt.go:124-149` は `last_analysis_results` entry の `entry["name"]` を読み、空なら `category` にフォールバックする。
  - VT v3 File/Analysis object の entry field は `category`, `engine_name`, `engine_update`, `engine_version`, `method`, `result` と公式リファレンスに記載されている。`name` は存在しない。
  - 実運用では `entry["name"]` が空になり、`category` の `malicious` が ThreatName になる。検出名が DB に出ない。
- **適切な対応**:
  - `result` を優先し、`engine_name` を併記する。例: `engine_name: result`。
  - `category` は最終フォールバックにする。
  - テスト fixture を v3 実レスポンス形状へ修正する。
  - known-good / known-bad hash で最小 1 回の実 API 確認を行い、mock fixture に固定する。

### `processFile` トランザクション

- **現在有効性**: 有効。重大度: 中。
- **根拠**:
  - Go code に `BeginTx`, `BEGIN`, `Rollback` が grep 0 件。
  - `addFinding`（`app/main.go:227-231`）、`cacheStore`（`app/main.go:218-225`）、`recordScanFile` / `recordFile`（`app/main.go:233-260`）が autocommit。
  - findings committed 後に `recordScanFile` 失敗すると `files` 行なしで findings だけ残る（`app/main.go:455-457`）。
  - `scan_error` 早期 return も findings committed 後に file 行を書く順序（`app/main.go:380-387`）。
- **適切な対応**:
  - `processFile` を 1 トランザクション化する。findings, file row, cacheStore, vt_rate_state を同一 tx 内で決める。
  - `incrementVTUsage` は実 API 呼出し後の計上なので commit 外に残す意味を明確化する。実呼出しは戻せない。

### VT test fixture

- **現在有効性**: 有効。テストが実 API 不一致を検出できない。
- **根拠**:
  - `app/scan_test.go:143-145`, `app/scan_test.go:203` が `stats` を fixture に使っている。
  - `TestVTThreatNameUsesEngineResultName`（`app/scan_test.go:225-230`）は fixture に `name` を使っている。テスト名は engine result name をうたうが、実際の v3 field `result`/`engine_name` を検証していない。
  - `TestVTDailyQuotaIsStoredInDB`（`app/scan_test.go:240-261`）と `TestScanCacheTTLStalePassIsNotUsed`（`app/scan_test.go:263-278`）は TTL / DB 化を検証済み。
- **適切な対応**:
  - fixture を `data.attributes.last_analysis_stats`, `last_analysis_results.<engine>` の `category`/`engine_name`/`result` 形状へ置換。
  - 追加検証:
    - `stats` key が無くて `last_analysis_stats` があり、`result` あり → hit。
    - analysis in progress / results 欠如 → `not_evaluated`。
    - `last_analysis_stats` が正しく read される。
    - 実 API 1 回確認を fixtures に固定し、常時 external call はしない。

### VT 軽微項目

- 403 が VT の日次 quota 超過として一般に返る可能性があり、現行 switch は 401/429 のみで 403 は `not_evaluated` に混在する。実レスポンス確認が必要。
- per-minute gate は process 内 `RateState.window`（`app/vt.go:59-70`）のみ。跨プロセスでは共有されない。単一 process 運用では問題なし。
- `incrementVTUsage` は実 fetch 後に呼ばれる（`app/vt.go:98-104`）。非 200 でも計上されるが cache hit は計上しない。
- `app/main.go:442` と `app/main.go:445` の `result.Status == "skipped"` 分岐は `cfg.VirusTotal.Enabled` gate 通過後なので到達不可。

## 項目別 現在有効性（YARA / PDF / acquisition / openGallery）

- 静的読解による評価。ファイル変更なし。

### A-7 YARA 実行失敗が `hit` になる

- **現在有効性**: 有効（確定）
- **根拠**:
  - `app/vt.go:215-235` の `RunYara` は `cmd.Output()` が error を返すと `[]string{"yara_error:" + err.Error()}` を返す（`app/vt.go:227-229`）。
  - `app/main.go:390-401` は返された文字列を `addFinding(db, rel, sha, "yara", rule)` に使い、`len(findings) > 0` で `status = "hit"` にする。
  - `export` と `pdf` は `hit` / `scan_error` を skip する（`app/export.go:127`, `app/pdf.go:98`）。YARA 呼出し失敗だけで書き出し対象から落ちる。
  - `cmd.Output()` は非 0 終了コードで必ず error を返す。`err == nil` の場合 `ProcessState.ExitCode()` は 0 なので、`app/vt.go:231` の `ExitCode() > 1` は到達不可。
  - `cfg.Yr` が相対パスの場合、`exec.CommandContext` にそのまま渡される（`app/vt.go:225`）。規則ファイルは `rulesFile()`（`app/main.go:831-837`）で root/dataDir に解決されるが、`yr` だけが cwd 依存。
  - YARA-X の exit code 1 = match 説は成立しない。YARA-X v1.22.1 の `tools/VERSIONS.md:7-12` では `EXIT_ERROR = 1` は error 時のみ。
- **適切な対応**:
  - `RunYara` を `(rules []string, err error)` または `(rules, status)` に分離する。
  - 実行失敗は finding ではなく `scan_error` または warning として扱う。
  - `processFile` は YARA 失敗時に status を `scan_error` にし summary へ反映する。
  - `yr` は dataDir 基準または絶対 path にする。
  - Python 参照実装側も `returncode not in (0,1)` を error 扱いに揃える。

### A-8 PDF page dictionary 閉じ不足

- **現在有効性**: 有効（確定）
- **根拠**:
  - `app/pdf.go:140-143` の page dictionary は `<<` が 3、`>>` が 2 で、page dictionary の閉じ `>>` が不足。
  - xref offsets は object 単位で正しく書かれるため xref 自体は壊れない。影響は object 構文のみ。
  - `app/export_test.go:169-182` は部分文字列検査だけなので、この欠陥を検出できない。
- **適切な対応**:
  - page object を `/Contents N 0 R >>` で閉じ、辞書ネストの `>>` 対応を揃える。
  - page object body を抽出し `strings.Count(body, "<<") == strings.Count(body, ">>")` を確認するテストを追加する。
  - `go vet .`, `go test -count=1 .` で検証する。

### acquisition の新規 run でも 2 ページ目 confirm

- **現在有効性**: 有効（確定）
- **根拠**:
  - `app/acquisition.go:132-144` は `overwrite=false` のとき `os.Stat(titlePath)` が dir で `confirm` を返す。
  - `titlePath` は page 1 の `os.MkdirAll(filepath.Dir(dest), ...)`（`app/acquisition.go:167`）で作られる。同一 run の page 2 以降は run 自身が作成した folder に対して `confirm` を返す。
  - handler は stateless で run identity がない。
  - UI は `app/ui.html:1544` で `acquisitionOverwrite = false` に初期化し、`runAcquisition()`（`1568-1605`）で run 開始時に false にリセットし、page ごとに `overwrite: acquisitionOverwrite` を送る。`confirm` で `window.confirm(...)` を表示する。
  - 追加影響:
    - 承認すると `acquisitionOverwrite = true`（`app/ui.html:1592`）が run 全体に引き継がれ、残りの全ページで per-file overwrite 保護が効かなくなる。
    - 拒否すると folder が存在し続けるため、以降の各ページで confirm が毎回再発する。
  - テストは 1 request = 1 page のみ（`app/viewer_test.go:1910-2055`）。UI string test（`2057-2101`）だけ。
- **適切な対応**:
  - run 開始時に folder 存在確認を 1 回だけ行い、run 単位で上書き可否を確定する。
  - サーバーに `run_id` / `confirmed` を持たせ、同一 run 内は folder check を skip する。
  - または UI 側で run 開始時に folder check を行い、server は per-file 判定だけ残す。
  - 現行挙動を正典化するなら、UI 確認文言を「この run で作成した folder」と区別できる文言にする。

### openGallery で同 gallery 開き直し時画像取得失敗

- **現在有効性**: 静的読解では有効。実機確認が必要。
- **根拠**:
  - `app/ui.html:1132-1137` の `openGallery` は冒頭で `prefetchAbort.abort()` と `backgroundSignal.abort()` を行うが、`backgroundGalleryId` をリセットしない。
  - `backgroundGalleryId` は `resetBackgroundPrefetch`（`app/ui.html:956-993`）でのみ設定される。
  - `ensureBackgroundPrefetch`（`app/ui.html:995-1003`）は `backgroundGalleryId !== current.id` のときだけ reset する。同 gallery 開き直しでは条件 false であり、abort済み signal で `runPriority(displayed, signal)` を呼ぶ。
  - `runPriority`（`app/ui.html:942-954`）は `signal.aborted` で即 return し、未キャッシュ表示画像が fetch されない。
  - `startBackgroundWorkers`（`app/ui.html:1041-1058`）は `resetBackgroundPrefetch` から呼ばれ、worker は `signal.aborted` で終了したままになる。
  - `manualRetryFailedFullImages`（`app/ui.html:1010-1039`）も `backgroundSignal.signal` を使い、abort済み signal で fetch すると AbortError。この関数に per-item `catch` がないため rejection が外に出る。
  - 再現経路: strip click（`app/ui.html:1496-1501`）、`#results` card click（`1490-1495`）、favorites/unread hash navigation（`1182-1185`, `1613-1615`）で同 gallery 開き直しがあり得る。
- **適切な対応**:
  - `openGallery` で abort したら `backgroundGalleryId = null` または `backgroundSignal = null` にして、`ensureBackgroundPrefetch` が新 controller を作る。
  - `manualRetryFailedFullImages` に `catch` を追加し、aborted signal を error として記録せず、または明示的に新しい signal を使う。
  - 同 gallery 開き直しを cancel 不要な no-op として扱う設計も可。
- **検証**:
  - UI string test: `openGallery` に `backgroundGalleryId = null` または signal 再生成があることを assert。
  - Playwright 実機: cache-less root で同一 gallery 内 strip click、`/api/image` 呼出し継続、`fullDone` 進行、`naturalWidth > 0` 増加分を確認。`backgroundSignal.signal.aborted` が abort 済みまま残るか page.evaluate で確認。対照実験で別 gallery から戻ると回復する経路を確認。
  - `ui.html` は `//go:embed` のため exe 再ビルド必須。

## 項目別 現在有効性（search / gg / order / 検索非効率）

- 静的読解による評価。ファイル変更なし。

### A-1 `gg.js` / index version

- **現在有効性**: TTL と force refresh は実装済み。指摘は部分解消。ただし `imageProxy` の 404 経路で force refresh が無制限に繰り返す問題は有効。
- **根拠**:
  - `EnsureGG(ctx, force)` の force 判定は `!force && source == "live" && age < ggCacheTTL`（`app/gg.go:85`）。`force=true` は TTL を無条件で bypass する。
  - `imageProxy` は storage 404 で `EnsureGG(ctx, true)` を呼ぶ。live 確認時のみ 1 回 retry（`app/viewer.go:484-504`）。1 リクエスト内は 1 回だが、リクエスト跨ぎの限制がない。
  - `Client` に force 用最終取得時刻がない。`ggLoadedAt` は TTL 用だけ（`app/hitomi.go:47`）。最小間隔なし。
  - `EnsureGG` fetch 前に `cacheMu` を release するため concurrent force 呼出はそれぞれ fetch（`app/gg.go:92`）。singleflight なし。
  - `parseGG` 後の bucket/CaseSet を旧値と比較しない。bucket が変わっていなくても retry する（`app/viewer.go:486`）。
  - `EnsureGG` は `c.get` を使う。一般 1000ms 共有 `c.last`（`app/gg.go:91`, `app/hitomi.go:78-80`）。gate は `MinIntervalMS` と `ImageMinIntervalMS` の 2 本（`app/hitomi.go:56-84`）。
  - UI prefetch concurrency は 4（`app/ui.html:217`）。server 側に `no_full` 負キャッシュがない（`app/viewer.go:508-513`）。UI の `no_full` は display source 判定除外（`app/ui.html:336-338`）で `prefetchPageImage` の guard を通らず再 fetch（`app/ui.html:787-801`, `864-871`）。
  - index version の `ensureIndexVersion(field, force)` force は本番経路で未使用。呼出は `indexVersion` の `false`（`app/searchindex.go:156`, `IndexIDs` `app/searchindex.go:216`）のみ、`true` はテストのみ。`indexNode`/`indexData` の 404 に version 再取得経路がない（`app/searchindex.go:197-212`）。
- **適切な対応**:
  - `Client` に `ggForceAt` と `ggForceMinInterval` を追加し、force 要求は mutex 内で「最後の force から N 秒以内なら skip、cache を返す」。
  - `imageProxy` で `ggBefore := GG()` を保持し、refresh 後の `Bucket`/`CaseSet` が同一なら retry せず `writeNoFull` にする。
  - singleflight は `golang.org/x/sync` を `app/go.mod` に追加しない限り、手書き `map[string]*call` + `sync.Mutex` を `Client` に追加する。依存追加は単体静的バイナリ方針で判断が必要。
  - `IndexIDs` が 404 を検知して `ensureIndexVersion(field, true)` を 1 回だけ呼ぶ。同一 minimum-interval guard を共有する。
- **テスト影響**:
  - `TestEnsureGGUsesTTLAndForceRefresh`（`app/hitomi_test.go:118-123`）は force 常時再取得を期待している。最小間隔導入ならテスト更新が必要。
  - `TestViewerImageProxyRefreshesGGOnceOnStorage404`（`app/viewer_test.go:332-359`）は per-request 検証のみ。跨リクエスト throttle、singleflight、bucket guard のテストがない。追加が必要。

### A-3 `orderbydirection:asc` 二重 reverse

- **現在有効性**: 有効（確定）
- **根拠**:
  - `applyOrderForInitial`（`app/searchhtml.go:302-311`）を `app/searchhtml.go:245` で適用し、`applySearchOrder`（`app/searchhtml.go:149-158`）を `app/searchhtml.go:297` で再適用する。
  - `asc` は reverse 2 回で元順に戻る。
  - `intersectIDs`, `excludeIDs`, `unionIDs`（`app/searchhtml.go:97-137`）は順序保存なので、二重 reverse は結果に observable。
  - `or` のみの経路（`app/searchhtml.go:246-256`）は initial apply がなく、経路によって挙動がずれる。
  - `isOrderTerm`（`app/searchhtml.go:25-30`）と `applyOrderTerm` の `sortbydirection|orderbydirection` → `state.orderbydir = right`（`app/searchhtml.go:55-56`）。既定は `desc`（`app/searchhtml.go:191`）。
  - `app/searchhtml_test.go` に order/asc 系テストがない。
- **適切な対応**:
  - `app/searchhtml.go:245` の call と `applyOrderForInitial`（`app/searchhtml.go:302-311`）を削除し、末尾 `applySearchOrder` だけ残す。
  - hitomi は末尾適用であり、`or` 経路も覆盖されるため末尾だけ残す案が正しい。initial だけ残す案は `or` 経路で順序が失われて誤り。
  - 単一 term + `orderbydirection:asc` で reverse されること、`or` 経路でも適用されることを検証するテストを追加する。

### 検索結果 cache

- **現在有効性**: 有効
- **根拠**:
  - `SearchHTML` に cache がない（`app/searchhtml.go:182-300`）。
  - `searchPageSlice`（`app/searchhtml.go:160-177`, `app/searchhtml.go:299`）が末尾 slice。ページ移動で全 term 再 fetch。
  - UI の `SEARCH_STORAGE_KEY` に ids だけ保存（`app/ui.html:1378-1408`）。card metadata は `searchCardCache` Map のみ。
- **適切な対応**:
  - `(area, tag, lang)` 単位の TTL cache。nozomi 成功結果のみ cache する。full ids を保持し、page slice は cache から取る。
  - call 数削減は 1000ms 方針と冲突しない。UI の localStorage/Map 併用可。

### B-tree node cache

- **現在有効性**: 有効
- **根拠**:
  - `IndexIDs` は term ごとに root（addr 0）から降下し、node cache がない（`app/searchindex.go:215-253`）。
  - `indexNode` は hop ごとに fetch（`app/searchindex.go:197-204`）。version cache 済み（`app/searchindex.go:155-195`）。
- **適切な対応**:
  - `map[string]*bnode` cache、key `field|version|addr`、TTL。root/internal node を cache する。leaf data は 1 term 1 回なので cache を必須としない。既存 `cacheMu` を使うか新 mutex。

### `recordDiscovered`

- **現在有効性**: 有効
- **根拠**:
  - `app/viewer.go:262-264` が 25 件 loop で `recordDiscovered` を呼ぶ。
  - `recordDiscovered` 定義（`app/main.go:839-845`）は単独 `db.Exec`。
  - `db.SetMaxOpenConns(1)`（`app/main.go:171`）で単一接続固定。
  - `BeginTx`/`BEGIN` は app 全体 0 件。
  - `searchCard` でも単独 write（`app/viewer.go:296`）。
- **適切な対応**:
  - `BeginTx` + prepared statement で 1 commit に束ねる。
  - `searchCard` の単独 write も同じ tx 内にするか、既存行なら skip。

### `json_ms`

- **現在有効性**: 有効（確定）
- **根拠**:
  - `jsonStart := time.Now()` 直後に `time.Since(jsonStart)`（`app/viewer.go:266-267`）。
  - 実際の `jsonOut` encode は `app/viewer.go:270` / `659-661` で行われるが測っていない。
  - `app/viewer_test.go:694-702` は key 存在確認だけ。
- **適切な対応**:
  - `jsonOut` の前後で実 encode を測る。buffer encode してから write するか、フィールドを削除する。

### ltn static gate

- **現在有効性**: 有効（ただし設計方針制約あり）
- **根拠**:
  - 静的 ltn 呼出（version/index/nozomi）は `c.get` を使い、一般 1000ms 共有 `c.last`（`app/hitomi.go:78-80`）。
  - 静的用 relaxed gate は未実装。
- **適切な対応**:
  - 1000ms を一律にかける緩和は API 方針更新が必要。`docs/ai/decisions/2026-11-01-hitomi-client-endpoints.md` と `docs/ai/decisions/2026-11-01-image-rate-and-prefetch-concurrency.md` L5 が 1000ms 維持を規定している。
  - 無依存で安全な対応は call 数削減（search cache, node cache）と ctx-aware sleep。

### `record_ms`

- **現在有効性**: 有効（計測自体は正しい）
- **根拠**:
  - `app/viewer.go:261-265` は実測。canonical exe 記録で `757〜892ms`、過去 `2425〜2559ms`。
  - 瓶颈は index gate wait（約 5.2s）。`record_ms` は副次要。

### gate / retry sleep ctx

- **現在有効性**: 有効
- **根拠**:
  - `time.Sleep` に `ctx.Done()` 分岐がない（`app/hitomi.go:71,114,133,147,165`）。
- **適切な対応**:
  - `select ctx.Done()` を使う。

### `PRAGMA synchronous`

- **現在有効性**: 有効（設定なし）
- **根拠**:
  - `openDB` に `synchronous` 設定がない（`app/main.go:156-173`）。WAL/busy_timeout/user_version は設定済み。
- **適切な対応**:
  - WAL 前提で `synchronous=NORMAL` を検討する。

### 検索言語指定が plain term に効かない

- **現在有効性**: 有効。ただし hitomi parity は仕様どおりの可能性。
- **根拠**:
  - `termIDs` plain term は `IndexIDs("galleries", term)`（`app/searchhtml.go:94`）で `state.language` を使わない。
  - colon term は `state.language` を set（`app/searchhtml.go:87`）→ `SearchNozomiParts(state.area, state.tag, state.language)`（`app/searchhtml.go:92`）で効く。
  - UI は lang を送る（`app/ui.html:110`, `1378-1394`）。server は `lang` を `SearchHTML` に渡す（`app/viewer.go:255`）。既定 `all`（`app/searchhtml.go:191`）。
  - `app/searchhtml.go:179-180` のコメントは `language:` term のみと明示。`galleriesindex` は言語非依存。
- **適切な対応**:
  - 最小・挙動不変案: UI と `docs/ai/usage-memo.md` の分区規則（`docs/ai/usage-memo.md:132-136`）に「lang は colon term / `language:` term にのみ効く。plain term は全言語」と 1 行明記する。
  - 一貫性重視案: plain term で `state.language != "all"` のとき `SearchNozomiParts("", "index", lang)`（既存 `language:` term 相当）と intersection する。+1 fetch / +1 gate wait。方針判断が必要。既存 `TestSearchHTMLPlainTermUsesIndexExactTerm`（`lang="all"`）は影響なし。新テスト 1 件が必要。
  - DB lang filter は非推奨。`recordDiscovered` は `title, lang` を NULL で insert（`app/main.go:840-843`）なので言語情報がない。

## 項目別 統合済み優先度

1. `imageProxy` の `gg.js` 無制限 force refresh
   - 根拠: `EnsureGG(force)` が TTL bypass、最小間隔/singleflight/bucket guard なし、共有 1000ms gate。
2. VT `last_analysis_stats` key と未分析 handling
   - 根拠: 実 API で positives が 0 になり誤 `pass` が 30 日 cache される。重大。
3. VT `result`/`engine_name` を基準にした `ThreatName`
   - 根拠: `entry["name"]` は VT v3 に存在しない。
4. VT 失敗で構造 `pass` を上書き
   - 根拠: export skip に直結し、Python parity と usage-memo 語彙に矛盾。
5. `openGallery` の同 gallery 開き直しが abort済み signal を使う
   - 根拠: 静的には有効、実機確認が必要。
6. A-3 `asc` 二重 reverse
   - 根拠: 単一 call削除で修正可能、テスト追加で確認可能。
7. YARA 実行失敗が `hit` になる
   - 根拠: `RunYara` error を rule として返す。テスト・status 語彙が必要。
8. 検索結果 cache / B-tree node cache / `recordDiscovered` tx
   - 根拠: 実測 bottleneck に直結。方針変更なしで call 数削減可能。
9. acquisition run 内 folder confirm 反復
   - 根拠: page 1 で作成した folder を page 2 以降で別 run として再確認する。設計選択が必要。
10. settings/security/schema の静かな default 化・schema migration・POST Host/Origin
   - 根拠: `use_live_gg` silent snapshot, `--config` docs contradiction, pragma connection persistence, schema migration, Origin-less POST。

## checkpoint（2026-11-02）: acquisition run confirm once / settings/security / pragma

- acquisition UI で同一 run の folder confirm を `acquisitionRunFolderConfirmed` flag で 1 回だけ行うよう変更した。2 ページ目の confirm は run 内 flag で skip する。UI test に flag declaration、run reset、`window.confirm` より前 guard を追加した。
- `use_live_gg` は `HitomiConfig` の custom `UnmarshalJSON` で sentinel を追加し、JSON 省略時 true、明示 `false` は false を維持する。
- `loadConfig` の `Hitomi` が `MinIntervalMS == 0` 時に全体を上書きせず、per-field default を適用する。省略時の `use_live_gg` default true は custom unmarshal で保持する。
- `--config` flag を追加し、`findCommand` の value flags に登録した。既定は `<data-dir>/config.example.json` で、explicit path を尊重する。usage-memo とも一致。
- viewer POST middleware を追加した。POST は loopback Host、Origin があれば same-origin、Content-Type `application/json` を要求する。cross-origin origin/simple text/plain は reject。
- SQLite pragma を DSN `_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)` に寄せた。既存 `SetMaxOpenConns(1)` の single-connection path も残す。
- schema migration と `schema.sql` の findings unique index path は既存 test で一致確認済み。

### Verification

- `go vet .`（`S:\Documents\Kilo_Commons\hitomi-hub\app`）→ clean。
- `go test -count=1 -timeout 300s .`（`S:\Documents\Kilo_Commons\hitomi-hub\app`）→ `ok` `28.023s`。
- Canonical server direct baseline（new canonical exe, process `bgp_19b8ebf180013h63AbArKgLF9y`, port `8731`）:
  - `GET http://127.0.0.1:8731/api/search?query=furry&lang=all&page=1&limit=25` → `200`
  - `total = 32882`
  - `ids.length = 25`
  - `X-Search-Timing` present (`search=8508;record=832;json=0`)
  - `content-length = 1063`, not stale `84`
- Canonical exe rebuild from `app` succeeded: `go build -trimpath -ldflags "-s -w" -o ..\output\hitomi-hub\hub.exe .`
- Strict UI actual physical confirmation はこの session で pass として記録していない。Playwright/headless screenshot は通常 OS screen capture ではないため strict UI に不合格。

## 未解決事項の追加調査（2026-11-02）

### Full actual pixels

- Target gallery `4299126` は `no_full 37/37` で、actual full image bytes が取得できない。
- `scratch/p3_canonical_fresh/cache/images` に既存の full image cache があるが、target gallery の hash と一致しない。
- Existing cache sample:
  - `86f0dba81c357b9a8f1822162a97ba9007570082fc599314201619f02ad50ed3`
  - direct `/api/image/...?format=avif` は `200`, `x-cache: cache`
  - browser decoding confirmed `1295x1812`
- Pass criteria:
  - UI-openable gallery contains one existing decodable cache hash, or acquisition fetches full bytes into persistent cache;
  - display source is `cache`, `local`, or successful remote full fetch;
  - displayed `img.naturalWidth > 0`;
  - normal OS physical screen capture shows actual reader pixels.
- Controlled cache/test fixtures must be recorded as controlled confirmation, not production target pass.

### Reproducible `errorDone`

- Natural production `error` is not guaranteed for `no_full` because server intentionally returns `404 no_full` with `x-cache: no_full`.
- Controlled actual UI network failure was confirmed:
  - `/api/image/...` returned `500` without `x-cache`;
  - fresh UI state showed `errorDone > 0`;
  - actual UI click processed `error` entries;
  - after completion, `errorDone` reached `37`, `manualRetryingKeys` empty, `manualRetryRunning` false.
- Remediation decision:
  - Use controlled actual UI network failure for UI behavior confirmation.
  - Do not mark it as direct server natural-failure strict visual pass.
  - The corrected `error` or `no_full` retry target condition remains the implementation criteria.

### Race detector

- Current implementation protects `Client.gg`, `Client.versions`, and `viewer.infoCache` with mutexes.
- Existing concurrent handler/cache tests pass under `go test -count=1 .`.
- `go test -race -count=1 .` remains blocked in this environment because `CGO_ENABLED=0` and `gcc` is unavailable.
- Pass criteria:
  - local pass: mutex lock structure and concurrent handler/cache tests;
  - race proof: run `go test -race -count=1 .` on CI or a machine with cgo/gcc.

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
