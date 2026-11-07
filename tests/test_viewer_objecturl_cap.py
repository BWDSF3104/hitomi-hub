import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "tests" / "viewer_objecturl_cap_harness.mjs"

def test_viewer_objecturl_cap_and_status_priority_behaviours():
    if shutil.which("node") is None:
        pytest.fail("node toolchain is required for the viewer objectURL cap harness")

    result = subprocess.run(["node", str(HARNESS)], capture_output=True, text=True, cwd=str(ROOT))
    if result.returncode != 0:
        pytest.fail("viewer objectURL cap harness failed:\n" + result.stdout + result.stderr)

    required = [
        # R-15 memory cap: default 4000, configurable through viewer.max_object_urls
        "objectURL cap default is the approved 4000",
        "the old fixed 2000 cap is removed",
        "the cap is passed to the page through the server config route",
        "the page applies the cap only when the server floor guard is satisfied",
        "the cap is applied at startup",
        "no notification UI for the cap (users are not informed)",
        "runtime cap starts at the approved default 4000",
        # memory eviction order: saved status first, LRU second; display protected
        "saved status is the first eviction basis, LRU is second",
        "evictFullObjectUrls runs the status-priority pass before the LRU pass",
        "current display and gallery keys stay protected in both passes",
        "the cap is enforced after eviction",
        "status-priority eviction removed the saved-status entry first, not the oldest",
        "the oldest cache-status entry is kept until LRU order reaches it",
        "the evicted objectURL is revoked",
        "without saved-status entries eviction is plain LRU order",
        "displayed saved-status key stays protected",
        "the LRU pass evicts the oldest unprotected key while the displayed key is protected",
        "size never exceeds the cap after eviction",
        "the configured cap (floor 1000) is respected",
        # whole-gallery prefetch kept (window shrink not adopted)
        "prefetch keeps the whole gallery queue by distance order (window shrink not adopted)",
    ]
    out = result.stdout
    for label in required:
        if "PASS " + label not in out:
            pytest.fail("viewer objectURL cap test must fix: " + label + "\ngot:\n" + out)
