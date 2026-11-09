import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "tests" / "viewer_prefetch_race_harness.mjs"

def test_viewer_prefetch_worker_race_guard():
    if shutil.which("node") is None:
        pytest.fail("node toolchain is required for the viewer prefetch race harness")

    result = subprocess.run(["node", str(HARNESS)], capture_output=True, text=True, cwd=str(ROOT))
    if result.returncode != 0:
        pytest.fail("viewer prefetch race harness failed:\n" + result.stdout + result.stderr)

    required = [
        # guard anchor in the worker (KI-20261107-01 fix)
        "worker keeps prefetchQueue.shift() after the await window",
        "guard anchor: single if (!item) return; in worker",
        "guard position: immediately after shift and before try",
        "worker catch keeps the canonical error handling call",
        # deterministic race reproduction (microtask order)
        "unguarded variant buildable from worker body for race reproduction",
        "race window exercised deterministically: unguarded variant leaks 1 TypeError reading 'entry' before any count with worker normal 1 exception 1",
        "guard version race: no exception, both workers end normally, undefined never reaches prefetch or counts",
        # error path semantics unchanged by the guard
        "error path identical with guard: defined item error counted once and worker continues to normal end",
    ]
    out = result.stdout
    for label in required:
        if "PASS " + label not in out:
            pytest.fail("viewer prefetch race test must fix: " + label + "\ngot:\n" + out)
