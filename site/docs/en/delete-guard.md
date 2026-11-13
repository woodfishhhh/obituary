# Delete guard

Agents wiping a home directory or a whole drive is no longer hypothetical; people have posted it more than once. When the delete guard is on and an agent runs `rm -rf build`, `rmdir`, `git clean -fdx`, `Remove-Item -Recurse` or a similar delete, Obituary stops that execution and tells the agent the equivalent command, such as `obit rm -rf build`. Once the agent uses it, things go to the graveyard instead of disappearing, and `obit resurrect <id>` puts them back.

```powershell
obit init -g --guard-rm                 # Claude Code; add --agent codex for Codex. Off by default
```

## One real run

With the delete guard on, the hook received one Claude Code Bash call, `rm -rf build`, and replied as follows; `rm` never ran:

```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Obituary's delete guard is on: `'C:/work/obit.exe' rm -rf build` provides restorable deletion; `obit resurrect` restores graveyard entries."}}
```

The agent then switches to Obituary; the folder goes to the graveyard and one command brings it back:

```console
$ obit rm -rf build
$ obit graveyard --here
ID  BURIED            PURGE  SIZE  PATH
#2  2026-11-11 00:36  in 7d  23 B  C:\work\build
$ obit resurrect --last
#2 resurrected  C:\work\build
```

## Principles

- **Deny, never rewrite.** Obituary does not rewrite the agent's command: Claude Code checks permissions against the rewritten command, so rewriting would defeat your `deny` rules. Denying and letting the agent issue the equivalent command keeps all your permission rules in force.
- **A literal delete is always stopped.** When `rm`, `rmdir`, `git clean` or `Remove-Item` (including aliases such as `del`, `rd`, `ri`) appears at any command position, whether after `&&`, `||`, `;`, in a pipeline, `$(…)`, a subshell, a condition or a loop, that execution is stopped: for example `rm -rf build && npm run build`, `rm -rf build 2>/dev/null` or `git clean -fdx && npm ci`. Stopping runs nothing, so stopping more can never delete the wrong thing. `rm` inside strings, comments or `echo` arguments does not count; previews such as `rm --help`, `git clean -n` and `Remove-Item -WhatIf` do not either; deletes run by another program (`sudo rm`, `xargs rm`, `find -delete`) pass through (recorded in the audit log with `OBIT_HOOK_AUDIT=1`).
- **Never falls back to a real delete.** Every path given to `obit rm` / `obit remove-item` / `obit git-clean` is buried; when one is refused (drive roots, system folders, the user profile itself and so on) the error follows `rm`'s format and the exit code is non-zero.
- **Afraid the command changes on the way.** Between the command an agent means and the one that runs sit shell quoting, escaping and sometimes a proxy that rewrites commands. Lose one character and `rm -rf "$OUT"/` becomes `rm -rf /`. So before `obit rm` touches anything it checks the real path on disk.
- Files tracked by Git are buried too, so uncommitted changes can be recovered (this is a deliberate delete by the agent, unlike automatic registration, which never touches tracked files).
- `obit rm -rf` on a folder that contains `.git` buries the whole repository: a real `rm -rf` would destroy it, and burial can at least be undone. Protections that cannot be forced (system folders, user folders) still apply.
- **The equivalent command is deliberately narrow.** It must never select more than the original, so Obituary does not translate across shells and only offers one for the forms in the table below. Otherwise the delete is still stopped; the reason says why no equivalent applies and that `obit bury <path>` (literal paths) and `obit git-clean` delete restorably, and the agent rewrites the command with them.

## What is stopped and what passes

| Host | Stopped, with an equivalent command | Stopped, with the reason and general guidance | Passes |
|---|---|---|---|
| Claude Code (Bash tool) | `rm`, `rmdir`, `git clean`, optionally preceded by a literal `cd x` (joined with `&&`, `;` or a newline); `rm -f *.log` (when no file in the folder starts with `-`), `rm -rf ../dist`, `rm -- "$target"` (variables after `--`) | Every other literal delete: other statements before or after (`npm …`, `trap`, `hash`), variables before `--`, POSIX absolute paths starting with `/`, redirections, `\|\|`, pipelines, `$(…)`, interactive deletes, `/usr/bin/rm` with a path | Commands that do not delete |
| Codex, under WSL's `/mnt/<drive>` | A single `rm` / `rmdir` whose arguments are relative paths without `..`, `~`, wildcards or quotes | Every other literal delete, including `git clean` and paths into WSL's own file system | A working directory inside WSL's own file system |
| Codex, declared Windows PowerShell | A single `Remove-Item` or alias with one literal path (`-Recurse`, `-Force` allowed) | Every other literal delete: multiple statements, pipelines, other parameters such as `-ErrorAction`, provider paths, `~`, wildcards, backticks | Text inside curly-quoted strings |
| Codex on Windows with an unknown shell, pwsh on Linux, `git rm` | — | — | Everything |

For an equivalent command, the delete must be the **last segment** of the command, joined only with `&&`, `;` or newlines, and file names ending in a dot or space get none (Windows would normalise them into another name). A literal delete in any of these is still stopped.

## Use it yourself

`obit rm`, `obit rmdir` and `obit git-clean` accept the common non-interactive options of `rm`, `rmdir` and `git clean` (unsupported ones such as `-i` or `rmdir -p` are errors, not silently ignored). `obit remove-item` takes one literal path and is not a full `Remove-Item`; for wildcards use `obit rm *.log` in bash.

Turn it off temporarily with `OBIT_GUARD_RM=0`. `obit stats` shows how many deletes were stopped and how much was buried.
