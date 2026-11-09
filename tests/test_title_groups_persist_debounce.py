import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "tests" / "title_groups_persist_debounce_harness.mjs"

def test_title_groups_persist_debounce_and_equal_skip_mocked():
    if shutil.which("node") is None:
        pytest.fail("node toolchain is required for the mocked localStorage debounce test")

    result = subprocess.run(["node", str(HARNESS)], capture_output=True, text=True, cwd=str(ROOT))
    if result.returncode != 0:
        pytest.fail("mocked localStorage harness failed:\n" + result.stdout + result.stderr)

    required = [
        "titleGroups persist keeps the full map payload through Object.fromEntries and localStorage.setItem(TITLE_GROUPS_STORAGE_KEY",
        "debounce clears the previous timer before rescheduling (existing debounce helper style)",
        "readListsDebounceMs canonical meaning untouched: 250 for /api/lists only and the titleGroups persist debounce uses its own constant",
        "group decision condition and call points unchanged: condition intact, save called only inside the condition, block boundaries keep manual and scheduler storage-free",
        "cold page writes nothing before the window end",
        "75 schedules collapse to one pending timer (rescheduling clears the previous timer)",
        "cold page window end flushes exactly one write",
        "warm round trip with unchanged content is equality skipped at flush (0 writes)",
        "mixed warm page schedules 15 and skips 10 before the window end (condition classes preserved)",
        "load seeds the equality baseline from the stored raw string",
        "sync baseline writes 75 per page at n=10,000 (old behavior)",
        "re-measured n=10,000 warm page: debounced single flush (<100 ms) vs sync 75 writes cumulative (>300 ms)",
    ]
    out = result.stdout
    for label in required:
        if "PASS " + label not in out:
            pytest.fail("mocked localStorage test must fix: " + label + "\ngot:\n" + out)
