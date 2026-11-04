# Canonical latest build confirmation rule

**Date**: 2026-11-02

## Decision

- UI confirmation must be performed against the served UI/network response from the freshly rebuilt canonical `hub.exe`, not against `app/ui.html` source string tests alone.
- `python -m pytest tests/ -v` must pass, including `tests/test_canonical_build_freshness.py`.
- `tests/test_canonical_build_freshness.py` builds from current `app`, verifies the scratch build embeds the exact `app/ui.html` bytes, and verifies canonical `output/hitomi-hub/hub.exe` also embeds those exact bytes.
- For UI changes, stop the canonical server, rebuild `output/hitomi-hub/hub.exe`, restart the canonical server from that exe, and confirm served UI from `http://127.0.0.1:8731/`.
- Canonical build command（正典置き場所。test re-anchor 2026-11-04 で追記・`docs/ai/decisions/2026-11-04-freshness-rule-test-re-anchor-to-pointer.md`）:
  ```text
  go build -o S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe .
  ```
- Canonical server command:
  ```text
  S:\Documents\Kilo_Commons\hitomi-hub\output\hitomi-hub\hub.exe view --data-dir S:\Documents\Kilo_Commons\hitomi-hub --root S:\Documents\Kilo_Commons\Common_Operations\scratch\p3_canonical_fresh --port 8731
  ```

## Reason

Go tests can pass while a stale `hub.exe` serves an old embedded UI. The running UI is defined by the compiled canonical executable, not by the source `ui.html` directly. Therefore a stale-binary UI confirmation is invalid.
