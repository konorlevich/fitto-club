#!/usr/bin/env python3
"""Build web-sized photos, masks and the entrance video in web/static/.

Dev-time only; output is committed and embedded. Needs Pillow (+ ffmpeg on
PATH for the video):

    .venv/bin/pip install pillow
    .venv/bin/python tools/assets/build_images.py

Sources are the club's own photos in materials/. EXIF is dropped on save
(Pillow does not copy it unless asked), orientation is applied first.
"""
import math
import subprocess
import sys
from pathlib import Path

from PIL import Image, ImageOps

ROOT = Path(__file__).resolve().parents[2]
MAT = ROOT / "materials"
IMG = ROOT / "web" / "static" / "img"
VID = ROOT / "web" / "static" / "video"
POSTS = MAT / "instagram" / "posts"

# Coach portraits: the club's "More about the trainer" covers. Their name is
# burnt into the lower quarter, and the site sets the name itself, so the crop
# keeps the photo above it (y < 1010 of 1350) at the site's 4:5.
COACHES = {
    "otar-chkadua": "2026-05-03_DX39FlFDMEA.jpg",
    "nikolai-starodubcev": "2026-04-04_DWtnsPADKoK_01.jpg",
    "vlad-zapolskikh": "2026-04-19_DXTrLc3DEqf_01.jpg",
    "gocha-butbaia": "2026-04-02_DWoUI_1DPQ4_01.jpg",
    "anastasia-novikova": "2026-04-11_DW_F_jJjGDi_01.jpg",
    "irakli-nikolaishvili": "2026-03-31_DWi1QWejCQG_01.jpg",
    "dmitrii-poshin": "2026-04-21_DXYzmAwjKpE_01.jpg",
    "grigol-janoashvili": "2026-03-27_DWYVeCUDByf_01.jpg",
    "ivan-alimamedov": "2026-04-15_DXJyv80DKFC_01.jpg",
    "veriko-kundukhashvili": "2026-07-30_DbamXaDDHGw_01.jpg",
    "joni-nadoyan": "2026-08-21_DcS34f6uUwl.jpg",
    "evan-kostylev": "2026-04-09_DW5-RhDjISC_01.jpg",
    "lizi-gagnidze": "2025-09-29_DPMHo8YjNSu_cover.jpg",
}

# Place photos. Widths are what the layout actually draws at 375 (x2) and
# 1280, so a phone never downloads a desktop file. The optional third item
# is a crop (aspect_w, aspect_h, anchor_y): the zone list draws every photo
# as a 4:3 landscape, so portrait sources are cropped at build time instead
# of shipping pixels that object-fit would throw away. anchor_y is where the
# crop window sits in the source, 0 = top, 1 = bottom.
PLACES = {
    # hall-neon-960 is the HealthClub image in JSON-LD; the hero uses hero().
    "hall-neon": (MAT / "google-maps/photos/gmaps_09.jpg", (960,)),
    "hall": (MAT / "google-maps/photos/gmaps_01.jpg", (480, 720, 960), (4, 3, 0.5)),
    "crossfit": (MAT / "google-maps/photos/gmaps_02.jpg", (480, 720, 960), (4, 3, 0.7)),
    "cardio": (MAT / "google-maps/photos/gmaps_04.jpg", (480, 720, 960), (4, 3, 0.45)),
    "lockers": (MAT / "google-maps/photos/gmaps_08.jpg", (480, 720, 960), (4, 3, 0.5)),
    # Portrait for the About page head (1:2 half ring) and the massage page,
    # plus a 4:3 crop of the same photo for the zone list.
    "lounge": (MAT / "google-maps/photos/gmaps_07.jpg", (480, 720, 960)),
    "lounge-wide": (MAT / "google-maps/photos/gmaps_07.jpg", (480, 720, 960), (4, 3, 0.6)),
    "massage-room": (MAT / "instagram/highlights/gym/gym_15_2025-07-20.jpg", (480, 960)),
    "massage-wide": (MAT / "instagram/highlights/gym/gym_15_2025-07-20.jpg", (480, 720, 960), (4, 3, 0.55)),
    # Drawn at 18rem everywhere: 480 for 1x, 640 for 2x and up.
    "facade": (MAT / "google-maps/photos/gmaps_06.jpg", (480, 640)),
    # Meme still for the About page: the "plank after the holidays" reel.
    "meme-1": (POSTS / "2026-01-21_DTxlgJzDMuJ_cover.jpg", (540,)),
}


def save_webp(im: Image.Image, path: Path, quality: int = 74) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    im.save(path, "WEBP", quality=quality, method=6)
    print(f"{path.relative_to(ROOT)}  {im.width}x{im.height}  {path.stat().st_size // 1024} KB")


