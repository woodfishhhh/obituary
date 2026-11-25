"""Linux fixture for the KF5 service-menu MIME/count contract; does not start Dolphin."""
import configparser
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import sys


def matches(entry, kinds):
    # KFileItemActions checks URL count, then includes/excludes every item by MIME.
    count = len(kinds)
    if not count or not int(entry.get("X-KDE-MinNumberOfUrls", "0")) <= count <= int(entry.get("X-KDE-MaxNumberOfUrls", str(count))):
        return False
    types = entry["MimeType"].strip(";").split(";")
    excluded = entry.get("ExcludeServiceTypes", "").split(",")
    return all(k not in excluded and (k in types or "all/all" in types or (k == "text/plain" and any(t in types for t in ("all/allfiles", "application/octet-stream")))) for k in kinds)


def run(exe, root):
    root.mkdir(mode=0o700)  # refuse reused or non-isolated fixtures
    # No companion daemon beside this binary: only the action under test can change the ledger.
    exe = Path(shutil.copy2(exe, root / "obit"))
    home = root / "user"
    data = root / "data"
    xdg = home / ".local/share"
    env = dict(os.environ, HOME=str(home), OBIT_USER_HOME=str(home), OBIT_HOME=str(data), OBIT_CONFIG_DIR=str(root/"config"),
               XDG_DATA_HOME=str(xdg), XDG_CONFIG_HOME=str(home/".config"), CODEX_HOME=str(home/".codex"), OBIT_TELEMETRY="0", OBIT_LANG="en")
    folder = home / "Downloads/example folder"
    folder.mkdir(parents=True)
    selected, sibling = folder / "selected file.txt", folder / "untouched.txt"
    selected.write_text("selected"); sibling.write_text("sibling")
    def obit(*args): return subprocess.check_output([exe, *args], cwd=home, env=env, text=True)
    obit("init", "-g", "--agent", "codex", "--context-menu", "--yes")
    menus = []
    for path in sorted((xdg/"kio/servicemenus").glob("obituary-*.desktop")):
        cfg = configparser.ConfigParser(interpolation=None)
        cfg.read(path, encoding="utf-8")
        menus.append((cfg["Desktop Entry"], cfg["Desktop Action obituary"]))
    def actions(kinds): return [a for entry,a in menus if matches(entry,kinds)]
    file, directory = "text/plain", "inode/directory"
    # Dolphin gives the same one directory URL to KIO for background and single-directory contexts.
    for name, kinds in [("background", [directory]), ("one selected folder", [directory]), ("no URLs", [])]:
        available = actions(kinds)
        assert not available, (name, [a["Name"] for a in available])
    labels = {"Add to Obituary · 1 day", "Add to Obituary · 7 days", "Add to Obituary · 30 days", "Bury now (recoverable)", "Resurrect the most recent burial"}
    for kinds,expected in [([file], labels), ([file,file], labels), ([file,directory], labels), ([directory,directory], labels | {"Set folder policy"})]:
        eligible = [a["Name"] for a in actions(kinds)]
        assert sorted(eligible) == sorted(expected), (kinds, eligible)
    def invoke(kinds, paths, label):
        count = 0
        for a in actions(kinds):
            if a["Name"] != label: continue
            # The fixture executable and env use simple absolute paths; selected paths remain separate argv entries.
            argv = shlex.split(a["Exec"]); i = argv.index("%F"); argv[i:i+1] = map(str,paths)
            subprocess.run(argv,cwd=home,env=env,check=True); count += 1
        return count
    assert invoke([file],[selected],"Add to Obituary · 7 days") == 1
    state = json.loads(obit("list", "--json")); assert [r["path"] for r in state] == [str(selected)], state
    def snapshot(): return {p.relative_to(data):p.read_bytes() for p in data.rglob("*") if p.is_file()}
    before = snapshot()
    for action in labels | {"Set folder policy"}: assert invoke([directory],[folder],action) == 0
    assert before == snapshot() and selected.read_text()=="selected" and sibling.read_text()=="sibling" and sorted(folder.iterdir())==sorted([selected,sibling])
    print("PASS: background 0 / single directory 0 / single file 5 / mixed selection 5 / two directories 6; only the selected file was tracked; background left files and ledger unchanged.")


if __name__ == "__main__":
    run(Path(sys.argv[1]).resolve(), Path(sys.argv[2]).resolve())
