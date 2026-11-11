# How it works

## The life of a file

1. **Register**: give a file or folder a lifespan, by hand with `obit add debug.log 2h` or [automatically by rule](./auto-register).
2. **Expire**: `obit list` shows what is left. Before then, `obit extend` adds time and `obit spare` stops tracking it while keeping the file.
3. **Bury**: when time is up it is reaped into the graveyard (`obit graveyard`), kept 7 days by default.
4. **Resurrect or purge**: during retention, `obit resurrect <id>` puts it back where it was and never overwrites; after retention it is purged (`obit purge`), the real delete.

Not sure something is still needed? `obit bury` it now: it stays restorable as it was for seven days.

## Where the graveyard lives

The graveyard is on the same drive (the data directory's graveyard on that drive, a hidden `.obituary` at the root of other drives), so burial is a rename: fast, and never half done.

Retention is set in the configuration. You can also send burials straight to the system Recycle Bin; then you restore from the Recycle Bin and `obit resurrect` cannot help:

```toml
[graveyard]
mode = "graveyard"      # or "recycle": straight to the system Recycle Bin
retention_days = 7      # days the graveyard keeps things
```

Configuration is `%APPDATA%\obituary\config.toml`; `obit config` shows it and `obit config --create` writes a commented default.

## Why nothing else can be deleted

- **Purging is the only permanent delete.** Every burial creates its own grave folder and records its file ID. Before purging or resurrecting, Obituary checks that the grave is still that folder, holds exactly one item, and that item is the object that was buried. A damaged ledger or a graveyard swapped for a link therefore cannot delete anything outside the graveyard.
- **Resurrection only goes back to where it came from.** If the name is taken there, nothing is overwritten: the item comes back next to it as `name (resurrected).ext`, both are kept, and the receipt shows the real path. A deleted parent folder is recreated; a parent that became a junction or symbolic link, a junction retargeted, or a drive letter now on another volume is refused (the grave stays as it is).
- **Files, not file names.** Every record keeps the volume serial number and 128-bit file ID: renamed or moved files are still found, and a different file now at the old path is left alone. Identity is checked again right before deleting. Files with more than one hard link are refused.
- **Protected paths.** Drive roots, `C:\Windows`, Program Files, ProgramData, and the user profile, Desktop, Documents, Downloads and similar folders themselves (and their parents) are refused, including redirected ones. Their contents can be given lifespans.
- **Git repositories.** `obit add` and `obit bury` on a folder containing `.git` at any depth need `--force`, and the check is repeated at expiry; if it cannot finish, the burial is refused.
- **When unsure, don't delete.** A record is given up only when the file is known to be gone. Access denied, a removed volume and similar cases are retried later.
- **Everything is recorded.** Every add, extension, cancellation and deletion goes into the audit log (`obit log`).

## Background

By default `obitd.exe` lives in the tray, starts at login and buries things on time to the second; the tray icon is its only interface. A Windows scheduled task also runs `obitd --once` every 5 minutes (and at each login) as a fallback, so expired items are still reaped and old graves purged if the tray is closed or never started. Agent hooks also do a round at session start and end; `obit reap` runs one now.

```powershell
obit daemon enable              # tray at login plus the fallback scheduled task (--dry-run lists what would be installed)
obit daemon status              # current mode and the next expiry (--json)
obit daemon disable             # remove the scheduled task and login start, and stop the running daemon (--dry-run previews)
```

Queries such as `list`, `graveyard`, `stats` and `log` are read-only and never bury anything.

## Where the data lives

Data lives in `%LOCALAPPDATA%\obituary`: `store.json` (ledger), `audit.jsonl` (audit log), `obitd.log` (daemon log), `graveyard\` (this drive's graveyard).

`OBIT_HOME` (data), `OBIT_CONFIG_DIR` (configuration) and `OBIT_USER_HOME` (user home) move these locations.
