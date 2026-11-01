# Backlog

完了項目は削除（実装・検証は `tasks/` と `decisions/` に記録）。

## P2

- #7 デバイス間同期 — 不要（ユーザー判断 2026-11-01）
- #8 AVIF の pdf embed（pure-Go decoder が無い。webp・png・jpeg は decoder あり）
- #9 `bookmark_queries.notify` の用途（現行実装では未使用）

## P3

- #10 検索速度改善の検証 — 検索が遅い場合のボトルネックを測定できるか確認する。候補: 外部 index/version/node/data 取得、nozomi fetch、JSON/HTML 生成、DB read_state/local 判定、UI 反映、キャッシュなし環境での再取得。現状は「ボトルネック測定 → 低速箇所の特定 → 改善案」が未実施。  
  参考: [p3-10-search-speed-measurement.md](backlog-items/p3-10-search-speed-measurement.md)  
  採用決定: [p3-implementation-decisions.md](decisions/2026-11-01-p3-implementation-decisions.md)
- #11 現在表示位置の簡単表示 — 検索結果ページなら「何分の何ページ」、ギャラリーなら「何ページ中の何ページ」を UI で明示する。現在 route/page 状態は内部で保持されているが、ユーザーへ短い表示として出ていない。ただし `#11` の採用範囲は gallery/viewer 側のみで、検索結果側は対象外。  
  参考: [p3-11-position-display.md](backlog-items/p3-11-position-display.md)  
  採用決定: [p3-implementation-decisions.md](decisions/2026-11-01-p3-implementation-decisions.md)
- #12 まとめダウンロード / フル画像キャッシュ移動 — 現在表示中の画像や該当 gallery をまとめて端末へダウンロードするボタンを追加できるか検討。ローカルで完結する想定なら、ダウンロードではなく、フル画像キャッシュを任意の場所へ移動・束ねる機能として扱う案。  
  現行フル画像キャッシュ管理: server 側永続キャッシュは `cache/images/<hash>.<format>`、thumb は `cache/images/thumbs/<hash>.<format>.<variant>`。client 側 `fullCache` は `galleryId|hash|format` をキーに `{ status, objectUrl }` を保持し、`activeObjectUrlKeys` が viewer 表示用 active objectURL を追跡する。`sourceStatusByEntry` / `sourceTypeByEntry` / `displayPassByEntry` は source truth と actual display truth を分離する。`MAX_FULL_CACHE_ENTRIES` は client objectURL 上限で、超過時に `revokeFullObjectUrls()` により objectURL を revoke し active keys を消す。server file cache は move 対象として明示されていない。  
  参考: [p3-12-cache-collect-download.md](backlog-items/p3-12-cache-collect-download.md)  
  採用決定: [p3-implementation-decisions.md](decisions/2026-11-01-p3-implementation-decisions.md)  
  更新決定: [p3-12-normalized-cache-acquisition.md](decisions/2026-11-01-p3-12-normalized-cache-acquisition.md)
- #13 検索結果の前後ページボタン — 検索結果一覧に previous / next page ボタンを付ける。ページ境界で route を維持し、UI だけで移動できるか確認。  
  参考: [p3-13-search-prev-next-buttons.md](backlog-items/p3-13-search-prev-next-buttons.md)  
採用決定: [p3-implementation-decisions.md](decisions/2026-11-01-p3-implementation-decisions.md)
- #14 フルスクリーン機能 — 本家と同様にブラウザ連動の全画面表示が可能か検証する。ブラウザ Fullscreen API を使う案。全画面時は `requestFullscreen()` / `exitFullscreen()`、UI 表示領域、ページ送り、Esc、フォールバック、表示モード分離を設計する。  
  参考: [p3-14-fullscreen.md](backlog-items/p3-14-fullscreen.md)  
採用決定: [p3-implementation-decisions.md](decisions/2026-11-01-p3-implementation-decisions.md)
- #15 シングル / 見開き / フルスクリーンのボタン化 — viewer の表示モードを single, spread, fullscreen としてボタン化し、従来通り keyboard operation も受け付ける。current page indicator、prev/next page、fullscreen state と状態表示を統合できるか検討。  
  参考: [p3-15-view-mode-buttons.md](backlog-items/p3-15-view-mode-buttons.md)  
採用決定: [p3-implementation-decisions.md](decisions/2026-11-01-p3-implementation-decisions.md)

- #16 タブタイトルを閲覧中の作品に  
  参考: [p3-16-tab-title-current-work.md](backlog-items/p3-16-tab-title-current-work.md)  
採用決定: [p3-implementation-decisions.md](decisions/2026-11-01-p3-implementation-decisions.md)
- #17 同名タイトル作品が複数あった際、言語Japanese以外を非表示にする。そのためには検索結果のキャッシュなども必要になるかもしれない。要検討。  
  参考: [p3-17-same-title-japanese-filter.md](backlog-items/p3-17-same-title-japanese-filter.md)  
採用決定: [p3-implementation-decisions.md](decisions/2026-11-01-p3-implementation-decisions.md)
- #18 見開きは右から左がデフォルト、また、切り替えボタンも実装  
  参考: [p3-18-spread-rtl-toggle.md](backlog-items/p3-18-spread-rtl-toggle.md)  
採用決定: [p3-implementation-decisions.md](decisions/2026-11-01-p3-implementation-decisions.md)





