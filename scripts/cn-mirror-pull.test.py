import os
import subprocess
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("cn-mirror-pull-legacy.sh")


class MirrorCommandTests(unittest.TestCase):
    def test_rejects_before_networking(self):
        env = os.environ.copy()
        env.pop("SSH_ORIGINAL_COMMAND", None)
        for tool in ("curl", "aria2c"):
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


if __name__ == "__main__":
    unittest.main()
