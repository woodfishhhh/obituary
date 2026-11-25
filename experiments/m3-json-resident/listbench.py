"""`obit list` latency next to k concurrent PostToolUse hooks, for several builds side by side.

Real `obit hook claude`, real `obit list`, real Git, a real headless obitd per build,
throw-away homes only (bench.Bench asserts every data directory is under its own root).
Builds are interleaved wave by wave (the order rotates every wave) and each build has its
own home and daemon, seeded with N active items and N buried entries.

usage: python listbench.py <bins> --arms base,skip --levels 0,2,4,8,16 --waves 21 --out raw.jsonl
<bins> holds bin-<arm>/ (obit.exe, obitd.exe, probe.exe). Level 0 is a lone `obit list`.
--hooks <arm> runs every hook from that build instead of the arm's own, so that only
`obit list` differs between the arms.
"""
import argparse
import concurrent.futures as cf
import hashlib
import json
import math
import platform
import statistics as st
import threading
import time
from pathlib import Path

import bench


def p95(values):
    a = sorted(values)
    return a[math.ceil(.95 * len(a)) - 1]


def one_wave(b, s, hook_sys, k, j):
    """k hooks, then one `obit list` 65 ms after the barrier; returns the list's own record."""
    paths = [b.repo / f"debug-{s.name}-{k}-{j}-{i}.log" for i in range(k)]
    for p in paths:
        p.write_text("x", encoding="utf8")
    barrier = threading.Barrier(k + 2)

    def hook(i):
        barrier.wait()
        time.sleep(.005 * (i % 4))
        return b.run(hook_sys, "obit", ["hook", "claude"], s.e, b.payload(f"session-{s.name}-{k}-{i}", paths[i]))[0]

    def reader():
        barrier.wait()
        time.sleep(.065)
        ms, out, err = b.run(s.name, "obit", ["list"], s.e)
        return ms, len(out), len(out.splitlines()), len(err), err[:200].decode("utf8", "replace").strip()

    with cf.ThreadPoolExecutor(k + 1) as pool, bench.CPUWindow() as cpu:
        fs = [pool.submit(hook, i) for i in range(k)]
        rd = pool.submit(reader)
        barrier.wait()
        hooks = [f.result() for f in fs]
        ms, size, rows, errlen, errtext = rd.result()
    assert errlen == 0 and rows >= s.n + 1, (rows, size, errtext)
    if k:
        s.expected += k
        s.drain()
    time.sleep(.6)
    return paths, dict(arm=s.name, k=k, wave=j, list_ms=ms, stdout_bytes=size, rows=rows, stderr_bytes=errlen, stderr=errtext,
                       hook_ms=hooks, cpu=cpu.pct)


def run_level(b, a, k, log):
    arms = a.arms.split(",")
    systems = {nm: bench.System(b, nm, a.n, f"-{k}") for nm in arms}
    written = {nm: [] for nm in arms}
    try:
        time.sleep(3)  # the resident's first reconciliation is not the steady state
        samples = {nm: [] for nm in arms}
        for j in range(a.waves):
            for nm in arms[j % len(arms):] + arms[:j % len(arms)]:
                paths, rec = one_wave(b, systems[nm], a.hooks or nm, k, j)
                written[nm] += paths
                samples[nm].append(rec["list_ms"])
                print(json.dumps(rec, separators=(",", ":")), file=log, flush=True)
                print(f"k={k} wave={j} {nm} list={rec['list_ms']:.0f} ms rows={rec['rows']}", flush=True)
        for nm, s in systems.items():
            logged = s.verify(written[nm])
            tail = p95(samples[nm])
            print(f"k={k} {nm} n={len(samples[nm])} p50={st.median(samples[nm]):.2f} p95={tail:.2f} ms", flush=True)
            if a.gate and k in (2, 4) and tail > a.gate:
                raise RuntimeError(f"k={k} {nm} p95 exceeds {a.gate} ms")
            print(f"k={k} {nm}: {len(written[nm])} writes registered once each; hook.log lines={logged}", flush=True)
    finally:
        for s in systems.values():
            s.stop()
    time.sleep(2)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("bins")
    ap.add_argument("--arms", default="base")
    ap.add_argument("--levels", default="0,2,4,8,16")
    ap.add_argument("--waves", type=int, default=21)
    ap.add_argument("--n", type=int, default=1000)
    ap.add_argument("--gate", type=float, help="maximum list p95 (ms) at 2 and 4 hooks")
    ap.add_argument("--hooks", default="")
    ap.add_argument("--out", default="list-raw.jsonl")
    a = ap.parse_args()
    b = bench.Bench(Path(a.bins), Path(a.out))
    print("OS", platform.platform(), "CPU", bench.CPU, "root", b.root, "n", a.n, "arms", a.arms, flush=True)
    with open(a.out, "a", encoding="utf8") as log:
        for k in [int(x) for x in a.levels.split(",")]:
            run_level(b, a, k, log)
    print("done", b.root)


if __name__ == "__main__":
    main()
