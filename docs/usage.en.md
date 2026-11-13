# User guide

[中文](usage.md) · [Home](../README.en.md)

Everything Obituary does, in detail. Installing is in the [install guide](install.en.md); the command summary is the README's [Quick start](../README.en.md#quick-start).

## For people: tray, small window, context menu

```powershell
obit init                                   # Claude Code hooks, and the tray starts at login
obit init --context-menu --yes              # compatibility flag; context menu is enabled by default (classic menu; on Windows 11 under "Show more options"; --dry-run lists every entry)
obit watch C:\Users\you\Downloads 2w        # files that land in Downloads are buried two weeks later, once they have been left untouched for 10 minutes (a proposal you approve first; expired subfolders get their own proposal)
obit watch --list                           # folder policies, their exemptions and quiet periods
obit watch --remove C:\Users\you\Downloads  # remove a policy and the lifespans it registered
```

- Tray menu: recent burials (click to resurrect), items due within 24 hours, pause / resume, Open Obituary (the small window), Quit (the scheduled task keeps working).
- `obit-gui.exe` must sit next to `obitd.exe` for the tray to open it. Every switch shows exactly what it will change and writes only after you confirm; trusting a `.obitrules` file trusts exactly the version shown in the window.
- Explorer dates come from a hidden `desktop.ini` in the folder and appear by default only in Downloads, on the Desktop and in folders under a policy. `[explorer]` in `config.toml` turns them off (`enabled = false`) or on everywhere (`everywhere = true`). Explorer caches folder views, so after extending a lifespan the same window may show the old date for up to a minute.
- The window, tray and context menu follow `[ui] language` in `config.toml` (`auto`, `zh-CN` or `en`; `auto` follows Windows). You can switch it in Settings.

## Claude Code

```powershell
obit init -g                         # all projects (~/.claude); obit init alone installs into this project (.claude/)
obit init -g --show --agent claude   # what is installed (without --agent: a one-line summary per agent, then the details)
obit init -g --uninstall             # remove only what Obituary added
```

`obit init` installs the selected agent's Agent Skill, context menus, a Start menu shortcut and a desktop shortcut by default. Skip them with `--no-skill` / `--no-context-menu` / `--no-shortcuts`; context menus and shortcuts can also be turned off later in Settings. Settings also controls the tray and taskbar icons; hiding the tray keeps background cleanup running.

`obit init` registers `obit hook claude` in `settings.json` (SessionStart, before each tool call, after writes, Stop, SessionEnd), writes an `OBIT.md` for the agent, and adds one reference line to `CLAUDE.md`: `@OBIT.md` in `~/.claude/CLAUDE.md` for the user scope (`-g`), `@.claude/OBIT.md` in the project's `CLAUDE.md` for a project.

- <details>
  <summary>How <code>obit init</code> edits your configuration</summary>

  - Your settings and key order stay as they are. Running it again adds nothing twice; if `obit.exe` moved, running it again refreshes the path.
  - Every edit is preceded by a backup: `.bak` the first time, then `.bak.1`, `.bak.2` …; old backups are never overwritten. If you save the file between the moment the change is computed and the write, Obituary refuses to write.
  - Only an `OBIT.md` carrying Obituary's marker is replaced or removed; your own compound commands that happen to contain `obit hook claude` are not mistaken for Obituary's. Only a hook whose program is an absolute path to `obit.exe` counts as Obituary's (`obit init` always writes the absolute path); one written with a relative path or a bare `obit` is never changed or removed.

  </details>

- Any error in a hook lets the agent continue silently (the error goes to `hook.log`). `OBIT_DISABLED=1` turns hooks off temporarily; `OBIT_HOOK_AUDIT=1` records what each hook received and did, readable with `obit hook audit`.

```powershell
obit doctor              # read-only check: PATH, obitd, runner, Claude Code / Codex integration, data directory, ledger, config (--json)
```

Once installed:

