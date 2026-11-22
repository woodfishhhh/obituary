"""Exercise the production user unit on a private display of a hosted Ubuntu VM."""

import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import time

import dbus
from resident_lifetime import lifetime


def run(*args):
    return subprocess.check_output(args, text=True).strip()


def wait(check, seconds):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        value = check()
        if value:
            return value
        time.sleep(.5)
    raise AssertionError(f"timeout: {check.__name__}")


def probe(root):
    cli, daemon = root / "obit", root / "obitd"
    for binary in (cli, daemon):
        run("go", "build", "-tags", "desktop", "-o", binary, "./cmd/" + binary.name)
    home, cfg = root / "data", root / "config"
    cfg.mkdir()
    (cfg / "config.toml").write_text("[update]\ncheck=false\n[telemetry]\nenabled=false\n")
    xdg = root / "xdg"
    os.environ.update(OBIT_HOME=str(home), OBIT_CONFIG_DIR=str(cfg), XDG_CONFIG_HOME=str(xdg), OBIT_TELEMETRY="0", EVIDENCE=str(root))
    unit = "obituary-tray.service"
    ctl = ["systemctl", "--user"]
    assert not run(*ctl, "show", unit, "--property=FragmentPath", "--value")
    run(*ctl, "import-environment", "DISPLAY", "XAUTHORITY")
    session = "obit-test-session"
    run("systemd-run", "--user", "--unit=" + session, "--property=Wants=graphical-session.target", "/usr/bin/sleep", "infinity")
    registered = root / "registered"
    host = subprocess.Popen(["/usr/bin/python3", "scripts/test-linux-desktop-session.py", "host"], env=dict(os.environ, HOME=str(root)))
    bus = dbus.SessionBus()
    try:
        wait(lambda: bus.name_has_owner("org.kde.StatusNotifierWatcher"), 10)
        run(cli, "daemon", "enable", "--resident")

        fallback = xdg / "systemd" / "user" / "obituary.timer"
        if not fallback.exists() or "OnCalendar=hourly" not in fallback.read_text():
            raise AssertionError("supervised desktop fallback is not hourly")

        def observed():
            pid = int(run(*ctl, "show", unit, "--property=MainPID", "--value"))
            if not pid or not registered.exists():
                return None
            sender, _ = json.loads(registered.read_text())
            try:
                owner = int(bus.get_object("org.freedesktop.DBus", "/org/freedesktop/DBus").GetConnectionUnixProcessID(sender, dbus_interface="org.freedesktop.DBus"))
            except dbus.exceptions.DBusException as err:
                if err.get_dbus_name() != "org.freedesktop.DBus.Error.NameHasNoOwner":
                    raise
                return None
            if owner != pid:
                return None
            assert Path(f"/proc/{pid}/exe").samefile(daemon)
            return pid

        if os.environ.get("OBIT_LONG_PROBE") == "true":
            def identity():
                pid = observed()
                return (pid, Path(f"/proc/{pid}/stat").read_text().rsplit(")", 1)[1].split()[19]) if pid else None

            def crash(expected):
                assert identity() == expected
                run(*ctl, "kill", "--kill-whom=main", "--signal=SIGKILL", unit)

            lifetime(cli, root, home, identity, crash)
            return
        first = wait(observed, 20)
        started = time.monotonic()
        # Signal the manager-owned main process of this exact unit, not a PID by name.
        run(*ctl, "kill", "--kill-whom=main", "--signal=SIGKILL", unit)
        second = wait(lambda: (p if (p := observed()) != first else None), 95)
        print(json.dumps(dict(first=first, restarted=second, seconds=time.monotonic()-started)), flush=True)
        run(cli, "daemon", "stop")
        time.sleep(70)
        assert not int(run(*ctl, "show", unit, "--property=MainPID", "--value"))
    finally:
        subprocess.run([*ctl, "stop", unit], check=False)
        subprocess.run([*ctl, "stop", session], check=False)
        host.terminate()
        host.wait(timeout=5)


if __name__ == "__main__":
    if os.getenv("RUNNER_ENVIRONMENT") != "github-hosted" or os.getenv("GITHUB_ACTIONS") != "true" or not os.getenv("DISPLAY"):
        raise SystemExit("requires a disposable hosted Ubuntu runner with Xvfb")
    with tempfile.TemporaryDirectory(dir=os.environ["RUNNER_TEMP"]) as tmp:
        probe(Path(tmp).resolve())
