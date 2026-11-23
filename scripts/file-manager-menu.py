"""Exercise the real file manager on the hosted Xvfb session; retain every menu frame."""
import csv
import io
import os
from pathlib import Path
import subprocess as sp
import sys
import time

manager, folder = sys.argv[1:]
assert os.environ.get("GITHUB_ACTIONS") == "true" and os.environ.get("DISPLAY")
out = Path(os.environ["EVIDENCE"])


def view(stage, phrases, timeout=10):
    image = out / f"{manager}-{stage}.png"
    until = time.monotonic() + timeout
    while time.monotonic() < until:
        sp.run(["import", "-window", "root", str(image)], check=True)
        raw = sp.check_output(["tesseract", str(image), "stdout", "--psm", "11", "tsv"], stderr=sp.DEVNULL, text=True)
        lines = {}
        for word in csv.DictReader(io.StringIO(raw), delimiter="\t", quoting=csv.QUOTE_NONE):
            text = word["text"].strip().lower()
            if text:
                lines.setdefault((word["block_num"], word["par_num"], word["line_num"]), []).append((text, word))
        for phrase in phrases:
            for line in lines.values():
                label = " ".join(w[0] for w in line)
                if phrase in label:
                    word = line[len(line)//2][1]
                    return phrase, str(int(word["left"]) + int(word["width"])//2), str(int(word["top"]) + int(word["height"])//2)
        time.sleep(.25)
    raise RuntimeError(f"{manager}: missing {phrases} in {stage}: {[x[0] for line in lines.values() for x in line]}")


def click(stage, phrases, button):
    phrase, x, y = view(stage, phrases)
    sp.run(["xdotool", "mousemove", x, y], check=True)
    if button:
        sp.run(["xdotool", "click", button], check=True)
    time.sleep(.7)
    return phrase


with (out / f"{manager}.log").open("w") as log:
    sp.Popen([manager.lower(), "--new-window", folder], stdout=log, stderr=log)
    click("file", ["sample.txt", "sample"], "3")
    if manager == "Nautilus":
        click("scripts", ["scripts"], "1")
        click("obituary", ["obituary"], "1")
    else:
        first = click("context", ["obituary", "actions"], None)
        if first == "actions":
            click("actions", ["obituary"], None)
    click("select", ["7 days", "7days"], "1")
