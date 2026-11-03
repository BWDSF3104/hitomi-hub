# 2026-11-03 判断推奨のユーザー承認

`assess-20261103-1220` の残項目で、D-02以外の判断項目は次の推奨で承認。

- B-06: Go `app/vt.go` の空 `last_analysis_results` 判定をPythonへ揃える。keyの有無ではなく「empty results + positives 0」で `not_evaluated`。VT cache TTLの正典は30日、negativeは2時間。記録の「60日」は誤り。
- B-07: Python参照実装のYARA error処理をGo表現へ揃える。構造passの `scan_status` を `scan_error`へ上書きせず `not_evaluated`を維持。YARA失敗は `scan_source` の `structural+yara_error` でretry用表現。VT gate副作用は現状許容（YARA errorファイルがVT gateに入るのはdecision 2026-11-02 #3と整合）。
- C-10: 本番gate interval 1000msは変えない。テスト専用gate interval injectで `go test` のsleep短縮のみ評価・実施。
- D-06: docs側をfaithful parse基準へ修正。固定polarity主張、TN display fallback古記述、check説明、README UI記述を正典決定へ寄せる。README具体行は実物確認後に修正。
- A-04/D-09/D-10: hitomi正典準拠確認後に判定。確認前はアプリ挙動を勝手に変更しない。準拠確認後はhitomi挙動へ寄せる。
- D-01: `.gitignore` に `*.db-wal`、`*.db-shm`、`Galleries/`、`app/app.exe` を追加。`app/docs/`は内容確認後にignore/commit判断。
- D-02: `config.json` を非追跡の正典config pathとし、`config.example.json` はkeyなしで追跡維持。fallbackとenv `VIRUSTOTAL_API_KEY`を維持。

承認は実装変更の実施指示ではなく、方針として確定。実装は実行計画の承認範囲と合格条件に従う。
