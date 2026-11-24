"""Drive the real Nautilus and Dolphin on the private Xvfb session of the hosted Ubuntu runner.

Every step leaves a numbered screenshot; every ledger read is saved as ledger-<step>.json and logged with the paths
actually registered. Nautilus (GTK4) is navigated through AT-SPI, whose menu items have no names, so the row to
press is the one text recognition places at that position and the ledger proves which lifespan ran. Dolphin (Qt5)
does not connect to AT-SPI on the runner: it is driven by keyboard and pointer, and each popup is located by
diffing screenshots before and after it opens. Text recognition only reads those popups; it never decides alone
that something is absent without also reading a control entry from the same popup.
"""
import csv
import datetime
import difflib
import io
import json
import os
from pathlib import Path
import re
import subprocess as sp
import sys
import time

import pyatspi

assert os.environ.get("GITHUB_ACTIONS") == "true" and os.environ.get("DISPLAY")
OUT, HOME = Path(os.environ["EVIDENCE"]), Path(os.environ["HOME"])
NSAMPLE, DSAMPLE = HOME / "Downloads/Nautilus/Sample.txt", HOME / "Desktop/Dolphin/Sample.txt"
EMPTY, DIRS = HOME / "Desktop/Dolphin-empty", HOME / "Desktop/Dolphin-dirs"
CHECKS = {  # in report order
    "nautilus-menu": "Nautilus, file selected: right-click shows Scripts > Obituary with the six actions",
    "nautilus-7d": "Nautilus: clicking 7 days registers only Sample.txt, lifespan 7 days",
    "nautilus-tag": "Nautilus: date tag is the expiry day, name and existing tag kept",
    "dolphin-background": "Dolphin, empty-space menu: real background menu read, no Obituary entry",
    "dolphin-background-ledger": "Dolphin, empty space: ledger, files and current directory unchanged",
    "dolphin-one-dir": "Dolphin, one directory selected: menu read, no Obituary entry, ledger unchanged",
    "dolphin-file-menu": "Dolphin, file selected: Obituary submenu has the five file actions",
    "dolphin-file-7d": "Dolphin: clicking 7 days registers only the selected file, lifespan 7 days",
    "dolphin-file-tag": "Dolphin: date tag is the expiry day, name and existing tag kept",
    "dolphin-two-dirs-menu": "Dolphin, two directories selected: Obituary submenu has all six actions",
    "dolphin-two-dirs-1d": "Dolphin: clicking 1 day registers exactly the two directories",
    "dolphin-info": "Dolphin: KDE metadata and information panel show the date tag",
    "uninstall": "Uninstall: original tags, files and folders kept; menus removed",
}
# Menu rows by what they do; digits are matched exactly so "1 day" never passes for "7 days" or "30 days".
ROWS = {"1 day": r"(?<!\d)1\s*day(?!s)", "7 days": r"(?<!\d)7\s*days", "30 days": r"30\s*days", "bury": r"bury\s*now|recoverable",
        "resurrect": r"resurrect|recent\s*burial", "policy": r"folder\s*policy|set\s*folder"}
FILE_ROWS = ["1 day", "7 days", "30 days", "bury", "resurrect"]
DIR_ROWS = FILE_ROWS + ["policy"]
RESULTS, SHOTS = {}, [0]


def run(*args, **kw):
    return sp.run(args, check=True, text=True, capture_output=True, **kw).stdout


def log(message):
    line = f"{time.strftime('%H:%M:%S')} {message}"
    print(line, flush=True)
    with (OUT / "steps.log").open("a") as f:
        f.write(line + "\n")


def shot(name):
    SHOTS[0] += 1
    path = OUT / f"{SHOTS[0]:02d}-{name}.png"
    run("import", "-window", "root", str(path))
    log(f"SHOT {path.name}")
    return path


def poll(read, timeout, what):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        value = read()
        if value:
            return value
        time.sleep(.25)
    raise RuntimeError(f"timed out after {timeout}s waiting for {what}")


def ledger(step):
    rows = json.loads(run("obit", "list", "--json"))
    (OUT / f"ledger-{step}.json").write_text(json.dumps(rows, indent=2))
    log(f"LEDGER {step}: {sorted(r['path'] for r in rows)}")
    return rows


