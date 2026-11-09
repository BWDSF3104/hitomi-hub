import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "tests" / "viewer_page_basis_harness.mjs"

def test_viewer_page_basis_files_unification():
    if shutil.which("node") is None:
        pytest.fail("node toolchain is required for the viewer page basis harness")

    result = subprocess.run(["node", str(HARNESS)], capture_output=True, text=True, cwd=str(ROOT))
    if result.returncode != 0:
        pytest.fail("viewer page basis harness failed:\n" + result.stdout + result.stderr)

    required = [
        # re-anchor: files-basis count and its execution on the 733697-shape sparse fixture
        "setGalleryMeta count anchor is files basis (const count = g.files.length)",
        "setGalleryMeta scene-ordinal count basis removed",
        "count executed from actual setGalleryMeta line is 210 for 733697-shape fixture (files 210, scenes 13)",
        # position display on the unified denominator
        "position display denominator is 210 at page 1 through actual renderPosition lines",
        "position display shows pair 32-33 over denominator 210 in double mode at scene start page 32",
        # final file reachable through the unified count
        "movePage on the unified count reaches final file page 210 and stops there (single mode)",
        "pagenum guard through actual condition accepts 210 and rejects 211",
        # read complete on the unified basis
        "read complete through actual helper lines is true at last file page 210",
        "read complete through actual helper lines is false at page 13 while final file is 210",
        # readerItems second consistency on the files basis
        "readerItems second is next file page 33 at sparse scene start page 32, not the next scene start page 61",
        "readerItems second is next file page 63 at mid-scene page 62",
        # canonical strip scene badge display kept on the files-basis items, group resolves for the final file page
        "strip items files-basis (210) with scene badge kept: badge 30 at scene start page 2, badge 0 at mid-scene page 33",
        "stripGroupPage at final file page 210 resolves group 5 within total 5 and the final file card is rendered (strip non-empty)",
        # unchanged behavior regressions: 1:1 + sentinel and no-scene galleries
        "1:1 + sentinel fixture unchanged: count 3, second at page 1 is next file page 2",
        "no-scene gallery unchanged: count 3, second at page 1 is next file page 2",
    ]
    out = result.stdout
    for label in required:
        if "PASS " + label not in out:
            pytest.fail("viewer page basis test must fix: " + label + "\ngot:\n" + out)
