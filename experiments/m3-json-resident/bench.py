"""Concurrency gate for the resident single writer with the JSON ledger.

Real `obit hook claude`, real `obit list`, real Git, throw-away homes only.
Two systems are driven side by side and interleaved wave by wave:

  old  the baseline build (origin/main): no resident, a mutex stands in for
       "a background process is present" and `probe replay` drains the queue,
       exactly as the earlier audit did
  new  this branch: a real headless obitd owns the ledger

usage: python bench.py wave  <bin-root> --n 1000 --waves 7 --conc 4,8,16
       python bench.py paired <bin-root> --n 1000 --pairs 21
       python bench.py chaos  <bin-root> --n 1000 --rounds 30 --conc 8
bin-root holds bin-old/ and bin-new/ (obit.exe, obitd.exe, probe.exe).
"""
import argparse
import concurrent.futures as cf
import csv
import ctypes as ct
import hashlib
import json
import math
import os
import platform
import random
import statistics as st
import subprocess as sp
import sys
import tempfile
import threading
import time
import uuid
from pathlib import Path

K = ct.windll.kernel32
K.CreateMutexW.restype = ct.c_void_p
CPU = os.cpu_count()


class Bench:
    def __init__(self, bins: Path, out: Path):
        self.bin = {"old": bins / "bin-old", "new": bins / "bin-new"}
        self.root = Path(tempfile.gettempdir()) / ("obit-resident-bench-" + uuid.uuid4().hex[:8])
        self.root.mkdir()
        self.out = out
        self.repo = self.root / "repo"
        self.repo.mkdir()
        base = {k: v for k, v in os.environ.items() if not k.startswith(("OBIT_", "GIT_", "CLAUDE_", "CODEX_"))}
        base.update(OBIT_TEST_NO_UPDATE="1", OBIT_TELEMETRY="0", DO_NOT_TRACK="1", OBIT_LANG="en",
                    OBIT_TEST_REGISTRY_ROOT=r"Software\obituary-test-resident-bench",
                    GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=str(self.root / "gitconfig"),
                    GIT_CEILING_DIRECTORIES=str(self.root), GIT_TERMINAL_PROMPT="0", PYTHONUTF8="1")
        for key, sub in dict(OBIT_USER_HOME="user", HOME="user", USERPROFILE="user", APPDATA="roaming",
                             LOCALAPPDATA="local", CODEX_HOME="codex", CLAUDE_CONFIG_DIR="claude",
                             OBIT_CONFIG_DIR="config", XDG_CONFIG_HOME="config").items():
            p = self.root / sub
            p.mkdir(exist_ok=True)
            base[key] = str(p)
        (self.root / "config" / "config.toml").write_text(
            '[proposals]\nmode="tiered"\n[explorer]\nenabled=false\n[update]\ncheck=false\n'
            '[agent_shells]\ncodex="bash"\n', encoding="utf8")
        self.base = base
        sp.run(["git", "init", "-q", str(self.repo)], env=base, check=True)
        self.guard()

    def guard(self):
        # Every run proves the data directories are throw-away ones.
        for key in ("OBIT_HOME", "OBIT_CONFIG_DIR", "OBIT_USER_HOME"):
            v = self.base.get(key)
            if v:
                assert Path(v).is_relative_to(self.root), (key, v)

    def env(self, name):
        p = self.root / name / "home"
        p.mkdir(parents=True, exist_ok=True)
        e = dict(self.base, OBIT_HOME=str(p))
        assert Path(e["OBIT_HOME"]).is_relative_to(self.root)
        return e

    def run(self, system, exe, args, env, payload=None, allow=False):
        data = None if payload is None else json.dumps(payload).encode()
        t = time.perf_counter_ns()
        r = sp.run([str(self.bin[system] / (exe + ".exe")), *args], input=data, stdout=sp.PIPE, stderr=sp.PIPE,
                   cwd=self.repo, env=env, creationflags=sp.CREATE_NO_WINDOW, timeout=180)
        ms = (time.perf_counter_ns() - t) / 1e6
        if not allow and r.returncode:
            raise RuntimeError((args, r.returncode, r.stderr[-1500:], r.stdout[-500:]))
        return ms, r.stdout, r.stderr

    def payload(self, sid, path):
        return dict(cwd=str(self.repo), source="approved", session_id=sid, hook_event_name="PostToolUse",
                    tool_name="Write", tool_input={"file_path": str(path)})

    def record(self, name, values):
        a = sorted(values)
        p50, p95 = st.median(a), a[math.ceil(.95 * len(a)) - 1]  # nearest rank
        print(f"{name} n={len(a)} p50={p50:.2f} p95={p95:.2f} max={a[-1]:.2f}", flush=True)
        with open(self.out, "a", encoding="utf8") as f:
            print(json.dumps([name, values]), file=f)


