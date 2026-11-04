import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "tests" / "strip_rebuild_dom_harness.mjs"

def test_strip_rebuild_mocked_dom_page_move_keeps_img_nodes():
    if shutil.which("node") is None:
        pytest.fail("node toolchain is required for the mocked DOM strip rebuild test")

    result = subprocess.run(["node", str(HARNESS)], capture_output=True, text=True, cwd=str(ROOT))
    if result.returncode != 0:
        pytest.fail("mocked DOM harness failed:\n" + result.stdout + result.stderr)

    required = [
        "gallery open does full rebuild once",
        "page movement does not regenerate strip img nodes",
        "strip child count unchanged on page movement",
        "strip img nodes are the same objects (not regenerated)",
        "active class reflects page movement on existing nodes",
        "second page movement still highlight-only",
        "openGallery resets strip rebuild state on open",
        "openGallery does not rebuild the strip again after showPage",
        "showPage updates strip via refreshStrip only",
        "renderStrip records the rendered gallery and group window",
        "next gallery open does full rebuild exactly once",
        "page movement after open stays highlight-only",
    ]
    out = result.stdout
    for label in required:
        if "PASS " + label not in out:
            pytest.fail("mocked DOM test must fix: " + label + "\ngot:\n" + out)

def test_strip_rebuild_harness_passes_after_crlf_checkout(tmp_path):
    if shutil.which("node") is None:
        pytest.fail("node toolchain is required for the CRLF checkout regression test")

    harness_src = HARNESS.read_text(encoding="utf-8")
    ui_src = (ROOT / "app" / "ui.html").read_text(encoding="utf-8")

    tests_dir = tmp_path / "tests"
    app_dir = tmp_path / "app"
    tests_dir.mkdir()
    app_dir.mkdir()
    (tests_dir / HARNESS.name).write_text(harness_src.replace("\n", "\r\n"), encoding="utf-8", newline="")
    (app_dir / "ui.html").write_text(ui_src.replace("\n", "\r\n"), encoding="utf-8", newline="")

    result = subprocess.run(
        ["node", str(tests_dir / HARNESS.name)],
        capture_output=True, text=True, cwd=str(tmp_path),
    )
    if result.returncode != 0:
        pytest.fail("harness must pass on CRLF content after line ending normalization:\n" + result.stdout + result.stderr)

    for label in [
        "TN display source rule unchanged",
        "TN display fallback policy text unchanged",
    ]:
        if "PASS " + label not in result.stdout:
            pytest.fail("CRLF run must still verify " + label + ":\ngot:\n" + result.stdout)
