import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
APP_DIR = ROOT / "app"
UI_PATHS = [APP_DIR / "viewer.html", APP_DIR / "search.html"]
CANONICAL_EXE = ROOT / "output" / "hitomi-hub" / "hub.exe"
SCRATCH_BUILD_EXE = ROOT / "scratch" / "canonical_build_freshness" / "hub.exe"

SEPARATION_DECISION = ROOT / "docs" / "ai" / "decisions" / "2026-11-04-search-viewer-separation-approach-approved.md"
BUILD_COMMAND_DECISION = ROOT / "docs" / "ai" / "decisions" / "2026-11-02-canonical-latest-build-confirmation-rule.md"
CANONICAL_BUILD_COMMAND = "go build -o S:\\Documents\\Kilo_Commons\\hitomi-hub\\output\\hitomi-hub\\hub.exe ."


def test_canonical_build_freshness_rule_exists():
    agents = ROOT / "AGENTS.md"
    if not agents.exists():
        pytest.fail("project AGENTS.md is missing")

    if not SEPARATION_DECISION.exists():
        pytest.fail("canonical decision is missing: docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md")

    if not BUILD_COMMAND_DECISION.exists():
        pytest.fail("canonical decision is missing: docs/ai/decisions/2026-11-02-canonical-latest-build-confirmation-rule.md")

    # AGENTS.md 条はポインタのみ（正典: docs/ai/decisions/2026-11-04-agents-md-rule-pointer-ization.md）
    rules = agents.read_text(encoding="utf-8")
    if "docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md" not in rules:
        pytest.fail("AGENTS.md Canonical rule is not a pointer to docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md")

    # 条の実質内容（項目4の3点）は参照先 decision が持つ
    decision = SEPARATION_DECISION.read_text(encoding="utf-8")
    for needle in (
        "app/viewer.html",
        "app/search.html",
        "失敗メッセージには両ファイル名",
        "served 画面",
    ):
        if needle not in decision:
            pytest.fail(
                "canonical decision docs/ai/decisions/2026-11-04-search-viewer-separation-approach-approved.md "
                f"does not document rule content: {needle}"
            )

    # build command の正典記載は decision 側（docs/ai/usage-memo.md は照合対象外）
    build_rule = BUILD_COMMAND_DECISION.read_text(encoding="utf-8")
    if CANONICAL_BUILD_COMMAND not in build_rule:
        pytest.fail(
            "canonical build command is not documented in "
            "docs/ai/decisions/2026-11-02-canonical-latest-build-confirmation-rule.md"
        )

def test_canonical_executable_embeds_current_pages():
    if not CANONICAL_EXE.exists():
        pytest.skip("canonical executable is missing (output/ is not tracked in git)")

    if shutil.which("go") is None:
        pytest.fail("go toolchain is required for canonical build freshness")

    SCRATCH_BUILD_EXE.parent.mkdir(parents=True, exist_ok=True)
    subprocess.check_call(["go", "build", "-o", str(SCRATCH_BUILD_EXE), "."], cwd=str(APP_DIR))

    scratch_bytes = SCRATCH_BUILD_EXE.read_bytes()
    canonical_bytes = CANONICAL_EXE.read_bytes()

    for path in UI_PATHS:
        html_bytes = path.read_bytes()
        if not html_bytes:
            pytest.fail(f"app/{path.name} is empty")
        if html_bytes not in scratch_bytes:
            pytest.fail(f"go build output does not embed current app/{path.name}")
        if html_bytes not in canonical_bytes:
            pytest.fail(f"canonical executable is stale: current app/{path.name} is not embedded in hub.exe")
