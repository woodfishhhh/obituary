"""Check the installed LaunchAgent on a disposable hosted GUI session."""

import json
import os
from pathlib import Path
import plistlib
import re
import signal
import subprocess
import tempfile
import time
from resident_lifetime import lifetime


def command(*args):
    return subprocess.check_output(args, text=True).strip()


def wait(check, limit):
    deadline = time.monotonic() + limit
    while time.monotonic() < deadline:
        result = check()
        if result:
            return result
        time.sleep(0.5)
    raise AssertionError(f"timed out: {check.__name__}")


def probe(root):
    cli, daemon = root / "obit", root / "obitd"
    for binary in (cli, daemon):
        command("go", "build", "-tags", "desktop", "-o", binary, "./cmd/" + binary.name)
    home, cfg = root / "data", root / "config"
    cfg.mkdir()
    (cfg / "config.toml").write_text("[update]\ncheck = false\n[telemetry]\nenabled = false\n")
    os.environ.update(OBIT_USER_HOME=str(root), OBIT_HOME=str(home), OBIT_CONFIG_DIR=str(cfg), OBIT_TELEMETRY="0")
    domain = f"gui/{os.getuid()}"
    label = "site.woodfish.obituary.tray.supervised"
    service = domain + "/" + label
    assert subprocess.run(["launchctl", "print", service], capture_output=True).returncode == 113

    def pid():
        result = subprocess.run(["launchctl", "print", service], text=True, capture_output=True)
        if result.returncode == 113:
            return None
        result.check_returncode()
        detail = result.stdout
        match = re.search(r"^\s*pid = (\d+)$", detail, re.MULTILINE)
        if not match:
            return None
        value = int(match[1])
        executable = command("ps", "-p", str(value), "-o", "comm=")
        assert Path(executable).samefile(daemon), executable
        return value

    try:
        command(cli, "daemon", "enable", "--resident")
        plist = root / "Library" / "LaunchAgents" / (label + ".plist")
        job = plistlib.loads(plist.read_bytes())
        assert job["KeepAlive"] == {"SuccessfulExit": False} and job["ThrottleInterval"] == 60
        assert job["ProgramArguments"][0] == str(daemon) and job["RunAtLoad"]
        if os.environ.get("OBIT_LONG_PROBE") == "true":
            def identity():
                value = pid()
                return (value, command("ps", "-p", str(value), "-o", "lstart=")) if value else None

            def crash(expected):
                assert identity() == expected
                command("launchctl", "kill", "SIGKILL", service)

            lifetime(cli, root, home, identity, crash)
            return
        first = wait(pid, 15)
        started = time.monotonic()
        os.kill(first, signal.SIGKILL)

        def restarted():
            current = pid()
            return current if current != first else None

        second = wait(restarted, 90)
        print(json.dumps(dict(first=first, restarted=second, seconds=time.monotonic()-started)), flush=True)
        command(cli, "daemon", "stop")
        time.sleep(70)
        assert not pid(), "launchd restarted the intentionally stopped tray"
    finally:
        subprocess.run(["launchctl", "bootout", service], capture_output=True)


if __name__ == "__main__":
    if os.uname().sysname != "Darwin" or os.getenv("RUNNER_ENVIRONMENT") != "github-hosted":
        raise SystemExit("requires a disposable hosted macOS runner")
    with tempfile.TemporaryDirectory(dir=os.environ["RUNNER_TEMP"]) as directory:
        probe(Path(directory).resolve())
