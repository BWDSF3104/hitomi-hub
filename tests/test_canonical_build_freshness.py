import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
APP_DIR = ROOT / "app"
UI_PATH = APP_DIR / "ui.html"
CANONICAL_EXE = ROOT / "output" / "hitomi-hub" / "hub.exe"
SCRATCH_BUILD_EXE = ROOT / "scratch" / "canonical_build_freshness" / "hub.exe"

def test_canonical_build_freshness_rule_exists():
    agents = ROOT / "AGENTS.md"
    if not agents.exists():
        pytest.fail("project AGENTS.md is missing")

    rules = agents.read_text(encoding="utf-8")
    if "tests/test_canonical_build_freshness.py" not in rules:
        pytest.fail("canonical build freshness test rule is not documented")

    if "go build -o S:\\Documents\\Kilo_Commons\\hitomi-hub\\output\\hitomi-hub\\hub.exe ." not in rules:
        pytest.fail("canonical build command is not documented")

    if "UI confirmation" not in rules:
        pytest.fail("UI confirmation procedure is not documented")

def test_canonical_executable_embeds_current_ui():
    if not CANONICAL_EXE.exists():
        pytest.skip("canonical executable is missing (output/ is not tracked in git)")

    if shutil.which("go") is None:
        pytest.fail("go toolchain is required for canonical build freshness")

    SCRATCH_BUILD_EXE.parent.mkdir(parents=True, exist_ok=True)
    subprocess.check_call(["go", "build", "-o", str(SCRATCH_BUILD_EXE), "."], cwd=str(APP_DIR))

    ui_bytes = UI_PATH.read_bytes()
    if not ui_bytes:
        pytest.fail("app/ui.html is empty")

    scratch_bytes = SCRATCH_BUILD_EXE.read_bytes()
    if ui_bytes not in scratch_bytes:
        pytest.fail("go build output does not embed current app/ui.html")

    canonical_bytes = CANONICAL_EXE.read_bytes()
    if ui_bytes not in canonical_bytes:
        pytest.fail("canonical executable is stale: current app/ui.html is not embedded in hub.exe")
