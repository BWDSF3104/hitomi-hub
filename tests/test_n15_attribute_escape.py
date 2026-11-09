import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "tests" / "n15_attribute_escape_harness.mjs"

REQUIRED = [
    "before renders 3 strip img and 3 reader img nodes",
    "after renders the same node count (3 strip, 3 reader)",
    "before breaks the single-quote attribute boundary (injected onerror attribute appears)",
    "after keeps the attribute boundary (no injected attribute)",
    "before truncates data-hash to abc, after round-trips the full value",
    "after data-hash round-trips for every entry",
    "after data-key round-trips for every entry",
    "after attribute count per img is unchanged across entries (boundary preserved)",
    "entrySource bytes for a valid 64 hex hash are unchanged",
    "entrySource encodes ' to %27 (before left it raw in the path)",
    "path segment percent encoding round-trips",
    "after dataset compare resolves the payload entry key",
    "before dataset compare cannot resolve the payload entry key (value truncated)",
    "selector strings no longer embed remote values (dataset compare)",
    "literal pins data-hash=' and data-key=' are still present",
    "entrySource pins are unchanged",
    "search queries href uses the single-quote helper",
    "search renders exactly one href attribute per saved query",
    "search route round-trips for every query including the ones with '",
    "link meaning is unchanged for queries without '",
]

def test_n15_attribute_escape_keeps_attribute_boundary_and_link_meaning():
    if shutil.which("node") is None:
        pytest.fail("node toolchain is required for the attribute escape harness")

    result = subprocess.run(["node", str(HARNESS)], capture_output=True, text=True, cwd=str(ROOT))
    if result.returncode != 0:
        pytest.fail("attribute escape harness failed:\n" + result.stdout + result.stderr)

    out = result.stdout
    for label in REQUIRED:
        if "PASS " + label not in out:
            pytest.fail("attribute escape harness must fix: " + label + "\ngot:\n" + out)
