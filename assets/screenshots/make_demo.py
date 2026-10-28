#!/usr/bin/env python3
"""Build the README demo GIF (check in a terminal, the report, the record timeline) and report-summary.png.

Inputs come from a real run against a test server:
  check.txt     stdout of `codedb-lens check ... --lang en`
  check.html    the report that run saved
  record.html   `codedb-lens report <recording folder> --lang en`
Host names are swapped with --rename old=new (repeatable), so the GIF shows sql-prod-01, not the container.

  python assets/screenshots/make_demo.py check.txt check.html record.html --rename 20fd962af3a0=sql-prod-01

Needs Pillow and a Chromium (CHROME_PATH, Edge/Chrome or Playwright's chrome-headless-shell) for the page shots.
Colours and fonts follow the codeDB brand kit (internal/report/tokens.css, Geist from the app's frontend/fonts).
"""

import argparse
import glob
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent.parent
FONTS = ROOT / "cmd" / "codedb-lens" / "frontend" / "fonts"
sys.path.insert(0, str(ROOT / "assets" / "icon"))
import generate  # noqa: E402  (browser lookup)

W, H = 960, 600
CAP_H, BAR_H = 56, 30
SRC_W = 1280  # pages render at desktop width, then scale to W
BREU, CARVAO, GRAFITE, ARDOSIA = (10, 12, 17), (18, 21, 28), (28, 33, 43), (42, 48, 60)
NEVOA, NUVEM, PAPEL, SINAL = (139, 147, 161), (201, 206, 214), (241, 242, 244), (255, 90, 31)
SEV = {"critical": (239, 68, 68), "high": (255, 159, 67), "medium": (242, 201, 76), "low": NEVOA}
GRADE = {"A": (34, 194, 122), "B": (139, 209, 74), "C": (242, 201, 76), "D": (255, 159, 67), "F": (239, 68, 68)}
COMMAND = "codedb-lens check --engine mssql --server sql-prod-01 --user codedb_lens"
REPORT_DIR = r"C:\Users\dba\Documents\codeDB Lens\reports\sql-prod-01"

CAPTIONS = {
    "check": (None, "One command, a read-only login, nothing installed on the server"),
    "summary": (None, "A–F grade with a coverage seal: no data never shows up as OK"),
    "tech": (None, "Every finding: evidence, what it means, next action, how to verify"),
    "record": ("NEW", "record: an hour or a day of samples, turned into episodes and charts"),
}


def font(name: str, size: int) -> ImageFont.FreeTypeFont:
    return ImageFont.truetype(str(FONTS / name), size)


MONO, SANS, SANS_B = font("GeistMono-400.ttf", 15), font("Geist-400.ttf", 19), font("Geist-600.ttf", 13)


def chromium() -> str:
    # Playwright's headless shell first: a desktop Edge/Chrome already running may swallow --screenshot.
    shells = glob.glob(os.path.expanduser("~/AppData/Local/ms-playwright/chromium_headless_shell-*/*/chrome-headless-shell*"))
    shells += glob.glob(os.path.expanduser("~/.cache/ms-playwright/chromium_headless_shell-*/*/chrome-headless-shell"))
    if os.environ.get("CHROME_PATH") or not shells:
        return generate.browser()
    return sorted(shells)[-1]


def shot(html: str, tmp: Path, name: str, height: int = 5000, src_w: int = SRC_W, out_w: int = W) -> Image.Image:
    """Full-height screenshot of an HTML string at src_w, cropped to the content and scaled to out_w."""
    page, out = tmp / f"{name}.html", tmp / f"{name}.png"
    page.write_text(html, encoding="utf-8")
    subprocess.run([chromium(), "--headless", "--disable-gpu", "--hide-scrollbars", "--force-device-scale-factor=1",
                    f"--user-data-dir={tmp / 'profile'}", f"--window-size={src_w},{height}", f"--screenshot={out}",
                    page.as_uri()], check=True, capture_output=True)
    img = Image.open(out).convert("RGB")
    bg = img.getpixel((2, img.height - 2))
    bottom = img.height
    while bottom > H and all(img.getpixel((x, bottom - 1)) == bg for x in range(0, img.width, 16)):
        bottom -= 1
    img = img.crop((0, 0, img.width, min(img.height, bottom + 24)))
    return img.resize((out_w, round(img.height * out_w / src_w)), Image.LANCZOS)


def caption(img: Image.Image, key: str) -> None:
    badge, text = CAPTIONS[key]
    d = ImageDraw.Draw(img)
    d.rectangle((0, H - CAP_H, W, H), fill=BREU)
    d.line((0, H - CAP_H, W, H - CAP_H), fill=GRAFITE, width=1)
    x = 24
    if badge:
        bw = d.textlength(badge, font=SANS_B) + 16
        d.rounded_rectangle((x, H - CAP_H + 16, x + bw, H - 16), radius=4, fill=SINAL)
        d.text((x + 8, H - CAP_H / 2), badge, font=SANS_B, fill=BREU, anchor="lm")
        x += bw + 12
    d.text((x, H - CAP_H / 2), text, font=SANS, fill=PAPEL, anchor="lm")


