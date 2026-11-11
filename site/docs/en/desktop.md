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

`obit-gui.exe` has five pages: Graveyard, Upcoming, Decisions, Settings and Logs. "Open Obituary" in the tray menu opens it; it must sit next to `obitd.exe` for the tray to find it.

- Every switch shows exactly what it will change and writes only after you confirm.
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

```powershell
obit init --context-menu --yes   # Explorer context menu (classic menu; on Windows 11 under "Show more options")
```

`--dry-run` lists every entry. Once installed, right-click a file in Explorer to give it a lifespan.

Burial dates shown in Explorer are covered in [Folder policies](./folder-policies#dates-in-explorer).
