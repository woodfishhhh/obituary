import html
import json
from pathlib import Path

HERE = Path(__file__).resolve().parent
THEMES = {
    "light": {"bg": "#F5F8FE", "border": "#DCE5F4", "card": "#FFFFFF", "card2": "#F3F5F9", "border2": "#B6C1D2", "chip": "#EAF2FF", "ink": "#0F1B33", "muted": "#53637D", "muted2": "#9AA8BC", "blue": "#0F56D9", "gold": "#E0960A", "warn": "#C2410C"},
    "dark": {"bg": "#0B1120", "border": "#24324D", "card": "#121C31", "card2": "#16213A", "border2": "#33425F", "chip": "#1B3155", "ink": "#E8EEF9", "muted": "#A1B1CC", "muted2": "#6F809E", "blue": "#8DBDFF", "gold": "#FFD35A", "warn": "#FFA27E"},
}
SANS = "-apple-system, BlinkMacSystemFont, 'Segoe UI', 'Noto Sans', 'Noto Sans CJK SC', 'Noto Sans CJK TC', 'Noto Sans JP', 'Noto Sans KR', 'Noto Sans Cyrillic', 'Microsoft YaHei', 'PingFang SC', 'Hiragino Sans', 'Malgun Gothic', sans-serif"
MONO = "ui-monospace, 'Cascadia Code', 'SF Mono', Consolas, monospace"


def width(text, size):
    return sum(size * (1.0 if ord(char) > 0x3000 else 0.55) for char in text)


def wrap(text, max_width, size):
    if width(text, size) <= max_width:
        return [text]
    punctuation = "，。！？、；：,.!?;:)]}」』》”’"
    if " " in text:
        words = text.split()
        lines, current = [], ""
        for word in words:
            candidate = word if not current else current + " " + word
            if current and width(candidate, size) > max_width:
                lines.append(current)
                current = word
            else:
                current = candidate
        if current:
            lines.append(current)
        return lines
    lines, current = [], ""
    for char in text:
        candidate = current + char
        if current and width(candidate, size) > max_width:
            if char in punctuation and lines:
                lines[-1] += char
            else:
                lines.append(current)
                current = char
        else:
            current = candidate
    if current:
        lines.append(current)
    return lines


def esc(text):
    return html.escape(text, quote=False)


def text(x, y, value, size, color, ident=None, max_width=None, weight=None, anchor=None, family=SANS, line_height=None):
    line_height = line_height or round(size * 1.25)
    lines = wrap(value, max_width, size) if max_width else [value]
    attrs = [f'x="{x}"', f'y="{y}"', f'font-family="{family}"', f'font-size="{size}"', f'fill="{color}"']
    if weight:
        attrs.append(f'font-weight="{weight}"')
    if anchor:
        attrs.append(f'text-anchor="{anchor}"')
    if ident:
        attrs.append(f'id="{ident}"')
    output = []
    for index, line in enumerate(lines):
        line_attrs = attrs.copy()
        line_attrs[1] = f'y="{y + index * line_height}"'
        output.append(f'<text {" ".join(line_attrs)}>{esc(line)}</text>')
    return chr(10).join(output)


def card(x, y, w, h, colors, fill=None, dashed=False):
    stroke = colors["border2"] if dashed else colors["border"]
    dash = ' stroke-dasharray="7 6"' if dashed else ""
    body = fill or colors["card2"] if dashed else colors["card"]
    fold = colors["border2"] if dashed else "#F7C98B"
    return (f'<path d="M{x+16} {y}H{x+w-24}L{x+w} {y+24}V{y+h-16}A16 16 0 0 1 {x+w-16} {y+h}H{x+16}A16 16 0 0 1 {x} {y+h-16}V{y+16}A16 16 0 0 1 {x+16} {y}Z" fill="{body}" stroke="{stroke}" stroke-width="2"{dash}/>'
            f'<path d="M{x+w-24} {y}V{y+24}H{x+w}Z" fill="{fold}" stroke="{stroke}" stroke-width="2" stroke-linejoin="round"/>')


def arrow(x1, x2, y, color, caption=None, caption_lines=None, caption_size=18):
    result = f'<path d="M{x1} {y}H{x2-12}M{x2} {y}l-13 -7.5v15Z" stroke="{color}" stroke-width="3" stroke-linecap="round" fill="none"/>'
    if caption or caption_lines:
        lines = caption_lines or [caption]
        line_height = round(caption_size * 1.25)
        first_y = y - 20 - (len(lines) - 1) * line_height / 2
        for index, line in enumerate(lines):
            result += text((x1 + x2) / 2, first_y + index * line_height, line, caption_size, "#53637D", anchor="middle")
    return result