- **Consent first.** The first time Obituary wants to register something by rule in a project, it creates a proposal instead of acting. The agent passes it on to you (in a real session Claude said: "Obituary has a pending proposal… I haven't acted on it."). `obit approve <id>` lets that project register automatically from then on; `obit reject <id>` means never ask about this project again.
- **Large things** (over 1 GB or 200 files) are proposed as well.
- **Session diff.** A snapshot is taken when a session starts (kept in the data directory, nothing is added to your repository) and compared when it ends. Items covered by rules are registered. Of the rest, only what Git ignores and tools regenerate (dependencies, build output, caches such as `node_modules`, `dist`, `.turbo`) is proposed; once approved it is buried after 3 days and stays restorable. New files Git does not ignore are code you have not committed yet, and ignored local configuration such as `.env` cannot be regenerated, so neither ever appears in a proposal, and the agent can safely recommend approving. At the next session start the agent sees what was left behind. Items you resurrected or spared are not registered again by the session diff until they are written again.
- **Git is asked again before burial.** Anything registered by rule or proposal is not buried if it has been committed by the time it expires.
- **Proposals expire on their own.** Files that no longer exist are not listed; the background runner asks Git per project in batches, drops entries that were committed or deleted, and withdraws proposals with nothing left (recorded in the audit log, not counted as a rejection). If the project folder or repository is temporarily unreachable (renamed, drive offline), the proposal is kept as is.
- **Grave notices.** When an agent's tool call touches a path that is buried and still restorable (or something inside a buried directory), Obituary tells it the grave number, when it was buried, how long it stays restorable and `obit resurrect <id>`. It only informs: the tool call is never blocked or changed, and each grave is mentioned once per session. Claude Code and Codex hear it before the tool runs; other agents at the point listed in `obit init --show`. Existing installations get it by running `obit init` again.
- **Session summary (optional).** To save the agent's tokens, a session starts with only the proposals that need your decision. Turn on "Session summary" in the desktop app's Settings, or set `[hooks] session_summary = true` in `config.toml`, and the agent also hears this project's items due within 24 hours and its recent restorable burials (`obit mktemp` scratch directories excluded).
- **Upcoming expiries (with the session summary on).** At session start the agent sees this project's items due within 24 hours.

```powershell
obit proposals           # proposals waiting for you
obit approve 3           # approve: register what it lists
obit reject 3            # reject
```

## Codex

```powershell
obit init -g --agent codex   # ~/.codex/hooks.json plus a block in AGENTS.md; obit init --agent codex installs into this project
```

- One `hooks.json` serves Windows and WSL: `command` is `/mnt/d/…/obit.exe hook codex` (WSL calls the Windows build through interop) and `commandWindows` is the Windows command. `/mnt/<drive>/…` paths from WSL become `<drive>:\…`; paths inside WSL's own file system (`/home/…`) are skipped for now.
- Codex edits files with `apply_patch`; Obituary reads the patch for added, changed and moved files (deleted ones do not count).
- **Codex runs only hooks you trusted.** After installing, open Codex, run `/hooks` and trust Obituary's hooks. Project hooks also need the project itself to be trusted.
- A project install refuses to write if `.codex`, `.claude` or a backup file is a link pointing outside the project.

## Other agents

```powershell
obit init -g --show                         # every supported agent, how far each integration goes, what is installed
obit init -g --agent gemini,cursor          # install one or several; without -g, into this project only
obit init -g --agent gemini --mcp --skill   # also register the MCP server and install the Skill (where supported)
obit init -g --agent gemini --uninstall     # remove only what Obituary added
```

Without `--agent` only Claude Code is installed (Codex as in the previous section); Obituary never installs into other agents on its own.

| Method | Agents |
|---|---|
| Native hooks | GitHub Copilot (VS Code and Copilot CLI), Cursor, Gemini CLI, Factory Droid, Trae / Trae CN, Kimi, Mistral Vibe, Google Antigravity; Tencent CodeBuddy (CLI and IDE), Qoder / Qoder CN / Lingma, Zhipu ZCode |
| Plugin (Obituary installs a small plugin that calls `obit hook`) | OpenCode, OpenClaw, Pi, Oh My Pi, Hermes, Cline, Kilo Code, MiniMax Code, DeepSeek Harness |
| Rules file and MCP only | Roo Code, Windsurf, WorkBuddy, QoderWork |

