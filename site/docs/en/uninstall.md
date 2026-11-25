# Uninstall

Uninstalling takes two steps: first Obituary removes what it set up, then you remove the programs themselves. The graveyard and the ledger stay by default, so files you buried can still be resurrected afterwards.

```powershell
obit uninstall --dry-run      # list what would be removed; change nothing
obit uninstall                # show the plan, then apply it after you confirm
```

If you installed with the script on Windows, one command is enough:

```powershell
.\install.ps1 -Uninstall      # runs obit uninstall --yes for you, then removes the program files, the PATH entry it added and its Start menu shortcut
```

## What gets removed

`obit uninstall` works on the user level and on the current Git project: the hooks, MCP and Skill of each agent, the scheduled job (Task Scheduler on Windows, a launchd user agent on macOS, a systemd user timer on Linux), the resident process and the tray, and folder policies (the files under a policy stay where they are). On Windows it also removes the login startup entry, the Explorer context menu and the date labels in Explorer.

On macOS and Linux it also removes the installed Finder, Nautilus and Dolphin actions, leaving user scripts untouched. It removes burial tags it added; other tags are kept.

It touches only what Obituary itself wrote. An entry you edited, or one left by another installation, is kept as it is, and the plan says which one and why. Where there is no terminal (a script, an agent's command line) add `--yes`; without it the command stops and tells you it needs a terminal.

Three things it does not do for you. The program files go the way you installed them (see below). If you installed project-level hooks in other projects, run `obit uninstall` in each of those projects before you remove the programs, or the hooks there will fail because `obit` is gone. And on macOS and Linux the tray's startup entry is not on the list: before uninstalling, turn off "Start the tray at sign-in" in the window's Settings, which removes the entry Obituary wrote. On Windows `obit uninstall` handles the tray's startup itself, but the Start menu and desktop shortcuts that `obit init` created are not on its list; `obit init --uninstall` removes them, unless you add `--no-shortcuts`.

## The graveyard, the ledger and your configuration

Three things are kept by default: the files in the graveyard, the ledger and audit log, and `config.toml`. `obit uninstall` tells you how much is still in the graveyard and where it is, and a later reinstall picks all of it up again. If you use Recycle Bin mode, expired items are already in the system Recycle Bin; uninstalling does not touch them, and they are restored from there because `obit resurrect` cannot reach them.

When you really want everything gone, add `--purge-data`:

```powershell
obit uninstall --purge-data
```

It lists every item in the graveyard first and asks once more after you confirm the uninstall plan. After that, and before it removes anything, it checks everything it is about to touch: that the data directory is Obituary's own, that every file in it is one it recognises, and that the graveyard folder and each grave to be purged can be verified. If a check fails, it stops there and says which, and the graveyard, the ledger and the services are all left as they were, so a purge is never half done. Only when everything passes does it delete those files permanently, then the ledger, the audit log and the other files Obituary keeps. In a script, `--yes --purge-data` together skip both questions.

It deletes only files it recognises. Other files, links or a subfolder Obituary does not know make the check stop, and it names the one it found. `config.toml` is never part of the purge, and you do not need to move it, even when it sits in the same folder as the ledger (the default on macOS): after the purge that folder holds only `config.toml`. When the configuration is somewhere else, the whole data directory is deleted.

## Removing the programs

How you remove the programs depends on how you installed them.

| Installed with | Remove with |
| --- | --- |
| The Windows install script | `.\install.ps1 -Uninstall` |
| Scoop | `obit uninstall`, then `scoop uninstall obituary` |
| winget | `obit uninstall`, then `winget uninstall woodfishhhh.Obituary` |
| The macOS window (DMG) | `obit uninstall` and turn off "Start the tray at sign-in", then drag Obituary.app from Applications to the Trash |
| Homebrew | `obit uninstall`, then `brew uninstall --cask obituary` |
| `install.sh` on macOS or Linux | `obit uninstall`, then delete `obit` and `obitd` from `~/.local/bin` |
| The Ubuntu desktop package (`.deb`) | turn off "Start the tray at sign-in" and quit the tray, then `sudo apt remove obituary-desktop` |

The macOS app and the Ubuntu `.deb` each bring their own `obit`, but neither puts it on your PATH. If you have not installed the command line separately, run it by its full path before you uninstall: `/Applications/Obituary.app/Contents/MacOS/obit` in the app, `/usr/libexec/obituary/obit` for the `.deb`. `apt remove` deletes the programs in that folder and leaves your graveyard, your settings and any command line you installed another way.

## Where the data lives

| | Data (ledger, graveyard, logs) | Configuration (`config.toml`) |
| --- | --- | --- |
| Windows | `%LOCALAPPDATA%\obituary` | `%APPDATA%\obituary` |
| macOS | `~/Library/Application Support/obituary` | the same folder |
| Linux | `~/.local/share/obituary` | `~/.config/obituary` |

On Linux, if `XDG_DATA_HOME` or `XDG_CONFIG_HOME` is set, each is the `obituary` folder under it. The environment variables `OBIT_HOME` and `OBIT_CONFIG_DIR` move the data and the configuration elsewhere, and `obit config` prints which ones are in use. On the drive that holds the data directory the graveyard is its `graveyard` folder; on other drives it is a hidden `.obituary` folder at the drive's root, so that burying is always a rename on one drive and never a copy.
