#!/usr/bin/env python3
"""ggset.go を live gg.js から再生成する正典スクリプト。

使い方（hitomi-hub ルートで）:
    python tools/gen_ggset.py [--root <repo root>]

`gg.m(g)` は switch にある value で 0、それ以外で 1（`var o = 1; switch (g) { ... o = 0; break; }`）。
switch values と bucket は回転するため、生成後は live で URL を 1 件検証すること
（`docs/ai/decisions/2026-11-01-ggset-generation-and-drift.md`）。
"""

from __future__ import annotations

import argparse
import re
import urllib.request
from pathlib import Path

GG_URL = "https://ltn.gold-usergeneratedcontent.net/gg.js"


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description="regenerate app/ggset.go from live gg.js")
    parser.add_argument("--root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args(argv)

    root = Path(args.root).resolve()
    text = urllib.request.urlopen(GG_URL, timeout=30).read()

    bucket_match = re.search(rb"b:\s*'([^']*)'", text)
    if not bucket_match:
        raise SystemExit("gg.b not found in gg.js")
    bucket = bucket_match.group(1).decode()
    cases = sorted({int(m.group(1)) for m in re.finditer(rb"case (\d+):", text)})

    out = root / "app" / "ggset.go"
    lines = [
        "package main",
        "",
        "// ggset.go は hitomi `common.js` が読み込む `gg.js` の switch values から生成（正典: `tools/gen_ggset.py`）。",
        "// gg.m(g) は switch にある value で 0、それ以外で 1。switch values と bucket は回転する。",
        f"var ggBucket = \"{bucket}\"",
        "",
        "var ggZeroSet = map[int]bool{",
    ]
    for n in cases:
        lines.append(f"\t{n}: true,")
    lines.extend([
        "}",
        "",
        "func ggM(g int) int {",
        "\tif ggZeroSet[g] {",
        "\t\treturn 0",
        "\t}",
        "\treturn 1",
        "}",
        "",
    ])
    body = "\n".join(lines)
    if args.dry_run:
        print(f"cases={len(cases)} bucket={bucket} (dry-run: nothing written)")
        return 0
    out.write_text(body, encoding="utf-8")
    print(f"cases={len(cases)} bucket={bucket} out={out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
