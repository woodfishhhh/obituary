from pathlib import Path
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
source_blocks = code_blocks(source)
source_heading_count = len(headings(source))
errors = []
for name in README_NAMES:
    path = ROOT / name
    if not path.exists():
        errors.append(f"{name}: missing")
        continue
    text = path.read_text(encoding="utf-8")
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
    image_paths = [p for p in links(text) if p.endswith((".svg", ".gif", ".png", ".jpg", ".jpeg"))]
    if len(image_paths) < 4:
        errors.append(f"{name}: too few images {len(image_paths)}")
if errors:
    print("\n".join(errors))
    sys.exit(1)
print(f"OK: {len(README_NAMES)} README files, {source_heading_count} headings, {len(source_blocks)} code blocks, links and images resolve")
