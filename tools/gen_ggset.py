#!/usr/bin/env python3
"""ggset.go を live gg.js から再生成する正典スクリプト。

使い方（hitomi-hub ルートで）:
    python tools/gen_ggset.py [--root <repo root>]

polarity は固定しない。live gg.js から観測した `var o` の初期値と switch 内 case 代入値を
そのまま snapshot に記録する（窓ごとに構造が rotation するため、生成時点の観測値のみを
書く。正典規則: `docs/ai/decisions/2026-11-03-gg-faithful-parse-not-fixed-polarity.md`）。
mapping 方向は固定で有効（0 → `w1`/`atn`、1 → `w2`/`btn`）。
switch values と bucket は回転するため、生成後は live で URL を 1 件検証すること
（`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`）。
"""

from __future__ import annotations

import argparse
import re
import urllib.request
from pathlib import Path

GG_URL = "https://ltn.gold-usergeneratedcontent.net/gg.js"

RE_BUCKET = re.compile(rb"b:\s*'([^']*)'")
RE_INITIAL_O = re.compile(rb"var o\s*=\s*(\d+)")
RE_SWITCH_BODY = re.compile(rb"(?s)switch\s*\(\s*g\s*\)\s*\{(.*?)\}")
RE_CASE = re.compile(rb"case (\d+):")
RE_CASE_ASSIGN = re.compile(rb"o\s*=\s*(\d+)")


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description="regenerate app/ggset.go from live gg.js")
    parser.add_argument("--root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args(argv)

    root = Path(args.root).resolve()
    text = urllib.request.urlopen(GG_URL, timeout=30).read()

    bucket_match = RE_BUCKET.search(text)
    if not bucket_match:
        raise SystemExit("gg.b not found in gg.js")
    bucket = bucket_match.group(1).decode()

    switch_body = RE_SWITCH_BODY.search(text)
    if not switch_body:
        raise SystemExit("gg switch not found in gg.js")
    initial_match = RE_INITIAL_O.search(text)
    if not initial_match:
        raise SystemExit("gg initial o not found in gg.js")
    assign_match = RE_CASE_ASSIGN.search(switch_body.group(1))
    if not assign_match:
        raise SystemExit("gg case assignment not found in gg.js")
    initial_o = int(initial_match.group(1))
    case_value = int(assign_match.group(1))
    cases = sorted({int(m.group(1)) for m in RE_CASE.finditer(switch_body.group(1))})
    if not cases:
        raise SystemExit("gg switch values not found in gg.js")

    out = root / "app" / "ggset.go"
    lines = [
        "package main",
        "",
        "// ggset.go は hitomi `common.js` が読み込む `gg.js` の switch values から生成（正典: `tools/gen_ggset.py`）。",
        "// polarity は固定しない。生成時点の live から観測した初期 o と case 代入を記録する",
        "// （構造は窓ごとに rotation: `docs/ai/decisions/2026-11-03-gg-faithful-parse-not-fixed-polarity.md`）。",
        "// mapping 方向は固定で有効（0 → `w1`/`atn`、1 → `w2`/`btn`）。",
        "// switch values と bucket は回転する。",
        f"var ggBucket = \"{bucket}\"",
        f"var ggInitialO = {initial_o}",
        f"var ggCaseValue = {case_value}",
        "",
        "var ggCaseSet = map[int]bool{",
    ]
    for n in cases:
        lines.append(f"\t{n}: true,")
    lines.extend([
        "}",
        "",
        "// ggM は生成時点の live 構造を faithful に保持した m(g)（case 内なら case 代入値、case 外なら初期 o）。",
        "func ggM(g int) int {",
        "\tif ggCaseSet[g] {",
        "\t\treturn ggCaseValue",
        "\t}",
        "\treturn ggInitialO",
        "}",
        "",
    ])
    body = "\n".join(lines)
    if args.dry_run:
        print(f"cases={len(cases)} bucket={bucket} initial_o={initial_o} case_value={case_value} (dry-run: nothing written)")
        return 0
    out.write_text(body, encoding="utf-8")
    print(f"cases={len(cases)} bucket={bucket} initial_o={initial_o} case_value={case_value} out={out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
