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

## Rules and proposals

```powershell
obit rules                       # the rules in effect here
obit rules check debug.log out/  # preview what would be registered and by which rule
obit rules apply debug.log       # register by rule
obit trust                       # trust this project's .obitrules (--yes when non-interactive)
obit proposals                   # proposals waiting for you, and what was registered without asking
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
obit config telemetry off # disable usage reporting and clear the local installation ID; update checks are separate
obit hook audit          # with OBIT_HOOK_AUDIT=1: what each hook received and did
obit uninstall           # remove everything Obituary set up (try --dry-run first)
```

Query and result commands (`list`, `graveyard`, `proposals`, `stats`, `mark`, `bury`, `resurrect` and others) support `--json` for scripts and agents; delete wrappers such as `obit rm` keep their original command's output format and have no `--json`.

## Environment variables

| Variable | Effect |
|---|---|
| `OBIT_DISABLED=1` | turn hooks off temporarily |
| `OBIT_TELEMETRY=0`, `DO_NOT_TRACK=1` | disable usage reporting for this process; CI does not report |
| `OBIT_GUARD_RM=0` | turn the delete guard off temporarily |
| `OBIT_HOOK_AUDIT=1` | record what each hook received and did |
| `OBIT_TRUST_PROJECT_RULES=1` | trust a project's `.obitrules` in CI |
| `OBIT_HOME`, `OBIT_CONFIG_DIR`, `OBIT_USER_HOME` | move the data, configuration and user home locations |



## Delete-wrapper compatibility

- git-clean accepts Git clean options -d, -f, -n, -q, -x, -X, -e pattern, --exclude pattern and --; it also accepts --dry-run, --force and --quiet.
- rm accepts the common -r, -R, -f, -d, -v, --recursive, --force, --dir, --verbose, --preserve-root and -- options, followed by paths.
- rmdir accepts only -v or --verbose and paths; options such as rmdir -p are rejected.
- Windows remove-item accepts only -Recurse, -Force, -Path or -LiteralPath and exactly one literal path. For wildcards in Bash, use obit rm.

<!-- generated-command-reference:start -->
## Parameter reference

The tables below are generated from the current `obit --help` output.

### `obit add`
Aliases: `add`, `mark`
Usage: `obit add <path>... <duration> \| add <path>... --at <time> [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--at` | none | `""` | expire at a local time: "2006-01-02 15:04", "2006-01-02" or "15:04" |
| `--force` | none | `false` | allow folders that contain a .git repository |
| `--help` | `-h` | `false` | help for add |
| `--json` | none | `false` | print JSON |
| `--no-daemon` | none | `false` | do not start the background daemon |
| `--note` | none | `""` | free-text note shown in the list |

Examples:

```text
obit add debug.log 2h
obit add output\shots 1d12h
obit add report.zip --at "2026-12-01 18:00"
```

