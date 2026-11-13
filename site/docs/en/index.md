# Quick start

Obituary gives files and folders a lifespan. When it runs out they are buried in a graveyard; you can resurrect them at any time during the retention period (7 days by default), and only after that are they really deleted. It is built for AI agents: the debug scripts, logs, screenshots and test output an agent produces carry a lifespan from the moment they are created. Nobody has to remember to clean up, and an agent's `rm -rf` no longer destroys anything for good.

## 1. Install

Windows 10 / 11, macOS and Linux (x64 and ARM64). macOS and Linux provide the CLI, graveyard, resurrection and automatic burial. Recycle Bin mode, the small window, tray and context menus are provided by the Windows edition.

**Easiest: let your agent do it.** Send this sentence to the agent you use (Claude Code, Codex, Cursor…). It installs Obituary, connects it to itself and stops wherever a decision is yours:

```text
Read https://obituary.woodfish.site/agent-install.md and follow it to install Obituary and connect it to yourself.
```

**By hand:** choose your operating system below, then run `obit doctor` to check the installation.

### macOS / Linux

On macOS, Homebrew installs it in one line:

```bash
brew install --cask woodfishhhh/tap/obituary
```

Or download and read the install script, then run it (macOS and Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/woodfishhhh/obituary/main/install.sh -o install.sh
bash install.sh --version 1.1.1
# Custom directory: --install-dir "$HOME/bin"; preview only: --dry-run
```

The script detects the OS and architecture, verifies SHA-256 from the same download source, and installs `obit` and `obitd` into `~/.local/bin`. Follow its PATH instructions, then run `obit --version`. Use `--mirror cn` for the mainland China mirror; its script is at `https://cn.obituary.woodfish.site/install.sh`.

On first use in macOS, run `obit --version` and `obitd --help`. If Gatekeeper asks for approval, open System Settings → Privacy & Security, choose Open Anyway for each executable whose source you verified, then retry the command.

In WSL, keep projects and agent configuration on the Linux filesystem (for example, under `~/projects`). Windows mounts such as `/mnt/c` may not support the safe renames required for installation; the operation reports an error and preserves the original configuration. You can also use the Windows edition of Obituary directly.

### Windows

::: code-group

```powershell [Scoop]
scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket
scoop install obituary
```

```powershell [winget]
# In review, available once approved
winget install woodfishhhh.Obituary
```

```powershell [Install script]
# Install script: download it, read it, then run it
# (installs to %LOCALAPPDATA%\Programs\obituary, adds it to your user PATH, creates a Start menu shortcut)
irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1
.\install.ps1 -Version 1.1.1          # -WhatIf previews, -NoPath leaves PATH alone
```

:::

- All three download the same archive from the GitHub release and check its SHA-256 before unpacking; a mismatch stops the install.
- Code signing: Free code signing provided by [SignPath.io](https://about.signpath.io/), certificate by [SignPath Foundation](https://signpath.org/). See the [code signing policy](code-signing) and the [privacy policy](privacy).
- Obituary is not code-signed yet. If you download the archive with a browser, Windows may show "Windows protected your PC"; choose "More info → Run anyway". Scoop, winget and the install script usually do not trigger this.
- The small window `obit-gui.exe` needs the WebView2 runtime (built into Windows 11 and present on most Windows 10 machines) and is x64 only; the ARM64 build has the command line and the tray.
Then check the installation:

```powershell
obit doctor              # read-only check: PATH, obitd, runner, Claude Code / Codex integration, data directory, ledger, config (--json)
```

## 2. Connect your agents

```powershell
obit init -g                 # Claude Code, all projects; also the Skill (and desktop integration on Windows)
obit init -g --agent codex   # Codex
```

Restart the agent afterwards. Other agents, the MCP server and the Agent Skill: [Connect agents](./agents).

On macOS / Linux, Claude Code uses `~/.claude`, Codex uses `~/.codex`, and shared Skills use `~/.agents/skills`. `CLAUDE_CONFIG_DIR` and `CODEX_HOME` overrides are also respected.

Enable the user scheduler to check expiries every 5 minutes:

```bash
obit daemon enable --dry-run
obit daemon enable
obit doctor
```

macOS uses the signed-in user's launchd; Linux uses `systemd --user`. On Linux without systemd, schedule `obitd --once` every 5 minutes in the user's crontab, supplying absolute paths for `obitd`, the `--home` data directory and the `--config-dir` configuration directory. Agent hooks also start a background check when work is due.

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
