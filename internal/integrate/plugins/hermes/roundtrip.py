"""Installed Hermes callback fixture; its subprocess transport talks to real Go."""
import importlib.util
import json
import os
from pathlib import Path
import sys
import types
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("obituary", sys.argv[1])
plugin = importlib.util.module_from_spec(spec)
spec.loader.exec_module(plugin)
config = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))


def invoke(argv, **kwargs):
    print(json.dumps({"request": json.loads(kwargs["input"])}), flush=True)
    return types.SimpleNamespace(returncode=0, stdout=sys.stdin.readline())


# These stand in for the pinned host's parser and task-aware resolver APIs.
# The parser's argument is checked: args.path and files_modified aren't evidence.
def parse_patch(text):
    assert text == "*** Begin Patch\n*** Update File: real.log\n@@\n-old\n+new\n*** End Patch"
    return [types.SimpleNamespace(operation="update", file_path="real.log")], None


paths = types.ModuleType("tools.file_tools_paths")
paths._resolve_path_for_task = lambda value, task: Path(config["cwd"], value).resolve()
paths._resolve_entry_for_task = paths._resolve_path_for_task
parser = types.ModuleType("tools.patch_parser")
parser.parse_v4a_patch = parse_patch
parser.OperationType = types.SimpleNamespace(ADD="add", UPDATE="update", DELETE="delete", MOVE="move")
hooks = {}
plugin.register(types.SimpleNamespace(register_hook=lambda name, callback: hooks.update({name: callback})))
runtime = {"cwd": config["cwd"], "local": True, "shell": "bash"}
with patch.dict(sys.modules, {"tools.file_tools_paths": paths, "tools.patch_parser": parser}), \
        patch.object(plugin, "_runtime", return_value=runtime), patch.object(plugin.subprocess, "run", side_effect=invoke):
    if config.get("patch"):
        hooks["post_tool_call"]("patch", {"mode": "patch", "path": "untouched.log", "patch":
            "*** Begin Patch\n*** Update File: real.log\n@@\n-old\n+new\n*** End Patch"},
            {"success": True, "files_modified": ["untouched.log", "real.log"]}, "task", session_id="s")
        result = None
    else:
        hooks["post_tool_call"]("write_file", {"path": "debug.log"}, {"success": True}, "task", session_id="s")
        result = hooks.get("pre_llm_call", lambda **kw: None)(task_id="task", session_id="s")
print(json.dumps({"result": result}), flush=True)
