"""Exercise the real file manager on the hosted Xvfb session; retain every menu frame."""
import csv
import io
import os
from pathlib import Path
import subprocess as sp
import sys
import time
import pyatspi

manager, folder = sys.argv[1:]
assert os.environ.get("GITHUB_ACTIONS") == "true" and os.environ.get("DISPLAY")
out = Path(os.environ["EVIDENCE"])


def accessible(phrases):
    def walk(node, depth):
        if depth > 18:
            return
        try:
            label = (node.name or "").lower().replace("_", "").replace("&", "")
            for phrase in phrases:
                if phrase in label and node.getState().contains(pyatspi.STATE_SHOWING):
                    box = node.queryComponent().getExtents(pyatspi.DESKTOP_COORDS)
                    if 0 <= box.x < 1280 and 0 <= box.y < 960 and box.width > 0 and box.height > 0:
                        return phrase, str(box.x + box.width//2), str(box.y + box.height//2)
            for child in node:
                match = walk(child, depth+1)
                if match: return match
        except Exception as exc:
            if depth == 0: print(f"AT-SPI: {exc}", flush=True)
        return None
    for app in pyatspi.Registry.getDesktop(0):
        if manager.lower() in (app.name or "").lower():
            result = walk(app, 0)
            if result: return result


def view(stage, phrases, timeout=10):
    image = out / f"{manager}-{stage}.png"
    until = time.monotonic() + timeout
    while time.monotonic() < until:
        sp.run(["import", "-window", "root", str(image)], check=True)
        hit = accessible(phrases)
        if hit: return hit
        scaled=sp.check_output(["convert",str(image),"-resize","200%","-colorspace","Gray","png:-"])
        raw = sp.check_output(["tesseract", "stdin", "stdout", "--psm", "11", "tsv"],input=scaled,stderr=sp.DEVNULL).decode()
        lines = {}
        for word in csv.DictReader(io.StringIO(raw), delimiter="\t", quoting=csv.QUOTE_NONE):
            text = word["text"].strip().lower()
            if text:
                lines.setdefault((word["block_num"], word["par_num"], word["line_num"]), []).append((text, word))
        for phrase in phrases:
            for line in lines.values():
                label = " ".join(w[0] for w in line)
                if phrase in label:
                    word = next((w for text,w in line if phrase.split()[0] in text),line[len(line)//2][1])
                    return phrase, str((int(word["left"]) + int(word["width"])//2)//2), str((int(word["top"]) + int(word["height"])//2)//2)
        time.sleep(.25)
    raise RuntimeError(f"{manager}: missing {phrases} in {stage}: {[x[0] for line in lines.values() for x in line]}")


def click(stage, phrases, button):
    phrase, x, y = view(stage, phrases)
    sp.run(["xdotool", "mousemove", x, y], check=True)
    if button:
        sp.run(["xdotool", "click", button], check=True)
    time.sleep(.7)
    return phrase


def verify_background(log):
    # An explicitly empty directory cannot contain a selected item. Require a readable Qt popup, not negative OCR.
    empty = Path(folder).parent / "Empty-background-fixture"
    empty.mkdir()
    def ledger():
        return sp.check_output(["obit", "list", "--json"])
    before = ledger()
    sp.Popen(["dolphin", "--new-window", str(empty)], stdout=log, stderr=log)
    time.sleep(2)
    sp.run(["xdotool", "key", "Menu"], check=True)
    view("empty-background", ["create new"])
    def menus(node):
        if node.getRole() in (pyatspi.ROLE_MENU, pyatspi.ROLE_POPUP_MENU) and node.getState().contains(pyatspi.STATE_SHOWING):
            yield [(child.name or "").replace("&", "").strip().lower() for child in node if child]
        for child in node:
            if child: yield from menus(child)
    available = [names for app in pyatspi.Registry.getDesktop(0) if "dolphin" in (app.name or "").lower() for names in menus(app) if "create new" in names and "properties" in names]
    assert len(available) == 1, ("background menu not identified", available)
    names = available[0]
    assert not any("obituary" in n or n in ("actions", "bury now (recoverable)", "set folder policy", "resurrect the most recent burial") for n in names), names
    (out / "Dolphin-background-items.txt").write_text("\n".join(names))
    sp.run(["xdotool", "key", "Escape"], check=True)
    assert ledger() == before and empty.is_dir() and not list(empty.iterdir())


with (out / f"{manager}.log").open("w") as log:
    if manager == "Dolphin": verify_background(log)
    sp.Popen([manager.lower(), "--new-window", "--select", str(Path(folder)/"Sample.txt")], stdout=log, stderr=log)
    time.sleep(2)
    try:
        click("file", ["sample.txt"], "3")
    except RuntimeError:
        # --select already selected the isolated sample, so open its keyboard context menu.
        sp.run(["xdotool","key","Menu"],check=True)
    time.sleep(.8)
    if manager == "Nautilus":
        click("scripts", ["scripts"], "1")
        click("obituary", ["obituary"], "1")
        click("select", ["7 days", "7days"], "1")
    else:
        # Dolphin exposes the same services on its background, where %F is the directory.
        try:
            view("selection-check", ["rename"], timeout=2)
        except RuntimeError:
            sp.run(["xdotool", "key", "Escape", "ctrl+a", "shift+F10"], check=True)
            view("selection-check", ["rename"])
        first = click("context", ["obituary", "actions"], "1")
        if first == "actions":
            click("actions", ["obituary"], "1")
        click("select", ["7 days", "7days"], "1")