def lifecycle(copy, colors):
    title_size, note_size = 36.0, 25.0
    reg_w = min(400, max(320, width(copy["register"], title_size) + 48, width(copy["register_note"], note_size) + 48))
    exp_w = min(320, max(230, width(copy["expire"], title_size) + 48, width(copy["expire_note"], note_size) + 48))
    bury_w = min(400, max(300, width(copy["bury"], title_size) + 48, width(copy["bury_note"], note_size) + 48))
    right_w = min(400, max(320, width(copy["resurrect"], title_size) + 76, width(copy["resurrect_note"], note_size) + 48, width(copy["purge"], title_size) + 76, width(copy["purge_note"], note_size) + 48))
    gap, top, card_h = 42, 112, 220
    x1, x2 = 48, 48 + reg_w + gap
    x3, x4 = x2 + exp_w + gap, x2 + exp_w + gap + bury_w + gap
    canvas_w, canvas_h = int(x4 + right_w + 48), 570
    title_x, star_x = x4 + 24, x4 + 24 + width(copy["resurrect"], title_size) + 12
    cmd_size = 22.0
    svg = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{canvas_w}" height="{canvas_h}" viewBox="0 0 {canvas_w} {canvas_h}" role="img" aria-labelledby="title desc">', f'<title id="title">{esc(copy["title"])}</title>', f'<desc id="desc">{esc(copy["life_desc"])}</desc>', '<defs><linearGradient id="spark" x1="0" y1="1" x2="1" y2="0"><stop stop-color="#0876FF"/><stop offset=".55" stop-color="#16B8FA"/><stop offset=".62" stop-color="#FFE052"/><stop offset="1" stop-color="#FFB329"/></linearGradient></defs>', f'<rect x="1" y="1" width="{canvas_w-2}" height="{canvas_h-2}" rx="28" fill="{colors["bg"]}" stroke="{colors["border"]}" stroke-width="2"/>', f'<g font-family="{SANS}">', text(52, 64, copy["title"], title_size, colors["muted"], ident="section", weight="600")]
    for x, w, title_key, note_key, cmd in [(x1, reg_w, "register", "register_note", "obit add debug.log 2h"), (x2, exp_w, "expire", "expire_note", "obit list"), (x3, bury_w, "bury", "bury_note", "obit graveyard")]:
        svg.append(card(x, top, w, card_h, colors))
        svg.append(text(x + 24, top + 48, copy[title_key], title_size, colors["ink"], ident=title_key, weight="700", max_width=w - 48))
        command_width = max(132, width(cmd, cmd_size) + 26)
        svg.append(f'<rect x="{x+20}" y="{top+92}" width="{command_width}" height="36" rx="8" fill="{colors["chip"]}"/>')
        svg.append(text(x + 33, top + 116, cmd, cmd_size, colors["blue"], family=MONO))
        svg.append(text(x + 24, top + 165, copy[note_key], note_size, colors["muted"], ident=note_key, max_width=w - 48))
    svg.append(arrow(x1 + reg_w, x2, top + 72, colors["blue"], caption_size=note_size))
    svg.append(arrow(x2 + exp_w, x3, top + 72, colors["blue"], copy["reap"], caption_size=note_size))
    svg.append(arrow(x3 + bury_w, x4, top + 72, colors["blue"], caption_size=note_size))
    svg.append(card(x4, 28, right_w, card_h, colors))
    svg.append(text(title_x, 80, copy["resurrect"], title_size, colors["ink"], ident="resurrect", weight="700", max_width=right_w-48))
    svg.append(f'<path id="resurrect-star" d="M17.6 9.4c.3 5.3 1.2 6.5 5.2 7-4 .6-4.9 2-5.2 7.1-.6-5.1-1.3-6.4-5.5-7.1 4.2-.6 4.9-1.8 5.5-7Z" fill="url(#spark)" transform="translate({star_x} 67) scale(1.55) translate(-17.5 -16.45)"/>')
    svg.append(f'<rect x="{x4+20}" y="112" width="{max(199, width("obit resurrect 3", cmd_size)+26)}" height="36" rx="8" fill="{colors["chip"]}"/>')
    svg.append(text(x4 + 33, 136, "obit resurrect 3", cmd_size, colors["blue"], family=MONO))
    svg.append(text(x4 + 24, 182, copy["resurrect_note"], note_size, colors["muted"], ident="resurrect-note", max_width=right_w - 48))
    svg.append(card(x4, 270, right_w, card_h, colors, dashed=True))
    svg.append(text(x4 + 24, 322, copy["purge"], title_size, colors["ink"], ident="purge", weight="700", max_width=right_w-48))
    svg.append(f'<rect x="{x4+20}" y="354" width="150" height="36" rx="8" fill="{colors["chip"]}"/>')
    svg.append(text(x4 + 33, 378, "obit purge", cmd_size, colors["blue"], family=MONO))
    svg.append(text(x4 + 24, 428, copy["purge_note"], note_size, colors["muted"], ident="purge-note", max_width=right_w - 48))
    bottom_y = 535
    svg.append(text(52, bottom_y, copy["before"], note_size, colors["muted"], ident="before"))
    before_w = width(copy["before"], note_size) + 10
    extend_x = 52 + before_w
    svg.append(f'<rect x="{extend_x}" y="510" width="145" height="36" rx="8" fill="{colors["chip"]}"/>')
    svg.append(text(extend_x + 13, 534, "obit extend", cmd_size, colors["blue"], family=MONO))
    note_x = extend_x + 157
    svg.append(text(note_x, 534, copy["extend_note"], note_size, colors["muted"], ident="extend-note"))
    spare_x = note_x + width(copy["extend_note"], note_size) + 14
    svg.append(f'<rect x="{spare_x}" y="510" width="134" height="36" rx="8" fill="{colors["chip"]}"/>')
    svg.append(text(spare_x + 13, 534, "obit spare", cmd_size, colors["blue"], family=MONO))
    svg.append(text(spare_x + 145, 534, copy["spare_note"], note_size, colors["muted"], ident="spare-note"))
    svg.append('</g></svg>')
    return chr(10).join(svg)