def open_rgb(p: Path) -> Image.Image:
    im = Image.open(p)
    im = ImageOps.exif_transpose(im)
    return im.convert("RGB")


def coaches() -> None:
    for slug, name in COACHES.items():
        im = open_rgb(POSTS / name)
        w, h = im.size
        ch = int(h * 0.748)          # above the burnt-in name
        cw = int(ch * 4 / 5)
        x = (w - cw) // 2
        crop = im.crop((x, 0, x + cw, ch))
        for width in (400, 800):
            out = crop.resize((width, width * 5 // 4), Image.LANCZOS)
            save_webp(out, IMG / "coaches" / f"{slug}-{width}.webp", 76)


def place_source(name: str, src: Path) -> Image.Image:
    """The club's original, or, when it is no longer in materials/, the
    largest webp already built for that photo (or for its base name, so
    "lounge-wide" can be cut from "lounge-960")."""
    if src.exists():
        return open_rgb(src)
    base = {"lounge-wide": "lounge", "massage-wide": "massage-room"}.get(name, name)
    built = sorted((IMG / "place").glob(f"{base}-*.webp"), key=lambda f: Image.open(f).width)
    if not built:
        raise FileNotFoundError(src)
    print(f"{name}: {src.relative_to(ROOT)} missing, cutting from {built[-1].relative_to(ROOT)}")
    return open_rgb(built[-1])


def places() -> None:
    for name, spec in PLACES.items():
        src, widths = spec[0], spec[1]
        im = place_source(name, spec[0])
        if len(spec) > 2:
            aw, ah, anchor = spec[2]
            ch = min(im.height, round(im.width * ah / aw))
            top = round((im.height - ch) * anchor)
            im = im.crop((0, top, im.width, top + ch))
        for width in widths:
            if im.width < width:
                continue
            h = round(im.height * width / im.width)
            save_webp(im.resize((width, h), Image.LANCZOS), IMG / "place" / f"{name}-{width}.webp")


def hero() -> None:
    """The hero window has two shapes: a 2:1 strip under the text on phones,
    a 1:2 half-disc beside it from 1024. Each gets its own crop so a phone
    never downloads a tall portrait to show a thin strip of it."""
    im = open_rgb(MAT / "google-maps/photos/gmaps_09.jpg")
    w, h = im.size
    strip_h = w // 2
    top = int(h * 0.42) - strip_h // 2
    strip = im.crop((0, top, w, top + strip_h))
    for width in (480, 800, 1200):
        save_webp(strip.resize((width, width // 2), Image.LANCZOS), IMG / "place" / f"hero-strip-{width}.webp", 72)
    tall_w = h // 2
    left = int(w * 0.3) - tall_w // 2
    tall = im.crop((max(0, left), 0, max(0, left) + tall_w, h))
    for width in (400, 800):
        save_webp(tall.resize((width, width * 2), Image.LANCZOS), IMG / "place" / f"hero-tall-{width}.webp", 72)


def masks() -> None:
    # Right half-disc: the "half ring" photo window.
    half = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 50 100">'
            '<path d="M0,0 A50,50 0 0 1 0,100 Z"/></svg>')
    # Bottom half-disc (flat edge on top): the hero window on phones.
    bottom = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50">'
              '<path d="M0,0 A50,50 0 0 0 100,0 Z"/></svg>')
    for name, body in (("mask-half-ring.svg", half), ("mask-half-bottom.svg", bottom)):
        (IMG / name).write_text(body)
        print(f"web/static/img/{name}")


def video() -> None:
    src = MAT / "instagram/highlights/location/location_video_01.mp4"
    VID.mkdir(parents=True, exist_ok=True)
    out = VID / "entrance.mp4"
    # Silent on purpose: the Instagram soundtrack is music we have no licence
    # to republish, and the walk reads fine without it.
    subprocess.run(["ffmpeg", "-v", "error", "-y", "-i", str(src), "-an",
                    "-vf", "scale=540:-2", "-c:v", "libx264", "-preset", "slow",
                    "-crf", "28", "-profile:v", "high", "-pix_fmt", "yuv420p",
                    "-movflags", "+faststart", str(out)], check=True)
    poster = IMG / "place" / "entrance-poster-540.webp"
    frame = VID / "_poster.png"
    subprocess.run(["ffmpeg", "-v", "error", "-y", "-ss", "18.5", "-i", str(src),
                    "-frames:v", "1", "-vf", "scale=540:-2", str(frame)], check=True)
    save_webp(Image.open(frame).convert("RGB"), poster, 72)
    frame.unlink()
    print(f"{out.relative_to(ROOT)}  {out.stat().st_size // 1024} KB")


def main() -> int:
    coaches()
    places()
    hero()
    masks()
    video()
    return 0


if __name__ == "__main__":
    sys.exit(main())
