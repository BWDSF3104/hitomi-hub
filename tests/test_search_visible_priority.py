import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "tests" / "search_visible_priority_harness.mjs"

def test_search_visible_priority_order_and_promotion():
    if shutil.which("node") is None:
        pytest.fail("node toolchain is required for the search visible priority harness")

    result = subprocess.run(["node", str(HARNESS)], capture_output=True, text=True, cwd=str(ROOT))
    if result.returncode != 0:
        pytest.fail("search visible priority harness failed:\n" + result.stdout + result.stderr)

    required = [
        "takeNextSearchCard picks the current display-area card first",
        "candidates near the display area come before the rest of the queue",
        "visibility comes from IntersectionObserver on the placeholder li",
        "observer root is the results scroll container",
        "worker pop uses priority selection takeNextSearchCard",
        "FIFO shift pop replaced by takeNextSearchCard",
        "no polling loop for the visibility judgement",
        "search generation guard and card selector kept",
        "per-card perf timing kept",
        "bounded workers kept",
        "worker concurrency constant stays 4",
        "restoreSearchState uses the same loadSearchCards path",
        "display-area cards are processed before ids-order-earlier cards outside the display area",
        "visible card 103 beats ids-order-earlier off-area cards 101 and 102",
        "one request per card keeps the total request count unchanged",
        "before any card is visible the ids-order pop keeps working",
        "card that enters the display area during processing is promoted ahead of the remaining queue",
        "the promoted card jumps ahead of the remaining ids-order queue",
        "promotion adds no extra request",
        "stale search generation is discarded by the generation guard",
    ]
    out = result.stdout
    for label in required:
        if "PASS " + label not in out:
            pytest.fail("search visible priority test must fix: " + label + "\ngot:\n" + out)