def delete_guard(copy, colors):
    title_size, note_size = 36.0, 25.0
    gap = 280
    card_w = [400, 400, 400]
    x1, x2, x3 = 48, 48 + card_w[0] + gap, 48 + card_w[0] + gap + card_w[1] + gap
    h, y, canvas_h = 220, 96, 360
    canvas_w = int(x3 + card_w[2] + 48)
    bridge_lines = wrap(copy["guard_bridge"], gap - 24, note_size)
    svg = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{canvas_w}" height="{canvas_h}" viewBox="0 0 {canvas_w} {canvas_h}" role="img" aria-labelledby="title desc">', f'<title id="title">{esc(copy["guard_title"])}: Obituary</title>', f'<desc id="desc">{esc(copy["guard_desc"])}</desc>', '<defs><linearGradient id="spark" x1="0" y1="1" x2="1" y2="0"><stop stop-color="#0876FF"/><stop offset=".55" stop-color="#16B8FA"/><stop offset=".62" stop-color="#FFE052"/><stop offset="1" stop-color="#FFB329"/></linearGradient></defs>', f'<rect x="1" y="1" width="{canvas_w-2}" height="{canvas_h-2}" rx="28" fill="{colors["bg"]}" stroke="{colors["border"]}" stroke-width="2"/>', f'<g font-family="{SANS}">']
    specs=[(x1,card_w[0],"guard_title","guard_first","rm -rf build",colors["warn"]),(x2,card_w[1],"guard_equiv","guard_second","obit rm -rf build",colors["blue"]),(x3,card_w[2],"guard_res","guard_res_note","obit resurrect <id>",colors["blue"])]
    cmd_size=max(18,note_size*.9)
    for x,w,title_key,note_key,cmd,cmd_color in specs:
        svg.append(card(x,y,w,h,colors)); svg.append(text(x+24,y+52,copy[title_key],title_size,colors["ink"],ident=title_key,weight="700",max_width=w-48))
        if title_key == "guard_res":
            star_x=x+24+width(copy[title_key],title_size)+12; svg.append(f'<path id="guard-star" d="M17.6 9.4c.3 5.3 1.2 6.5 5.2 7-4 .6-4.9 2-5.2 7.1-.6-5.1-1.3-6.4-5.5-7.1 4.2-.6 4.9-1.8 5.5-7Z" fill="url(#spark)" transform="translate({star_x} {y+40}) scale(1.55) translate(-17.5 -16.45)"/>')
        command_width=max(156,width(cmd,cmd_size)+26); svg.append(f'<rect x="{x+20}" y="{y+92}" width="{command_width}" height="36" rx="8" fill="{colors["chip"]}"/>'); svg.append(text(x+33,y+120,cmd,cmd_size,cmd_color,family=MONO)); svg.append(text(x+24,y+165,copy[note_key],note_size,colors["muted"],ident=note_key,max_width=w-48))
    svg.append(arrow(x1+card_w[0],x2,180,colors["muted2"],caption_lines=bridge_lines,caption_size=note_size)); svg.append(arrow(x2+card_w[1],x3,180,colors["blue"],caption_size=note_size)); svg.append('</g></svg>')
    return chr(10).join(svg)


strings = json.loads((HERE / "strings.json").read_text(encoding="utf-8"))
for language, copy in strings.items():
    target = HERE / language
    target.mkdir(exist_ok=True)
    for theme, colors in THEMES.items():
        (target / f"lifecycle-{theme}.svg").write_text(lifecycle(copy, colors), encoding="utf-8")
        (target / f"delete-guard-{theme}.svg").write_text(delete_guard(copy, colors), encoding="utf-8")
print("generated", len(strings) * 4, "SVG files")
