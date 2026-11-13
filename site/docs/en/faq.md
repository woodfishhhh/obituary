# FAQ

## Why not just delete?

Something you are not sure about cannot come back once deleted. Obituary buries it in the graveyard instead, kept 7 days by default, and `obit resurrect` puts it back at any time until then; only after retention is it really deleted. If an agent reaches for it within those seven days, Obituary tells it the grave number and the resurrect command on the spot (see [Grave notices](./grave-notices)).

## macOS and Linux?

Yes. Both provide x64 / ARM64 command-line and background programs with a graveyard, resurrection and automatic burial. macOS uses launchd and Linux uses a systemd user timer to check expiries; agent hooks also start checks when needed. See [Quick start](./index). Recycle Bin mode, the small window, tray and context menus are provided by the Windows edition.

## Windows says "Windows protected your PC"?

Obituary is not code-signed yet. If you download the archive with a browser, choose "More info → Run anyway". Scoop, winget and the install script usually do not trigger this, and all three check the archive's SHA-256 first.

## Is there a small window on ARM64?

No. `obit-gui.exe` is x64 only; the ARM64 build has the command line and the tray.

## Could it register code I have not committed yet?

Automatic registration only touches untracked items inside a Git work tree, and leftovers no rule covers are only proposed, not registered, because new untracked files may be code you have not committed yet. Anything registered by rule or proposal is not buried if it has been committed by the time it expires. See [Automatic lifespans](./auto-register).

## What if Git fails, or my `.gitignore` is very broad?

Nothing is put at risk. Before registering anything, Obituary asks Git whether it is tracked; if Git cannot answer (not installed, timed out, a damaged repository), nothing is registered. Folders outside a Git repository are never registered automatically; only you or an agent can give them a lifespan with `obit mark`. A broad `.gitignore` only makes more things untracked: automatic registration still covers just the built-in rules and the `.obitrules` you trusted, and any other leftover is only proposed until you approve it.

## Can an agent approve proposals for me?

No. Approving or rejecting proposals, trusting `.obitrules`, purging the graveyard and changing configuration have no MCP tools; the agent can only pass a proposal on to you.

## Does `obit resurrect` work in recycle mode?

No. With `mode = "recycle"` in `config.toml`, burials go to the system Recycle Bin and you restore them from there.

## How do I turn it off for a while?

- Hooks: `OBIT_DISABLED=1`
- Delete guard: `OBIT_GUARD_RM=0`
- Automatic cleanup: "Pause" in the tray menu

## Where is the data?

`%LOCALAPPDATA%\obituary`: the ledger, the audit log, the daemon log and this drive's graveyard. Configuration is `%APPDATA%\obituary\config.toml`. See [How it works](./how-it-works#where-the-data-lives).

## Does uninstalling delete the graveyard?

No. `obit uninstall` keeps the graveyard and ledger by default; add `--purge-data` if you really want them gone. See [Uninstall](./uninstall).
