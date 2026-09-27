#!/usr/bin/env python3
"""Build the self-hosted WOFF2 subsets in web/static/fonts/.

Dev-time only; the output is committed and embedded in the binary, so the
deploy never runs this. Needs fonttools + brotli:

    python3 -m venv .venv && .venv/bin/pip install fonttools brotli
    .venv/bin/python tools/assets/build_fonts.py

Every family is OFL. Sources are downloaded into FONT_CACHE (default
~/.cache/fitto-fonts) on first run.

Subsets follow the script split in DESIGN_TOKENS.css: a Latin page never
downloads Cyrillic or Georgian bytes, because each face is declared with a
unicode-range and the browser only fetches the ranges the page uses.
"""
import io
import os
import sys
import urllib.request
from pathlib import Path

from fontTools import subset
from fontTools.ttLib import TTFont
from fontTools.varLib import instancer

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / "web" / "static" / "fonts"
CACHE = Path(os.environ.get("FONT_CACHE", Path.home() / ".cache" / "fitto-fonts"))

SOURCES = {
    "Jost[wght].ttf": "https://github.com/google/fonts/raw/main/ofl/jost/Jost%5Bwght%5D.ttf",
    "ArchivoBlack-Regular.ttf": "https://github.com/google/fonts/raw/main/ofl/archivoblack/ArchivoBlack-Regular.ttf",
    "NotoSansGeorgian[wdth,wght].ttf": "https://github.com/google/fonts/raw/main/ofl/notosansgeorgian/NotoSansGeorgian%5Bwdth,wght%5D.ttf",
    "FiraGO-Regular.ttf": "https://github.com/bBoxType/FiraGO/raw/master/Fonts/FiraGO_TTF_1001/Roman/FiraGO-Regular.ttf",
    "FiraGO-SemiBold.ttf": "https://github.com/bBoxType/FiraGO/raw/master/Fonts/FiraGO_TTF_1001/Roman/FiraGO-SemiBold.ttf",
}

# Unicode ranges - must match the unicode-range lines in fonts.css.
LATIN = "U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+2000-206F,U+20AC,U+20BE,U+2122,U+2190-2193,U+2212,U+2215,U+2248,U+2605,U+2713,U+FEFF,U+FFFD"
CYRILLIC = "U+0400-045F,U+0490-0491,U+04B0-04B1,U+2116"
GEORGIAN = "U+10A0-10FF,U+1C90-1CBF,U+2D00-2D2F"
# Archivo Black carries only prices and trainer initials.
NUMERIC = "U+0020-007E,U+00A0,U+2009,U+2013-2014,U+2248"


def src(name: str) -> Path:
    CACHE.mkdir(parents=True, exist_ok=True)
    p = CACHE / name
    if not p.exists():
        print(f"downloading {name}")
        urllib.request.urlretrieve(SOURCES[name], p)
    return p


def ranges(spec: str) -> list[int]:
    out: list[int] = []
    for part in spec.split(","):
        part = part.strip().removeprefix("U+")
        if "-" in part:
            a, b = part.split("-")
            out.extend(range(int(a, 16), int(b, 16) + 1))
        else:
            out.append(int(part, 16))
    return out


def build(font: TTFont, spec: str, out_name: str) -> None:
    opts = subset.Options()
    opts.flavor = "woff2"
    opts.layout_features = ["kern", "liga", "calt", "tnum", "lnum", "case", "ccmp", "locl", "mark", "mkmk"]
    opts.name_IDs = ["*"]
    opts.notdef_outline = True
    opts.drop_tables += ["DSIG"]
    sub = subset.Subsetter(opts)
    sub.populate(unicodes=ranges(spec))
    sub.subset(font)
    path = OUT / out_name
    font.flavor = "woff2"
    font.save(path)
    print(f"{out_name:40s} {path.stat().st_size / 1024:6.1f} KB")


def load(path: Path, axes: dict | None = None) -> TTFont:
    f = TTFont(path, lazy=False)
    if axes:
        f = instancer.instantiateVariableFont(f, axes)
        # Round-trip so the subsetter sees plain, fully-loaded tables.
        buf = io.BytesIO()
        f.save(buf)
        buf.seek(0)
        f = TTFont(buf, lazy=False)
    return f


def main() -> int:
    OUT.mkdir(parents=True, exist_ok=True)
    # Jost: display only, 400-600 is all the tokens use.
    for spec, suffix in ((LATIN, "latin"), (CYRILLIC, "cyrillic")):
        build(load(src("Jost[wght].ttf"), {"wght": (400, 600)}), spec, f"jost-{suffix}.woff2")
    build(load(src("ArchivoBlack-Regular.ttf")), NUMERIC, "archivo-black-numeric.woff2")
    for weight, file in ((400, "FiraGO-Regular.ttf"), (600, "FiraGO-SemiBold.ttf")):
        for spec, suffix in ((LATIN, "latin"), (CYRILLIC, "cyrillic"), (GEORGIAN, "georgian")):
            build(load(src(file)), spec, f"firago-{weight}-{suffix}.woff2")
    # Noto Sans Georgian: headings only, normal width, 500-700.
    build(load(src("NotoSansGeorgian[wdth,wght].ttf"), {"wdth": 100, "wght": (500, 700)}),
          GEORGIAN, "noto-sans-georgian.woff2")
    return 0


if __name__ == "__main__":
    sys.exit(main())
