# Commands

Replace `<id>` with the number from an `obit add` or `obit bury` receipt, or from `obit list` / `obit graveyard`. Durations: `s` `m` `h` `d` `w`, combinable (`1d12h`).

## Lifespans

```powershell
obit add debug.log 2h                       # bury it in 2 hours (alias: mark)
obit add output\shots 1d12h                 # folders work too
obit add report.zip --at "2026-12-01 18:00" # at a given moment
obit list                                   # what is left and for how long
obit extend <id> 1d                         # one more day
obit spare <id>                             # stop tracking, keep the file (alias: cancel)
obit reap                                   # bury everything that is due now (alias: sweep; --dry-run previews)
```

## The graveyard

```powershell
obit bury old-build                         # bury right away
obit graveyard                              # what is in the graveyard and when it will be purged (--here: this folder only)
obit resurrect <id>                         # put it back; if the name is taken, it comes back renamed, never overwriting
obit resurrect --last                       # undo the last deletion: everything one obit bury / obit rm buried comes back
obit purge                                  # purge what is past retention (--all for everything, after confirming; --dry-run previews)
obit purge <id>                             # purge this grave now (permanent; asks first, --yes outside a terminal)
```

A bare number in `resurrect` and `purge` (`5`, `#5`) is always an ID; an unknown ID is an error and never falls back to a file name. For a file whose name is a number, write `./5` or an absolute path.

## Delete wrappers

```powershell
obit rm -rf build            # rm's common options; every path is buried
obit rmdir empty-dir
obit git-clean -fdx
obit remove-item .\build     # one literal path only
```

## Automatic lifespans and proposals

```powershell
obit rules                       # the rules in effect here
obit rules check debug.log out/  # preview what would be registered and by which rule
obit rules apply debug.log       # register by rule
obit trust                       # trust this project's .obitrules (--yes when non-interactive)
obit proposals                   # proposals waiting for you
obit approve 3                   # approve
obit reject 3                    # reject
$dir = obit mktemp 1d            # a temporary folder that buries itself
```

## Folder policies and Docker

```powershell
obit watch C:\Users\you\Downloads 2w
obit watch --list
obit watch --remove C:\Users\you\Downloads
obit docker add web-1 3d
obit docker list
```

## Integration and background

```powershell
obit init                       # Claude Code hooks and Skill, context menu, Start menu and desktop shortcuts; the tray starts at login
obit init -g --agent codex      # Codex
obit init -g --guard-rm         # delete guard
obit init -g --mcp              # add the MCP server
obit init --no-context-menu --no-shortcuts --no-skill   # skip the context menu, shortcuts or Skill
obit init -g --show             # what is installed
obit daemon enable              # tray at login plus the fallback scheduled task
obit daemon status
obit daemon disable
```

## Queries and checks

```powershell
obit doctor              # read-only check (--json)
obit stats               # eulogy: how much was buried, space freed, resurrections (alias: eulogy)
obit log                 # the full record of everything done
obit config              # show the configuration; --create writes a commented default
obit hook audit          # with OBIT_HOOK_AUDIT=1: what each hook received and did
obit uninstall           # remove everything Obituary set up (try --dry-run first)
```

Query and result commands (`list`, `graveyard`, `proposals`, `stats`, `mark`, `bury`, `resurrect` and others) support `--json` for scripts and agents; delete wrappers such as `obit rm` keep their original command's output format and have no `--json`.

## Environment variables

| Variable | Effect |
|---|---|
| `OBIT_DISABLED=1` | turn hooks off temporarily |
| `OBIT_GUARD_RM=0` | turn the delete guard off temporarily |
| `OBIT_HOOK_AUDIT=1` | record what each hook received and did |
| `OBIT_TRUST_PROJECT_RULES=1` | trust a project's `.obitrules` in CI |
| `OBIT_HOME`, `OBIT_CONFIG_DIR`, `OBIT_USER_HOME` | move the data, configuration and user home locations |
