# Window, tray and context menu

Resurrect files, approve proposals and change settings without the command line.

## macOS app and menu bar icon

Download the universal macOS DMG and checksums file from the same release. Verify the SHA-256, open the DMG, and drag Obituary.app to Applications or the user's Applications folder; eject the DMG before enabling login startup.

The app contains the Wails small window, the menu bar icon, and matching obit and obitd helpers. The five-minute launchd user agent remains a separate fallback. In Settings, enable Start the tray at sign-in to install Obituary's own user LaunchAgent; disable it there to remove only that entry.

Removing `Obituary.app` leaves your data in `~/Library/Application Support/obituary/` by default: `store.json` is the ledger, `config.toml` holds your settings, and `graveyard/` holds files buried on that volume.

### First opening an unsigned app

The release is not Developer ID signed or notarized. After checking the SHA-256 from a trusted release, try opening the app once. If macOS says it cannot verify the developer, open System Settings, Privacy and Security, choose Open Anyway for Obituary, confirm the prompt, and open it again. Do not disable Gatekeeper globally or clear quarantine recursively. Stop if macOS reports a damaged app, malware, or an organisation policy that forbids opening it.

### Homebrew Cask proposal

The cask should install the same universal DMG, copy Obituary.app to Applications, and expose obit and obitd from the app bundle. Keep the existing CLI archive cask separate until the desktop DMG has a stable release URL and a reviewed uninstall receipt.
## Ubuntu 24.04 (amd64)

Download the `obituary_<version>_linux_amd64.deb` desktop package and `checksums.txt` from the same release. From the download directory, install the verified package (replace `VERSION` with the version you downloaded):

```bash
file=obituary_VERSION_linux_amd64.deb
grep -F "  $file" checksums.txt | sha256sum --check --strict - && sudo apt install "./$file"
```

Open **Obituary** from Applications. The window uses GTK 3 and WebKitGTK 4.1; apt installs the required libraries. To start the tray at your next desktop login, enable **Settings → Start the tray at sign-in**. Turn it off there to remove Obituary's XDG autostart entry.

The tray needs a StatusNotifierItem host. On GNOME without one, open the same window from Applications; no shell extension is required. To remove the package, turn off tray autostart, quit the tray and run `sudo apt remove obituary-desktop`. Your graveyard, settings, existing CLI and user timer stay in place.

## The Windows tray

`obit init` starts a tray icon at login. Its menu has:

- recent burials (click one to resurrect it);
- items due within 24 hours;
- pause / resume automatic cleanup;
- Open Obituary (the small window; a notification explains when `obit-gui.exe` is not next to `obitd.exe`);
- Quit (the scheduled task keeps working).

## The Windows window

`obit-gui.exe` has five pages: Upcoming, Graveyard, Decisions, Settings and Logs. "Obituary" in Start, the desktop shortcut and "Open Obituary" in the tray menu all open it.

- The home page's top bar says "Auto registration: off" or "on"; click it to open Settings. Both the top of Decisions and Settings switch it: Off (the default: only what agents and you register goes in), Tiered, Fully automatic and Ask about everything; see [What goes in](./auto-register). What went in without asking is listed further down under Handled automatically, each with one-click Undo (the countdown stops, it stays where it is) and Keep longer.
- Items that were due but could not be buried (another program holding a file open, say) are listed under "Burials that failed" on Decisions, with the reason and when it is tried next. Obituary tries again after a while, waiting longer each time, and after repeated failures it stops and waits for you: Retry buries it again now, Stop tracking leaves it where it is.
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

To skip them at install time: `obit init --no-context-menu` leaves out the context menu, `--no-shortcuts` the Start menu and desktop shortcuts.

Burial dates shown in Explorer are covered in [Folder policies](./folder-policies#dates-in-explorer).
