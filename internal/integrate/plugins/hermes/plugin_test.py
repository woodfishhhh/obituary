"""Executable host-callback contract tests; no Hermes installation required."""
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import types
import threading
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("obituary", sys.argv.pop(1))
plugin = importlib.util.module_from_spec(spec)
spec.loader.exec_module(plugin)


class PluginTests(unittest.TestCase):
    def setUp(self):
        self.calls = []
        self.reply = {"version": 1, "deny": True, "reason": "obit rm -- scratch.log"}
        self.runtime = patch.object(plugin, "_runtime", return_value={"cwd": "C:\\repo", "shell": "bash", "local": True})
        self.runtime.start()
        self.addCleanup(self.runtime.stop)
        self.process = patch.object(plugin.subprocess, "run", side_effect=self.run_obit)
        self.process.start()
        self.addCleanup(self.process.stop)
        paths = types.ModuleType("tools.file_tools_paths")
        paths._resolve_path_for_task = lambda value, task: Path("C:/repo", value)
        paths._resolve_entry_for_task = paths._resolve_path_for_task
        self.paths = patch.dict(sys.modules, {"tools.file_tools_paths": paths})
        self.paths.start()
        self.addCleanup(self.paths.stop)

    def run_obit(self, argv, **kwargs):
        self.assertEqual(argv, ["C:\\Program Files\\obit.exe", "hook", "hermes"])
        self.assertFalse(kwargs["shell"])
        self.assertLess(kwargs["timeout"], 3)
        self.assertEqual(kwargs["encoding"], "utf-8")
        self.calls.append(json.loads(kwargs["input"]))
        return types.SimpleNamespace(returncode=0, stdout=json.dumps(self.reply))

    def test_guard_blocks_and_keeps_original_arguments(self):
        args = {"command": "rm -- scratch.log"}
        result = plugin._pre_tool_call("terminal", args, "task", session_id="session")
        self.assertEqual(result, {"action": "block", "message": "obit rm -- scratch.log"})
        self.assertEqual(args, {"command": "rm -- scratch.log"})
        self.assertEqual(self.calls[-1]["session"], "session")
        self.assertEqual(self.calls[-1]["event"], "PreToolUse")
        self.assertFalse(self.calls[-1]["context_delivery"])

    def test_fail_open(self):
        for failure in (subprocess.TimeoutExpired("obit", 1.5), OSError("missing")):
            with patch.object(plugin.subprocess, "run", side_effect=failure):
                self.assertIsNone(plugin._pre_tool_call("terminal", {"command": "rm x"}, "task"))
        for reply in ("broken", "null", '{"version":2,"deny":true}'):
            with patch.object(plugin.subprocess, "run", return_value=types.SimpleNamespace(returncode=0, stdout=reply)):
                self.assertIsNone(plugin._pre_tool_call("terminal", {"command": "rm x"}, "task"))
        with patch.object(plugin, "_runtime", return_value=None):
            self.assertIsNone(plugin._pre_tool_call("terminal", {"command": "rm x"}, "task"))
        self.assertEqual(self.calls, [])

    def test_observation_only_callbacks_and_failed_writes(self):
        self.reply = {"version": 1, "context": "should stay silent"}
        for result in ({"success": False}, {"error": "denied"}, "wrote C:/fake.txt"):
            plugin._post_tool_call("write_file", {"path": "out.log"}, result, "task", session_id="session")
        self.assertEqual(self.calls, [])
        self.assertIsNone(plugin._post_tool_call("write_file", {"path": "out.log"}, '{"success":true}', "task", session_id="session"))
        self.assertEqual(self.calls[-1]["paths"], [str(Path("C:/repo/out.log"))])
        self.assertFalse(self.calls[-1]["context_delivery"])
        self.assertIsNone(plugin._on_session_start("session"))
        self.assertIsNone(plugin._on_session_end("session"))
        self.assertEqual(self.calls[-1]["event"], "Stop")

    def test_backend_evidence(self):
        self.runtime.stop()
        class LocalEnvironment:
            cwd = "C:\\repo"
        local = types.ModuleType("tools.environments.local")
        local.LocalEnvironment = LocalEnvironment
        terminal = types.SimpleNamespace(
            _tenv=lambda key, default: "local", _has_isolation_overrides=lambda task: False,
            _resolve_container_task_id=lambda task: task, _env_lock=threading.Lock(),
            _active_environments={"task": LocalEnvironment()}, get_session_cwd=lambda task: "C:\\repo",
        )
        tools = types.ModuleType("tools")
        tools.terminal_tool = terminal
        with patch.dict(sys.modules, {"tools": tools, "tools.environments.local": local}):
            self.assertEqual(plugin._runtime("task")["shell"], "bash")
            self.assertIsNone(plugin._runtime("missing"))
            terminal._active_environments["task"] = object()
            self.assertIsNone(plugin._runtime("task"))
            terminal._active_environments["task"] = LocalEnvironment()
            terminal._tenv = lambda key, default: "ssh"
            self.assertIsNone(plugin._runtime("task"))
            terminal._tenv = lambda key, default: "local"
            terminal._env_lock.acquire()
            try:
                self.assertIsNone(plugin._runtime("task"))
            finally:
                terminal._env_lock.release()

    def test_patch_paths_come_from_host_operations(self):
        parser = types.ModuleType("tools.patch_parser")
        parser.OperationType = types.SimpleNamespace(ADD="add", UPDATE="update", MOVE="move", DELETE="delete")
        operations = [types.SimpleNamespace(operation=kind, file_path=path, new_path="moved.log") for kind, path in
                      [("update", "real.log"), ("add", "new.log"), ("delete", "old.log"), ("move", "source.log")]]
        parser.parse_v4a_patch = lambda text: (operations, None)
        args = {"mode": "patch", "path": "untouched.log", "patch": "host parsed patch"}
        with patch.dict(sys.modules, {"tools.patch_parser": parser}):
            plugin._post_tool_call("patch", args, {"success": True, "files_modified": ["untouched.log"]}, "task")
            self.assertEqual(self.calls[-1]["paths"], [str(Path("C:/repo", p)) for p in ["real.log", "new.log", "moved.log"]])
            self.calls.clear()
            parser.parse_v4a_patch = lambda text: ([], "invalid")
            plugin._post_tool_call("patch", args, {"success": True}, "task")
            plugin._post_tool_call("patch", {"mode": "unknown", "path": "untouched.log"}, {"success": True}, "task")
            self.assertEqual(self.calls, [])
        plugin._post_tool_call("patch", {"mode": "replace", "path": "real.log"}, {"success": True}, "task")
        self.assertEqual(self.calls[-1]["paths"], [str(Path("C:/repo/real.log"))])

    def test_context_requires_local_backend_and_finish_is_silent(self):
        self.reply = {"version": 1, "context": "pending proposal"}
        self.assertEqual(plugin._pre_llm_call("session", "task"), {"context": "pending proposal"})
        self.assertTrue(self.calls[-1]["context_delivery"])
        count = len(self.calls)
        with patch.object(plugin, "_runtime", return_value=None):
            self.assertIsNone(plugin._pre_llm_call("session", "task"))
        self.assertEqual(len(self.calls), count)
        self.assertIsNone(plugin._on_session_end("session", task_id="task"))
        self.assertFalse(self.calls[-1]["context_delivery"])

    def test_register_and_opt_in(self):
        hooks = {}
        plugin.register(types.SimpleNamespace(register_hook=lambda name, callback: hooks.update({name: callback})))
        self.assertEqual(set(hooks), {"pre_tool_call", "post_tool_call", "on_session_start", "on_session_end", "pre_llm_call"})
        with patch.dict(plugin.SETTINGS, {"guard": False}):
            self.assertIsNone(hooks["pre_tool_call"]("terminal", {"command": "rm x"}, "task"))
        for tool, args in (("execute_code", {"command": "rm x"}), ("terminal", {"command": "rm x", "background": True})):
            self.assertIsNone(plugin._pre_tool_call(tool, args, "task"))
        self.assertEqual(self.calls, [])


if __name__ == "__main__":
    unittest.main()