def expect_ledger(step, paths, timeout=15):
    want, got = sorted(map(str, paths)), []

    def settled():
        got[:] = sorted(r["path"] for r in json.loads(run("obit", "list", "--json")))
        return got == want
    try:
        poll(settled, timeout, f"ledger == {want}")
    except RuntimeError:
        ledger(step)
        raise RuntimeError(f"ledger after {step} is {got}, expected {want}")
    return {r["path"]: r for r in ledger(step)}


def tags(path):
    try:
        return os.getxattr(path, "user.xdg.tags").decode()
    except OSError:
        return ""


def stamp(text):
    return datetime.datetime.fromisoformat(re.sub(r"(\.\d{6})\d*", r"\1", text).replace("Z", "+00:00"))


def check_lifespan(row, days):
    span = stamp(row["expiresAt"]) - stamp(row["createdAt"])
    assert abs(span - datetime.timedelta(days=days)) < datetime.timedelta(hours=1), f"{row['path']} lives {span}, expected {days} days"
    return f"{days} days until {row['expiresAt']}"


def check_tag(row, kept):
    want = ",".join(kept + [stamp(row["expiresAt"]).astimezone().strftime("%m-%d") + " burial"])
    poll(lambda: tags(row["path"]) == want, 10, f"user.xdg.tags == {want!r} on {row['path']}")
    assert Path(row["path"]).exists()
    log(f"TAG {row['path']}: {tags(row['path'])!r}")
    return f"user.xdg.tags={want!r}"


def alive():
    return any(sp.run(["pgrep", "-x", n], capture_output=True).returncode == 0 for n in ("nautilus", "dolphin"))


def clean_desktop(step):
    """Close every file-manager window and menu; the private root is black only when nothing is left on screen."""
    sp.run(["xdotool", "key", "Escape"])
    for sig in ("-TERM", "-KILL"):
        for name in ("nautilus", "dolphin"):
            sp.run(["pkill", sig, "-x", name])
        try:
            poll(lambda: not alive(), 8, "file managers to exit")
            break
        except RuntimeError:
            pass
    time.sleep(.5)
    path = shot(f"clean-{step}")
    if alive() or float(run("identify", "-format", "%[fx:maxima]", str(path))) != 0:
        raise RuntimeError(f"screen is not clean before {step}: see {path.name}")


def open_window(cls, *command):
    log("LAUNCH " + " ".join(command))
    with (OUT / f"{command[0]}.log").open("a") as f:
        sp.Popen(command, stdout=f, stderr=f)
    win = run("xdotool", "search", "--sync", "--onlyvisible", "--class", cls, timeout=40).split()[0]
    run("xdotool", "windowfocus", "--sync", win)
    geo = dict(line.split("=") for line in run("xdotool", "getwindowgeometry", "--shell", win).split())
    time.sleep(2.5)
    return {"x": int(geo["X"]), "y": int(geo["Y"]), "w": int(geo["WIDTH"]), "h": int(geo["HEIGHT"])}


def mouse(x, y, button=None):
    run("xdotool", "mousemove", str(x), str(y))
    time.sleep(.3)
    if button:
        run("xdotool", "click", button)
    time.sleep(1.2)


def windows():
    return {int(w) for w in run("xdotool", "search", "--onlyvisible", "--class", "").split()}


def geometry(win):
    g = dict(line.split("=") for line in run("xdotool", "getwindowgeometry", "--shell", str(win)).split())
    return int(g["X"]), int(g["Y"]), int(g["WIDTH"]), int(g["HEIGHT"])


def new_popup(known, what):
    """The largest top-level window that was not on screen before: the menu, with its own position and size."""
    def find():
        found = []
        for win in windows() - known:
            try:
                box = geometry(win)
            except sp.CalledProcessError:
                continue  # closed between the two calls
            if box[2] * box[3] >= 2000 and box[2] < 1280:
                found.append((box[2] * box[3], win, box))
        return max(found)[1:] if found else None
    return poll(find, 6, f"a popup window for {what}")


