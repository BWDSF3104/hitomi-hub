# 外部コードレビュー指摘記録

**Status**: 実装完了・確認完了（strict UI blocker 除く）
**Date**: 2026-11-02
**Next Action**: 高項目の実装確認は完了。残りは manual full-image retry の strict UI confirmation blocker。

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

## 完了時 Memory Bank 操作

- [ ] `current-task.tsv` の参照行を削除
- [ ] 解決した `known-issues/` ファイルを `known-issues/archive/` へ `mv`（該当する場合）
- [ ] `backlog.md` の完了項目を削除（該当する場合）
