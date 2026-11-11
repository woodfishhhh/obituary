"""Regenerate docs/assets/demo/demo-{dark,light}.gif on Windows (Python 3.11+).

From the repository root in Git Bash: python scripts/demo/generate.py
From WSL: python.exe scripts/demo/generate.py
Requires native Windows Go and Git for Windows; installs nothing. Optional:
  --agg C:/path/to/agg.exe   reuse the checksum-verified renderer

The real CLI is built into default %TEMP%; five user/config directories are
redirected for every obit call. Evidence is registered with `obit mktemp 2d`
in that isolated ledger, whose path is printed. No obitd is built or installed;
the script stops the isolated daemon in finally. Retained evidence can be
removed after review; its lifespan needs an explicitly run isolated reaper.

Raw command output and exit codes are captured unchanged in local evidence.
Only the display layer replaces the evidence and bootstrap directory prefixes
with C:\\work (C:/work for Bash paths), before wrapping and indenting every
output. The rendered text must contain no local username, AppData or Temp.
Prompts and reading pauses are scripted, not wall-clock timings.
The Claude request is sent to the real hook, never to a real agent or rm.
`obit rm` is silent, so `obit graveyard --here` supplies the real numbered
receipt. The complete hook JSON and commands remain in evidence/transcript.json.

References: internal/e2e/testdata/claude-bash-pre-tool-use.json (hook payload),
https://docs.asciinema.org/manual/asciicast/v2/ (recording format), and
https://github.com/asciinema/agg/releases/tag/v1.9.0 (Windows renderer).
The pinned SHA-256 is the digest published by that release's GitHub Assets API.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import textwrap
import unicodedata
import urllib.request
import uuid


ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / "docs/assets/demo"
AGG_URL = ("https://github.com/asciinema/agg/releases/download/v1.9.0/"
           "agg-x86_64-pc-windows-msvc.exe")
AGG_SHA256 = "810baf5506e74ca65d8ed85be3db58791086c8b7b0a17c9018d7fede473f0055"
RESET = "\x1b[0m"
# Explicit ANSI palettes keep blue prompts and green receipts legible on white.
THEMES = {
    "dark": "0d1117,e6edf3,484f58,ff7b72,7ee787,d29922,79c0ff,d2a8ff,76e3ea,e6edf3",
    "light": "ffffff,1f2328,24292f,cf222e,1a7f37,9a6700,0969da,8250df,1b7c83,6e7781",
}


def write_json(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n",
                    encoding="utf-8")


def roots_snapshot():
    result = {}
    for name in ("C:/.obituary", "D:/.obituary", "D:/.obituary/graveyard"):
        path = Path(name)
        result[name] = sorted(p.name for p in path.iterdir()) if path.exists() else None
    result["TEMP/obit-tests-*"] = sorted(
        p.name for p in Path(os.environ["TEMP"]).glob("obit-tests-*"))
    return result


class Cast:
    def __init__(self, *temporary_roots):
        self.events = []
        self.time = 0.0
        self.pages = []
        self.rules = []
        self.aliases = []
        for root in temporary_roots:
            self.aliases.extend([(str(root).replace("/", "\\"), "C:\\work"),
                                 (str(root).replace("\\", "/"), "C:/work")])

    def display(self, text):
        for actual, neutral in self.aliases:
            text = text.replace(actual, neutral)
        return text.replace("\r\n", "\n")

    def emit(self, text):
        self.events.append([round(self.time, 3), "o", text.replace("\n", "\r\n")])

    def page(self, at, title):
        self.time = at
        self.pages.append(["  讣告 Obituary  /  " + title, "", ""])
        self.emit("\x1b[2J\x1b[H\x1b[?25l\x1b[1;34m"
                  "  讣告 Obituary" + RESET + "  /  " + title + "\n")
        self.rules.append(len(self.events))
        self.emit("")  # Fill the rule after measuring all four pages.

    def command(self, command, output, color=0, wrap=False):
        command, output = self.display(command), self.display(output).rstrip("\n")
        if wrap:
            output = "\n".join(textwrap.fill(line, width=72, break_long_words=False,
                                           break_on_hyphens=False) for line in output.split("\n"))
        output = "\n".join("  " + line for line in output.split("\n")) if output else ""
        self.pages[-1].extend(["  demo $ " + command, *output.splitlines(), ""])
        self.emit("  \x1b[34mdemo $ " + RESET + "\x1b[?25h")
        for i in range(0, len(command), 2):
            self.time += 0.045
            self.emit(command[i:i + 2])
        self.time += 0.12
        self.emit("\x1b[?25l\n")
        if output:
            self.emit(f"\x1b[{color}m" + output + RESET + "\n")
        self.emit("\n")
        self.time += 0.55

    def save(self, path):
        # Leave room for the final newline so it cannot scroll the title away.
        # Empty lines supply bottom padding; keep two columns at both sides.
        width = max(sum(2 if unicodedata.east_asian_width(c) in "WF" else 1 for c in line)
                    for page in self.pages for line in page) + 2
        height = max(len(page) for page in self.pages) + 1
        for index in self.rules:
            self.events[index][2] = "  " + "─" * (width - 4) + "\r\n\r\n"
        rendered = re.sub(r"\x1b\[[0-?]*[ -/]*[@-~]", "", "".join(e[2] for e in self.events))
        forbidden = {"AppData", "Temp", os.environ.get("USERNAME", ""),
                     Path(os.environ["USERPROFILE"]).name} - {""}
        if any(token.casefold() in rendered.casefold() for token in forbidden):
            raise AssertionError("Private path fragment in rendered text; GIF rendering refused")
        header = {"version": 2, "width": width, "height": height,
                  "title": "Obituary: rm -rf, then resurrect --last",
                  "env": {"TERM": "xterm-256color", "SHELL": "bash"}}
        lines = [header, *self.events]
        path.write_text("\n".join(json.dumps(x, ensure_ascii=False) for x in lines)
                        + "\n", encoding="utf-8")
        path.with_suffix(".display.txt").write_text(rendered, encoding="utf-8")
        return {"columns": width, "rows": height, "page_rows": [len(p) for p in self.pages],
                "privacy_checked": True}


def record(obit, evidence, run):
    project = evidence / "project"
    project.mkdir()
    (project / "build").mkdir()
    (project / "src").mkdir()
    (project / "README.md").write_text("# Demo project\n", encoding="utf-8")
    (project / "src/app.js").write_text('console.log("hello");\n', encoding="utf-8")
    files = {"app.js": b'console.log("hello");\n',
             "app.css": b"body { color: #126cff; }\n",
             "manifest.json": '{"version":"1.0.0","message":"文件还在"}\n'.encode()}
    for name, content in files.items():
        (project / "build" / name).write_bytes(content)
    shutil.copytree(project / "build", evidence / "before")
    bash = Path(os.environ["ProgramFiles"]) / "Git/bin/bash.exe"

    def shell(command):
        return run([bash, "--noprofile", "--norc", "-c", command], project)

    cast = Cast(obit.parent, evidence)
    cast.page(0, "01 / 04  删除之前")
    cast.command("ls -F", shell("/usr/bin/ls -F"))
    cast.command("ls build", shell("/usr/bin/ls build"))
    before = shell("/usr/bin/cat build/manifest.json")
    cast.command("cat build/manifest.json", before)

    payload = {"hook_event_name": "PreToolUse", "session_id": "obit-demo",
               "cwd": str(project), "tool_name": "Bash",
               "tool_input": {"command": "rm -rf build"}}
    reply = run([obit, "hook", "claude"], project, json.dumps(payload))
    decision = json.loads(reply)["hookSpecificOutput"]
    if decision["permissionDecision"] != "deny":
        raise RuntimeError("The real hook did not deny rm -rf build")
    if not (project / "build").is_dir():
        raise RuntimeError("The hook changed the demo files")
    write_json(evidence / "hook-input.json", payload)
    write_json(evidence / "hook-output.json", json.loads(reply))
    cast.page(5.5, "02 / 04  agent 请求删除 · 真实 Claude hook")
    cast.command("rm -rf build", "PreToolUse: " + decision["permissionDecision"]
                 + "\n\n" + decision["permissionDecisionReason"], color=31, wrap=True)

    cast.page(12.5, "03 / 04  按建议下葬 · 墓地保留 7 天")
    cast.command("obit rm -rf build", run([obit, "rm", "-rf", "build"], project))
    if (project / "build").exists():
        raise RuntimeError("build still exists after obit rm")
    cast.command("obit graveyard --here", run([obit, "graveyard", "--here"], project))
    graves = json.loads(run([obit, "graveyard", "--here", "--json"], project))
    if len(graves["burials"]) != 1 or graves["pendingCount"] != 0:
        raise RuntimeError("Expected exactly one committed burial")
    cast.command("ls -F", shell("/usr/bin/ls -F"))

    cast.page(19, "04 / 04  一条命令 · 全部复活")
    cast.command("obit resurrect --last", run([obit, "resurrect", "--last"], project), 32)
    cast.command("ls build", shell("/usr/bin/ls build"))
    after = shell("/usr/bin/cat build/manifest.json")
    cast.command("cat build/manifest.json", after)
    comparison = "/usr/bin/diff -r ../before build && echo '3 files identical'"
    cast.command("diff -r ../before build && echo '3 files identical'", shell(comparison), 32)
    restored = {p.name: p.read_bytes() for p in (project / "build").iterdir()}
    if restored != files or before != after:
        raise RuntimeError("Restored content differs from the original")
    if json.loads(run([obit, "graveyard", "--json"], project))["burials"]:
        raise RuntimeError("A burial remains after resurrection")
    if cast.time > 27:
        raise RuntimeError("Demo exceeds its 30-second reading budget")
    layout = cast.save(evidence / "demo.cast")
    return {**layout, "restored_files": len(files), "all_bytes_equal": True,
            "hook_decision": decision["permissionDecision"],
            "burial_id": graves["burials"][0]["id"], "last_event": cast.events[-1][0]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agg", type=Path, help="Existing official agg v1.9.0 Windows binary")
    args = parser.parse_args()
    if os.name != "nt":
        parser.error("Use native Windows Python (python.exe from WSL)")
    base = Path(os.environ["TEMP"]) / ("obit-demo-bootstrap-" + uuid.uuid4().hex[:8])
    base.mkdir()
    before = roots_snapshot()
    env = {k: v for k, v in os.environ.items()
           if not k.startswith(("OBIT_", "GIT_"))
           and k not in ("BASH_ENV", "ENV", "CDPATH", "CLAUDE_CODE_SESSION_ID", "CODEX_THREAD_ID")}
    for key, name in {"OBIT_HOME": "ledger", "OBIT_CONFIG_DIR": "config",
                      "OBIT_USER_HOME": "user", "CLAUDE_CONFIG_DIR": "claude",
                      "CODEX_HOME": "codex"}.items():
        path = base / name
        path.mkdir()
        env[key] = str(path)
    env.update(OBIT_SESSION_ID="obit-demo", OBIT_GUARD_RM="1")
    (base / "config/config.toml").write_text('[explorer]\nenabled = false\n', encoding="utf-8")
    transcript = []

    def run(argv, cwd, input_text=None):
        result = subprocess.run([str(a) for a in argv], cwd=cwd, env=env,
                                input=input_text, capture_output=True, text=True,
                                encoding="utf-8", creationflags=subprocess.CREATE_NO_WINDOW)
        transcript.append({"argv": [str(a) for a in argv], "cwd": str(cwd),
                           "exit_code": result.returncode, "stdout": result.stdout,
                           "stderr": result.stderr})
        result.check_returncode()
        return result.stdout

    obit = base / "obit.exe"
    evidence = base
    try:
        run(["go.exe", "build", "-o", obit, "./cmd/obit"], ROOT)
        try:
            evidence = Path(run([obit, "mktemp", "2d", "--name", "obit-demo"], base).strip())
        except subprocess.CalledProcessError:
            evidence = Path(os.environ["TEMP"]) / ("obit-demo-" + uuid.uuid4().hex[:8])
            evidence.mkdir()
            print("obit mktemp failed; evidence uses a mkdir-created TEMP directory")
        write_json(evidence / "roots-before.json", before)
        write_json(evidence / "environment.json", {k: v for k, v in env.items()
                   if k.startswith("OBIT_") or k in ("CLAUDE_CONFIG_DIR", "CODEX_HOME")})
        agg = args.agg.resolve() if args.agg else evidence / "agg.exe"
        if not args.agg:
            urllib.request.urlretrieve(AGG_URL, agg)
        digest = hashlib.sha256(agg.read_bytes()).hexdigest()
        if digest != AGG_SHA256:
            raise RuntimeError(f"agg SHA-256 mismatch: {digest}")
        write_json(evidence / "renderer.json", {"url": AGG_URL, "sha256": digest,
                   "release": "https://github.com/asciinema/agg/releases/tag/v1.9.0",
                   "version": run([agg, "--version"], ROOT).strip()})
        summary = record(obit, evidence, run)
        OUTPUT.mkdir(parents=True, exist_ok=True)
        for theme, palette in THEMES.items():
            target = OUTPUT / f"demo-{theme}.gif"
            run([agg, "--theme", palette, "--font-size", "18",
                 "--font-family", "Consolas,Microsoft YaHei", "--line-height", "1.4",
                 "--fps-cap", "20", "--idle-time-limit", "30", "--last-frame-duration", "3",
                 evidence / "demo.cast", target], ROOT)
            if target.stat().st_size > 2_000_000:
                raise RuntimeError(f"{target.name} exceeds 2 MB")
            summary[target.name] = target.stat().st_size
        write_json(evidence / "summary.json", summary)
        print(json.dumps(summary, ensure_ascii=False, indent=2))
    finally:
        if obit.exists():
            run([obit, "daemon", "stop"], base)
        write_json(evidence / "transcript.json", transcript)
        after = roots_snapshot()
        write_json(evidence / "roots-after.json", after)
        print(f"Evidence: {evidence}\nIsolated ledger: {base / 'ledger'}")
        for name in ("C:/.obituary", "D:/.obituary", "D:/.obituary/graveyard"):
            if after[name] != before[name]:
                raise RuntimeError(f"Volume-root directory changed: {name}")


if __name__ == "__main__":
    main()