def cpu_times():
    class FT(ct.Structure):
        _fields_ = [("l", ct.c_uint32), ("h", ct.c_uint32)]
    i, k, u = FT(), FT(), FT()
    K.GetSystemTimes(ct.byref(i), ct.byref(k), ct.byref(u))
    v = lambda x: (x.h << 32) | x.l
    return v(i), v(k), v(u)


class CPUWindow:
    """Whole-machine CPU use between two points (kernel time includes idle)."""
    def __enter__(self):
        self.a = cpu_times()
        return self

    def __exit__(self, *_):
        b = cpu_times()
        idle, kern, user = (b[n] - self.a[n] for n in range(3))
        self.pct = 100.0 * (1 - idle / (kern + user)) if kern + user else 0.0


def mutex(e):
    sid = next(csv.reader([sp.check_output(["whoami", "/user", "/fo", "csv", "/nh"], encoding="utf8").strip()]))[1]
    name = "Local\\Obituary-obitd-" + sid + "-" + hashlib.sha256(e["OBIT_HOME"].upper().encode()).hexdigest()[:32]
    h = K.CreateMutexW(None, False, name)
    assert h
    return h


def read_state(home: Path):
    for _ in range(50):
        try:
            return json.loads((home / "store.json").read_bytes())
        except (PermissionError, FileNotFoundError, json.JSONDecodeError):
            time.sleep(.01)
    raise RuntimeError("store.json unreadable")


def queued(home: Path):
    n = len(list((home / "observations").glob("*.json")))
    p = home / "observations.jsonl"
    try:
        n += sum(1 for line in p.read_bytes().splitlines() if line.strip())
    except FileNotFoundError:
        pass
    return n


