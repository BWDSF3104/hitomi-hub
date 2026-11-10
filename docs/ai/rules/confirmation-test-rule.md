# 本プロジェクトルール: 確認テスト系正典（単一ルール）

**Scope**: hitomi-hub

## 1. Canonical build / served confirmation

- UI 確認は freshly rebuilt canonical `hub.exe` が served する UI/network 応答に対して行う。`app/*.html` の source string テストだけでは不合格。
- `python -m pytest tests/ -v` が通ること（`tests/test_canonical_build_freshness.py` を含む）。
- UI 変更時は canonical server を stop → canonical build → 同じ exe から server を起動して served 画面を確認する。
- 正典 build command と server command の記録: `docs/ai/decisions/2026-11-02-canonical-latest-build-confirmation-rule.md`（AGENTS.md ポインタ化の記録: `docs/ai/decisions/2026-11-04-agents-md-rule-pointer-ization.md`）。

## 2. Cold 確認 hygiene

cold state を claim する UI/network 実測は、測定時点で次の3条件を満たすこと。

1. **Canonical build freshness**: 現行 `app` からの fresh canonical `hub.exe`（freshness test 合格）起動 server で測定
2. **Fresh server instance**: in-memory `infoCache` は server インスタンスごとに閉じる。同一 instance が他の tab・run に応答済みなら cold claims 無効（warm として記録、または server 再起動で再測定）
3. **Isolated origin**: 測定 tab は state のない isolated origin（clean profile・localStorage clear）。共有 localStorage・stale tab と同一 origin を共有しない

既知の汚染源: `restoreSearchState()` の起動時 fetch（`app/search.html`）が測定前に server instance の `infoCache` に membership を作りうる。再発時は network request 全件 + `cacheDistribution` + root disk cache を突合し、どの条件が破れていたか記録する。

## 3. 本プロジェクト固有の固定寸法契約

- 例 `--search-card-height: 130px`。画面確認サイズの基準は正典 rules §2、切り取りの確認も同条どおり。
