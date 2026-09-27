#!/usr/bin/env python3
"""Render the static "Getting here" map to web/static/img/place/map-*.webp.

A static picture, not an iframe: no third-party request on page load, no
cookie banner implications, nothing to block the main thread. The picture
links out to Google Maps / Apple Maps.

Tiles are OpenStreetMap standard tiles (ODbL). The site prints the required
"© OpenStreetMap contributors" credit under the image. Fifteen tiles, fetched
once at dev time with an identifying User-Agent, per the tile usage policy.
"""
import io
import math
import sys
import urllib.request
from pathlib import Path

from PIL import Image, ImageDraw, ImageOps

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / "web" / "static" / "img" / "place"
LAT, LON, Z = 41.7106845, 44.7558247, 17
UA = "fitto-club-site-build/1.0 (+https://fitto.club; one-off static map render)"
ORANGE = (255, 68, 1)


def tile_xy(lat: float, lon: float, z: int) -> tuple[float, float]:
    n = 2 ** z
    x = (lon + 180) / 360 * n
    y = (1 - math.asinh(math.tan(math.radians(lat))) / math.pi) / 2 * n
    return x, y


def fetch(x: int, y: int) -> Image.Image:
    req = urllib.request.Request(f"https://tile.openstreetmap.org/{Z}/{x}/{y}.png", headers={"User-Agent": UA})
    with urllib.request.urlopen(req, timeout=20) as r:
        return Image.open(io.BytesIO(r.read())).convert("RGB")


def main() -> int:
    fx, fy = tile_xy(LAT, LON, Z)
    tx, ty = int(fx), int(fy)
    # 5x3 tiles: the club sits near a tile edge, so a 3x3 block runs out of
    # map on one side of a centred crop.
    canvas = Image.new("RGB", (256 * 5, 256 * 3))
    for dx in (-2, -1, 0, 1, 2):
        for dy in (-1, 0, 1):
            canvas.paste(fetch(tx + dx, ty + dy), ((dx + 2) * 256, (dy + 1) * 256))
    px = (fx - tx + 2) * 256
    py = (fy - ty + 1) * 256
    w, h = 640, 400
    box = (int(px - w / 2), int(py - h / 2), int(px + w / 2), int(py + h / 2))
    m = canvas.crop(box)

    # Dark, desaturated map so it sits on the black surface without a white
    # slab; the only colour left is the club's orange marker.
    g = ImageOps.grayscale(m)
    g = ImageOps.invert(g)
    g = ImageOps.autocontrast(g, cutoff=2)
    dark = Image.eval(g, lambda v: int(18 + v * 0.55)).convert("RGB")

    d = ImageDraw.Draw(dark)
    cx, cy = w / 2, h / 2
    # Marker in the pattern's shape: a thick ring, upper-left quarter open.
    ro, ri = 26, 12
    d.pieslice((cx - ro, cy - ro, cx + ro, cy + ro), -90, 180, fill=ORANGE)
    d.ellipse((cx - ri, cy - ri, cx + ri, cy + ri), fill=(0, 0, 0))
    d.ellipse((cx - 4, cy - 4, cx + 4, cy + 4), fill=ORANGE)

    OUT.mkdir(parents=True, exist_ok=True)
    path = OUT / "map-640.webp"
    dark.save(path, "WEBP", quality=80, method=6)
    print(f"{path.relative_to(ROOT)} {path.stat().st_size // 1024} KB")
    return 0


if __name__ == "__main__":
    sys.exit(main())
