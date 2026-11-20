from pathlib import Path
import os
import re
import sys

ROOT = Path(__file__).resolve().parents[1]
README_NAMES = ["README.md", "README.zh-CN.md", "README.zh-TW.md", "README.ja.md", "README.ko.md", "README.es.md", "README.fr.md", "README.de.md", "README.pt-BR.md", "README.ru.md"]
BT = chr(96)

def slug(text):
    text = re.sub(r"<[^>]+>", "", text.strip().lower().replace(BT, ""))
    text = re.sub(r"[^\w\u0080-\uffff -]", "", text, flags=re.UNICODE)
    return re.sub(r"\s+", "-", text).strip("-")

def code_blocks(text):
    fence = re.escape(BT * 3)
    return re.findall(fence + r"[^\n]*\n(.*?)" + fence, text, re.S)

def links(text):
    values = re.findall(r"(?:href|src)=?[\"\(]([^\")]+)", text)
    values += re.findall(r"!?\[[^\]]*\]\(([^)]+)\)", text)
    return [v.split("#", 1)[0].strip("<>") for v in values if v and not v.startswith(("http:", "https:", "mailto:", "#"))]

def headings(text):
    return {slug(m.group(1)) for m in re.finditer(r"^##\s+(.+?)\s*$", text, re.M)}

source = (ROOT / "README.md").read_text(encoding="utf-8")
source_language_bar = source.partition("\n")[0]
source_blocks = code_blocks(source)
source_heading_count = len(headings(source))
errors = []
for name in README_NAMES:
    path = ROOT / name
    if not path.exists():
        errors.append(f"{name}: missing")
        continue
    text = path.read_text(encoding="utf-8")
    if text.partition("\n")[0] != source_language_bar:
        errors.append(f"{name}: language bar differs from README.md")
    for link in links(text):
        if not (ROOT / link).exists():
            errors.append(f"{name}: missing link {link}")
    anchors = set(re.findall(r'href="#([^\"]+)"', text))
    missing = sorted(anchors - headings(text))
    if missing:
        errors.append(f"{name}: missing anchors {', '.join(missing)}")
    if len(headings(text)) != source_heading_count:
        errors.append(f"{name}: heading count {len(headings(text))} != {source_heading_count}")
    if code_blocks(text) != source_blocks:
        errors.append(f"{name}: code blocks differ")
    image_paths = [p for p in links(text) if p.endswith((".svg", ".gif", ".png", ".jpg", ".jpeg")) and not p.startswith("docs/assets/flags/")]
    if len(image_paths) < 4:
        errors.append(f"{name}: too few images {len(image_paths)}")
svg_root = Path(os.environ.get("README_SVG_ROOT", str(ROOT / "docs" / "assets" / "readme" / "i18n")))
for language in ["en", "zh-CN", "zh-TW", "ja", "ko", "es", "fr", "de", "pt-BR", "ru"]:
    for diagram in ["lifecycle", "delete-guard"]:
        for theme in ["light", "dark"]:
            path = svg_root / language / f"{diagram}-{theme}.svg"
            if not path.exists():
                errors.append(f"{path}: missing")
                continue
            svg = path.read_text(encoding="utf-8")
            if "textLength=" in svg or "lengthAdjust=" in svg:
                errors.append(f"{path}: horizontal text compression is forbidden")
            width_match = re.search(r'<svg[^>]+width="([0-9.]+)"', svg)
            canvas_width = float(width_match.group(1)) if width_match else 880.0
            for match in re.finditer(r'<text([^>]*)>(.*?)</text>', svg, re.S):
                attrs, value = match.groups()
                font_match = re.search(r'font-size="([0-9.]+)"', attrs)
                if not font_match:
                    continue
                font_size = float(font_match.group(1))
                display_size = font_size * 880 / canvas_width
                if display_size < 11:
                    errors.append(f"{path}: displayed font below 11px: {value[:40]}")
                if font_size >= 22 and display_size < 16:
                    errors.append(f"{path}: displayed title below 16px: {value[:40]}")
            if diagram == "lifecycle" and "resurrect-star" not in svg:
                errors.append(f"{path}: missing resurrect star")
            if diagram == "delete-guard" and "guard-star" not in svg:
                errors.append(f"{path}: missing guard star")
            for ident in ["resurrect-star", "guard-star"]:
                if ident in svg and "transform=\"translate(" not in svg:
                    errors.append(f"{path}: {ident} has no measured anchor")
if errors:
    print("\n".join(errors))
    sys.exit(1)
print(f"OK: {len(README_NAMES)} README files, matching language bars, {source_heading_count} headings, {len(source_blocks)} code blocks, links/images and 40 SVG static checks pass (layout: node scripts/check-readme-diagrams.mjs)")
