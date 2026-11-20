import html
import json
import re
from pathlib import Path

HERE = Path(__file__).resolve().parent
SANS = "-apple-system, BlinkMacSystemFont, 'Segoe UI', 'Noto Sans', 'Noto Sans CJK SC', 'Noto Sans CJK TC', 'Noto Sans JP', 'Noto Sans KR', 'Noto Sans Cyrillic', 'Microsoft YaHei', 'PingFang SC', 'Hiragino Sans', 'Malgun Gothic', sans-serif"
TOKENS = {"TITLE": "title", "LIFE_DESC": "life_desc", "REGISTER": "register", "REGISTER_NOTE": "register_note", "EXPIRE": "expire", "EXPIRE_NOTE": "expire_note", "REAP": "reap", "BURY": "bury", "BURY_NOTE": "bury_note", "RESURRECT": "resurrect", "RESURRECT_NOTE": "resurrect_note", "PURGE": "purge", "PURGE_NOTE": "purge_note", "BEFORE": "before", "EXTEND_NOTE": "extend_note", "SPARE_NOTE": "spare_note", "GUARD_TITLE": "guard_title", "GUARD_DESC": "guard_desc", "GUARD_FIRST": "guard_first", "GUARD_BRIDGE": "guard_bridge", "GUARD_EQUIV": "guard_equiv", "GUARD_SECOND": "guard_second", "GUARD_RES": "guard_res", "GUARD_RES_NOTE": "guard_res_note"}
FONT_SIZES = {"register": 28, "expire": 28, "bury": 26, "resurrect": 28, "purge": 26, "guard_title": 28, "guard_equiv": 28, "guard_res": 28}
WIDTHS = {"register": 235, "register_note": 235, "expire": 132, "expire_note": 132, "reap": 49, "bury": 160, "bury_note": 160, "resurrect": 238, "resurrect_note": 238, "purge": 238, "purge_note": 238, "before": 108, "extend_note": 50, "spare_note": 60, "guard_title": 260, "guard_first": 260, "guard_bridge": 120, "guard_equiv": 260, "guard_second": 260, "guard_res": 260, "guard_res_note": 260}

def fit(value, limit, font_size=18):
    value = html.escape(value, quote=False)
    estimate = sum(1.0 if ord(char) > 0x3000 else 0.55 for char in value)
    if estimate * font_size <= limit:
        return value, ""
    return value, f' textLength="{limit}" lengthAdjust="spacingAndGlyphs"'

def render(template, copy):
    output = template
    for token, key in TOKENS.items():
        value, extra = fit(copy[key], WIDTHS.get(key, 260), FONT_SIZES.get(key, 18))
        output = output.replace("__" + token + "__", value)
        if extra:
            ident = key.replace("_", "-")
            output = re.sub(r'(id="' + re.escape(ident) + r'"[^>]*)(>)', lambda match: match.group(1) + extra + match.group(2), output, count=1)
    return output

strings = json.loads((HERE / "strings.json").read_text(encoding="utf-8"))
for language, copy in strings.items():
    target = HERE / language
    target.mkdir(exist_ok=True)
    for theme in ("light", "dark"):
        for name in ("lifecycle", "delete-guard"):
            template = (HERE / f"{name}-{theme}.svg.tmpl").read_text(encoding="utf-8")
            (target / f"{name}-{theme}.svg").write_text(render(template, copy), encoding="utf-8")
print("generated", len(strings) * 4, "SVG files")
