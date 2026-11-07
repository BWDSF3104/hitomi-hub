import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "tests" / "r12_startup_single_run_harness.mjs"

def test_r12_startup_card_fetch_runs_once_per_page_load():
    if shutil.which("node") is None:
        pytest.fail("node toolchain is required for the R-12 startup single run harness")

    result = subprocess.run(["node", str(HARNESS)], capture_output=True, text=True, cwd=str(ROOT))
    if result.returncode != 0:
        pytest.fail("R-12 startup single run harness failed:\n" + result.stdout + result.stderr)

    required = [
        "searchGeneration starts at 0 for the page load",
        "runSearch advances the generation for its own card run",
        "page load runs restoreSearchState then runSearch",
        "restoreSearchState keeps rendering the saved page status",
        "generation guard is kept in the card run",
        "restoreSearchState renders placeholders only and does not call loadSearchCards",
        "page load with saved state runs the startup sequence without error",
        "page load with saved state fetches cards once per id (25 card requests, 1 run)",
        "page load with saved state fetches no card twice (no restore run)",
        "card fetch follows the /api/search response on page load (no pre-response restore prefetch)",
        "page load with saved state still processes and displays the same 25 cards",
        "page load without saved state runs the startup sequence without error",
        "page load without saved state fetches cards once per id (25 card requests, 1 run)",
        "page load without saved state has no restore run to duplicate",
        "page load without saved state keeps card fetch after the search response",
        "page load without saved state processes the same 25 cards",
    ]
    out = result.stdout
    for label in required:
        if "PASS " + label not in out:
            pytest.fail("R-12 startup single run test must fix: " + label + "\ngot:\n" + out)
