import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
HARNESS = ROOT / "tests" / "n19_spread_button_label_harness.mjs"

REQUIRED = [
    "startup assignment renders Japanese label for default rtl (no raw label from startup block)",
    "internal value stays raw rtl after startup",
    "startup assignment renders Japanese label for stored ltr",
    "internal value stays raw ltr from storage round-trip",
    "before raw assignment would show raw label; fixed startup cannot show a raw label",
    "toggle keeps internal raw value and stores raw value (saved value meaning unchanged)",
    "toggle renders Japanese label after switch (display behavior unchanged)",
    'default pin let spreadDirection = "rtl"; unchanged',
    "storage key pin unchanged",
    "setSpreadDirection stores the raw internal value (not the label)",
    "loadSpreadDirection still accepts only raw values",
    "spreadButtonText mapping meaning unchanged (rtl=右→左・ltr=左→右)",
    "startup block maps through spreadButtonText (raw assignment removed)",
]

def test_n19_spread_button_label_maps_display_and_keeps_raw_internal_value():
    if shutil.which("node") is None:
        pytest.fail("node toolchain is required for the spread button label harness")

    result = subprocess.run(["node", str(HARNESS)], capture_output=True, text=True, cwd=str(ROOT))
    if result.returncode != 0:
        pytest.fail("spread button label harness failed:\n" + result.stdout + result.stderr)

    out = result.stdout
    for label in REQUIRED:
        if "PASS " + label not in out:
            pytest.fail("spread button label harness must fix: " + label + "\ngot:\n" + out)