### `obit approve`
Usage: `obit approve <proposal id>... [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for approve |
| `--json` | none | `false` | print JSON |

### `obit bury`
Usage: `obit bury <path>... [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--force` | none | `false` | 允许包含 .git 仓库的文件夹 |
| `--help` | `-h` | `false` | help for bury |
| `--json` | none | `false` | 输出 JSON |
| `--note` | none | `""` | 随下葬保存的自由文本备注 |

### `obit cancel`
Aliases: `cancel`, `spare`
Usage: `obit cancel <id\|path>... [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for cancel |
| `--json` | none | `false` | print JSON |

### `obit completion`
Usage: `obit completion [command]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for completion |

### `obit completion bash`
Usage: `obit completion bash`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for bash |
| `--no-descriptions` | none | `false` | disable completion descriptions |

### `obit completion fish`
Usage: `obit completion fish [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for fish |
| `--no-descriptions` | none | `false` | disable completion descriptions |

### `obit completion powershell`
Usage: `obit completion powershell [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for powershell |
| `--no-descriptions` | none | `false` | disable completion descriptions |

### `obit completion zsh`
Usage: `obit completion zsh [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for zsh |
| `--no-descriptions` | none | `false` | disable completion descriptions |

### `obit config`
Usage: `obit config [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--create` | none | `false` | write a config file with the defaults |
| `--help` | `-h` | `false` | help for config |
| `--language` | none | `""` | set the language: auto, en or zh-CN |

### `obit config telemetry`
Usage: `obit config telemetry on\|off [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for telemetry |

### `obit daemon`
Usage: `obit daemon [command]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for daemon |

### `obit daemon disable`
Usage: `obit daemon disable [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--dry-run` | none | `false` | only print the changes |
| `--help` | `-h` | `false` | help for disable |

### `obit daemon enable`
Usage: `obit daemon enable [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--dry-run` | none | `false` | only print the changes |
| `--help` | `-h` | `false` | help for enable |
| `--resident` | none | `false` | enable the tray without changing the scheduled fallback |

### `obit daemon start`
Usage: `obit daemon start [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for start |

### `obit daemon status`
Usage: `obit daemon status [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for status |
| `--json` | none | `false` | print JSON |

### `obit daemon stop`
Usage: `obit daemon stop [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for stop |

### `obit docker`
Usage: `obit docker [command]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for docker |

### `obit docker add`
Usage: `obit docker add <container name\|ID> <lifespan> [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for add |
| `--json` | none | `false` | print JSON |

### `obit docker list`
Usage: `obit docker list [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for list |
| `--json` | none | `false` | print JSON |

### `obit doctor`
Usage: `obit doctor [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for doctor |
| `--json` | none | `false` | print JSON |
| `--verbose` | none | `false` | include agent capability details |

### `obit extend`
Usage: `obit extend <id\|path>... <duration> [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for extend |
| `--json` | none | `false` | print JSON |

Examples:

```text
obit extend 3 1d
obit extend debug.log 30m
```

### `obit git-clean`
Usage: `obit git-clean [options] [--] [pathspec]... [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for git-clean |

### `obit graveyard`
Aliases: `graveyard`, `graves`
Usage: `obit graveyard [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for graveyard |
| `--here` | none | `false` | show only original paths in the current directory and its descendants |
| `--json` | none | `false` | print JSON |

### `obit help`
Usage: `obit help [command] [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for help |

### `obit hook`
Usage: `obit hook [command]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for hook |

### `obit hook antigravity`
Usage: `obit hook antigravity [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for antigravity |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook audit`
Usage: `obit hook audit [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for audit |
| `--lines` | `-n` | `40` | number of lines per source |

### `obit hook claude`
Usage: `obit hook claude [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for claude |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook cline`
Usage: `obit hook cline [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for cline |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook codebuddy`
Usage: `obit hook codebuddy [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for codebuddy |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook codebuddy-ide`
Usage: `obit hook codebuddy-ide [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for codebuddy-ide |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook codex`
Usage: `obit hook codex [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for codex |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook copilot`
Usage: `obit hook copilot [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for copilot |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook cursor`
Usage: `obit hook cursor [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for cursor |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook droid`
Usage: `obit hook droid [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for droid |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook gemini`
Usage: `obit hook gemini [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for gemini |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook hermes`
Usage: `obit hook hermes [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for hermes |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook kilocode`
Usage: `obit hook kilocode [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for kilocode |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook kimi`
Usage: `obit hook kimi [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for kimi |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook lingma`
Usage: `obit hook lingma [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for lingma |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook minimax-code`
Usage: `obit hook minimax-code [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for minimax-code |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook omp`
Usage: `obit hook omp [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for omp |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook openclaw`
Usage: `obit hook openclaw [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for openclaw |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook opencode`
Usage: `obit hook opencode [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for opencode |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook pi`
Usage: `obit hook pi [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for pi |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook qoder`
Usage: `obit hook qoder [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for qoder |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook qoder-cn`
Usage: `obit hook qoder-cn [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for qoder-cn |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook trae`
Usage: `obit hook trae [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for trae |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook trae-cn`
Usage: `obit hook trae-cn [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for trae-cn |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook vibe`
Usage: `obit hook vibe [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for vibe |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit hook zcode`
Usage: `obit hook zcode [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--event` | none | `""` | host event name when omitted from the payload |
| `--help` | `-h` | `false` | help for zcode |
| `--notice-only` | none | `false` | deliver advisory context without the deletion guard |

### `obit init`
Usage: `obit init [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--agent` | none | `[claude]` | agents to integrate with (repeat or comma-separate; default: claude) |
| `--context-menu` | none | `false` | install classic Explorer context menus (enabled by default) |
| `--dry-run` | none | `false` | only print the changes |
| `--format` | none | `"text"` | output format for --show: text or markdown (agent coverage) |
| `--global` | `-g` | `false` | user-wide instead of this project |
| `--guard-rm` | none | `false` | deny supported deletion commands with restorable obit alternatives |
| `--help` | `-h` | `false` | help for init |
| `--mcp` | none | `false` | register the Obituary MCP stdio server |
| `--no-context-menu` | none | `false` | skip Explorer context menus |
| `--no-shortcuts` | none | `false` | skip Start menu and desktop shortcuts |
| `--no-skill` | none | `false` | skip installing the Obituary Agent Skill |
| `--show` | none | `false` | show what is installed |
| `--skill` | none | `false` | install the Obituary Agent Skill (enabled by default) |
| `--uninstall` | none | `false` | remove what Obituary added |
| `--yes` | none | `false` | apply without asking |

Examples:

```text
obit init -g                 # Claude Code, for every project (~/.claude)
obit init                    # Claude Code, this project only (.claude/)
obit init -g --agent codex   # Codex (~/.codex/hooks.json + AGENTS.md)
obit init -g --uninstall     # remove only what Obituary added
obit init -g --show          # what is installed
obit init --show --format markdown > docs/agents.md
```

### `obit list`
Aliases: `list`, `ls`
Usage: `obit list [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for list |
| `--json` | none | `false` | 输出 JSON |

### `obit log`
Usage: `obit log [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for log |
| `--json` | none | `false` | print JSON |
| `--lines` | `-n` | `20` | number of events |

### `obit mcp`
Usage: `obit mcp [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for mcp |

### `obit mktemp`
Usage: `obit mktemp <lifespan> [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for mktemp |
| `--json` | none | `false` | print JSON |
| `--name` | none | `"tmp"` | name prefix for the directory |
| `--session` | none | `""` | agent session the directory belongs to |

Examples:

```text
obit mktemp 2h --name screenshots
```

### `obit proposals`
Usage: `obit proposals [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for proposals |
| `--json` | none | `false` | print JSON |

### `obit purge`
Aliases: `purge`, `cremate`
Usage: `obit purge [id\|original path]... [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--all` | none | `false` | purge every burial, not only those past their retention |
| `--dry-run` | none | `false` | only show what would happen |
| `--help` | `-h` | `false` | help for purge |
| `--json` | none | `false` | print JSON |
| `--yes` | none | `false` | purge --all or the selected burials without asking (agents: ask the user first) |

### `obit reject`
Usage: `obit reject <proposal id>... [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for reject |
| `--json` | none | `false` | print JSON |

### `obit remove-item`
Usage: `obit remove-item [-Recurse] [-Force] [-Path\|-LiteralPath] <path> [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for remove-item |

### `obit resurrect`
Aliases: `resurrect`, `raise`
Usage: `obit resurrect <id\|original path>... \| --last [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for resurrect |
| `--json` | none | `false` | 输出 JSON |
| `--last` | none | `false` | 复活当前会话或 Git 项目中最近一批删除的内容 |

### `obit rm`
Usage: `obit rm [options] <path>... [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for rm |

### `obit rmdir`
Usage: `obit rmdir [options] <path>... [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for rmdir |

### `obit rules`
Usage: `obit rules [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for rules |
| `--json` | none | `false` | print JSON |

### `obit rules apply`
Usage: `obit rules apply <path>... [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for apply |
| `--json` | none | `false` | print JSON |
| `--session` | none | `""` | agent session the paths came from |

### `obit rules check`
Usage: `obit rules check <path>... [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for check |
| `--json` | none | `false` | print JSON |
| `--session` | none | `""` | agent session the paths came from |

### `obit stats`
Aliases: `stats`, `eulogy`
Usage: `obit stats [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for stats |
| `--json` | none | `false` | print JSON |

### `obit sweep`
Aliases: `sweep`, `reap`
Usage: `obit sweep [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--dry-run` | none | `false` | only show what would happen |
| `--help` | `-h` | `false` | help for sweep |
| `--json` | none | `false` | print JSON |

### `obit trust`
Usage: `obit trust [rules file] [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for trust |
| `--yes` | none | `false` | trust without asking (only after reviewing the rules) |

### `obit uninstall`
Usage: `obit uninstall [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--dry-run` | none | `false` | only print the uninstall plan; do not change files or services |
| `--help` | `-h` | `false` | help for uninstall |
| `--purge-data` | none | `false` | permanently purge listed graves and remove owned data; keep unknown files |
| `--yes` | none | `false` | confirm the plan (and data purge only when --purge-data is also given) |

### `obit untrust`
Usage: `obit untrust [rules file] [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--help` | `-h` | `false` | help for untrust |

### `obit update`
Usage: `obit update [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--apply` | none | `false` | install the cached newer stable release |
| `--check` | none | `false` | check GitHub now, including when automatic checks are disabled |
| `--help` | `-h` | `false` | help for update |
| `--relaunch-gui` | none | `false` | reopen the desktop window after updating (Windows) |
| `--wait-pid` | none | `0` | wait for this process to exit before updating (Windows) |

### `obit watch`
Usage: `obit watch <directory> <lifespan> \| watch --list \| watch --remove <directory> [flags]`

| Option | Short | Default | Description |
| --- | --- | --- | --- |
| `--age` | none | `"first-seen"` | age basis: first-seen or modified (requires approval) |
| `--clear-exclusion` | none | `0` | explicitly clear this exclusion ID (with --policy) |
| `--help` | `-h` | `false` | help for watch |
| `--json` | none | `false` | print JSON |
| `--list` | none | `false` | list folder policies |
| `--no-daemon` | none | `false` | do not start the background runner |
| `--policy` | none | `0` | policy ID whose exclusion should be cleared |
| `--remove` | none | `""` | remove a policy and cancel its registered lifespans |
| `--remove-id` | none | `0` | remove a folder policy by ID, including missing paths |

<!-- generated-command-reference:end -->