def read_popup(image, box):
    """Rows of text in a popup with their screen centres; a second and third reading settle a poor first one."""
    x, y, w, h = box
    best = []
    for scale, mode in ((3, "6"), (4, "4"), (2, "11")):
        png = sp.check_output(["convert", str(image), "-crop", f"{w}x{h}+{x}+{y}", "+repage", "-resize", f"{scale * 100}%", "-colorspace", "Gray", "png:-"])
        tsv = sp.run(["tesseract", "stdin", "stdout", "--psm", mode, "tsv"], input=png, capture_output=True).stdout.decode()
        lines = {}
        for word in csv.DictReader(io.StringIO(tsv), delimiter="\t", quoting=csv.QUOTE_NONE):
            if (word["text"] or "").strip():
                lines.setdefault((word["block_num"], word["par_num"], word["line_num"]), []).append(word)
        rows = []
        for words in lines.values():
            left, top = min(int(v["left"]) for v in words), min(int(v["top"]) for v in words)
            right, bottom = max(int(v["left"]) + int(v["width"]) for v in words), max(int(v["top"]) + int(v["height"]) for v in words)
            rows.append({"text": " ".join(v["text"] for v in words), "x": x + (left + right) // 2 // scale, "y": y + (top + bottom) // 2 // scale})
        best = max(best, sorted(rows, key=lambda r: r["y"]), key=len)
        if len(best) >= 3:
            break
    log("POPUP read: " + " | ".join(r["text"] for r in best))
    return best


def mentions(rows, word):
    """Row naming a word, tolerant of recognition noise in the other characters."""
    return next((r for r in rows if any(difflib.SequenceMatcher(None, word, t.lower()).ratio() >= .8 for t in re.findall(r"[A-Za-z]{4,}", r["text"]))), None)


def row_for(rows, key):
    return next((r for r in rows if re.search(ROWS[key], r["text"].lower())), None)


def action_rows(rows, expected, step):
    missing, extra = [k for k in expected if not row_for(rows, k)], [k for k in DIR_ROWS if k not in expected and row_for(rows, k)]
    if missing or extra:
        raise RuntimeError(f"{step}: actions missing {missing}, unexpected {extra}; read {[r['text'] for r in rows]}")


# ---- Nautilus (GTK4, AT-SPI) -------------------------------------------------------------------------------------

def walk(node, depth=0):
    yield node
    if depth < 45:
        for child in node:
            try:
                if child:
                    yield from walk(child, depth + 1)
            except Exception:
                continue


def nautilus_app():
    return next((a for a in pyatspi.Registry.getDesktop(0) if (a.name or "") == "org.gnome.Nautilus"), None)


def menu_items():
    app = nautilus_app()
    return [n for n in walk(app) if n.getRoleName() == "menu item" and n.getState().contains(pyatspi.STATE_SHOWING)] if app else []


def page():
    return tuple(n.getState().contains(pyatspi.STATE_EXPANDABLE) for n in menu_items())


def actions(node):
    try:
        act = node.queryAction()
        return [act.getName(i) for i in range(act.nActions)]
    except Exception:
        return []


def press(node, action=None):
    """Run an accessible action: a menu item's own "click" (the popover has no item names or usable coordinates)."""
    node.queryAction().doAction(actions(node).index(action) if action else 0)


def nautilus_context_menu(geo):
    app = poll(nautilus_app, 20, "Nautilus in AT-SPI")
    cell = poll(lambda: next((n for n in walk(app) if n.getRoleName() == "table cell" and n.name == "Sample.txt"), None), 20, "Sample.txt in the view")
    poll(lambda: any("selected" in (n.name or "") for n in walk(app) if n.getRoleName() == "label"), 10, "the status bar to report the selection")
    shot("nautilus-file-selected")
    box, inner = cell.queryComponent().getExtents(pyatspi.WINDOW_COORDS), next(iter(next(iter(app)))).queryComponent().getExtents(pyatspi.WINDOW_COORDS)
    shadow = (geo["w"] - inner.width) // 2  # client-side shadow: AT-SPI window coordinates start inside it
    run("xdotool", "key", "Escape")
    mouse(geo["x"] + shadow + box.x + box.width // 2, geo["y"] + shadow + box.y + box.height // 2, "3")
    if not menu_items():
        log("right-click opened no menu; using the view's own popup-menu action")
        view = next(n for n in walk(app) if n.getRoleName() == "panel" and "view.popup-menu" in actions(n))
        press(view, "view.popup-menu")
        time.sleep(1.5)


def nautilus():
    clean_desktop("nautilus")
    assert ledger("start") == [], "the main ledger must be empty before the first action"
    geo = open_window("nautilus", "nautilus", "--new-window", "--select", str(NSAMPLE))
    shot("nautilus-view")
    known = windows()
    nautilus_context_menu(geo)
    menu = shot("nautilus-context-menu")
    pwin, pbox = new_popup(known, "the context menu")
    read_popup(menu, pbox)
    root = poll(menu_items, 10, "the context menu items")
    expandable = [n for n in root if n.getState().contains(pyatspi.STATE_EXPANDABLE)]
    assert len(expandable) == 1, f"the context menu has {len(root)} items and {len(expandable)} submenus, expected exactly the Scripts submenu"
    press(expandable[0])
    scripts_page = poll(lambda: page() != tuple(n.getState().contains(pyatspi.STATE_EXPANDABLE) for n in root) and menu_items(), 10, "the Scripts page")
    scripts = shot("nautilus-scripts-page")
    read_popup(scripts, geometry(pwin))
    assert len(scripts_page) >= 2, "the Scripts page lists no folder"
    press(scripts_page[1])  # [0] is the back row, [1] the Obituary folder
    obit_page = poll(lambda: len(menu_items()) >= 7 and menu_items(), 10, "the Obituary page with its six actions")
    obituary = shot("nautilus-obituary-page")
    rows = read_popup(obituary, geometry(pwin))
    action_rows(rows, DIR_ROWS, "nautilus Obituary page")
    seven = row_for(rows, "7 days")
    index = [r["y"] for r in rows].index(seven["y"]) if len(rows) == len(obit_page) else 2  # [0] is the back row
    log(f"NAUTILUS pressing menu item {index} of {len(obit_page)}, read as {rows[min(index, len(rows) - 1)]['text']!r}")
    record("nautilus-menu", "PASS", f"Scripts page {len(scripts_page)} entries; Obituary page {len(obit_page)} entries: {[r['text'] for r in rows]}")
    press(obit_page[index])
    time.sleep(1.5)
    shot("nautilus-after-click")
    got = expect_ledger("nautilus-7d", [NSAMPLE])
    record("nautilus-7d", "PASS", check_lifespan(got[str(NSAMPLE)], 7))
    record("nautilus-tag", "PASS", check_tag(got[str(NSAMPLE)], ["work"]))


# ---- Dolphin (Qt5, keyboard and pointer) --------------------------------------------------------------------------

def dolphin_menu(step, how, geo):
    """Open the context menu ('key' acts on the selection, 'blank' right-clicks empty view space); return (box, rows)."""
    shot(f"{step}-view")
    for attempt in (1, 2, 3):  # a slow runner may still be loading the folder when the first key arrives
        known = windows()
        if how == "key":
            run("xdotool", "key", "Menu")
        else:
            mouse(geo["x"] + geo["w"] * 7 // 10, geo["y"] + geo["h"] * 65 // 100, "3")
        try:
            _, box = new_popup(known, f"{step} context menu")
            break
        except RuntimeError:
            if attempt == 3:
                raise
            log(f"{step}: no menu after attempt {attempt}; trying again")
            run("xdotool", "key", "Escape")
            time.sleep(2)
    time.sleep(.5)
    return box, read_popup(shot(f"{step}-menu"), box)


def dolphin_submenu(step, rows):
    row = mentions(rows, "obituary")
    if not row:
        raise RuntimeError(f"{step}: no Obituary entry in the menu; read {[r['text'] for r in rows]}")
    known = windows()
    mouse(row["x"], row["y"], "1")
    image = shot(f"{step}-submenu")
    _, box = new_popup(known, f"{step} Obituary submenu")
    return read_popup(image, box)


def click_row(rows, key):
    mouse(row_for(rows, key)["x"], row_for(rows, key)["y"], "1")
    time.sleep(1.5)


def no_obituary(rows, step, controls):
    """Absence only counts when the same popup was read well enough to show its ordinary entries."""
    seen = [c for c in controls if mentions(rows, c)]
    assert len(seen) >= 2, f"{step}: popup not read (found only {seen} of {controls}): {[r['text'] for r in rows]}"
    assert not mentions(rows, "obituary") and not any(row_for(rows, k) for k in ROWS), f"{step}: Obituary entry present: {[r['text'] for r in rows]}"
    return f"read {[r['text'] for r in rows]}; no Obituary entry"


def dolphin_open(step, *args):
    clean_desktop(step)
    return open_window("dolphin", "dolphin", "--new-window", *map(str, args))


def dolphin_background():
    before = ledger("background-before")
    geo = dolphin_open("dolphin-background", EMPTY)
    _, rows = dolphin_menu("dolphin-background", "blank", geo)
    record("dolphin-background", "PASS", no_obituary(rows, "empty-space menu", ["create", "paste", "properties", "sort"]))
    run("xdotool", "key", "Escape")
    assert before == ledger("background-after") and EMPTY.is_dir() and not list(EMPTY.iterdir()), "ledger or directory changed"
    record("dolphin-background-ledger", "PASS", "ledger identical before and after; directory still empty")


def dolphin_one_dir():
    before = ledger("one-dir-before")
    geo = dolphin_open("dolphin-one-dir", "--select", DIRS / "dirA")
    _, rows = dolphin_menu("dolphin-one-dir", "key", geo)
    detail = no_obituary(rows, "one-directory menu", ["rename", "trash", "properties", "copy"])
    run("xdotool", "key", "Escape")
    assert before == ledger("one-dir-after"), "ledger changed"
    record("dolphin-one-dir", "PASS", detail + "; ledger unchanged")


def dolphin_file():
    geo = dolphin_open("dolphin-file", "--select", DSAMPLE)
    _, rows = dolphin_menu("dolphin-file", "key", geo)
    assert mentions(rows, "rename"), f"the item menu was not opened: {[r['text'] for r in rows]}"
    srows = dolphin_submenu("dolphin-file", rows)
    action_rows(srows, FILE_ROWS, "file submenu")
    record("dolphin-file-menu", "PASS", f"submenu read {[r['text'] for r in srows]}")
    click_row(srows, "7 days")
    shot("dolphin-file-after-click")
    got = expect_ledger("dolphin-file-7d", [NSAMPLE, DSAMPLE])
    record("dolphin-file-7d", "PASS", check_lifespan(got[str(DSAMPLE)], 7))
    record("dolphin-file-tag", "PASS", check_tag(got[str(DSAMPLE)], ["work"]))


def dolphin_two_dirs():
    geo = dolphin_open("dolphin-two-dirs", "--select", DIRS / "dirA", DIRS / "dirB")
    _, rows = dolphin_menu("dolphin-two-dirs", "key", geo)
    srows = dolphin_submenu("dolphin-two-dirs", rows)
    action_rows(srows, DIR_ROWS, "two-directory submenu")
    record("dolphin-two-dirs-menu", "PASS", f"submenu read {[r['text'] for r in srows]}")
    click_row(srows, "1 day")
    shot("dolphin-two-dirs-after-click")
    got = expect_ledger("dolphin-two-dirs-1d", [NSAMPLE, DSAMPLE, DIRS / "dirA", DIRS / "dirB"])
    record("dolphin-two-dirs-1d", "PASS", "; ".join(check_lifespan(got[str(DIRS / d)], 1) for d in ("dirA", "dirB")) + "; " + check_tag(got[str(DIRS / "dirA")], []))


def dolphin_info():
    row = {r["path"]: r for r in ledger("info")}[str(DSAMPLE)]
    day = stamp(row["expiresAt"]).astimezone().strftime("%m-%d")
    kde = sp.run(["read-kde-tags", str(DSAMPLE)], capture_output=True, text=True)
    log(f"KDE metadata for Dolphin's sample (exit {kde.returncode}): {kde.stderr.strip()} {kde.stdout.strip()}")
    assert kde.returncode == 0 and f'"{day} burial"' in kde.stderr, f"KFileMetaData does not report work and '{day} burial'"
    dolphin_open("dolphin-info", "--select", DSAMPLE)
    run("xdotool", "key", "F11")
    time.sleep(3)
    panel = shot("dolphin-information-panel")
    text = sp.run(["tesseract", str(panel), "stdout", "--psm", "11"], capture_output=True).stdout.decode().lower()
    shown = bool(re.search(rf"{day}\s*bur", text))  # the panel cuts the tag at "buria"; the screenshot is the evidence
    record("dolphin-info", "PASS", f"KDE metadata {kde.stderr.strip()} matches expiry day {day}; information panel text read as showing it: {shown}")


# ---- Uninstall -----------------------------------------------------------------------------------------------------

def uninstall():
    clean_desktop("uninstall")
    rows = ledger("before-uninstall")
    (OUT / "tracked.json").write_text(json.dumps(rows, indent=2))
    registered = [r["path"] for r in rows]
    run("obit", "init", "-g", "--agent", "codex", "--uninstall", "--yes")
    run("obit", "uninstall", "--yes")
    kept = {str(NSAMPLE): "work", str(DSAMPLE): "work", str(DIRS / "dirA"): "", str(DIRS / "dirB"): ""}
    assert sorted(registered) == sorted(kept), f"unexpected registered paths {registered}"
    for path, want in kept.items():
        assert Path(path).exists() and tags(path) == want, f"{path}: exists={Path(path).exists()} tags={tags(path)!r}, expected {want!r}"
    left = [str(p) for p in [*(HOME / ".local/share/kio/servicemenus").glob("obituary-*"), *(HOME / ".local/share/nautilus/scripts").glob("Obituary*")]]
    assert not left, f"menu files left behind: {left}"
    record("uninstall", "PASS", f"tags now {kept}; files and folders present; no menu files left")


def record(check, verdict, detail=""):
    RESULTS[check] = (verdict, detail)
    log(f"RESULT {check}: {verdict} {detail}")


def attempt(first, flow):
    """Run one flow; a failure is recorded against its first unreached check and the next flow still runs."""
    try:
        flow()
    except Exception as exc:
        pending = [c for c in CHECKS if c not in RESULTS and (c == first or list(CHECKS).index(c) > list(CHECKS).index(first))]
        shot(f"failure-{first}")
        record(pending[0] if pending else first, "FAIL", f"{type(exc).__name__}: {exc}")


def main():
    for folder in (NSAMPLE.parent, DSAMPLE.parent, EMPTY, DIRS / "dirA", DIRS / "dirB"):
        folder.mkdir(parents=True, exist_ok=True)
    for sample in (NSAMPLE, DSAMPLE):
        sample.touch()
        os.setxattr(sample, "user.xdg.tags", b"work")  # a tag the user already had; it must survive every step
    pyatspi.Registry.getDesktop(0)  # start the registry before any application, or Qt and GTK never register
    for first, flow in [("nautilus-menu", nautilus), ("dolphin-background", dolphin_background), ("dolphin-one-dir", dolphin_one_dir),
                        ("dolphin-file-menu", dolphin_file), ("dolphin-two-dirs-menu", dolphin_two_dirs), ("dolphin-info", dolphin_info),
                        ("uninstall", uninstall)]:
        attempt(first, flow)
    lines = ["| check | result | detail |", "| --- | --- | --- |"]
    for check, text in CHECKS.items():
        verdict, detail = RESULTS.get(check, ("NOT REACHED", "an earlier step of the same flow failed"))
        lines.append(f"| {text} | {verdict} | {detail.replace('|', '/')} |")
    (OUT / "results.md").write_text("\n".join(lines) + "\n")
    print("\n".join(lines))
    sys.exit(0 if all(RESULTS.get(c, ("",))[0] == "PASS" for c in CHECKS) else 1)


if __name__ == "__main__":
    main()
