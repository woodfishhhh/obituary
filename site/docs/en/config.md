# Configuration reference

Obituary keeps its settings in `config.toml` in the configuration directory. The file is optional: without it, or with only a few lines in it, everything else uses its default. `obit config` tells you where the file is, whether it exists, and the value every setting actually has right now. `obit config --create` writes a commented file with all the defaults, which is the easiest place to start editing.

The configuration directory is `%APPDATA%\obituary` on Windows, `~/Library/Application Support/obituary` on macOS and `~/.config/obituary` on Linux (or `obituary` under `XDG_CONFIG_HOME` when that is set). To put it somewhere else, set the environment variable `OBIT_CONFIG_DIR`.

| Key | Default | Accepted values |
| --- | --- | --- |
| `graveyard.mode` | `graveyard` | `graveyard`, `recycle` |
| `graveyard.retention_days` | `7` | an integer from 1 to 3650 |
| `proposals.mode` | `off` | `off`, `tiered`, `auto`, `ask` |
| `hooks.session_cleanup` | `true` | `true`, `false` |
| `hooks.session_summary` | `false` | `true`, `false` |
| `watch.stable_window` | `10m` | a positive duration such as `30s`, `10m`, `1h30m` |
| `explorer.enabled` | `true` | `true`, `false` |
| `explorer.everywhere` | `false` | `true`, `false` |
| `explorer.in_git` | `false` | `true`, `false` |
| `ui.language` | `auto` | `auto`, `en`, `zh-CN` |
| `ui.tray` | `true` | `true`, `false` |
| `ui.taskbar` | `true` | `true`, `false` |
| `update.check` | `true` | `true`, `false` |
| `telemetry.enabled` | `true` | `true`, `false` |
| `agent_shells` | unset | a table: agent name = `bash` or `powershell` |
| `codex_windows_shell` | unset | `bash`, `powershell` |

## Where expired items go

By default an item whose lifespan has run out goes into Obituary's own graveyard. For `graveyard.retention_days` days (7 unless you change it) you can put it back with `obit resurrect`; only after that is it deleted for good. The number can be anything from 1 to 3650.

On Windows and Linux, set `graveyard.mode` to `recycle` to use the system Trash. Restore from the Trash, not with `obit resurrect`. macOS supports the graveyard only.

## Automatic registration and agent sessions

`proposals.mode` decides whether Obituary gives files a lifespan on its own. The default, `off`, does nothing automatically: only what an agent or you register through commands, the context menu or folder policies goes in. With `tiered`, files the agent created in this session and containers the session started go in at once and you are told afterwards, and `obit spare` takes one back; everything else waits in `obit proposals` for your approval. `auto` registers everything automatic registration finds and tells you afterwards. `ask` asks about every item first. In every mode, files Git already tracks and dependencies, build output and caches (`node_modules`, `dist`, `__pycache__` and the like) are never registered automatically. The rules are in [What goes in](./auto-register).

`hooks.session_cleanup` is on by default. Near the end of each session the agent hears about the files it created that have no lifespan and that Git does not track, and is asked to give its scratch files a lifespan (`obit mark`) or bury them; deliverables stay. `hooks.session_summary` is off by default; when you turn it on, the agent is also told at the start of a session which items in the project expire within 24 hours and what was buried recently. It is off to save tokens, because the agent is told anyway when it touches a buried path.

## Folder policies and dates in Explorer

`watch.stable_window` is the patience of a folder policy: a new file has to stay unchanged for this long before it is moved, so a file that is still downloading is not carried off. It uses Go's duration format, a number followed by `s`, `m` or `h`, which can be chained as in `1h30m`, and it must be greater than zero.

The `explorer.*` settings control Explorer display names on Windows and native date tags in Finder and Dolphin. None of these changes the real file name. By default this happens in Downloads, on the Desktop (including subfolders) and in places a folder policy manages. With `explorer.everywhere` on, other tracked locations show it too; Git workspaces are left out unless you set `explorer.in_git` to `true`; and `explorer.enabled = false` switches it off entirely. See [Folder policies](./folder-policies).

## Interface

`ui.language` sets the language of the command line, the window, the tray and the context menu. Only `en` and `zh-CN` exist so far. `auto` looks at the environment variable `OBIT_LANG`, then `LC_ALL`, `LC_MESSAGES` and `LANG`, and finally the system's interface language. `ui.tray` shows or hides the tray icon (the menu bar icon on macOS); when it is hidden, background cleanup and update checks carry on. `ui.taskbar` applies to Windows only and decides whether the window takes a taskbar button while it is open.

## Update checks and usage statistics

With `update.check` on, Obituary reads the public release information on GitHub at most once a day and sends no usage data. When it is off you can still run `obit update --check` whenever you like. `telemetry.enabled` controls the anonymous usage statistics. It is a separate switch from update checks: `obit config telemetry off` turns off statistics only, and clears the installation ID and any pending report from this computer. The details are in the [privacy policy](./privacy).

## Which shell an agent uses on Windows

The delete guard has to read the delete command an agent is about to run in order to offer the equivalent `obit` command. On Windows an agent may be using bash or PowerShell, and Obituary cannot tell which, so you declare it: this agent runs on this Windows machine, in this shell. Without a declaration, commands whose meaning is ambiguous pass through unblocked.

```toml
[agent_shells]
codex = "powershell"
```

The keys of `agent_shells` are agent names. Right now `codex`, `codebuddy`, `codebuddy-ide` and `minimax-code` read it. `codex_windows_shell` is the older spelling for Codex alone; if it and `agent_shells.codex` are both set with different values, the configuration is rejected.

## What a mistake looks like

Obituary does not quietly ignore a key it does not know. If it did, `retention = 30` would leave the 7-day retention in force while you believed it had become 30. So `obit config` reports the file path, the line number and the full key name of anything it does not recognise, and suggests the right key when the spelling is close:

```text
obit: config file has errors: …\config.toml:2: unknown key "graveyard.retention" (did you mean "graveyard.retention_days"?)
```

A value out of range is reported the same way, with the setting and what it allows, for example `graveyard.retention_days must be between 1 and 3650`. A TOML syntax error also makes `obit config` fail. Fix the file and run it again; when it stops complaining, it is fixed.

## Common commands

```powershell
obit config                  # file location, data directory and the values in force
obit config --create         # write the commented default file
obit config --language en    # save the interface language: auto, en or zh-CN
obit config telemetry off    # turn usage statistics off (on turns them back on)
```

`--create` never overwrites an existing file; if the file is there, it reports an error and stops. `--language` creates the file with the defaults when it does not exist yet.
