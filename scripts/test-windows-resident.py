"""Exercise Task Scheduler restart semantics on a disposable interactive runner."""

import csv
import ctypes
import datetime as dt
import json
import os
import subprocess
import sys
import tempfile
import time
import xml.etree.ElementTree as ET
from ctypes import wintypes
from pathlib import Path
from xml.sax.saxutils import escape


def command(*argv):
    return subprocess.check_output(argv, text=True, errors="replace").strip()


def wait(check, seconds):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        value = check()
        if value:
            return value
        time.sleep(0.4)
    raise AssertionError(f"timeout after {seconds}s waiting for {check.__name__}")


def terminate_verified_process(receipt, daemon, code):
    # Pin the handle and match the creation time before forcibly ending a PID.
    kernel = ctypes.WinDLL("kernel32", use_last_error=True)
    kernel.OpenProcess.restype = wintypes.HANDLE
    handle = kernel.OpenProcess(0x1001, False, receipt["pid"])
    if not handle:
        raise ctypes.WinError(ctypes.get_last_error())
    handle = wintypes.HANDLE(handle)
    try:
        times = [ctypes.c_uint64() for _ in range(4)]
        if not kernel.GetProcessTimes(handle, *(ctypes.byref(v) for v in times)):
            raise ctypes.WinError(ctypes.get_last_error())
        size = wintypes.DWORD(32768)
        path = ctypes.create_unicode_buffer(size.value)
        if not kernel.QueryFullProcessImageNameW(handle, 0, path, ctypes.byref(size)):
            raise ctypes.WinError(ctypes.get_last_error())
        assert times[0].value == receipt["created"] and Path(path.value).samefile(daemon), receipt
        if not kernel.TerminateProcess(handle, wintypes.UINT(code)):
            raise ctypes.WinError(ctypes.get_last_error())
    finally:
        kernel.CloseHandle(handle)


def probe(root, name, daemon, cli, trigger, code):
    home, cfg = root / "ledger", root / "config"
    cfg.mkdir()
    (cfg / "config.toml").write_text("[update]\ncheck = false\n[telemetry]\nenabled = false\n")
    os.environ.update(OBIT_HOME=str(home), OBIT_CONFIG_DIR=str(cfg), OBIT_TELEMETRY="0")
    sid = next(csv.reader([command("whoami", "/user", "/fo", "csv", "/nh")]))[1]
    args = subprocess.list2cmdline(["--supervise", "--home", str(home), "--config-dir", str(cfg)])
    # A one-shot clock trigger exercises the normal scheduler path without logging off the VM.
    when = (dt.datetime.now() + dt.timedelta(seconds=12)).isoformat(timespec="seconds")
    clock = f"<TimeTrigger><StartBoundary>{when}</StartBoundary></TimeTrigger>" if trigger == "clock" else ""
    xml = f"""<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <Triggers>{clock}<LogonTrigger><UserId>{escape(sid)}</UserId></LogonTrigger></Triggers>
  <Principals><Principal id="Author"><UserId>{escape(sid)}</UserId><LogonType>InteractiveToken</LogonType><RunLevel>LeastPrivilege</RunLevel></Principal></Principals>
  <Settings><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy><DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries><StopIfGoingOnBatteries>false</StopIfGoingOnBatteries><ExecutionTimeLimit>PT0S</ExecutionTimeLimit><RestartOnFailure><Interval>PT1M</Interval><Count>3</Count></RestartOnFailure></Settings>
  <Actions Context="Author"><Exec><Command>{escape(str(daemon))}</Command><Arguments>{escape(args)}</Arguments></Exec></Actions>
</Task>"""
    definition = root / "resident.xml"
    definition.write_text(xml, encoding="utf-16")
    receipt = home / "obitd.process.json"

    def read():
        try:
            return json.loads(receipt.read_bytes())
        except (FileNotFoundError, json.JSONDecodeError):
            return {}

    if trigger == "manual":
        command(cli, "daemon", "enable", "--resident")
        installed = ET.fromstring((home / "obitd.login-task.xml").read_text(encoding="utf-8"))
        name = installed.findtext("{*}RegistrationInfo/{*}URI")
        assert name and installed.findtext("{*}Settings/{*}ExecutionTimeLimit") == "PT0S"
        assert installed.findtext("{*}Actions/{*}Exec/{*}Command") == str(daemon)
        command(cli, "daemon", "stop")
        command(cli, "daemon", "start")
    else:
        command("schtasks", "/create", "/tn", name, "/xml", definition)
    try:
        print(command("schtasks", "/query", "/tn", name, "/xml"), flush=True)
        first = wait(read, 60)
        parent = json.loads((home / "obitd.supervisor.json").read_bytes())
        assert not parent["closed"] and first["pid"] != parent["pid"], (first, parent)
        print("supervisor", parent, flush=True)
        print("first", first, flush=True)
        started = time.monotonic()
        terminate_verified_process(first, daemon, code)

        def restarted():
            r = read()
            return r if r and not r["closed"] and r["created"] > first["created"] and r["pid"] != first["pid"] else None

        second = wait(restarted, 100)
        print("restart", second, "after", time.monotonic() - started, "seconds", flush=True)
        command(cli, "daemon", "stop")
        time.sleep(75)
        closed = read()
        assert closed["closed"] and closed["pid"] == second["pid"] and not closed.get("error"), closed
        print("intentional stop remained stopped", closed)
    finally:
        print(command("schtasks", "/query", "/tn", name, "/v", "/fo", "list"), flush=True)
        if (home / "obitd.log").exists():
            print((home / "obitd.log").read_text(encoding="utf-8"))
        if trigger == "manual":
            command(cli, "daemon", "disable")
            result = subprocess.run(["schtasks", "/query", "/tn", name, "/HRESULT"], capture_output=True)
            assert result.returncode & 0xffffffff == 0x80070002, result
        else:
            command("schtasks", "/delete", "/tn", name, "/f")


if __name__ == "__main__":
    if sys.platform != "win32" or os.getenv("RUNNER_ENVIRONMENT") != "github-hosted" or os.getenv("GITHUB_ACTIONS") != "true":
        sys.exit("requires a disposable GitHub-hosted Windows runner")
    base = Path(tempfile.mkdtemp(prefix="obit-resident-", dir=os.environ["RUNNER_TEMP"]))
    daemon, cli = base / "obitd.exe", base / "obit.exe"
    command("go", "build", "-ldflags=-H=windowsgui", "-o", daemon, "./cmd/obitd")
    command("go", "build", "-o", cli, "./cmd/obit")
    print(dt.datetime.now(dt.timezone.utc), base, flush=True)
    probe(base, base.name, daemon, cli, sys.argv[1], int(sys.argv[2], 0))
