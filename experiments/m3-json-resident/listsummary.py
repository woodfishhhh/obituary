"""Summarise listbench.py output: per level and build, p50 / p95 (nearest rank) / max and the count over 250 ms,
plus the paired difference to the reference build (same wave index, same level).

usage: python listsummary.py raw.jsonl [--ref base] [--gate 250]
"""
import argparse
import json
import math
import statistics as st
from collections import defaultdict


def p95(values):
    a = sorted(values)
    return a[math.ceil(.95 * len(a)) - 1]


def hook_cell(values):
    return f"{st.median(values):.0f} / {p95(values):.0f}" if values else "-"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("raw", nargs="+")
    ap.add_argument("--ref", default="base")
    ap.add_argument("--gate", type=float, default=250)
    a = ap.parse_args()
    rows = [json.loads(line) for path in a.raw for line in open(path, encoding="utf8") if line.strip()]
    cell = defaultdict(dict)  # (k, arm) -> wave -> ms
    cpu = defaultdict(list)
    hooks = defaultdict(list)
    for r in rows:
        cell[(r["k"], r["arm"])][r["wave"]] = r["list_ms"]
        cpu[(r["k"], r["arm"])].append(r["cpu"])
        hooks[(r["k"], r["arm"])] += r.get("hook_ms", [])
    print(f"| level | build | n | p50 | p95 | max | >{a.gate:g} ms | paired vs {a.ref} (median) | faster | CPU p50 | hook p50 / p95 |")
    print("|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
    for (k, arm) in sorted(cell, key=lambda x: (x[0], x[1] != a.ref, x[1])):
        v = cell[(k, arm)]
        vals = list(v.values())
        ref = cell.get((k, a.ref), {})
        diffs = [ref[w] - v[w] for w in v if w in ref] if arm != a.ref else []
        pair = f"{st.median(diffs):+.1f}" if diffs else "-"
        faster = f"{sum(d > 0 for d in diffs)}/{len(diffs)}" if diffs else "-"
        cpu_cell = f"{st.median(cpu[(k, arm)]):.0f}%" if cpu[(k, arm)] else "-"
        print(f"| {'solo' if k == 0 else k} | {arm} | {len(vals)} | {st.median(vals):.1f} | {p95(vals):.1f} | {max(vals):.1f} | "
              f"{sum(x > a.gate for x in vals)} | {pair} | {faster} | {cpu_cell} | {hook_cell(hooks[(k, arm)])} |")


if __name__ == "__main__":
    main()
