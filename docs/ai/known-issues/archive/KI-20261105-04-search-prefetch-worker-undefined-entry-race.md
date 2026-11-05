# search prefetch worker で undefined entry console error の race

**Status**: クローズ（2026-11-05 18:07 ユーザー判定・現行 HEAD に記載の race 経路なし）

## 症状

- `search.html` の search page worker で `TypeError: Cannot read properties of undefined (reading 'entry')` が console に出ることがある。
- worker loop は `pending.length` を check してから `await currentPriorityDone` で interrupt を待つ。
- await 中に sibling worker が pending を drain すると、`takeNextSearchCard()` が返した ID が `pending.splice(...)` 前に消える可能性があり、`visibleIds`/`pending` の update raceで undefined entry を参照する。

## 本項目への影響

- 本項目の実測で visual pass と reset pass に影響しなかった。
- 本項目の実装では修正しない（正典「非必須問題は known-issues に記録してタスクを先に進める」に従う）。

## 次の対応

- worker loop の pending snapshot を await 後に再確認し、`pending.indexOf(id) >= 0` を check してから `entry` を参照する形へ修正するか、`processCard()` の guard を race-safeにする。
- 修正には別の実装承認と test re-anchorが必要。

## クローズ判定（2026-11-05 18:07）

- 現行 HEAD の worker loop（`app/search.html` 486-492）は `takeNextSearchCard()` と `pending.splice()` の間に await がない同期ブロック。記載の race（await 中に sibling worker が pending を drain）は成立しない
- `currentPriorityDone` は search page から除去済みで、`app/viewer_test.go:2579,2632` が search.html での復活をテスト固定
- 記載経路は現行ソースに存在しない → 修復対象なし
