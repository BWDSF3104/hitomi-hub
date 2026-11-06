import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "tests" / "search_prefetch_harness.mjs"

def test_search_prefetch_scheduler_fixed_behaviours():
    if shutil.which("node") is None:
        pytest.fail("node toolchain is required for the search prefetch harness")

    result = subprocess.run(["node", str(HARNESS)], capture_output=True, text=True, cwd=str(ROOT))
    if result.returncode != 0:
        pytest.fail("search prefetch harness failed:\n" + result.stdout + result.stderr)

    required = [
        # fixed constants and marker
        "scheduler keeps the fixed constants idle 800ms, concurrency 4, cooldown 5 minutes",
        "prefetch ahead page count is variableized as PREFETCH_AHEAD_PAGES",
        "prefetch requests carry the background marker as a query param",
        "prefetch requests go through the existing api() helper with the marker",
        "api stays no-store and exposes the response status for the 429/5xx stop rule",
        # (a) current page completed plus idle elapses starts the next-page prefetch
        "runSearch arms the idle timer after the current page cards complete",
        "idle wait uses the 800ms timer constant, not an immediate start",
        "no prefetch request before the idle window elapses",
        "first prefetch request is the next page id list in the current runSearch query format",
        "one meta request per next-page id and one TN cover request per card",
        "covers are warmed after the meta phase",
        "TN cover warm uses the current cover URL format with the marker",
        "prefetch runs exactly four workers in parallel",
        # (b) interrupt conditions stop the next request
        "query input interrupts the prefetch",
        "opening a gallery from a card interrupts the prefetch",
        "a hidden tab interrupts the prefetch",
        "new search generation stops the prefetch before the next request",
        "interrupted prefetch issues no cover requests after the stop",
        "query input interrupt stops the prefetch before the next request",
        "interrupted run stops at the request unit without re-issuing",
        # (c) 429/5xx stops everything and the cooldown blocks resume
        "429 sets a 5 minute cooldown in memory",
        "the failed prefetch request is not retried from the UI side",
        "no prefetch resumes during the cooldown",
        "prefetch may resume after the cooldown expires",
        # (d) the target never exceeds one target page
        "prefetch does not start when there is no target page",
        "the prefetch targets the configured ahead-page offset only, never beyond it",
        "one prefetch run covers at most one target page worth",
        # exclusions and Phase 1 harmony
        "saved-query startup does not trigger the idle prefetch",
        "the search form re-enables the idle prefetch",
        "no polling loop or resident periodic work for the prefetch",
        "prefetch failures are not rendered on screen",
        "prefetch targets meta and TN covers only, never full images",
        "cooldown is memory-only, never persisted",
        "prefetch is bound to the existing searchGeneration concept",
        "prefetch runs bounded workers at concurrency 4",
        "Phase 1 visible priority and foreground concurrency 8 are applied",
        "the interrupt condition is checked per completed request",
        "next button hover or focus starts the prefetch immediately",
    ]
    out = result.stdout
    for label in required:
        if "PASS " + label not in out:
            pytest.fail("search prefetch test must fix: " + label + "\ngot:\n" + out)
