"""Build the README demo SVGs (source) and their motion specs.

Writes demo-{zh,en}-{light,dark}.svg and matching .motion.json next to this
file. Render a GIF with the beautify-github-readme renderer:

    python -I render_motion_gif.py demo-zh-light.svg ../../demo/demo-light.gif \
        --spec demo-zh-light.motion.json

The terminal lines are real `obit` output (paths shortened to a demo folder);
the right column is the graveyard, drawn after the brand tombstone.
"""

import json
from pathlib import Path

HERE = Path(__file__).resolve().parent

THEMES = {
    "light": {
        "bg": "#F5F8FD", "dot": "#DCE5F3", "ink": "#0F1B33", "muted": "#5B6B85",
        "term": "#0E1628", "term_line": "#24324D", "term_text": "#E6ECF5", "term_muted": "#7D8BA5",
        "blue": "#2F7BFF", "yellow": "#FFC83D", "warn": "#FFB547", "ok": "#3DD68C",
        "chip": "#FFFFFF", "chip_line": "#C9D6EC", "stone": "#FFFFFF", "ground": "#E3EBF7",
        "step_dot": "#C3D0E6",
    },
    "dark": {
        "bg": "#0B1120", "dot": "#18233A", "ink": "#E8EEF8", "muted": "#93A2BC",
        "term": "#111A2E", "term_line": "#24324D", "term_text": "#E6ECF5", "term_muted": "#7D8BA5",
        "blue": "#4D8DFF", "yellow": "#FFCB4D", "warn": "#FFB547", "ok": "#3DD68C",
        "chip": "#16213A", "chip_line": "#2C3C5E", "stone": "#121C31", "ground": "#16233B",
        "step_dot": "#33425F",
    },
}

COPY = {
    "zh": {
        "title": "墓地", "sub": "七天内原样放回",
        "steps": ["AI 想 rm -rf", "讣告拦下，换成 obit", "进墓地，编号 #2", "原样复活"],
        "chip_note": "#2 · 7 天内可复活",
        "desc": "agent 执行 rm -rf build 被讣告的删除护栏拦下，改用 obit rm -rf build 后 build 进了墓地，obit resurrect --last 把它原样放回。",
    },
    "en": {
        "title": "Graveyard", "sub": "Back as it was, for 7 days",
        "steps": ["The AI runs rm -rf", "The guard swaps in obit", "Buried as #2", "Resurrected"],
        "chip_note": "#2 · restorable for 7 days",
        "desc": "An agent's rm -rf build is stopped by Obituary's delete guard; obit rm -rf build buries it instead, and obit resurrect --last puts it back as it was.",
    },
}

MONO = "'Cascadia Mono', Consolas, 'JetBrains Mono', monospace"
SANS = {"zh": "'Microsoft YaHei UI', 'Microsoft YaHei', 'PingFang SC', 'Segoe UI', sans-serif",
        "en": "'Segoe UI', -apple-system, 'Helvetica Neue', Arial, sans-serif"}


