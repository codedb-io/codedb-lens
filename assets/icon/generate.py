#!/usr/bin/env python3
"""Render lens-icon.svg into icon.ico, PNGs and the GitHub social preview, and copy the PNGs into the app.

lens-icon.svg is the app icon from the codeDB brand kit (03-favicon-e-icones/icone-app-escuro.svg).

Needs Pillow and a Chromium (CHROME_PATH, Playwright's chrome-headless-shell, Edge or Chrome) for headless SVG rendering.
ponytail: headless browser instead of cairo/resvg, no native deps on Windows.
"""

import glob
import os
import shutil
import subprocess
import tempfile
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

HERE = Path(__file__).resolve().parent
SVG = HERE / "lens-icon.svg"
BREU = (10, 12, 17)
PAPEL = (241, 242, 244)
SINAL = (255, 90, 31)
NEVOA = (139, 147, 161)
ROOT = HERE.parent.parent
FONTS = Path(os.environ.get("GEIST_DIR", ROOT.parent / "MonitorSQL" / "estrategia_marca" / "05-tipografia"))

BROWSERS = [
    os.environ.get("CHROME_PATH", ""),
    r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
    r"C:\Program Files\Google\Chrome\Application\chrome.exe",
    "google-chrome", "chromium", "chromium-browser", "microsoft-edge",
]


def browser() -> str:
    # Playwright's headless shell first: a desktop Edge/Chrome already running may swallow --screenshot.
    shells = glob.glob(os.path.expanduser("~/AppData/Local/ms-playwright/chromium_headless_shell-*/*/chrome-headless-shell*"))
    shells += glob.glob(os.path.expanduser("~/.cache/ms-playwright/chromium_headless_shell-*/*/chrome-headless-shell"))
    if shells and not os.environ.get("CHROME_PATH"):
        return sorted(shells)[-1]
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
             "--default-background-color=00000000", f"--window-size={size},{size}", f"--user-data-dir={Path(tmp) / 'profile'}",
             f"--screenshot={out}", page.as_uri()],
            check=True, capture_output=True,
        )
        return Image.open(out).convert("RGBA").crop((0, 0, size, size))


def social_preview() -> Image.Image:
    # The symbol is the "c" of codeDB, so the preview uses the horizontal logo, never icon + "codeDB" text.
    img = Image.new("RGB", (1280, 640), BREU)
    logo = Image.open(FONTS.parent / "01-logo" / "png" / "codeDB-Lens-claro-sobre-escuro-480px.png").convert("RGBA")
    logo = logo.resize((880, round(880 * logo.height / logo.width)), Image.LANCZOS)
    img.paste(logo, ((1280 - logo.width) // 2, 220), logo)
    d = ImageDraw.Draw(img)
    try:
        small = ImageFont.truetype(str(FONTS / "Geist-400.ttf"), 32)
    except OSError:
        small = ImageFont.load_default()
    for y, text, fill in [(370, "See what your database isn't showing you.", PAPEL),
                          (420, "SQL Server · PostgreSQL · free & read-only", NEVOA)]:
        d.text((640, y), text, font=small, fill=fill, anchor="mt")
    return img


def main() -> None:
    big = render(1024)
    big.resize((512, 512), Image.LANCZOS).save(HERE / "icon-512.png", optimize=True)
    big.resize((256, 256), Image.LANCZOS).save(
        HERE / "icon.ico", sizes=[(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]
    )
    social_preview().save(HERE / "social-preview.png", optimize=True)
    for dst, size in [("cmd/codedb-lens/winres/icon.png", 512), ("docs/assets/icon.png", 512),
                      ("internal/report/icon.png", 96)]:
        big.resize((size, size), Image.LANCZOS).save(ROOT / dst, optimize=True)
    print("icon-512.png, icon.ico, social-preview.png + app copies")


if __name__ == "__main__":
    main()
