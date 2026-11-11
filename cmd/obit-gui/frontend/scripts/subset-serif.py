"""Subset Noto Serif SC (OFL 1.1) to the characters the GUI's own copy uses.

The serif looks set their headings, notices and figures in this face. The
full font is 20+ MB; the subset keeps Basic Latin, Latin-1, common
punctuation and every CJK character in src/i18n/zh-CN.ts, the Chinese copy
table, so a change of copy cannot leave a character out. Text the user
supplies (notes, Chinese file names) falls back to the next font in the stack.

Run again after changing UI copy:

    uv run --with fonttools --with brotli python scripts/subset-serif.py [NotoSerifSC-VF.ttf]

The source defaults to the installed C:/Windows/Fonts/NotoSerifSC-VF.ttf
(https://github.com/notofonts/noto-cjk, Serif/Variable/TTF).
"""

import pathlib
import re
import sys

from fontTools import subset
from fontTools.ttLib import TTFont

root = pathlib.Path(__file__).resolve().parent.parent
source = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else "C:/Windows/Fonts/NotoSerifSC-VF.ttf")
target = root / "public" / "fonts" / "noto-serif-sc-subset.woff2"

cjk = re.compile(r"[\u3000-\u303f\u3400-\u9fff\uff00-\uffef]")
chars = set(cjk.findall((root / "src" / "i18n" / "zh-CN.ts").read_text(encoding="utf-8")))
# Weekdays, Chinese numerals and month words the code builds at runtime.
chars.update("〇一二三四五六七八九十日月年周星期")

unicodes = [ord(c) for c in chars]
unicodes += list(range(0x20, 0x7F)) + list(range(0xA0, 0x100))
unicodes += list(range(0x2010, 0x2028)) + list(range(0x2030, 0x203B)) + [0x2190, 0x2192, 0x2212]

options = subset.Options()
options.flavor = "woff2"
options.layout_features = ["kern", "liga", "locl", "palt", "halt", "vert", "vrt2", "vpal", "vhal", "tnum", "pnum", "lnum"]
options.hinting = False
options.name_IDs = ["*"]
options.notdef_outline = True

font = TTFont(source)
sub = subset.Subsetter(options)
sub.populate(unicodes=unicodes)
sub.subset(font)
font.flavor = "woff2"
font.save(target)
print(f"{len(chars)} CJK characters, {target.stat().st_size // 1024} KB -> {target.relative_to(root)}")