def esc(text: str) -> str:
    return text.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def svg(lang: str, theme: str) -> str:
    t, c = THEMES[theme], COPY[lang]
    sans = SANS[lang]
    x0, y0 = 76, 166
    # The terminal: each line is its own layer so the renderer can type it in.
    def who(label: str, color: str) -> str:
        return (f'<text x="{x0}" font-family="{MONO}" font-size="20" font-weight="600" fill="{color}">{label}</text>')

    lines = [
        ("line1", y0, f'{who("agent ›", t["yellow"])}<text x="{x0 + 108}" font-family="{MONO}" font-size="24" fill="{t["term_text"]}">rm -rf build</text>'),
        ("line2", y0 + 52, f'<text x="{x0}" font-family="{MONO}" font-size="21" fill="{t["warn"]}">✕ Obituary&apos;s delete guard is on:</text>'
                           f'<text x="{x0 + 26}" y="34" font-family="{MONO}" font-size="21" fill="{t["warn"]}">`obit rm -rf build` provides restorable deletion</text>'),
        ("line3", y0 + 140, f'{who("agent ›", t["yellow"])}<text x="{x0 + 108}" font-family="{MONO}" font-size="24" fill="{t["term_text"]}">obit rm -rf build</text>'),
        ("line4", y0 + 218, f'{who("you ›", t["blue"])}<text x="{x0 + 108}" font-family="{MONO}" font-size="24" fill="{t["term_text"]}">obit resurrect --last</text>'),
        ("line5", y0 + 266, f'<text x="{x0}" font-family="{MONO}" font-size="22" fill="{t["ok"]}">#2 resurrected  C:\\work\\shop\\build</text>'),
    ]
    term = "\n".join(f'    <g id="{i}" transform="translate(0 {y})">{body}</g>' for i, y, body in lines)

    # The graveyard column: the brand tombstone outline with its sparkle.
    gx, gy = 980, 292
    stone = (f'M {gx - 78} {gy + 92} V {gy - 40} Q {gx - 78} {gy - 92} {gx - 26} {gy - 104} '
             f'L {gx + 34} {gy - 104} L {gx + 78} {gy - 60} V {gy + 92} Z')
    fold = f'M {gx + 34} {gy - 104} V {gy - 68} Q {gx + 34} {gy - 60} {gx + 42} {gy - 60} H {gx + 78}'
    spark = (f'M {gx} {gy - 46} C {gx + 4} {gy - 14} {gx + 10} {gy - 8} {gx + 40} {gy - 4} '
             f'C {gx + 10} {gy} {gx + 4} {gy + 6} {gx} {gy + 38} '
             f'C {gx - 4} {gy + 6} {gx - 10} {gy} {gx - 40} {gy - 4} '
             f'C {gx - 10} {gy - 8} {gx - 4} {gy - 14} {gx} {gy - 46} Z')

    steps = "\n".join(
        f'    <g id="step{i + 1}" transform="translate(842 {437 + i * 30})">'
        f'<circle cx="0" cy="-7" r="5" fill="{t["blue"] if i < 3 else t["ok"]}"/>'
        f'<text x="18" font-family="{sans}" font-size="20" fill="{t["ink"]}">{esc(s)}</text></g>'
        for i, s in enumerate(c["steps"]))
    step_dots = "\n".join(
        f'    <circle cx="842" cy="{430 + i * 30}" r="5" fill="{t["step_dot"]}"/>' for i in range(4))

    chip = (f'    <g id="chip">'
            f'<rect x="{gx - 70}" y="{gy + 18}" width="140" height="44" rx="12" fill="{t["chip"]}" stroke="{t["chip_line"]}"/>'
            f'<path d="M {gx - 50} {gy + 32} h 12 l 4 4 h 14 v 16 h -30 Z" fill="{t["yellow"]}"/>'
            f'<text x="{gx - 12}" y="{gy + 47}" font-family="{MONO}" font-size="20" fill="{t["ink"]}">build/</text></g>')

    return f'''<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="560" viewBox="0 0 1200 560" role="img" aria-labelledby="t d">
  <title id="t">讣告 Obituary demo</title>
  <desc id="d">{esc(c["desc"])}</desc>
  <defs>
    <linearGradient id="brand" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="{t["blue"]}"/><stop offset="0.55" stop-color="#7FB2FF"/><stop offset="1" stop-color="{t["yellow"]}"/>
    </linearGradient>
    <pattern id="dots" width="24" height="24" patternUnits="userSpaceOnUse"><circle cx="2" cy="2" r="1.4" fill="{t["dot"]}"/></pattern>
    <clipPath id="frame"><rect width="1200" height="560" rx="28"/></clipPath>
  </defs>
  <g clip-path="url(#frame)">
    <rect width="1200" height="560" fill="{t["bg"]}"/>
    <rect x="780" width="420" height="560" fill="url(#dots)"/>
    <rect x="40" y="40" width="712" height="480" rx="22" fill="{t["term"]}"/>
    <text x="76" y="92" font-family="{MONO}" font-size="18" fill="{t["term_muted"]}">PS C:\\work\\shop&gt;</text>
    <line x1="76" y1="112" x2="716" y2="112" stroke="{t["term_line"]}"/>
    <text x="76" y="486" font-family="{MONO}" font-size="18" fill="{t["term_muted"]}">PS C:\\work\\shop&gt;</text>
    <rect x="252" y="470" width="11" height="20" rx="2" fill="{t["term_muted"]}"/>
    <text x="842" y="102" font-family="{sans}" font-size="30" font-weight="700" fill="{t["ink"]}">{esc(c["title"])}</text>
    <text x="842" y="136" font-family="{sans}" font-size="20" fill="{t["muted"]}">{esc(c["sub"])}</text>
    <ellipse cx="{gx}" cy="{gy + 96}" rx="150" ry="16" fill="{t["ground"]}"/>
    <path d="{stone}" fill="{t["stone"]}" stroke="url(#brand)" stroke-width="7" stroke-linejoin="round"/>
    <path d="{fold}" fill="none" stroke="url(#brand)" stroke-width="5" stroke-linejoin="round"/>
    <path d="{spark}" fill="url(#brand)"/>
{step_dots}
{term}
{chip}
{steps}
  </g>
</svg>
'''


def spec() -> dict:
    reveal = lambda i, s, e: {"id": i, "axis": "x", "start": s, "end": e, "exit": {"start": 6.4, "end": 6.9}}
    return {
        "width": 1200, "fps": 30, "duration": 7.0, "colors": 256, "dither": "none",
        "clip_to_base_alpha": True, "max_size_mb": 2.5,
        "reveals": [
            reveal("line1", 0.30, 0.75), reveal("step1", 0.40, 0.85),
            reveal("line2", 1.00, 1.55), reveal("step2", 1.10, 1.55),
            reveal("line3", 1.90, 2.30), reveal("step3", 2.50, 2.95),
            reveal("line4", 3.50, 3.90), reveal("line5", 4.00, 4.35), reveal("step4", 4.05, 4.50),
        ],
        "layers": [
            {"id": "chip", "enter": {"start": 2.35, "end": 3.15, "from": [-330, -150]},
             "exit": {"start": 4.00, "end": 4.70, "to": [-330, 70]}},
        ],
    }


for lang in COPY:
    for theme in THEMES:
        name = f"demo-{lang}-{theme}"
        (HERE / f"{name}.svg").write_text(svg(lang, theme), encoding="utf-8")
        (HERE / f"{name}.motion.json").write_text(json.dumps(spec(), indent=2) + "\n", encoding="utf-8")
print("written", sorted(p.name for p in HERE.glob("demo-*")))