class System:
    def __init__(self, b: Bench, name, n, tag=""):
        self.b, self.name, self.n = b, name, n
        self.e = b.env(name + tag)
        self.home = Path(self.e["OBIT_HOME"])
        b.run(name, "probe", ["seed", str(n), str(b.repo)], self.e)
        (self.home / "migrated-1.3-auto-off").touch()
        self.expected = n
        self.daemon = None
        if name == "old":
            self.mutex = mutex(self.e)  # a background process "is present", as in the audit
        else:
            self.start()

    def start(self):
        b = self.b
        self.daemon = sp.Popen([b.bin[self.name] / "obitd.exe", "--tray=false", "--home", self.home,
                                "--config-dir", b.base["OBIT_CONFIG_DIR"]], cwd=b.repo, env=self.e,
                               creationflags=sp.CREATE_NO_WINDOW)
        pipe = r"\\.\pipe\obituary-resident-" + hashlib.sha256(str(self.home).encode()).hexdigest()
        for _ in range(500):
            if K.WaitNamedPipeW(pipe, 20):
                return
            assert self.daemon.poll() is None, "obitd exited"
            time.sleep(.02)
        raise RuntimeError("resident endpoint not ready")

    def kill(self):
        """TerminateProcess: no shutdown code runs, like a crash or a power cut of the process."""
        if self.daemon and self.daemon.poll() is None:
            self.daemon.kill()
            self.daemon.wait()

    def drain(self, timeout=90):
        """Block until every accepted write is in the ledger; returns when it is."""
        until = time.monotonic() + timeout
        while True:
            if self.name == "old":
                self.b.run("old", "probe", ["replay"], self.e)
            items = len(read_state(self.home).get("items", []))
            if items == self.expected and queued(self.home) == 0:
                return
            if time.monotonic() > until:
                log = (self.home / "obitd.log")
                raise RuntimeError((self.name, items, self.expected, queued(self.home),
                                    log.read_text(encoding="utf8")[-1200:] if log.exists() else ""))
            time.sleep(.04)

    def stop(self):
        if self.daemon:
            (self.home / "obitd.stop").touch()
            try:
                self.daemon.wait(timeout=15)
            except sp.TimeoutExpired:
                self.daemon.terminate()
                self.daemon.wait()

    def verify(self, paths):
        items = [i for i in read_state(self.home)["items"] if Path(i["path"]).parent == self.b.repo]
        got = {i["path"] for i in items}
        assert len(items) == len(got) == len(paths) and got == {str(p) for p in paths}, (self.name, len(items), len(paths))
        assert all(i["rev"] == 1 for i in items), "an item was registered twice"
        log = self.home / "hook.log"
        assert not log.exists() or not log.read_text(encoding="utf8").strip(), log.read_text(encoding="utf8")[-800:]


def wave(b: Bench, s: System, k, j):
    paths = [b.repo / f"debug-{s.name}-{k}-{j}-{i}.log" for i in range(k)]
    for p in paths:
        p.write_text("x", encoding="utf8")
    barrier = threading.Barrier(k + 2)

    def hook(i):
        barrier.wait()
        time.sleep(.005 * (i % 4))
        return b.run(s.name, "obit", ["hook", "claude"], s.e, b.payload(f"session-{s.name}-{k}-{i}", paths[i]))[0]

    def reader():
        barrier.wait()
        time.sleep(.065)
        return b.run(s.name, "obit", ["list"], s.e)[0]

    with cf.ThreadPoolExecutor(k + 1) as pool, CPUWindow() as cpu:
        fs = [pool.submit(hook, i) for i in range(k)]
        rd = pool.submit(reader)
        t0 = time.perf_counter()
        barrier.wait()
        hooks = [f.result() for f in fs]
        group = (time.perf_counter() - t0) * 1000
        listed = rd.result()
    backlog = queued(s.home)
    s.expected += k
    s.drain()
    drained = (time.perf_counter() - t0) * 1000
    time.sleep(.6)
    return paths, hooks, group, listed, backlog, drained, cpu.pct


def cmd_wave(a):
    b = Bench(Path(a.bins), Path(a.out))
    print("OS", platform.platform(), "CPU", CPU, "root", b.root, "n", a.n, "waves", a.waves, flush=True)
    for k in [int(x) for x in a.conc.split(",")]:
        systems = {nm: System(b, nm, a.n, f"-{k}") for nm in ("old", "new")}
        acc = {nm: dict(hook=[], wave=[], list=[], backlog=[], drain=[], cpu=[]) for nm in systems}
        allp = {nm: [] for nm in systems}
        try:
            for j in range(a.waves):
                for nm in (["old", "new"] if j % 2 == 0 else ["new", "old"]):  # interleave
                    paths, hooks, group, listed, backlog, drained, cpu = wave(b, systems[nm], k, j)
                    allp[nm] += paths
                    m = acc[nm]
                    m["hook"] += hooks; m["wave"].append(group); m["list"].append(listed)
                    m["backlog"].append(backlog); m["drain"].append(drained); m["cpu"].append(cpu)
                    print(f"k={k} wave={j} {nm} wave={group:.0f} list={listed:.0f} hookmax={max(hooks):.0f} backlog={backlog} drain={drained:.0f} cpu={cpu:.0f}%", flush=True)
            for nm in systems:
                systems[nm].verify(allp[nm])
                for key, vs in acc[nm].items():
                    b.record(f"n={a.n}/k={k}/{nm}/{key}", vs)
        finally:
            for s in systems.values():
                s.stop()
        time.sleep(2)
    print("done", b.root)


