# P3 #12 まとめダウンロード / フル画像キャッシュ移動

## 現在の実装状況

- Server persistent cache is under runtime root `cache/images/`.
- Full cache path: `cache/images/<hash>.<format>`.
- Thumb cache path: `cache/images/thumbs/<hash>.<format>.<variant>`.
- Cache files do not include gallery ID or page order in filename.
- Page order exists in DB `pageorder` and `exportPages()`.
- Downloaded gallery files are under `Galleries/<id>/<hash>.<format>`.
- CLI has download/export functions, but UI has no download/export endpoint.
- `fullCache` is client memory only, not disk.

## 実現可能性

- Server-side cache collect into gallery directory is possible.
- Browser arbitrary-path download is not possible through the app because the app cannot write outside its allowed folder.
- If "端末へ" means moving within runtime root, server-side collect/bundle is reasonable.
- If bundle is required, export CBZ over HTTP can produce one file inside runtime root.

## 制限事項

- App must keep its own writes inside its own folder.
- Cache bytes may not have passed `processFile()` structural evaluation. If collecting cache, status must be recorded after collection.
- Moving cache outside runtime root breaks DB `files.path`, `verifyMissing()`, `local` status, and external removal semantics.
- Browser cannot move client objectURL cache to disk.
- Uncached fetch still requires hitomi rate gate `250 ms` per image.
- Progress and abort are required for large collections.

## 推奨実装案

- Treat as cache acquisition, not arbitrary browser download.
- Add server-side normalized acquisition endpoint.
  - Suggested endpoint: `POST /api/cache/acquire`
- Request body:
  - `gallery_id`
  - `page_entries` or `hashes`
  - `overwrite` flag, only true after explicit UI confirmation
- Destination:
  - preconfigured acquisition folder inside app-writable root
  - create gallery title folder
  - copy cache files as `page_number.ext`
- Page number filename:
  - use `pageorder` order when available
  - fallback to `current.files` display order
  - recommended padded form: `001.jpg`, `002.png`
- Existing title folder handling:
  - UI must show overwrite confirmation before writing.
  - Server should refuse overwrite unless `overwrite=true` is explicitly sent after UI confirmation.
- Source cache must not be moved.
  - If cache file exists, copy from `cache/images/<hash>.<format>`.
  - If cache file is missing, either skip and report missing cache, or fetch through normal image rate-gated acquisition, depending on mode.
- UI button should be named acquisition, not download.
- UI must show progress, abort, overwrite confirmation, and per-page result.

## その他実装案

- Client-side sequential `a[download]` for each cached objectURL. It cannot choose arbitrary folder and cannot bundle.
- CLI-only command called from UI button.
- Cache manifest inside root to preserve gallery/page order for cache-only entries.

## 検証方法

- Go tests are allowed to call external APIs in `hitomi-hub` by decision, but mock tests remain preferred for repeatable unit tests.
- Required tests:
  - acquisition copies cache file into preconfigured title folder
  - cache source file remains in `cache/images/`
  - page filename uses page number, not hash
  - existing title folder requires explicit overwrite confirmation
  - rejects path outside preconfigured writable root
- UI string test for acquisition button, overwrite confirmation, progress, abort, and handler wiring.
- Screen verify with cache-warm gallery and cache-less root.
- Confirm acquisition does not trigger unnecessary network calls when cache exists.

## 採用決定

Recommended implementation is adopted, but updated by user request on 2026-11-01.

Adopted form:

- server-side normalized cache acquisition, not browser download
- copy cache files into preconfigured acquisition folder
- create gallery title folder inside that folder
- copy as page-number filenames, such as `001.jpg`
- existing title folder requires explicit overwrite confirmation
- cache itself is not moved
- UI should show acquisition, not download

Decision: [2026-11-01-p3-12-normalized-cache-acquisition.md](../decisions/2026-11-01-p3-12-normalized-cache-acquisition.md)