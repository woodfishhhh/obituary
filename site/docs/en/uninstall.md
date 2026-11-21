# Uninstall

```powershell
obit uninstall --dry-run      # see what would be removed
obit uninstall                # remove everything Obituary set up, after you confirm
.\install.ps1 -Uninstall      # if you used the install script: also remove the program files, PATH entry and Start menu shortcut
```

`obit uninstall` removes the user-level and current Git project's agent hooks, MCP and Skill, login start, the scheduled task, the running daemon, the context menu, Explorer date labels and folder policies, and only what Obituary itself added.

## The graveyard and ledger are kept

The graveyard and ledger are kept by default, and you are told how much is in the graveyard and where. If you really want them gone, add `--purge-data`, which lists the graveyard first and asks once more.

## Hooks in other projects

If you installed project-level hooks in other projects too, run `obit uninstall` in each of them before removing the program; otherwise their hooks break when `obit` is gone.

## Scoop and winget

Run `obit uninstall` first, then their own uninstall command.
## Uninstall scope and data locations

`obit uninstall --dry-run` lists removable items for the current user and Git project. The real command removes integrations, hooks, MCP, Skills, tray startup and schedulers, but does not remove project integrations from other projects; run uninstall in each project where it was installed.

| Location or component | Windows | macOS | Linux |
| --- | --- | --- | --- |
| Program | Install directory: `obit`, `obitd`, optional `obit-gui` | `/Applications/Obituary.app` or the CLI directory | CLI directory; desktop package is `.deb` |
| Hooks, MCP, Skills | Agent config in the user or project scope | Agent config scope | Agent config scope |
| Background scheduling | Scheduled task, optional resident process and tray | User LaunchAgent and optional menu-bar tray | systemd user timer; desktop tray may have XDG autostart |
| Project configuration | Obituary-owned parts of `.claude/`, `.codex/` and other agent files | Project configuration | Project configuration |

Default data is `%LOCALAPPDATA%\obituary` with `%APPDATA%\obituary\config.toml` on Windows, `~/Library/Application Support/obituary` on macOS, and `$XDG_DATA_HOME/obituary` on Linux (or `~/.local/share/obituary`). Configuration uses the platform config directory. `OBIT_HOME`, `OBIT_CONFIG_DIR` and `OBIT_USER_HOME` redirect data, configuration and the user home; a same-volume `.obituary` directory may also contain a graveyard.

The graveyard, ledger, audit log and configuration stay by default. `--purge-data` removes only owned, verified graveyard data and keeps unknown files; it is not permission to delete a home directory or another project. In graveyard mode use `resurrect`; in recycle mode restore from the system bin because `resurrect` is unavailable.