- Agents differ in what they support: registration after writes, the session-start notice, the delete guard, MCP and the Skill. The [agent coverage table](agents.md) lists "supported / limited / unsupported" with reasons; it is generated by `obit init --show --format markdown` from the capability declarations in the code, and a test fails if the two disagree. The delete guard is enabled only where the host proves the command runs locally and in which shell.
- Where public documentation is not enough to write safely (OpenClaw's JSON5 config, Windsurf's and Cursor's global rules, some MCP locations), `obit init` prints the complete manual steps and reports "incomplete, manual steps required". It never pretends to have installed something and never writes to guessed folders.
- Plugins need you to confirm they run locally (for example `OBIT_OPENCODE_LOCAL=1` or `OBIT_CLINE_LOCAL_SHELL=bash`); the install output says what to set. Operations that run remotely or in the cloud never register a local file with the same name.

## Docker containers

```powershell
obit docker add web-1 3d       # register a container by name or ID; it expires in three days
obit docker list               # registered containers (--json works too)
obit resurrect <id>            # start a container that was stopped at expiry
```

- Obituary manages containers only. Volumes hold data and removing one is permanent, so volumes, images and networks are never touched.
- **Expiry means stopping.** After `docker stop` the container and its writable layer remain; `obit resurrect <id>` runs `docker start` and stops tracking its lifespan. Only after the retention period (`[graveyard] retention_days`, 7 days by default) is it removed with `docker rm`, never with `-v` or `-f`; if it is running by then (you or someone started it), it counts as resurrected and is kept. `obit purge` and `--all` also wait for the retention period for containers. Recycle mode does not apply to containers: they are still stopped first and removed after retention.
- **Containers are identified by full ID.** Names are only for display; before every stop, start or remove the full ID and creation time are checked again, and nothing happens if they differ. Obituary also records the Docker context and daemon a container lives on and only ever acts on that daemon, so switching `docker context` cannot redirect it. A container you removed outside Obituary is recorded as removed externally and its record is cleared. A container started with `--rm` (AutoRemove) would be deleted by Docker when stopped, so Obituary refuses to register it.
- **Containers from an agent session are only proposed.** When a session ends, Obituary lists in the background the containers created during it: only those whose compose working directory or bind mounts are inside the project; others may be ones you started elsewhere and are not listed. Nothing is tracked until you `obit approve`. Hooks never call Docker; if Docker is not installed or not running, this is skipped quietly and `obit doctor` explains why.

## Delete guard: anything an agent deletes can come back

```powershell
obit init -g --guard-rm                 # Claude Code; add --agent codex for Codex. Off by default
```

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/readme/delete-guard-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="assets/readme/delete-guard-light.svg">
    <img alt="Delete guard: the agent issues rm -rf build; Obituary stops this one execution (deny, never rewrite) and tells the agent the equivalent obit rm -rf build; the agent runs that, the folder goes to the graveyard, and obit resurrect &lt;id&gt; puts it back" src="assets/readme/delete-guard-light.svg" width="100%">
  </picture>
</p>

When it is on and an agent runs `rm -rf build`, `rmdir`, `git clean -fdx`, `Remove-Item -Recurse` or a similar delete, Obituary stops that execution and tells the agent the equivalent command, such as `obit rm -rf build`. Once the agent uses it, things go to the graveyard instead of disappearing, and `obit resurrect <id>` puts them back.

- **Deny, never rewrite.** Obituary does not rewrite the agent's command: Claude Code checks permissions against the rewritten command, so rewriting would defeat your `deny` rules. Denying and letting the agent issue the equivalent command keeps all your permission rules in force.
- **A literal delete is always stopped.** When `rm`, `rmdir`, `git clean` or `Remove-Item` (including aliases such as `del`, `rd`, `ri`) appears at any command position, whether after `&&`, `||`, `;`, in a pipeline, `$(…)`, a subshell, a condition or a loop, that execution is stopped: for example `rm -rf build && npm run build`, `rm -rf build 2>/dev/null` or `git clean -fdx && npm ci`. Stopping runs nothing, so stopping more can never delete the wrong thing. `rm` inside strings, comments or `echo` arguments does not count; previews such as `rm --help`, `git clean -n` and `Remove-Item -WhatIf` do not either; deletes run by another program (`sudo rm`, `xargs rm`, `find -delete`) pass through (recorded in the audit log with `OBIT_HOOK_AUDIT=1`).
- **Never falls back to a real delete.** Every path given to `obit rm` / `obit remove-item` / `obit git-clean` is buried; when the guard refuses one (drive roots, system folders, the user profile itself and so on) the error follows `rm`'s format and the exit code is non-zero.
- Files tracked by Git are buried too, so uncommitted changes can be recovered (this is a deliberate delete by the agent, unlike automatic registration, which never touches tracked files).
- `obit rm -rf` on a folder that contains `.git` buries the whole repository: a real `rm -rf` would destroy it, burial can at least be undone, and refusing might push the agent to `/bin/rm`. Protections that cannot be forced (system folders, user folders) still apply.
- **The equivalent command is deliberately narrow.** It must never select more than the original, so Obituary does not translate across shells and only offers one for the forms in the table below. Otherwise the delete is still stopped; the reason says why no equivalent applies and that `obit bury <path>` (literal paths) and `obit git-clean` delete restorably, and the agent rewrites the command with them.

  <details>
  <summary>What is stopped and what passes, per host</summary>

  | Host | Stopped, with an equivalent command | Stopped, with the reason and general guidance | Passes |
  |---|---|---|---|
  | Claude Code (Bash tool) | `rm`, `rmdir`, `git clean`, optionally preceded by a literal `cd x` (joined with `&&`, `;` or a newline); `rm -f *.log` (when no file in the folder starts with `-`), `rm -rf ../dist`, `rm -- "$target"` (variables after `--`) | Every other literal delete: other statements before or after (`npm …`, `trap`, `hash`), variables before `--` (they may expand to options), POSIX absolute paths starting with `/`, redirections, `\|\|`, pipelines, `$(…)`, interactive deletes, `/usr/bin/rm` with a path | Commands that do not delete |
  | Codex, under WSL's `/mnt/<drive>` | A single `rm` / `rmdir` whose arguments are relative paths without `..`, `~`, wildcards or quotes (use `obit.exe` through interop) | Every other literal delete, including `git clean` and paths into WSL's own file system | A working directory inside WSL's own file system |
  | Codex, declared Windows PowerShell | A single `Remove-Item` or alias with one literal path (`-Recurse`, `-Force` allowed) | Every other literal delete: multiple statements, pipelines, other parameters such as `-ErrorAction`, provider paths, `~`, wildcards, backticks | Text inside curly-quoted strings |
  | Codex on Windows with an unknown shell, pwsh on Linux, `git rm` | — | — | Everything |

  More rules for the equivalent command: the delete must be the **last segment** of the command, joined only with `&&`, `;` or newlines (`obit rm` and `rm` do not succeed and fail in exactly the same cases, so a later command that depends on the exit code could diverge). File names ending in a dot or space get none (Windows would normalise them into another name), and neither do Bash commands containing carriage returns, NUL bytes, backticks, or both a backslash continuation and a `#` comment, or more than one continuation. A literal delete in any of these is still stopped. Use the same `obit` path in commands as the installed hook.

  </details>

- You can use them yourself: `obit rm`, `obit rmdir` and `obit git-clean` accept the common non-interactive options of `rm`, `rmdir` and `git clean` (unsupported ones such as `-i` or `rmdir -p` are errors, not silently ignored). `obit remove-item` takes one literal path (no wildcards, ignores `$WhatIfPreference`) and is not a full `Remove-Item`; for wildcards use `obit rm *.log` in bash. Known limits: [docs/architecture.md](architecture.md#威胁模型).
- Turn it off temporarily with `OBIT_GUARD_RM=0`. `obit stats` shows how many deletes were stopped and how much was buried.

## MCP server and Agent Skill

```powershell
obit init -g --mcp --skill                 # Claude Code; add --agent codex for Codex. Skill is installed by default; MCP is optional
obit init -g --show                        # also shows whether mcp / skill are installed and current
```

- **`obit mcp`** is a stdio MCP server exposing Obituary's operations as tools the agent can call: `obit_mark`, `obit_mktemp`, `obit_bury`, `obit_resurrect`, `obit_extend`, `obit_spare`, and the queries `obit_list`, `obit_graveyard`, `obit_proposals`, `obit_stats` (`obit_graveyard` also records verified interrupted resurrections; the rest are read-only).
- **Your decisions stay yours.** Approving or rejecting proposals, trusting `.obitrules`, purging the graveyard and changing configuration have no MCP tools; the tool list is fixed by tests. The agent can only pass a proposal on to you.
- Paths must be absolute: the MCP server's working directory is not necessarily the agent's, so relative paths are not guessed. IDs are written `5` or `#5`; an unknown ID is an error and never falls back to a file name. At most 100 items per call; notes and session text up to 4 KiB (the same as the CLI); `obit_list`, `obit_graveyard` and `obit_proposals` return up to 200 entries per page, paged by ID with `limit` / `after`. Receipts carry a `version`; pass it back when paging, and if the list changed between pages you get `changed: true` and should read again from the start. Overlong text in receipts is truncated and marked, so receipts always fit the protocol's size limit.
- <details>
  <summary>Where it is registered</summary>

  - For Claude Code's user scope Obituary uses the official command (`claude mcp add --scope user --transport stdio obituary -- <obit.exe> mcp` / `claude mcp remove --scope user obituary`; `--dry-run` prints the full command). This needs Claude Code 2.1.318 or newer (`claude update`): older versions write that file without taking a lock. `~/.claude.json` is a state file Claude Code rewrites constantly, so Obituary only reads it. Project scope writes `.mcp.json` directly.
  - Codex gets `[mcp_servers.obituary]` in `config.toml`, wrapped in `# obit:mcp:begin` / `# obit:mcp:end` comments. Uninstalling removes the whole marked block, including options you added inside it (such as `enabled` or `env`). If the command or arguments inside the block were changed, or `obituary` is also defined outside it, Obituary does not recognise the block and neither installs nor removes it.

  </details>

- The **Agent Skill** is a `SKILL.md` telling the agent when to give its output a lifespan, how to delete restorably, and which decisions belong to you. Claude Code gets it in `~/.claude/skills/obituary/`, Codex in `~/.agents/skills/obituary/` (projects: `.claude/skills/`, `.agents/skills/`). A `.obituary-skill` file next to it records the hash at install time: once you edit the Skill, Obituary no longer overwrites or removes it.
- A plain `obit init -g` installs or refreshes the Skill (`--no-skill` skips it) and refreshes an existing Codex MCP block (for example after `obit.exe` moved). MCP registration remains opt-in. Claude Code's user-scope MCP registration is the exception: the official command can only remove and re-add, which is not atomic, so a plain refresh only reports "outdated" with the manual command; pass `--mcp` to replace it. A broken extension config never blocks installing the hooks; it only produces a warning.
- The installer runs only a `claude.exe` from an absolute PATH directory (for npm installs it follows `claude.cmd` to the exe behind it) and refuses one in the current folder or repository, so running `obit init` inside someone else's repository cannot execute a program planted there. The check uses the folder's identity on disk, not its spelling, so another name (a junction, `\\localhost\C$`, a short name, a hard link) does not get around it. Manual commands whose arguments contain double quotes need PowerShell 7.3 or newer.
- With `mode = "recycle"` in `config.toml`, burials go to the system Recycle Bin; the MCP tool descriptions say so and `obit resurrect` cannot bring things back from there.
- Restart the agent after installing. `obit doctor` checks whether both are installed and point at the current `obit.exe` (not installed is fine; they are optional). It only shows the configuration is right, not that the agent has connected.

## Automatic lifespans for agent output

```powershell
obit rules                       # the rules in effect here (built-in plus a trusted .obitrules)
obit rules check debug.log out/  # preview what would be registered and by which rule (changes nothing)
obit rules apply debug.log       # register by rule
$dir = obit mktemp 1d            # a temporary folder that buries itself: .obit/tmp/ in a project (not in Git), otherwise the system temp folder; in bash: dir=$(obit mktemp 1d)
```

Built-in rules cover common agent clutter: `.playwright-mcp/`, `test-results/`, `__pycache__/`, `*.log` and more. A `.obitrules` file at the project root uses `.gitignore` syntax with a lifespan after each pattern:

```
output/**        7d      # every item under output gets its own lifespan
!output/keep/**          # never touch (negation wins, even over registering a parent folder)
test-results/    off     # turn off a built-in rule
```

Unlike `.gitignore`: negation wins rather than the last match; on Windows matching ignores case (including non-ASCII letters, regardless of Git's `core.ignorecase`); an invalid pattern (such as an unclosed `[`) is an error naming the file and line, and automatic registration in that project pauses instead of quietly treating it as no match. The same happens if a trusted `.obitrules` is deleted, until you run `obit untrust`.

**Hard rules for automatic registration:** only untracked items inside a Git work tree; a folder with even one tracked file is never touched; if Git cannot answer, skip; an existing lifespan is never shortened.

**Project rules must be trusted first.** A repository's own `.obitrules` has no effect until you have reviewed it and run `obit trust` (so someone else's repository cannot use rules to take your private files). Trust records the file's SHA-256; any change needs a new review. Non-interactive sessions need `--yes`; in CI you can set `OBIT_TRUST_PROJECT_RULES=1`.

Durations: `s` `m` `h` `d` `w`, combinable (`1d12h`). Query and result commands (`list`, `graveyard`, `proposals`, `stats`, `mark`, `bury`, `resurrect` and others) support `--json` for scripts and agents; delete wrappers such as `obit rm` keep their original command's output format and have no `--json`.

A bare number in `resurrect` and `purge` (`5`, `#5`) is always an ID; an unknown ID is an error and never falls back to a file name. For a file whose name is a number, write `./5` or an absolute path. Paths given with short names (such as `DIRECT~1`) are expanded to long names first.

## Safety

- **Bury, then purge.** Expired items move to the graveyard and are kept for 7 days (`retention_days` under `[graveyard]` in `config.toml`) before the real delete; until then, `obit resurrect` brings them back. The graveyard is on the same drive (the data directory's graveyard on that drive, a hidden `.obituary` at the root of other drives), so burial is a rename: fast, and never half done. You can also send burials straight to the system Recycle Bin (`mode = "recycle"` under `[graveyard]`); then you restore from the Recycle Bin and `obit resurrect` cannot help.
- **Purging is the only permanent delete.** Every burial creates its own grave folder and records its file ID. Before purging or resurrecting, Obituary checks that the grave is still that folder, holds exactly one item, and that item is the object that was buried. A damaged ledger or a graveyard swapped for a link therefore cannot delete anything outside the graveyard.
- **Resurrection only goes back to where it came from.** If the name is taken there, nothing is overwritten: the item comes back next to it as `name (resurrected).ext`, both are kept, and the receipt shows the real path. The original path is the original location: if a parent folder was deleted it is recreated, and if it was replaced by another ordinary folder the item still goes there. What is refused is being redirected: a parent that became a junction or symbolic link, a junction retargeted, a drive letter now on another volume (the grave stays as it is). The target folder is pinned for the final move, so a rename or relink after the check cannot change where it lands. Resurrecting by path prefers an exact match; a match that differs only in letter case must be unique, otherwise use the ID.
- **Files, not file names.** Every record keeps the volume serial number and 128-bit file ID: renamed or moved files are still found, and a different file now at the old path is left alone. Identity is checked again right before deleting. Files with more than one hard link are refused.
- **Protected paths.** Drive roots, `C:\Windows`, Program Files, ProgramData, and the user profile, Desktop, Documents, Downloads and similar folders themselves (and their parents) are refused, including redirected ones. Their contents can be given lifespans. Short names and junctions in parent folders are resolved to real paths first; links themselves are always refused.
- **Git repositories.** `obit add` and `obit bury` on a folder containing `.git` at any depth need `--force`, and the check is repeated at expiry; if it cannot finish, the burial is refused. The exception is the delete guard's `obit rm -rf`, which buries the whole thing exactly as a real `rm -rf` would delete it (see [Delete guard](#delete-guard-anything-an-agent-deletes-can-come-back)).
- **When unsure, don't delete.** A record is given up only when the file is known to be gone. Access denied, a removed volume and similar cases are retried later.
- **Everything is recorded.** Every add, extension, cancellation and deletion goes into the audit log (`obit log`).

## Background

By default `obitd.exe` lives in the tray, starts at login and buries things on time to the second; the tray icon is its only interface. A Windows scheduled task also runs `obitd --once` every 5 minutes (and at each login) as a fallback, so expired items are still reaped and old graves purged if the tray is closed or never started. The task is hidden, runs on battery, and catches up on missed runs (shutdown, sleep). Agent hooks also do a round at session start and end; `obit reap` runs one now. Queries such as `list`, `graveyard`, `stats` and `log` are read-only and never bury anything.

```powershell
obit init                       # tray starts at login (HKCU\...\Run); --uninstall removes it
obit daemon enable              # tray at login plus the fallback scheduled task (--dry-run lists what would be installed)
obit daemon status              # current mode and the next expiry (--json)
obit daemon disable             # remove the scheduled task and login start, and stop the running daemon (--dry-run previews)
```

After registering (`add`, `approve`, `mktemp`, `rules apply`) or burying, if no runner serves your data directory, Obituary starts a temporary resident process and suggests `enable`. A scheduled task serving a different data directory (for example after moving it with `OBIT_HOME`) counts as none; `obit daemon status` points this out.

When several agents work at once and a hook cannot get the ledger lock in time, the registration is queued on disk and applied by the next process that holds the lock; `obit doctor` shows the queue if it backs up.

Data lives in `%LOCALAPPDATA%\obituary`: `store.json` (ledger), `audit.jsonl` (audit log), `obitd.log` (daemon log), `graveyard\` (this drive's graveyard); the full list is in [docs/architecture.md](architecture.md#数据布局). Configuration is `%APPDATA%\obituary\config.toml`; `obit config` shows it and `obit config --create` writes a commented default:

```toml
[graveyard]
mode = "graveyard"      # or "recycle": straight to the system Recycle Bin
retention_days = 7      # days the graveyard keeps things
```

A misspelled key makes the configuration invalid instead of being ignored: the error and `obit doctor` name the file, line and key, and suggest the closest correct key (for example `retention` → `retention_days`). Otherwise a typo would leave the default in force and graves would be purged earlier than you expect.

`OBIT_HOME` (data), `OBIT_CONFIG_DIR` (configuration) and `OBIT_USER_HOME` (user home) move these locations; when looking for Claude Code and Codex configuration, `obit init` also honours `CLAUDE_CONFIG_DIR` and `CODEX_HOME`. The tests use these variables for isolation.

## Build

Go 1.26+. On Windows:

```bash
go build -o bin/obit.exe ./cmd/obit
go build -ldflags=-H=windowsgui -o bin/obitd.exe ./cmd/obitd   # required, or a console window appears at login
CGO_ENABLED=0 go build -tags desktop,production -ldflags "-H windowsgui" -o bin/obit-gui.exe ./cmd/obit-gui
go test -short ./...                                           # the fast tier; the full suite runs in CI
```

On macOS / Linux there are two programs, the command line and the daemon:

```bash
go build -o bin/obit ./cmd/obit
go build -o bin/obitd ./cmd/obitd
```

Keep the three Windows executables in one folder, and the two macOS / Linux programs likewise. The small window's frontend is in `cmd/obit-gui/frontend` (React, pnpm); the built `dist/` is committed, so changing Go code needs no Node. When changing the UI: `pnpm install`, `pnpm dev` for a preview, `pnpm build` to update `dist/`. Real Recycle Bin tests are skipped unless `OBIT_TEST_RECYCLE=1`, in which case test files go to your Recycle Bin.

