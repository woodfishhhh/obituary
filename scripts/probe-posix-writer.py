"""Measure writer exclusion on an isolated regular file, without changing kernel settings."""
import fcntl
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time


def writer(path, timeout):
    subprocess.run([sys.executable, "-c", "import sys; open(sys.argv[1], \"ab\").write(b\"data\")", str(path)],
                   timeout=timeout, check=True, close_fds=True)


with tempfile.TemporaryDirectory(dir=os.environ["OBIT_USER_HOME"]) as directory:
    path = Path(directory, "closed-download")
    path.touch()
    with path.open("rb") as reader:
        fcntl.flock(reader, fcntl.LOCK_EX | fcntl.LOCK_NB)
        writer(path, 5)
        print("flock: ordinary writer succeeded while lock was held", flush=True)
        fcntl.flock(reader, fcntl.LOCK_UN)
        signal.signal(signal.SIGIO, signal.SIG_IGN)
        # Darwin SDK fcntl.h declares F_SETLEASE=106 and F_GETLEASE=107.
        setlease = 106 if sys.platform == "darwin" else fcntl.F_SETLEASE
        getlease = 107 if sys.platform == "darwin" else fcntl.F_GETLEASE
        try:
            fcntl.fcntl(reader, setlease, fcntl.F_RDLCK)
        except OSError as err:
            print("read lease: unavailable:", err)
        else:
            start = time.monotonic()
            try:
                writer(path, 90)
            except subprocess.TimeoutExpired:
                print("read lease: writer blocked for 90s; indefinite exclusion not proven")
            else:
                print("read lease: FD still open; writer succeeded after %.1fs; lease=%d" %
                      (time.monotonic() - start, fcntl.fcntl(reader, getlease)))
