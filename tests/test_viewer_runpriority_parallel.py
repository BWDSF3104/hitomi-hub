import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "tests" / "viewer_runpriority_parallel_harness.mjs"

def test_viewer_runpriority_parallel_dispatch():
    if shutil.which("node") is None:
        pytest.fail("node toolchain is required for the viewer runPriority parallel harness")

    result = subprocess.run(["node", str(HARNESS)], capture_output=True, text=True, cwd=str(ROOT))
    if result.returncode != 0:
        pytest.fail("viewer runPriority parallel harness failed:\n" + result.stdout + result.stderr)

    required = [
        "dispatch order stays array order (iterate display array without sorting)",
        "display pair is dispatched in parallel (promise created per item, no await at dispatch)",
        "runPriority waits the dispatch set instead of serial await per item",
        "no serial await remains inside runPriority",
        "abort check kept at per-item dispatch start",
        "generation guard kept at per-item dispatch start",
        "viewer_test.go pinned substrings unchanged (runPriority header and await currentPriorityDone)",
        "render display order (array order plus RTL reversal) unchanged",
        "serial reference variant buildable from the same body for the before/after comparison",
        "serial reference still costs 2x network wall (before baseline reproduced)",
        "current code costs network plus one lane interval (cold pair wall halved)",
        "measured improvement: parallel wall is about half of serial wall",
        "parallel dispatch keeps the 60 ms image lane dispatch interval",
        "every parallel trial respects the image lane interval (no faster dispatch)",
        "dispatch order remains array order when per-file network times differ",
        "completion order can follow arrival order (agreed side-effect of parallel dispatch)",
    ]
    out = result.stdout
    for label in required:
        if "PASS " + label not in out:
            pytest.fail("runPriority parallel test must fix: " + label + "\ngot:\n" + out)
