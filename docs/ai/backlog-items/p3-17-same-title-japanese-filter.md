# P3 #17 同名タイトル作品が複数ある際、言語 Japanese 以外を非表示

## 現在の実装状況

- 同名タイトル filter / hide logic は未実装。
- `app/searchhtml.go` の `SearchHTML()` は galleriesindex/token 共通部分から gallery ID 配列と total のみを返す。
- タイトル・言語は client の lazy card fetch で `/api/search-card/{id}` により取得する。
- `app/viewer.go` の `searchResult()` は `title`、`japanese_title`、`language` を返す。
- client cache `searchCardCache` は session 内 Map で card metadata を保持する。
- DB `galleries` に `title` と `lang` 列はあるが、`recordDiscovered()` は title/lang を NULL で挿入する。`upsertGallery()` は gallery meta 取得時にだけ埋める。
- localStorage search state は `{query, lang, page, ids, total}` のみを保存し、title/language を含まない。

## 実現可能性

- client 側増分 dedup として限定的に可能。
- 判定対象を「現在読み込んだ結果 + cache 済み card」に限定すれば、`searchCardCache`、`language`、`japanese_title` を基に同名グループ化できる。

## 制限事項

- 増分 dedup になる。未読み込みページの同名翻訳は、同じグループの Japanese entry をまだ見ていないため表示される。ページ間で挙動が非対称になる。
- total 表示と実表示件数の乖離。raw ids count と dedup 後の表示 count が意味を失う。
- 同名別作品の誤分组リスク。
- DB 主体案は `recordDiscovered` の title/lang NULL 制約に依存する。
- 全結果の title 揃えは hitomi rate gate `1000ms` で非現実的。nozomi index から title を全取得できるか要確認。

## 推奨実装案

- client 側増分グループ化:
  - `titleGroups` Map を追加。
  - key は正規化 `japanese_title || title`。
  - `language == 'japanese'` の entry を best entry として保持。
- `loadSearchCards()` の card render 直前、`renderSearchCard()` で group を参照。
- 同一 group に Japanese entry が存在する場合は non-Japanese card を hide、または collapsed display。
- `localStorage` に永続 map を保存。key は `SEARCH_STORAGE_KEY` と分離した別 key を使う。
- page change、reload、`hashchange` restore で group cache を再適用。

## その他実装案

- server 側 filter を小規模追加。`/api/search` の IDs に対し、DB で `galleries.lang` / `title` が埋まっている行だけ Japanese を優先 filter。
- `check` 経路で記録済み gallery の title/lang を活用する DB 主体案。
- UI フラグ「同名は日本語のみ」を opt-in とし、既定 off で既存 hitomi parity を壊さない。
- 全セット dedup をあきらめ、表示 card 上に「同名の日本語版: #id」anchor を出す低リスク案。

## 検証方法

- `app/searchhtml_test.go`、`app/hitomi_test.go` 様式で mock fetch を使用。
- `renderSearchCard` の group key、Japanese priority、anchor generation を string test で assert。
- External API calls are allowed for `hitomi-hub` tests by decision, but mock tests remain preferred for repeatable unit tests.
- `go test .`・`go vet .` を `app` で実行。
- cache-less runtime root で同名タイトル検索結果を使い、anchor、group display、reload 後の group cache 適用を確認。
- 例: `--title "kemo nee-san 10"`、既存 usage memo で 22 件、`4252697` を含む。

## 採用決定

全セット dedup は採用しない。低リスク案を採用する:

- client 側 `titleGroups` Map を保持し、正規化 `japanese_title || title` を key とする。
- `language == 'japanese'` の entry を best entry とする。
- card render 時に、同名の日本語版 best entry が既に表示済みまたは cache済みなら、non-Japanese card に `同名の日本語版: #id` anchor を出す。
- hide は既定でしない。必要なら collapsed marker として `同名の日本語版: #id` を使う。
- `localStorage` に group cache を保存し、page change、reload、hash restore で anchor を再適用する。
- full dedup は将来の別項目または opt-in option として扱う。