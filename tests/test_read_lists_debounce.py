import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "tests" / "read_lists_debounce_harness.mjs"

def test_read_lists_debounce_mocked_fetch_collapses_to_single_call():
    if shutil.which("node") is None:
        pytest.fail("node toolchain is required for the mocked fetch debounce test")

    result = subprocess.run(["node", str(HARNESS)], capture_output=True, text=True, cwd=str(ROOT))
    if result.returncode != 0:
        pytest.fail("mocked fetch harness failed:\n" + result.stdout + result.stderr)

    required = [
        "read/lists debounce uses the 250ms delay constant",
        "debounce helper exists for the lists display",
        "read debounce clears the previous timer before rescheduling",
        "consecutive page movements coalesce to one debounced read post",
        "consecutive page movements coalesce to one debounced lists fetch",
        "debounced post targets the latest gallery and page",
        "duplicate post for the same gallery and page is skipped",
        "post runs again when the page changes",
        "stale show generation does not schedule a debounced post",
        "showLists waits for the debounce window before fetching",
        "showLists shares one in-flight lists fetch per window",
        "concurrent showLists calls inside one window fetch once",
        "full image display path unchanged: no-store fetch, objectURL display, x-cache judgement, TN display fallback disabled",
    ]
    out = result.stdout
    for label in required:
        if "PASS " + label not in out:
            pytest.fail("mocked fetch test must fix: " + label + "\ngot:\n" + out)
