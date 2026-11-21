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
