# Window, tray and context menu

Resurrect files, approve proposals and change settings without the command line.

## The tray

`obit init` starts a tray icon at login. Its menu has:

- recent burials (click one to resurrect it);
- items due within 24 hours;
- pause / resume automatic cleanup;
- Open Obituary (the small window);
- Quit (the scheduled task keeps working).

## The small window

`obit-gui.exe` has five pages: Upcoming, Graveyard, Decisions, Settings and Logs. "Obituary" in Start, the desktop shortcut and "Open Obituary" in the tray menu all open it.

- Trusting a `.obitrules` file trusts exactly the version shown in the window.
- The window, tray and context menu follow `[ui] language` in `config.toml` (`auto`, `zh-CN` or `en`; `auto` follows Windows). You can switch it in Settings.
- It needs the WebView2 runtime (built into Windows 11 and present on most Windows 10 machines) and is x64 only.

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

`obit init` installs the Explorer context menu by default: right-click a file or folder, choose "Obituary" and a lifespan (on Windows 11 under "Show more options"). `obit init --dry-run` lists every entry and its command.

## Icons and menus

`obit init` sets all of these up by default, and each has a switch under Settings > Icons and menus:

| Switch | What it covers |
|---|---|
| Tray icon | Obituary in the notification area; hidden, cleanup keeps running |
| Taskbar icon | Whether the window takes a taskbar slot while open |
| Start menu | "Obituary" in Start |
| Desktop shortcut | "Obituary" on the desktop |
| Context menu | The "Obituary" menu in Explorer |

To skip them at install time: `obit init --no-context-menu` leaves out the context menu, `--no-shortcuts` the Start menu and desktop shortcuts.

Burial dates shown in Explorer are covered in [Folder policies](./folder-policies#dates-in-explorer).