def terminal(lines: list[tuple[str, tuple]], cursor: bool) -> Image.Image:
    img = Image.new("RGB", (W, H), BREU)
    d = ImageDraw.Draw(img)
    d.rounded_rectangle((16, 14, W - 16, H - CAP_H - 14), radius=10, fill=CARVAO, outline=GRAFITE)
    for i in range(3):
        d.ellipse((32 + i * 18, 28, 42 + i * 18, 38), fill=ARDOSIA)
    d.text((W / 2, 33), "Terminal", font=SANS_B, fill=NEVOA, anchor="mm")
    top, step = 56, 21
    rows = (H - CAP_H - 30 - top) // step
    for i, (text, colour) in enumerate(lines[-rows:]):
        y = top + i * step
        if text.startswith("PS> "):
            d.text((32, y), "PS>", font=MONO, fill=SINAL)
            d.text((32 + d.textlength("PS> ", font=MONO), y), text[4:], font=MONO, fill=PAPEL)
        else:
            d.text((32, y), text, font=MONO, fill=colour)
    if cursor:
        last = lines[-rows:][-1][0]
        x = 32 + d.textlength(last, font=MONO) + 2
        y = top + (min(len(lines), rows) - 1) * step
        d.rectangle((x, y + 1, x + 8, y + 17), fill=PAPEL)
    caption(img, "check")
    return img


def colour_of(line: str) -> tuple:
    if m := re.search(r"Grade: ([A-F?])", line):
        return GRADE.get(m.group(1), NUVEM)
    if m := re.match(r"\s*\[(\w+)\s*\]", line):
        return SEV.get(m.group(1), NUVEM)
    return NUVEM


def check_frames(check_txt: str) -> list[tuple[Image.Image, int]]:
    out = []
    for i in range(0, len(COMMAND) + 1, 3):
        out.append((terminal([("PS> " + COMMAND[:i], PAPEL)], True), 40))
    out.append((terminal([("PS> " + COMMAND, PAPEL)], True), 390))
    shown = [("PS> " + COMMAND, PAPEL)]
    for line in check_txt.splitlines():
        line = line.rstrip()
        if line.lstrip().startswith("Want this every minute"):
            continue
        shown.append((line, colour_of(line)))
        slow = line.startswith(("Connecting", "Connected")) or "Grade:" in line or "[" in line
        out.append((terminal(shown, False), 220 if slow else 70))
    out[-1] = (out[-1][0], 1470)
    return out


def browser(page: Image.Image, url: str, y: int, key: str) -> Image.Image:
    img = Image.new("RGB", (W, H), BREU)
    view = page.crop((0, y, W, y + H - CAP_H - BAR_H))
    img.paste(view, (0, BAR_H))
    d = ImageDraw.Draw(img)
    d.rectangle((0, 0, W, BAR_H), fill=CARVAO)
    for i in range(3):
        d.ellipse((12 + i * 15, 11, 20 + i * 15, 19), fill=ARDOSIA)
    d.rounded_rectangle((70, 6, W - 200, BAR_H - 6), radius=9, fill=GRAFITE)
    d.text((82, BAR_H / 2), url, font=font("Geist-400.ttf", 11), fill=NUVEM, anchor="lm")
    caption(img, key)
    return img


