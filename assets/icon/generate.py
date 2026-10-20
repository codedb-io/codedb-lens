#!/usr/bin/env python3
"""Render lens-icon.svg into icon.ico, PNGs and the GitHub social preview.

Needs Pillow and a Chromium browser (Edge or Chrome) for headless SVG rendering.
ponytail: headless browser instead of cairo/resvg, no native deps on Windows.
"""

import os
import shutil
import subprocess
import tempfile
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

HERE = Path(__file__).resolve().parent
SVG = HERE / "lens-icon.svg"
NAVY = (10, 14, 23)

BROWSERS = [
    os.environ.get("CHROME_PATH", ""),
    r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
    r"C:\Program Files\Google\Chrome\Application\chrome.exe",
    "google-chrome", "chromium", "chromium-browser", "microsoft-edge",
]


def browser() -> str:
    for b in BROWSERS:
        if b and (Path(b).is_file() or shutil.which(b)):
            return b
    raise SystemExit("No Chromium browser found; set CHROME_PATH.")


def render(size: int) -> Image.Image:
    with tempfile.TemporaryDirectory() as tmp:
        page = Path(tmp) / "icon.html"
        page.write_text(
            "<html><body style='margin:0;background:transparent'>"
            f"<img src='{SVG.as_uri()}' width='{size}' height='{size}'></body></html>",
            encoding="utf-8",
        )
        out = Path(tmp) / "out.png"
        subprocess.run(
            [browser(), "--headless", "--disable-gpu", "--hide-scrollbars",
             "--default-background-color=00000000", f"--window-size={size},{size}",
             f"--screenshot={out}", page.as_uri()],
            check=True, capture_output=True,
        )
        return Image.open(out).convert("RGBA").crop((0, 0, size, size))


def social_preview(icon: Image.Image) -> Image.Image:
    img = Image.new("RGB", (1280, 640), NAVY)
    icon = icon.resize((360, 360), Image.LANCZOS)
    img.paste(icon, (90, 140), icon)
    d = ImageDraw.Draw(img)
    try:
        big = ImageFont.truetype("segoeuib.ttf", 96)
        small = ImageFont.truetype("segoeui.ttf", 32)
    except OSError:
        big = small = ImageFont.load_default()
    d.text((500, 220), "codeDB", font=big, fill=(255, 255, 255))
    d.text((500 + d.textlength("codeDB ", font=big), 220), "Lens", font=big, fill=(56, 189, 248))
    d.text((504, 350), "See what your database isn't showing you.", font=small, fill=(186, 230, 253))
    d.text((504, 400), "SQL Server · PostgreSQL · free & read-only", font=small, fill=(125, 211, 252))
    return img


def main() -> None:
    big = render(1024)
    big.resize((512, 512), Image.LANCZOS).save(HERE / "icon-512.png", optimize=True)
    big.resize((256, 256), Image.LANCZOS).save(
        HERE / "icon.ico", sizes=[(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]
    )
    social_preview(big).save(HERE / "social-preview.png", optimize=True)
    print("icon-512.png, icon.ico, social-preview.png")


if __name__ == "__main__":
    main()
