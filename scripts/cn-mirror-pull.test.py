import fcntl
import hashlib
import json
import os
import signal
import subprocess
import tempfile
import time
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("cn-mirror-pull-legacy.sh")


class MirrorCommandTests(unittest.TestCase):
    def test_rejects_before_networking(self):
        env = os.environ.copy()
        env.pop("SSH_ORIGINAL_COMMAND", None)
        for tool in ("ssh", "curl"):
            env[f"BASH_FUNC_{tool}%%"] = '() { echo "unexpected network request" >&2; exit 99; }'
        invalid = (None, "", "ls", "pull", "pull v1.4.1 extra", "pull v1.4.1; rm",
                   "pull ../v1.4.1", "pull v1.4.1-rc.1", "pull  v1.4.1",
                   "pull v1.4.1\n", "pull v1.4.1\nls", "pull v1.4.1 ", " pull v1.4.1")
        for command in invalid:
            with self.subTest(command=command):
                run_env = env if command is None else dict(env, SSH_ORIGINAL_COMMAND=command)
                result = subprocess.run(["bash", str(SCRIPT)], env=run_env, text=True,
                                        capture_output=True, timeout=5)
                self.assertEqual(result.returncode, 1)
                self.assertEqual(result.stderr, "Expected: pull vX.Y.Z\n")
                self.assertEqual(result.stdout, "")


class MirrorPullTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        self.public = self.root / "download"
        self.public.mkdir()
        self.env = dict(os.environ, FIXTURE=temp.name, SSH_ORIGINAL_COMMAND="pull v1.2.3",
                        NO_PROXY="*", PATH=temp.name + ":" + os.environ["PATH"])
        # Exercise the forced command without exposing a configurable destination over SSH.
        dest = "download=/opt/blog-stack/sites/obituary/download"
        source = SCRIPT.read_text()
        self.assertEqual(source.count(dest), 1)
        self.command = ["bash", "-c", source.replace(dest, 'download="$FIXTURE/download"')]
        self.tool("ssh", '''import signal
args = sys.argv[1:]
assert 'ExitOnForwardFailure=yes' in args and not Path('/proc/self/fd/9').exists()
assert args[args.index('-D') + 1] == '127.0.0.1:31761'
assert 'LocalCommand=echo ready' in args and 'PermitLocalCommand=yes' in args
if (base/'ssh-fail').exists(): sys.exit(255)
signal.signal(signal.SIGTERM, lambda *_: sys.exit(0))
try:
    print('ready', flush=True)
    signal.pause()
finally:
    (base/'closed').touch()
''')
        self.tool("curl", '''import time
args = sys.argv[1:]
assert args[args.index('--proxy') + 1] == 'socks5h://127.0.0.1:31761'
assert args[args.index('--noproxy') + 1] == '' and '--remove-on-error' in args
assert not Path('/proc/self/fd/9').exists()
out = Path(args[args.index('-o') + 1]); name = out.name
(base/'called').touch()
if name.startswith('obituary_') and (base/'interrupt').exists():
    out.write_bytes(b'partial'); (base/'pid').write_text(str(os.getpid())); time.sleep(60)
if name.startswith('obituary_') and (base/'fail').exists():
    out.write_bytes(b'partial'); sys.exit(18)
out.write_bytes((base/name).read_bytes())
''')
        name = 'obituary_1.2.3_linux_amd64.tar.gz'
        data = b'test archive'
        (self.root/name).write_bytes(data)
        (self.root/'checksums.txt').write_text(f'{hashlib.sha256(data).hexdigest()}  {name}\n')
        api = 'https://api.github.com/repos/woodfishhhh/obituary/releases/assets/'
        assets = [dict(id=i, name=n, url=api+str(i)) for i, n in enumerate(['checksums.txt', name], 1)]
        release = json.dumps(dict(tag_name='v1.2.3', draft=False, prerelease=False, assets=assets))
        for n in ('release.json', 'github-latest.json', 'install.sh', 'install.ps1'):
            (self.root/n).write_text(release if n.endswith('.json') else n)

    def tool(self, name, body):
        tool = self.root/name
        tool.write_text('#!/usr/bin/env python3\nimport sys, os\nfrom pathlib import Path\n'
                        'base = Path(os.environ["FIXTURE"])\n' + body)
        tool.chmod(0o700)

    def run_pull(self):
        return subprocess.run(self.command, env=self.env, capture_output=True, text=True, timeout=5)

    def assert_clean(self, *published):
        self.assertCountEqual([p.name for p in self.public.iterdir()], ['.pull.lock', *published])
        with (self.public/'.pull.lock').open() as lock:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)

    def test_bind_failure_never_downloads(self):
        (self.root/'ssh-fail').touch()
        result = self.run_pull()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('attvps SOCKS tunnel', result.stderr)
        self.assertFalse((self.root/'called').exists())
        self.assert_clean()

    def test_success_closes_tunnel_and_preserves_identical_release(self):
        result = self.run_pull()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue((self.root/'closed').exists())
        published = [self.public/'latest.json', *(self.public/'v1.2.3').iterdir()]
        before = [p.stat().st_mtime_ns for p in published]
        result = self.run_pull()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(before, [p.stat().st_mtime_ns for p in published])
        self.assert_clean('latest.json', 'v1.2.3')

    def test_failed_partial_is_not_published(self):
        (self.root/'fail').touch()
        self.assertEqual(self.run_pull().returncode, 18)
        self.assertTrue((self.root/'closed').exists())
        self.assert_clean()

    def test_interrupted_download_is_reaped_and_unlocks(self):
        (self.root/'interrupt').touch()
        with subprocess.Popen(self.command, env=self.env, stderr=subprocess.PIPE) as proc:
            try:
                marker = self.root/'pid'
                deadline = time.monotonic() + 4
                while not marker.exists() and proc.poll() is None and time.monotonic() < deadline:
                    time.sleep(0.02)
                self.assertTrue(marker.exists(), 'did not reach the asset transfer')
                proc.send_signal(signal.SIGTERM)
                self.assertEqual(proc.wait(timeout=2), 143)
                with self.assertRaises(ProcessLookupError):
                    os.kill(int(marker.read_text()), 0)
            finally:
                if proc.poll() is None: proc.kill()
        self.assertTrue((self.root/'closed').exists())
        self.assert_clean()


if __name__ == "__main__":
    unittest.main()