def cmd_paired(a):
    b = Bench(Path(a.bins), Path(a.out))
    print("OS", platform.platform(), "CPU", CPU, "root", b.root, "n", a.n, flush=True)
    es = {nm: b.env(nm) for nm in ("old", "new")}
    for nm, e in es.items():
        b.run(nm, "probe", ["seed", str(a.n), str(b.repo)], e)
    samples = {nm: [] for nm in es}
    for i in range(a.pairs):
        for nm in (["old", "new"] if i % 2 == 0 else ["new", "old"]):
            samples[nm].append(b.run(nm, "probe", ["due"], es[nm])[0])
    for nm in es:
        b.record(f"n={a.n}/direct-write/{nm}", samples[nm])
    b.record(f"n={a.n}/direct-write/new-minus-old", [y - x for x, y in zip(samples["old"], samples["new"])])


def cmd_chaos(a):
    """Hooks keep arriving while the resident is killed and restarted at random moments."""
    b = Bench(Path(a.bins), Path(a.out))
    print("OS", platform.platform(), "CPU", CPU, "root", b.root, "n", a.n, "rounds", a.rounds, flush=True)
    s = System(b, "new", a.n, "-chaos")
    rng = random.Random(a.seed)
    allp, hook_ms, kills = [], [], 0
    for rnd in range(a.rounds):
        paths = [b.repo / f"debug-chaos-{rnd}-{i}.log" for i in range(a.conc)]
        for p in paths:
            p.write_text("x", encoding="utf8")
        plan = dict(hook=[rng.uniform(0, .8) for _ in paths], die=rng.uniform(0, 1.2), down=rng.uniform(0, .5))
        stop = threading.Event()

        def killer():
            nonlocal kills
            time.sleep(plan["die"])
            s.kill()
            kills += 1
            time.sleep(plan["down"])
            s.start()
        barrier = threading.Barrier(len(paths) + 1)

        def hook(i):
            barrier.wait()
            time.sleep(plan["hook"][i])
            return b.run("new", "obit", ["hook", "claude"], s.e, b.payload(f"session-chaos-{rnd}-{i}", paths[i]))[0]
        with cf.ThreadPoolExecutor(len(paths) + 1) as pool:
            kt = pool.submit(killer)
            fs = [pool.submit(hook, i) for i in range(len(paths))]
            barrier.wait()
            hook_ms += [f.result() for f in fs]
            kt.result()
        allp += paths
        s.expected += len(paths)
        print(f"round {rnd} killed at {plan['die']:.2f}s, down {plan['down']:.2f}s", flush=True)
    s.drain(timeout=180)
    s.verify(allp)
    b.record(f"chaos/n={a.n}/hooks={len(allp)}/kills={kills}/hook", hook_ms)
    print("chaos ok: every write registered exactly once (rev 1), queue empty, no hook.log entries", flush=True)
    s.stop()


if __name__ == "__main__":
    ap = argparse.ArgumentParser()
    ap.add_argument("mode", choices=["wave", "paired", "chaos"])
    ap.add_argument("bins")
    ap.add_argument("--n", type=int, default=1000)
    ap.add_argument("--waves", type=int, default=7)
    ap.add_argument("--conc", default="4,8,16")
    ap.add_argument("--pairs", type=int, default=21)
    ap.add_argument("--rounds", type=int, default=30)
    ap.add_argument("--seed", type=int, default=7)
    ap.add_argument("--out", default="bench-raw.jsonl")
    args = ap.parse_args()
    {"wave": cmd_wave, "paired": cmd_paired, "chaos": cmd_chaos}[args.mode](args)
