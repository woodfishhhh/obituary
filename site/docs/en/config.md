# Configuration Reference

Obituary reads `config.toml` from its configuration directory. Every key is optional; omitted keys use the defaults below. `obit config` shows the file location and effective values.

## Keys

| Key | Default | Values or format | Effect |
| --- | --- | --- | --- |
| `codex_windows_shell` | unset | `bash`, `powershell` | Declares the shell used by Codex on Windows; ambiguous delete commands pass through when unset. |
| `agent_shells.<agent>` | unset | `bash`, `powershell` | Declares the Windows shell for one hook agent. |
| `graveyard.mode` | `graveyard` | `graveyard`, `recycle` | Moves expired items to the graveyard or the system Recycle Bin. |
| `graveyard.retention_days` | `7` | Integer from `1` to `3650` | Days a graveyard item stays restorable. |
| `watch.stable_window` | `10m` | Go duration, such as `30s`, `10m`, `1h` | Waits for a folder-policy item to become stable before moving it. Must be positive. |
| `explorer.enabled` | `true` | `true`, `false` | Writes Explorer date labels. |
| `explorer.everywhere` | `false` | `true`, `false` | Shows dates in every supported folder. |
| `explorer.in_git` | `false` | `true`, `false` | Shows dates inside Git worktrees. |
| `ui.language` | `auto` | `auto`, `en`, `zh-CN` | Language used by the CLI, window and tray. |
| `ui.tray` | `true` | `true`, `false` | Enables the tray or menu-bar entry. |
| `ui.taskbar` | `true` | `true`, `false` | Gives the Windows window its own taskbar button. |
| `hooks.session_summary` | `false` | `true`, `false` | Shows a summary when a session starts. |
| `hooks.session_cleanup` | `true` | `true`, `false` | Cleans up files newly created by this session that remain uncategorized. |
| `proposals.mode` | `off` | `off`, `tiered`, `auto`, `ask` | Controls which automatic registrations wait for approval. |
| `update.check` | `true` | `true`, `false` | Checks for stable releases automatically. |
| `telemetry.enabled` | `true` | `true`, `false` | Allows usage reporting. This is independent of update checks. |

`agent_shells` is a table keyed by agent name, for example `[agent_shells] codex = "powershell"`. `codex_windows_shell` is the legacy Codex spelling; conflicting values are rejected.

## Formats and errors

Durations use Go syntax: a number followed by `ns`, `us`, `ms`, `s`, `m` or `h`, and they can be combined such as `1h30m`. Retention must be between `1` and `3650` days. `graveyard.mode`, `ui.language` and `proposals.mode` accept only the listed values.

Unknown keys are not ignored. Obituary reports the config path, line number and complete key, with a suggestion when the spelling is close. TOML syntax errors also make `obit config` fail; fix the file and run it again.

## Common commands

```powershell
obit config
obit config --create
obit config --language en
obit config telemetry off
obit config telemetry on
```

`--create` writes the commented default template only when the file does not exist. `--language` changes the interface language; `auto`, `en` and `zh-CN` are the accepted values. `telemetry on|off` changes only `telemetry.enabled`; it does not change `update.check`, which must be edited separately under `[update]`.
