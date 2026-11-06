import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "tests" / "search_manual_jp_harness.mjs"
EXISTING_HARNESS = ROOT / "tests" / "search_prefetch_harness.mjs"

def test_manual_japanese_search_button_behaviours():
    if shutil.which("node") is None:
        pytest.fail("node toolchain is required for the manual Japanese search harness")

    result = subprocess.run(["node", str(HARNESS)], capture_output=True, text=True, cwd=str(ROOT))
    if result.returncode != 0:
        pytest.fail("manual Japanese search harness failed:\n" + result.stdout + result.stderr)

    required = [
        # existing labels must remain and the manual helper must stay outside the scheduler block
        "existing prefetch constants are kept at 2 / 4 / 800 / 300000 and the manual helper stays outside the scheduler block",
        # render rule: inline on the existing row, no new row
        "manual button renders inline on the existing .meta row without a new row",
        "manual path never touches saved query storage, TN requests, or innerHTML",
        "manual handler is wired through the results click handler",
        "duplicate clicks on the same normalized key are guarded by the in-flight map",
        # query builder
        "manual term helper prefers japanese_title and mirrors lowercase and underscore to space",
        "query builder is the canonical exact term plus language condition with lang=all and limit=1, not lang=japanese only",
        # request path
        "manual path requests only /api/search and /api/search-card for the clicked card",
        "manual path issues no TN request",
        "manual path does not change the saved query",
        # result application
        "found state replaces the button with the link on the clicked row only",
        "the title group is applied only for a Japanese result card without error",
        # guards and states
        "stale search generation does not update the clicked card anchor",
        "busy state disables the button while the foreground fetch is in flight",
        "duplicate click on the same normalized key is stopped by the in-flight map with busy state",
        "colon title click is an unsupported message without fetch",
        "known Japanese group click replaces the button with the link without fetch",
        "manual click respects the prefetch cooldown before fetching",
        "manual fetch proceeds after the cooldown expires",
        "error state shows the message on the same row and releases the in-flight key",
    ]
    out = result.stdout
    for label in required:
        if "PASS " + label not in out:
            pytest.fail("manual Japanese search test must fix: " + label + "\ngot:\n" + out)

def test_existing_search_prefetch_labels_are_preserved():
    if shutil.which("node") is None:
        pytest.fail("node toolchain is required for the existing search prefetch harness")

    result = subprocess.run(["node", str(EXISTING_HARNESS)], capture_output=True, text=True, cwd=str(ROOT))
    if result.returncode != 0:
        pytest.fail("existing search prefetch harness failed:\n" + result.stdout + result.stderr)

    out = result.stdout
    for label in [
        "scheduler keeps the fixed constants idle 800ms, concurrency 4, cooldown 5 minutes",
        "prefetch ahead page count is fixed at the p95 measured recommendation 2",
        "success patch helper is outside schedulerBlock and uses appendChild not innerHTML",
        "prefetch targets meta and TN covers only, never full images",
    ]:
        if "PASS " + label not in out:
            pytest.fail("existing search prefetch label must be preserved: " + label + "\ngot:\n" + out)