def scroll(page: Image.Image, url: str, key: str, stops: list[int], hold: list[int]) -> list[tuple[Image.Image, int]]:
    """Hold on each stop, gliding between them in 70 ms steps."""
    view = H - CAP_H - BAR_H
    stops = [min(s, max(0, page.height - view)) for s in stops]
    out, prev = [], stops[0]
    for stop, ms in zip(stops, hold):
        n = max(1, abs(stop - prev) // 60)
        for k in range(1, n):
            out.append((browser(page, url, prev + (stop - prev) * k // n, key), 70))
        out.append((browser(page, url, stop, key), ms))
        prev = stop
    return out


def anchor(html: str, tmp: Path, name: str, selector: str) -> int:
    """Page y (at W) of the first element matching selector, from a JS-stamped title."""
    probe = html.replace("</body>", "<script>addEventListener('load',()=>{const e=document.querySelector("
                         f"{selector!r});document.body.innerHTML='<pre id=y>'+(e?e.getBoundingClientRect().top+scrollY:0)"
                         "+'</pre>'})</script></body>")
    page = tmp / f"{name}-probe.html"
    page.write_text(probe, encoding="utf-8")
    dom = subprocess.run([chromium(), "--headless", "--disable-gpu", f"--user-data-dir={tmp / 'profile'}",
                          f"--window-size={SRC_W},1000", "--dump-dom", page.as_uri()],
                         check=True, capture_output=True, text=True, encoding="utf-8").stdout
    m = re.search(r'<pre id="y">([\d.]+)</pre>', dom)
    return round(float(m.group(1)) * W / SRC_W) if m else 0


def end_card(tmp: Path) -> Image.Image:
    logo = (ROOT / "assets" / "icon" / "logo-dark.svg").read_text(encoding="utf-8")
    fonts = FONTS.as_uri()
    pills = "".join(f"<span>{p}</span>" for p in ["Single binary", "Windows · Linux · Docker", "No telemetry", "Open source"])
    html = f"""<!doctype html><meta charset="utf-8"><style>
@font-face {{ font-family: Geist; src: url({fonts}/Geist-400.ttf); }}
@font-face {{ font-family: "Geist Mono"; src: url({fonts}/GeistMono-400.ttf); }}
html, body {{ margin: 0; height: 100%; background: #0a0c11; color: #f1f2f4; font-family: Geist, system-ui; }}
body {{ display: grid; place-content: center; justify-items: center; gap: 36px; height: {round(H * SRC_W / W)}px; }}
svg {{ width: 720px; height: auto; }}
p {{ margin: 0; font-size: 30px; color: #c9ced6; }}
div {{ display: flex; gap: 14px; }}
span {{ border: 1px solid #2a303c; border-radius: 999px; padding: 8px 20px; font-size: 22px; color: #c9ced6; }}
code {{ font: 26px "Geist Mono", ui-monospace; color: #8b93a1; }}
</style><body>{logo}<p>Free, read-only health check for SQL Server and PostgreSQL</p><div>{pills}</div>
<code>github.com/codedb-io/codedb-lens</code></body>"""
    return shot(html, tmp, "end", round(H * SRC_W / W)).crop((0, 0, W, H))


def save_gif(frames: list[tuple[Image.Image, int]], path: Path) -> None:
    pal = [f.quantize(colors=255, method=Image.Quantize.FASTOCTREE) for f, _ in frames]
    pal[0].save(path, save_all=True, append_images=pal[1:], duration=[ms for _, ms in frames], loop=0,
                optimize=True, disposal=1)


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("check_txt"), ap.add_argument("check_html"), ap.add_argument("record_html")
    ap.add_argument("--rename", action="append", default=[], help="old=new, applied to the text and the pages")
    ap.add_argument("--demo-report", default=str(ROOT / "docs" / "en" / "demo" / "report.html"),
                    help="report used for report-summary.png")
    a = ap.parse_args()

    def clean(s: str) -> str:
        for pair in a.rename:
            old, new = pair.split("=", 1)
            s = s.replace(old, new)
        return s

    check_txt = clean(Path(a.check_txt).read_text(encoding="utf-8"))
    check_txt = re.sub(r"Report saved: .*[\\/]", lambda _: "Report saved: " + REPORT_DIR + "\\", check_txt)
    report = clean(Path(a.check_html).read_text(encoding="utf-8"))
    record = clean(Path(a.record_html).read_text(encoding="utf-8"))
    tech = report.replace("</body>", "<script>addEventListener('load',()=>showTab('tech'))</script></body>")
    base = "file:///" + REPORT_DIR.replace("\\", "/") + "/"
    report_url = base + Path(a.check_html).name
    record_url = base + "recordings/codedb-lens-record-sql-prod-01/" + Path(a.record_html).name

    with tempfile.TemporaryDirectory() as t:
        tmp = Path(t)
        summary_page, tech_page, record_page = shot(report, tmp, "summary"), shot(tech, tmp, "tech"), shot(record, tmp, "record")
        first_story = anchor(tech, tmp, "tech", ".story") - 90
        charts = anchor(record, tmp, "record", ".charts") - 120
        frames = check_frames(check_txt)
        frames += scroll(summary_page, report_url, "summary", [0, 240], [1170, 1970])
        frames += scroll(tech_page, report_url, "tech", [0, first_story, first_story + 260], [1300, 1600, 1970])
        frames += scroll(record_page, record_url, "record", [0, charts], [1470, 1970])
        frames.append((end_card(tmp), 2300))
        save_gif(frames, HERE / "demo.gif")

        # report-summary.png: the summary tab of the docs demo report at 1200 px wide, cut after the codeDB invite.
        demo = shot(Path(a.demo_report).read_text(encoding="utf-8"), tmp, "demo", src_w=1200, out_w=1200)
        demo.crop((0, 0, 1200, min(demo.height, 1060))).save(HERE / "report-summary.png", optimize=True)
    print(f"demo.gif ({len(frames)} frames), report-summary.png")


if __name__ == "__main__":
    main()
