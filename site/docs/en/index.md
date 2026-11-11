# Quick start

Obituary gives files and folders a lifespan. When it runs out they are buried in a graveyard; you can resurrect them at any time during the retention period (7 days by default), and only after that are they really deleted. It is built for AI agents: the debug scripts, logs, screenshots and test output an agent produces carry a lifespan from the moment they are created. Nobody has to remember to clean up, and an agent's `rm -rf` no longer destroys anything for good.

## 1. Install

Windows 10 / 11 (x64 and ARM64).

**Easiest: let your agent do it.** Send this sentence to the agent you use (Claude Code, Codex, Cursor…). It installs Obituary, connects it to itself and stops wherever a decision is yours:

```text
Read https://obituary.woodfish.site/agent-install.md and follow it to install Obituary and connect it to yourself.
```

**By hand:** pick one of the three, then run `obit doctor` to check the installation.

::: code-group

```powershell [Scoop]
scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket
scoop install obituary
```

```powershell [winget]
winget install woodfishhhh.Obituary
```

```powershell [Install script]
# Install script: download it, read it, then run it
# (installs to %LOCALAPPDATA%\Programs\obituary, adds it to your user PATH, creates a Start menu shortcut)
irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1
.\install.ps1 -Version 1.0.0          # -WhatIf previews, -NoPath leaves PATH alone
```

:::

- All three download the same archive from the GitHub release and check its SHA-256 before unpacking; a mismatch stops the install.
- 1.0.0 is not code-signed yet. If you download the archive with a browser, Windows may show "Windows protected your PC"; choose "More info → Run anyway". Scoop, winget and the install script usually do not trigger this.
- The small window `obit-gui.exe` needs the WebView2 runtime (built into Windows 11 and present on most Windows 10 machines) and is x64 only; the ARM64 build has the command line and the tray.
Then check the installation:

```powershell
obit doctor              # read-only check: PATH, obitd, runner, Claude Code / Codex integration, data directory, ledger, config (--json)
```

## 2. Connect your agents

```powershell
obit init -g                 # Claude Code, all projects; also the Skill, context menu and shortcuts
obit init -g --agent codex   # Codex
```

Restart the agent afterwards. Other agents, the MCP server and the Agent Skill: [Connect agents](./agents).

## 3. Common commands

Replace `<id>` with the number from an `obit add` or `obit bury` receipt, or from `obit list` / `obit graveyard`.

```powershell
obit add debug.log 2h        # bury it in 2 hours (alias: mark)
obit list                    # what is left and for how long
obit extend <id> 1d          # one more day
obit bury old-build          # bury right away
obit graveyard               # what is in the graveyard and when it will be purged
obit resurrect <id>          # put it back, never overwriting
obit resurrect --last        # undo the last deletion
```

Everything else: [Commands](./commands).

## Next

| To find out | Read |
|---|---|
| How registering, burying, resurrecting and purging work, and where the graveyard is | [How it works](./how-it-works) |
| Connecting Claude Code, Codex and other agents | [Connect agents](./agents) |
| How agent output gets a lifespan on its own, and what needs your approval | [Automatic lifespans](./auto-register) |
| Turning an agent's `rm -rf` into a burial you can undo | [Delete guard](./delete-guard) |
| What happens when an agent reaches for a buried file | [Grave notices](./grave-notices) |
| Lifespans for folders such as Downloads and Desktop | [Folder policies](./folder-policies) |
| Using Obituary without the command line | [Window, tray and context menu](./desktop) |
| Docker containers started in a session | [Docker containers](./docker) |
| Every command | [Commands](./commands) |
| Removing Obituary | [Uninstall](./uninstall) |
| Anything else | [FAQ](./faq) |
