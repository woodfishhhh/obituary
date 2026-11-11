"""Hermes plugin transport; protocol v1, host API verified at b766b769.

References: NousResearch/hermes-agent tools/terminal_tool.py and
 tools/environments/local.py define the host's execution-environment contract.
Only a cached LocalEnvironment proves that tool paths and Bash are local.
Before that evidence is available the hook leaves the call alone.
"""

import json
import os
import subprocess

SETTINGS = json.loads(__OBIT_SETTINGS__)


def _runtime(task_id):
    from tools import terminal_tool as terminal
    from tools.environments.local import LocalEnvironment

    if terminal._tenv("TERMINAL_ENV", "") != "local" or terminal._has_isolation_overrides(task_id):
        return None
    key = terminal._resolve_container_task_id(task_id)
    if not terminal._env_lock.acquire(blocking=False):
        return None
    try:
        env = terminal._active_environments.get(key)
    finally:
        terminal._env_lock.release()
    if type(env) is not LocalEnvironment:
        return None
    cwd = terminal.get_session_cwd(task_id) or env.cwd
    if not isinstance(cwd, str) or not os.path.isabs(cwd):
        return None
    return {"cwd": cwd, "local": True, "shell": "bash"}


def _call(event, host_event, task_id=None, session_id=None, **fields):
    try:
        runtime = _runtime(task_id or session_id)
        if runtime is None:
            return {}
        request = {
            "version": 1, "agent": "hermes", "event": event,
            "host_event": host_event, "session": session_id or task_id or "",
            "context_delivery": False, **runtime, **fields,
        }
        payload = json.dumps(request, ensure_ascii=False)
        if len(payload.encode("utf-8")) > 4 * 1024 * 1024:
            return {}
        result = subprocess.run(
            [SETTINGS["executable"], "hook", "hermes"], input=payload,
            shell=False, timeout=1.5, capture_output=True, text=True,
            encoding="utf-8", creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0),
        )
        if result.returncode != 0 or len(result.stdout) > 16384:
            return {}
        response = json.loads(result.stdout)
        return response if isinstance(response, dict) and response.get("version") == 1 else {}
    except Exception:
        return {}


def _pre_tool_call(tool_name=None, args=None, task_id=None, **kwargs):
    if not SETTINGS["guard"] or tool_name != "terminal" or not isinstance(args, dict):
        return None
    command = args.get("command")
    if not isinstance(command, str) or args.get("background") or args.get("pty") or args.get("workdir"):
        return None
    response = _call("PreToolUse", "pre_tool_call", task_id,
                     kwargs.get("session_id"), tool=tool_name, command=command)
    # The intentional veto is outside the fail-open transport catch.
    if response.get("deny") is True and isinstance(response.get("reason"), str):
        return {"action": "block", "message": response["reason"]}
    return None


def _post_tool_call(tool_name=None, args=None, result=None, task_id=None, **kwargs):
    try:
        if tool_name not in ("write_file", "patch") or not isinstance(args, dict):
            return
        result = json.loads(result) if isinstance(result, str) else result
        if not isinstance(result, dict) or result.get("success") is not True or result.get("error"):
            return
        task = task_id or kwargs.get("session_id")
        if _runtime(task) is None:
            return
        paths = _written_paths(tool_name, args, task)
        if not paths:
            return
        _call("PostToolUse", "post_tool_call", task_id, kwargs.get("session_id"),
              tool=tool_name, paths=paths, success=True)
    except Exception:
        pass


def _written_paths(tool, args, task_id):
    # Reuse b766b769's task-aware resolver and V4A parser. files_modified includes
    # args.path even in patch mode, where that argument is only a precheck.
    from tools.file_tools_paths import _resolve_path_for_task, _resolve_entry_for_task
    if tool == "write_file" or args.get("mode", "replace") == "replace":
        path = args.get("path")
        return [str(_resolve_path_for_task(path, task_id))] if isinstance(path, str) and path else []
    if args.get("mode") != "patch" or not isinstance(args.get("patch"), str):
        return []
    from tools.patch_parser import parse_v4a_patch, OperationType
    operations, error = parse_v4a_patch(args["patch"])
    if error:
        return []
    paths = []
    for op in operations:
        if op.operation in (OperationType.ADD, OperationType.UPDATE):
            paths.append(str(_resolve_path_for_task(op.file_path, task_id)))
        elif op.operation == OperationType.MOVE:
            paths.append(str(_resolve_entry_for_task(op.new_path, task_id)))
    return paths


def _pre_llm_call(session_id=None, task_id=None, **kwargs):
    # Hermes grants this channel by loading the declared hook; _call still
    # requires a cached local environment before acknowledging any notice.
    response = _call("Context", "pre_llm_call", task_id, session_id, context_delivery=True)
    context = response.get("context")
    if isinstance(context, str) and context:
        return {"context": context}


def _on_session_start(session_id=None, **kwargs):
    _call("SessionStart", "on_session_start", kwargs.get("task_id"), session_id)


def _on_session_end(session_id=None, **kwargs):
    _call("Stop", "on_session_end", kwargs.get("task_id"), session_id)


def register(ctx):
    for name, callback in (
        ("pre_tool_call", _pre_tool_call), ("post_tool_call", _post_tool_call),
        ("on_session_start", _on_session_start), ("on_session_end", _on_session_end),
        ("pre_llm_call", _pre_llm_call),
    ):
        ctx.register_hook(name, callback)
