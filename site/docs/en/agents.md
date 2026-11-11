# Connect agents

## Claude Code

```powershell
obit init -g                         # all projects (~/.claude); obit init alone installs into this project (.claude/)
obit init -g --show --agent claude   # what is installed
obit init -g --uninstall             # remove only what Obituary added
```

`obit init` registers `obit hook claude` in `settings.json` (SessionStart, before each tool call, after writes, Stop, SessionEnd), writes an `OBIT.md` for the agent, and adds one reference line to `CLAUDE.md`: `@OBIT.md` in `~/.claude/CLAUDE.md` for the user scope (`-g`), `@.claude/OBIT.md` in the project's `CLAUDE.md` for a project.

- Your settings and key order stay as they are. Running it again adds nothing twice; if `obit.exe` moved, running it again refreshes the path.
- Every edit is preceded by a backup: `.bak` the first time, then `.bak.1`, `.bak.2` …; old backups are never overwritten.
- Any error in a hook lets the agent continue silently (the error goes to `hook.log`). `OBIT_DISABLED=1` turns hooks off temporarily.

## Codex

```powershell
obit init -g --agent codex   # ~/.codex/hooks.json plus a block in AGENTS.md; obit init --agent codex installs into this project
```

- **Codex runs only hooks you trusted.** After installing, open Codex, run `/hooks` and trust Obituary's hooks. Project hooks also need the project itself to be trusted.
- One `hooks.json` serves Windows and WSL. `/mnt/<drive>/…` paths from WSL become `<drive>:\…`; paths inside WSL's own file system (`/home/…`) are skipped for now.
- Codex edits files with `apply_patch`; Obituary reads the patch for added, changed and moved files (deleted ones do not count).

## Other agents

```powershell
obit init -g --show                         # every supported agent, how far each integration goes, what is installed
obit init -g --agent gemini,cursor          # install one or several; without -g, into this project only
obit init -g --agent gemini --mcp --skill   # also register the MCP server and install the Skill (where supported)
obit init -g --agent gemini --uninstall     # remove only what Obituary added
```

Without `--agent` only Claude Code is installed; Obituary never installs into other agents on its own.

| Method | Agents |
|---|---|
| Native hooks | GitHub Copilot (VS Code and Copilot CLI), Cursor, Gemini CLI, Factory Droid, Trae / Trae CN, Kimi, Mistral Vibe, Google Antigravity; Tencent CodeBuddy (CLI and IDE), Qoder / Qoder CN / Lingma, Zhipu ZCode |
| Plugin (Obituary installs a small plugin that calls `obit hook`) | OpenCode, OpenClaw, Pi, Oh My Pi, Hermes, Cline, Kilo Code, MiniMax Code, DeepSeek Harness |
| Rules file and MCP only | Roo Code, Windsurf, WorkBuddy, QoderWork |

::: tip What each agent supports
Agents differ in what they support (registration after writes, the session-start notice, the delete guard, MCP and the Skill); the repository's [agent coverage table](https://github.com/woodfishhhh/obituary/blob/main/docs/agents.md) lists "supported / limited / unsupported" with reasons.
:::

Where public documentation is not enough to write safely, `obit init` prints the complete manual steps and reports "incomplete, manual steps required". It never pretends to have installed something.

## MCP server and Agent Skill

```powershell
obit init -g --mcp --skill                 # Claude Code; add --agent codex for Codex. Neither is installed by default
```

- **`obit mcp`** is a stdio MCP server exposing Obituary's operations as tools the agent can call: `obit_mark`, `obit_mktemp`, `obit_bury`, `obit_resurrect`, `obit_extend`, `obit_spare`, and the queries `obit_list`, `obit_graveyard`, `obit_proposals`, `obit_stats`.
- **Your decisions stay yours.** Approving or rejecting proposals, trusting `.obitrules`, purging the graveyard and changing configuration have no MCP tools. The agent can only pass a proposal on to you.
- The **Agent Skill** is a `SKILL.md` telling the agent when to give its output a lifespan, how to delete restorably, and which decisions belong to you. Once you edit the Skill, Obituary no longer overwrites or removes it.
- Restart the agent after installing. `obit doctor` checks whether both are installed and point at the current `obit.exe`.
