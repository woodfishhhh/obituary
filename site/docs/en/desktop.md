# Window, tray and context menu

Resurrect files, approve proposals and change settings without the command line.

## What each of the three platforms has

The window, the tray and desktop notifications exist on Windows, macOS and Ubuntu 24.04; the context menu, the date labels in Explorer and Recycle Bin mode exist only on Windows. The desktop editions for macOS and Ubuntu are previews for now. Other Linux distributions have no desktop edition and use the command line.

| | Windows | macOS | Ubuntu 24.04 |
| --- | --- | --- | --- |
| Window | `obit-gui.exe`, x64 only | `Obituary.app`, universal for Intel and Apple silicon | a `.deb` package, amd64 only |
| Tray | notification area | menu bar | tray, needs a desktop that provides StatusNotifierItem |
| Tray at login | on by default after `obit init` | switch in Settings | switch in Settings |
| Expiry check | resident tray process | LaunchAgent | systemd |
| Desktop notifications | yes | yes | yes |
| Context menu | in Explorer | no | no |
| Date labels in Explorer | yes | no | no |
| Recycle Bin mode | yes | no, graveyard only | no, graveyard only |
| Installed with | the install script, Scoop, winget or the archive | the DMG | the `.deb` |
| First opening | an archive downloaded by hand may meet a SmartScreen warning | needs manual approval, see [below](#first-opening-an-unsigned-app) | opens directly |
| Requirements | the WebView2 runtime | 1.6.0 needs macOS 26, fixed in 1.6.1 | GTK 3 and WebKitGTK 4.1, installed by apt |

Windows starts the tray with a scheduled task at sign-in, macOS uses a user LaunchAgent, and Linux uses a user systemd service in a graphical session. After a crash the system waits 60 seconds before restarting; after the third crash in 15 minutes, it stops trying.

With desktop supervision enabled, the tray handles expiries as they occur and an independent job checks once an hour. Editions without a tray check every five minutes.

On Windows, an old scheduled task is migrated only if it matches the one Obituary created for this installation. Its original definition is saved for rollback; modified or disabled tasks stay as they are.

## macOS app and menu bar icon

Download the universal DMG (`obituary_<version>_darwin_universal.dmg`) and `checksums.txt` from the release page, check the SHA-256, open the DMG, drag Obituary.app into Applications, eject the DMG, and open the app from Applications. Drag it out first because the login startup entry records where the app is, and an app run straight from the DMG is no longer there once the DMG is ejected.

The app contains the window, the menu bar icon, and matching `obit` and `obitd` programs. Turning on "Start the tray at sign-in" in Settings installs Obituary's own user LaunchAgent so that the menu bar icon appears when you sign in; turning it off removes only that entry.

Install the independent expiry job with `obit daemon enable`. Closing the window or the menu bar icon does not disable it.

Removing `Obituary.app` leaves your data in `~/Library/Application Support/obituary/` by default: `store.json` is the ledger, `config.toml` holds your settings, and `graveyard/` holds files buried on that volume.

### First opening an unsigned app

The release has no Developer ID signature and is not notarized, so macOS stops it the first time. After making sure the download came from a trusted release page and checking the SHA-256, open it normally once. If the system says it cannot verify the developer, open System Settings, then Privacy & Security, find Obituary at the bottom of the page, choose Open Anyway, confirm once more, and open the app again. Do not turn off Gatekeeper globally for this, and do not clear the quarantine attribute recursively. If the system reports that the app is damaged or contains malware, or a company policy forbids opening it, stop there.

### 1.6.0 needs macOS 26

The macOS window in 1.6.0 needs macOS 26. On macOS 15 and earlier the system says the app is not compatible with this version, and it does not open. This is fixed in 1.6.1.

## Ubuntu 24.04 (amd64)

Download the desktop package `obituary_<version>_linux_amd64.deb` and `checksums.txt` from the release page. From the download directory, run the commands below, replacing `VERSION` with the version you downloaded; the package is installed only if the check succeeds:

```bash
file=obituary_VERSION_linux_amd64.deb
grep -F "  $file" checksums.txt | sha256sum --check --strict - && sudo apt install "./$file"
```

Afterwards open Obituary from the application list. The window uses GTK 3 and WebKitGTK 4.1, and apt installs them with the package. The programs live in `/usr/libexec/obituary/`, which is not on your PATH; to use `obit` in a terminal, install the command line separately with the install script.

The tray needs a desktop that provides StatusNotifierItem, and GNOME does not by default. There you open the same window from the application list, with no extension, but there is no tray icon. To have the tray appear at your next login, turn on "Start the tray at sign-in" in Settings; turning it off deletes the startup entry Obituary wrote. For uninstalling, see [Uninstall](./uninstall).

## The Windows tray

`obit init` starts a tray icon at login. Its menu has:

- recent burials (click one to resurrect it);
- items due within 24 hours;
- pause / resume automatic cleanup;
- Open Obituary (the small window; a notification explains when `obit-gui.exe` is not next to `obitd.exe`);
- Quit (the scheduled task keeps working).

## The Windows window

`obit-gui.exe` has five pages: Expiring, Graveyard, For you, Settings and Logs. "Obituary" in Start, the desktop shortcut and "Open Obituary" in the tray menu all open it.

- The home page's top bar says "Auto registration: off" or "on"; click it to open Settings. Both the top of For you and Settings switch it: Off (the default: only what agents and you register goes in), Tiered, Fully automatic and Ask about everything; see [What goes in](./auto-register). What went in without asking is listed further down under Handled automatically, each with one-click Undo (the countdown stops, it stays where it is) and Keep longer.
- Items that were due but could not be buried (another program holding a file open, say) are listed under "Burials that failed" on For you, with the reason and when it is tried next. Obituary tries again after a while, waiting longer each time, and after repeated failures it stops and waits for you: Retry buries it again now, Stop tracking leaves it where it is.
- While Docker is not running, a container proposal's Approve is unavailable and the card says why; approve it once Docker is up.
- Trusting a `.obitrules` file trusts exactly the version shown in the window.
- The window, tray and context menu follow `[ui] language` in `config.toml` (`auto`, `zh-CN` or `en`; `auto` follows Windows). You can switch it in Settings.
- It needs the WebView2 runtime (built into Windows 11 and present on most Windows 10 machines) and is x64 only.

### Updates

Install a newer stable release from Settings > About and updates. Obituary closes the window and the running tray, updates through the original Scoop, WinGet or script channel, then attempts to restore the tray and reopen the window. Failures and log paths are shown on the same page.

A copy you unpacked yourself updates in place too, and its button reads "Download and restart to update". Obituary downloads the release from GitHub (from the mainland China mirror when GitHub does not answer within ten seconds), checks its SHA-256, and only then closes the window and replaces Obituary's own programs in that folder (`obit`, `obitd` and `obit-gui`). Every other file in the folder stays as it is, and PATH and the Start menu are not touched. The previous programs wait for a day in a hidden `.obituary-update-…` folder in the same place, then go to the graveyard, where they can still be resurrected; if any step fails, including a new `obit` that does not start, the previous version is put back. This needs a stable release (not a development build), every program of the release in the folder as a plain file, and a folder the current user can write to without administrator rights. When one of these does not hold, the page says which, and you can still download a replacement from the release page. From a terminal, `obit update --apply` does the same. "What changed" shows the notes in the window's language, in English when there is no translation.

### Eight looks

Switch in Settings → Appearance. It changes only how the window looks on this computer, at once.

| Look | What it is |
|---|---|
| Calendar | A column per day; each file sits on the day it is buried or cremated. Drag it to a later day to extend it. |
| One line | One line of input, all from the keyboard: find a file, press Enter to resurrect or extend. |
| Tunnel | A 3D tunnel of time: the closer, the sooner it expires. Scroll to move through time. |
| Vigil | Dark, with a lamp for each file; the dimmer the lamp, the sooner it expires. |
| Notices | A newspaper's obituary page: serif columns, a stamp when you resurrect. |
| Horizon | Tracked files above the ground, the graveyard below. |
| Mica | Native Windows 11 feel with a sidebar. |
| Night | Quiet dark theme with a top bar. |

## Context menu

`obit init` installs the Explorer context menu by default (for the current user only). Right-click a file or folder, choose "Add to Obituary", then:

- **1 day / 7 days / 30 days**: buried when the time is up; use it as usual until then;
- **Bury now (recoverable)**: into the graveyard right away, restorable during the retention period (7 days by default).

Select many files and add them at once. A notification says how many went in, when they will be buried, and how many failed. Windows 11's new menu does not show it: click "Show more options" (or hold Shift while right-clicking). `obit init --dry-run` lists every entry and its command, and `obit doctor` checks that it is installed correctly.

If all Explorer menu labels are in another supported language after an upgrade, the menu still works. `obit doctor` reports this as information (`installation: label-language-mismatch` in JSON). Run `obit init --context-menu --yes` to switch the labels to the current interface language.

macOS and Linux have no context menu yet; use the commands: `obit mark <path> 7d` to add, `obit bury <path>` to bury now.

## Icons and menus

`obit init` sets all of these up by default, and each has a switch under Settings > Icons and menus:

| Switch | What it covers |
|---|---|
| Tray icon | Obituary in the notification area; hidden, cleanup keeps running |
| Taskbar icon | Whether the window takes a taskbar slot while open |
| Start menu | "Obituary" in Start |
| Desktop shortcut | "Obituary" on the desktop |
| Context menu | The "Add to Obituary" menu in Explorer |

To skip them at install time, add `obit init --no-context-menu` to leave out the context menu or `--no-shortcuts` to leave out the Start menu and desktop shortcuts. The same options apply when you uninstall: `obit init --uninstall --no-shortcuts` leaves the existing shortcuts in place and removes the rest as usual, and `--no-context-menu` does the same for the context menu.

Burial dates shown in Explorer are covered in [Folder policies](./folder-policies#dates-in-explorer).
