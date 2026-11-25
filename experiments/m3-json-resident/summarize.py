"""Turn bench.py's raw samples into the report table (nearest-rank percentiles).

usage: python summarize.py results/*.jsonl
Each line of a jsonl file is [name, [samples]]; names look like
n=1000/k=16/new/hook, n=1000/direct-write/old, chaos/...
Several files with the same names are pooled and also listed per file.
"""
import json
import math
import statistics as st
import sys
from collections import defaultdict


def rank(values, q):
    a = sorted(values)
    return a[math.ceil(q * len(a)) - 1]


def main(paths):
    per_file = defaultdict(dict)
    pooled = defaultdict(list)
    for path in paths:
        for line in open(path, encoding="utf8"):
            name, values = json.loads(line)
            per_file[name][path] = values
            pooled[name] += values
    for name in sorted(pooled):
        parts = " | ".join(
            f"{p.split('/')[-1]}: n={len(v)} p50={st.median(v):.2f} p95={rank(v, .95):.2f}"
            for p, v in per_file[name].items())
        v = pooled[name]
        print(f"{name}: pooled n={len(v)} p50={st.median(v):.2f} p95={rank(v, .95):.2f} max={max(v):.2f}  [{parts}]")


if __name__ == "__main__":
    main(sys.argv[1:])
