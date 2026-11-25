"""Byte parity of `obit list` between builds on one seeded home, with a resident of the first build running.

usage: python listparity.py <bins> <resident-arm> <other-arm>[,<other-arm>...]
Compares default, --json and OBIT_LANG=zh-CN output (and exit codes, stderr) of every arm on the same ledger,
with the resident running (so a resident-served list is compared with a file-read list) and with it stopped.
"""
import json
import subprocess as sp
import sys
from pathlib import Path

import bench


def run(b, arm, args, env):
    r = sp.run([str(b.bin[arm] / "obit.exe"), *args], stdout=sp.PIPE, stderr=sp.PIPE, cwd=b.repo, env=env,
               creationflags=sp.CREATE_NO_WINDOW, timeout=60)
    return r.returncode, r.stdout, r.stderr


def main():
    bins, resident, others = Path(sys.argv[1]), sys.argv[2], sys.argv[3].split(",")
    b = bench.Bench(bins, Path("parity-raw.jsonl"))
    s = bench.System(b, resident, 1000, "-parity")
    report = []
    try:
        import time
        time.sleep(2)
        for state in ("resident-running", "resident-stopped"):
            if state == "resident-stopped":
                s.stop()
                time.sleep(1)
            for label, args, extra in (("default", ["list"], {}), ("json", ["list", "--json"], {}),
                                       ("zh", ["list"], {"OBIT_LANG": "zh-CN"}), ("ls-alias", ["ls"], {}),
                                       ("bad-arg", ["list", "extra"], {})):
                env = dict(s.e, **extra)
                got = {arm: run(b, arm, args, env) for arm in [resident, *others]}
                ref = got[resident]
                same = all(v == ref for v in got.values())
                report.append(dict(state=state, case=label, identical=same, exit=ref[0], stdout_bytes=len(ref[1]), stderr_bytes=len(ref[2])))
                print(report[-1], flush=True)
                assert same, (state, label, {k: (v[0], len(v[1]), len(v[2])) for k, v in got.items()})
    finally:
        s.stop()
    json.dump(report, open("parity-report.json", "w"), indent=1)
    print("parity ok")


if __name__ == "__main__":
    main()
