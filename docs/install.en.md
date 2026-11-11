# Installing Obituary

[中文](install.md) · [Home](../README.en.md)

The easiest way is to let your agent install it: send it the sentence from the README. This page is for installing by hand.

Obituary runs on Windows 10 and 11 (x64 and ARM64). The small window `obit-gui.exe` uses the WebView2 runtime, which comes with Windows 11 and is present on most Windows 10 machines.

## 1. Install the programs

Pick one of the three:

```powershell
# Scoop
scoop bucket add obituary https://github.com/woodfishhhh/scoop-bucket
scoop install obituary

# winget (in review, available once approved)
winget install woodfishhhh.Obituary

# Install script: download it, read it, then run it
# (installs to %LOCALAPPDATA%\Programs\obituary, adds it to your user PATH, creates a Start menu shortcut)
irm https://raw.githubusercontent.com/woodfishhhh/obituary/main/packaging/install.ps1 -OutFile install.ps1
.\install.ps1 -Version 1.0.0          # -WhatIf previews, -NoPath leaves PATH alone
```

In mainland China, the install script downloads from the mainland mirror:

```powershell
irm https://cn.obituary.woodfish.site/install.ps1 -OutFile install.ps1
.\install.ps1 -Version 1.0.0 -Mirror cn
```

- All three install the same archive and check its SHA-256 before unpacking it.
- If you download the archive with a browser, Windows may show "Windows protected your PC"; choose "More info → Run anyway". Scoop, winget and the install script go straight through.

Open a new terminal afterwards and check that `obit --version` runs.

## 2. Connect it to your agent

```powershell
obit init --show                          # the agents you can connect, with their names
obit init -g --agent claude --dry-run     # see what would change
obit init -g --agent claude,codex --yes   # do it; several agents at once
obit init -g --agent claude --guard-rm --yes   # also turn on the delete guard: rm -rf becomes a restorable obit rm -rf
```

`-g` applies to all projects; without it only the current project is connected. Restart the agent afterwards. In Codex, run `/hooks` once to trust Obituary's hooks.

## 3. Check the installation

```powershell
obit doctor
```

It checks PATH, the background process, each agent connection, the data folder and the configuration, and says what to do about anything it finds.

## Updates

Once a day at most, Obituary checks whether a new stable release is out. It reads public release information only and sends no usage data; when GitHub is unreachable it checks the mainland China mirror instead. When there is a new release, the tray menu and the small window tell you, and Settings > About and updates shows the command for the way you installed it:

- Scoop: `scoop update obituary`
- winget: `winget upgrade woodfishhhh.Obituary`
- Install script: run it again with the new version number

Obituary checks and tells you; you run the update. In a terminal, `obit update` shows your version and the update command, and `obit update --check` checks right away.

To keep it from going online on its own, turn off "Check for updates automatically" in Settings, or put this in `config.toml`:

```toml
[update]
check = false
```

Checking by hand still works.

## Uninstall

```powershell
obit uninstall --dry-run      # see what would be removed
obit uninstall                # remove everything Obituary installed, after you confirm
.\install.ps1 -Uninstall      # installed with the script: also remove the program files, PATH entry and Start menu shortcut
```

`obit uninstall` removes only what Obituary added:
- each agent's hooks, MCP server and Skill, for your user and in the current Git project;
- the login start, the scheduled task and the resident process;
- the context menu, the date labels in Explorer and folder policies.

The graveyard and the ledger stay by default, and it tells you how much is still in the graveyard and where. If you really want them gone, add `--purge-data`; it lists what is in the graveyard and asks once more.

If you installed project-level hooks in other projects, run `obit uninstall` in each of them before removing the programs. With Scoop or winget, run `obit uninstall` first, then their own uninstall command.
