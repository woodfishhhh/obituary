"""Long-running acceptance shared by the three disposable desktop runners."""

from datetime import datetime
import json
import os
from pathlib import Path
import subprocess
import time


def until(check, limit):
    deadline = time.monotonic() + limit
    while time.monotonic() < deadline:
        value = check()
        if value:
            return value
        time.sleep(1)
    raise AssertionError(f"timed out after {limit}s: {check.__name__}")


def measure(cli, root, tag, timeout):
    path = root / (tag + ".txt")
    path.write_bytes(tag.encode())
    item = json.loads(subprocess.check_output([cli, "mark", path, "4s", "--no-daemon", "--json"]))[0]["item"]
    until(lambda: not path.exists(), timeout)

    def burial():
        rows = json.loads(subprocess.check_output([cli, "graveyard", "--json"]))["burials"] or []
        found = [row for row in rows if row["id"] == item["id"]]
        if not found:
            return None
        row, = found
        assert Path(row["path"]) == path and Path(row["grave"]).read_bytes() == tag.encode(), row
        return row

    row = until(burial, 10)
    stamp = lambda text: datetime.fromisoformat(text.replace("Z", "+00:00"))
    lag = (stamp(row["buriedAt"]) - stamp(item["expiresAt"])).total_seconds()
    assert 0 <= lag < timeout, row
    print(json.dumps(dict(probe=tag, expiresAt=item["expiresAt"], buriedAt=row["buriedAt"], delay=lag, grave=row["grave"])), flush=True)


def lifetime(cli, root, home, identity, terminate):
    assert os.environ.get("RUNNER_ENVIRONMENT") == "github-hosted"
    started = time.monotonic()
    current = until(identity, 20)
    # No scheduled runner is installed yet: this burial must come from the resident.
    measure(cli, root, "healthy", 25)
    subprocess.run([cli, "daemon", "enable"], check=True)
    pause = home / "obitd.paused"
    pause.touch(exist_ok=False)
    target = root / "paused.txt"
    target.write_text("paused")
    subprocess.run([cli, "mark", target, "4s", "--no-daemon"], check=True)
    time.sleep(max(0, started + 31 * 60 - time.monotonic()))
    assert current == identity() and target.read_text() == "paused", current
    print("same live process and paused payload after", time.monotonic() - started, "seconds:", current, flush=True)
    pause.unlink()
    until(lambda: not target.exists(), 25)
    for count in range(1, 4):
        at = time.monotonic()
        terminate(current)
        if count < 3:
            current = until(lambda: (v if (v := identity()) != current else None), 100)
            print("failure", count, "replacement", current, "after", time.monotonic() - at, flush=True)
    state = home / "obitd.supervision.json"
    until(lambda: json.loads(state.read_bytes())["Abandoned"], 100)
    assert not identity() and len(json.loads(state.read_bytes())["Failures"]) == 3
    measure(cli, root, "fallback", int(os.getenv("OBIT_FALLBACK_TIMEOUT", "3695")))
    assert not identity(), "a fourth business process started"
