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

No. Automatic registration is off by default: what goes in is what an agent registers with `obit mark` and what you add from the context menu. Turned on, it still only touches untracked items inside a Git work tree; new files no rule covers (such as code you have not committed yet) are never listed, and dependencies, build output and caches are left alone. Anything registered by rule or proposal is not buried if it has been committed by the time it expires. See [What goes in](./auto-register).

## What if Git fails, or my `.gitignore` is very broad?

Nothing is put at risk. Before registering anything, Obituary asks Git whether it is tracked; if Git cannot answer (not installed, timed out, a damaged repository), nothing is registered. Folders outside a Git repository are never registered automatically; only you or an agent can give them a lifespan with `obit mark`. A broad `.gitignore` only makes more things untracked: automatic registration still covers just the built-in rules and the `.obitrules` you trusted.

## Could an agent pass my files off as its own and skip the proposal?

No. Automatic registration is off by default, so nothing goes in without your approval. With the tiered mode on, only files provably created by the agent in its session go in without your approval: the hook watched it write them, the session's start snapshot does not list them, and the file system created them after the session started. Files that existed before the session, or that another program made, are still proposed. Even if an agent were talked into renaming one of your old folders to a name a rule recognises, its creation time stays old, so it only becomes a proposal. Git-tracked files are never registered automatically in any mode; whatever goes in directly is reported to you afterwards, `obit spare` undoes it in one command, and burials stay restorable for seven days.

## Can an agent approve proposals for me?

No. Approving or rejecting proposals, trusting `.obitrules`, purging the graveyard and changing configuration (the proposal mode included) have no MCP tools; the agent can only pass a proposal on to you.

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
