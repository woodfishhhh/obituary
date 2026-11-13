"""Exercise install.sh through HTTP, including real release archives when supplied."""
import hashlib
import http.server
import io
import json
import os
from pathlib import Path
import platform
import subprocess
import tarfile
import tempfile
import threading
import unittest


ROOT = Path(__file__).resolve().parents[1]


class InstallerTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        # Installation rejects symlink ancestors; macOS temporary paths use /var.
        cls.root = Path(tempfile.mkdtemp(prefix="obit-install-sh-", dir=os.environ.get("OBIT_INSTALL_TEST_ROOT"))).resolve()
        cls.platform = "darwin" if platform.system() == "Darwin" else "linux"
        cls.arch = "arm64" if platform.machine() in ("aarch64", "arm64") else "amd64"
        cls.version = "0.0.0-test"
        cls.archive = f"obituary_{cls.version}_{cls.platform}_{cls.arch}.tar.gz"
        cls.payloads = {}
        cls.requests = []

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                cls.requests.append(self.path)
                data = cls.payloads.get(self.path)
                self.send_response(200 if data is not None else 404)
                self.end_headers()
                self.wfile.write(data or b"missing")

            def log_message(self, *_args):
                pass

        cls.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        cls.url = f"http://127.0.0.1:{cls.server.server_port}"
        print(f"Installer evidence: {cls.root}", flush=True)

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        cls.thread.join()

    def fixture(self, name, entries=None, corruption=False, duplicate=False):
        if entries is None:
            entries = {"obit": b"#!/bin/sh\nprintf 'obit fixture\\n'\n", "obitd": b"#!/bin/sh\nexit 0\n"}
        buf = io.BytesIO()
        with tarfile.open(fileobj=buf, mode="w:gz") as tar:
            for path, content in entries.items():
                info = tarfile.TarInfo(path)
                info.size, info.mode = len(content), 0o755
                tar.addfile(info, io.BytesIO(content))
        data = buf.getvalue()
        checksum = f"{hashlib.sha256(data).hexdigest()}  {self.archive}\n".encode()
        self.payloads[f"/{name}/{self.archive}"] = data + (b"corrupt" if corruption else b"")
        self.payloads[f"/{name}/checksums.txt"] = checksum * (2 if duplicate else 1)

    def install(self, name, version=None, extra=()):
        destination = self.root / name / "bin with spaces"
        fake_home = self.root / name / "home"
        fake_home.mkdir(parents=True, exist_ok=True)
        env = dict(os.environ, HOME=str(fake_home), OBIT_HOME=str(fake_home / "data"),
                   OBIT_CONFIG_DIR=str(fake_home / "config"), OBIT_USER_HOME=str(fake_home),
                   CLAUDE_CONFIG_DIR=str(fake_home / ".claude"), CODEX_HOME=str(fake_home / ".codex"),
                   XDG_CONFIG_HOME=str(fake_home / ".config"), XDG_DATA_HOME=str(fake_home / ".local/share"))
        cmd = ["bash", str(ROOT / "install.sh"), "--version", version or self.version,
               "--base-url", f"{self.url}/{name}", "--install-dir", str(destination), *extra]
        result = subprocess.run(cmd, env=env, text=True, capture_output=True, timeout=30)
        (self.root / name / "result.txt").write_text(result.stdout + result.stderr)
        return result, destination, env

    def test_verified_install_and_upgrade(self):
        self.fixture("valid")
        for _ in range(2):
            result, dest, env = self.install("valid")
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(subprocess.check_output([str(dest / "obit")], env=env, text=True), "obit fixture\n")
            self.assertTrue((dest / "obitd").stat().st_mode & 0o111)
            self.assertIn("SHA-256 verified", result.stdout)
        self.assertEqual(list(Path(env["HOME"]).iterdir()), [])

    def test_invalid_downloads_preserve_existing_binaries(self):
        for case, kwargs in {"corrupt": {"corruption": True}, "duplicate": {"duplicate": True},
                             "traversal": {"entries": {"obit": b"x", "obitd": b"x", "../escaped": b"x"}},
                             "missing": {"entries": {"obit": b"x"}}}.items():
            with self.subTest(case=case):
                self.fixture(case, **kwargs)
                destination = self.root / case / "bin with spaces"
                destination.mkdir(parents=True)
                (destination / "obit").write_text("existing obit")
                (destination / "obitd").write_text("existing daemon")
                result, _, _ = self.install(case)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertEqual((destination / "obit").read_text(), "existing obit")
                self.assertEqual((destination / "obitd").read_text(), "existing daemon")
                self.assertFalse((destination.parent / "escaped").exists())

    def test_arguments_and_mirror_preview(self):
        args = ["bash", str(ROOT / "install.sh"), "--version", self.version]
        for extra in (["--architecture", "riscv64"], ["--base-url", "http://127.0.0.1:80.evil/path"],
                      ["--mirror", "cn", "--base-url", "https://example.com"], ["--version", "../oops"]):
            result = subprocess.run([*args, *extra, "--dry-run"], text=True, capture_output=True)
            self.assertNotEqual(result.returncode, 0, result.stdout)
        result = subprocess.run([*args, "--mirror", "cn", "--dry-run"], text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("https://cn.obituary.woodfish.site/download/", result.stdout)

    @unittest.skipUnless(os.environ.get("OBIT_INSTALL_TEST_DIST"), "supply snapshot archives to test a real binary")
    def test_real_snapshot(self):
        dist = Path(os.environ["OBIT_INSTALL_TEST_DIST"])
        archives = list(dist.glob(f"obituary_*_{self.platform}_{self.arch}.tar.gz"))
        self.assertEqual(len(archives), 1, archives)
        archive = archives[0]
        version = archive.name.removeprefix("obituary_").removesuffix(f"_{self.platform}_{self.arch}.tar.gz")
        self.payloads[f"/snapshot/{archive.name}"] = archive.read_bytes()
        self.payloads["/snapshot/checksums.txt"] = (dist / "checksums.txt").read_bytes()
        result, dest, env = self.install("snapshot", version)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        output = subprocess.check_output([str(dest / "obit"), "--version"], env=env, text=True)
        self.assertIn(version, output)
        (self.root / "snapshot" / "version.txt").write_text(output)
        self.verify_agent_integration(dest, env)

    def verify_agent_integration(self, dest, env):
        native_root = os.environ.get("OBIT_INSTALL_TEST_HOME_ROOT")
        if native_root:
            # A native filesystem is required for POSIX no-replace renames.
            # Keep release artifacts on the caller's volume; isolate only HOME.
            with tempfile.TemporaryDirectory(prefix="obit-posix-release-", dir=native_root) as home:
                self.verify_native_agent_integration(dest, env, Path(home).resolve())
        else:
            self.verify_native_agent_integration(dest, env, Path(env["HOME"]))

    def verify_native_agent_integration(self, dest, env, fake_home):
        env = dict(env, HOME=str(fake_home), OBIT_HOME=str(fake_home / "data"),
                   OBIT_CONFIG_DIR=str(fake_home / "config"), OBIT_USER_HOME=str(fake_home),
                   CLAUDE_CONFIG_DIR=str(fake_home / ".claude"), CODEX_HOME=str(fake_home / ".codex"),
                   XDG_CONFIG_HOME=str(fake_home / ".config"), XDG_DATA_HOME=str(fake_home / ".local/share"))
        fake_home = Path(env["HOME"])
        config = Path(env["OBIT_CONFIG_DIR"])
        config.mkdir()
        (config / "config.toml").write_text("[update]\ncheck = false\n")
        tools = fake_home / "test-tools"
        tools.mkdir()
        docker = tools / "docker"
        docker.write_text("#!/bin/sh\necho 'Docker disabled in installer acceptance' >&2\nexit 1\n")
        docker.chmod(0o700)
        env = dict(env, PATH=f"{tools}:{dest}:{env['PATH']}")
        args = [str(dest / "obit"), "init", "-g", "--agent", "claude,codex", "--guard-rm", "--yes"]
        result = subprocess.run(args, env=env, text=True, capture_output=True, timeout=30)
        (dest.parent / "init.txt").write_text(result.stdout + result.stderr)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for relative in (".claude/settings.json", ".codex/hooks.json"):
            self.assertIn(str(dest / "obit"), (fake_home / relative).read_text())
        self.assertTrue((fake_home / ".agents/skills/obituary/SKILL.md").exists())
        result = subprocess.run([str(dest / "obit"), "doctor", "--json"], env=env, text=True, capture_output=True, timeout=30)
        (dest.parent / "doctor.json").write_text(result.stdout)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        checks = {row["name"]: row for row in json.loads(result.stdout)}
        self.assertFalse([row for row in checks.values() if row["status"] == "FAIL"])
        for name in ("path", "obitd", "claude", "codex", "skill"):
            self.assertEqual(checks[name]["status"], "PASS", checks[name])
        self.assertIn("automatic-burial", checks)
        self.assertEqual(checks["automatic-burial"]["status"], "INFO")
        self.assertIn("no-replace rename", checks["automatic-burial"]["detail"])
        self.assertEqual(checks["codex-windows-shell"]["status"], "INFO")


if __name__ == "__main__":
    unittest.main(verbosity=2)
