# Troubleshooting

When something goes wrong, start with `obit doctor`. It prints a short summary: the first line says whether everything is fine or how many problems it found, and below it each area (installation, background runner, agents, right-click menu, data, hooks and so on) gets one line marked `PASS`, `WARN` or `FAIL`. Under an area that needs attention it gives the reason and a fix you can copy as it is. Every run also saves the full report as a self-contained HTML file in the `reports` folder of the data directory (it stays on your computer and is buried after 1 day); the last lines say where it is. `obit doctor --open` opens it in your browser, and `obit doctor --verbose` prints the full text in the terminal: one line per check beginning with `PASS`, `INFO`, `WARN` or `FAIL`, followed by the reason and what to do, plus what each agent can do. With any `FAIL` the command exits with a non-zero code. Add `--json` to pass the result to someone else (its format has not changed). The lines quoted further down this page, such as `path` or `obitd`, are the individual checks that `--verbose` and the report show.

Each entry below follows the same order: what you see, why, what to do, and how to tell it is fixed.

If all Explorer menu labels are in another supported language after an upgrade, the menu still works. `obit doctor` reports this as information (`installation: label-language-mismatch` in JSON). Run `obit init --context-menu --yes` to switch the labels to the current interface language.

## The shell cannot find `obit`

The terminal says `command not found`, or PowerShell does not recognise `obit`. The program is on disk, but its folder is not on your PATH. The Windows install script adds `%LOCALAPPDATA%\Programs\obituary` to your user PATH, but a terminal that was already open does not see the new PATH. `install.sh` puts the programs in `~/.local/bin` and never edits PATH, and some systems do not have that folder on PATH to begin with. The Ubuntu `.deb` puts them in `/usr/libexec/obituary/`, which is not on PATH either.

On Windows, close the terminal and open a new one. On macOS and Linux, add `~/.local/bin` to your shell configuration, for zsh by putting `export PATH="$HOME/.local/bin:$PATH"` in `~/.zshrc`, then open a new terminal. If you installed only the `.deb`, use the full path `/usr/libexec/obituary/obit`, or install the command line separately with the install script.

`obit --version` prints a version, and the `path` line in `obit doctor` is no longer `WARN`: that is how you know.

## `obit doctor` cannot find `obitd`

A `FAIL obitd` line in `obit doctor` means the background program `obitd` is not next to `obit`. Obituary looks for it in the same folder, so copying `obit` alone is not enough. Put `obit` and `obitd` together, or run the installer again for the way you installed. When `obit doctor` shows `PASS` on that line, it is fixed.

## A downloaded file fails its checksum

When you use the install script it does this check itself and stops, installing nothing, if the hash is wrong. You only have to check by hand when you downloaded an archive, DMG or `.deb` yourself. A mismatch usually means the download is incomplete, the file was changed on the way, or `checksums.txt` came from a different release, since the hashes are per release.

Download the package and `checksums.txt` again from the same release page (from the mirror if you are in mainland China) and check once more; if it still does not match, do not run it. Replace `VERSION` and the file name with the ones you downloaded. On Linux:

```bash
file=obituary_VERSION_linux_amd64.deb
grep -F "  $file" checksums.txt | sha256sum --check --strict -
```

On macOS, replace `sha256sum --check --strict -` on the last line with `shasum -a 256 -c -`. In Windows PowerShell:

```powershell
$file = "obituary_VERSION_windows_amd64.zip"
(Get-FileHash $file -Algorithm SHA256).Hash -eq ((Select-String -Path checksums.txt -SimpleMatch $file).Line -split '\s+')[0]
```

Linux and macOS print the file name followed by `OK`, and PowerShell prints `True`, when the hashes match.

## Windows says "Windows protected your PC"

Windows SmartScreen may stop an archive you downloaded by hand in the browser, because Obituary's programs are not code-signed yet. Scoop, winget and the install script rarely run into it. First check the SHA-256 as described above; once it matches, choose "More info" and then "Run anyway". If you cannot check it, or Windows reports malware rather than only an unknown publisher, do not run it.

## The Windows window does not open

Clicking Obituary in the Start menu or the tray does nothing, or you are told a runtime is missing. The window is drawn by the Microsoft Edge WebView2 runtime. Windows 11 includes it and most Windows 10 machines already have it, but where it is missing the window cannot start. Download and install the WebView2 runtime from Microsoft (the Evergreen version) and click Obituary again; when the window opens, it is fixed.

The window exists only for x64 so far. The ARM64 Windows archive has no `obit-gui.exe`, so there you use the command line. If "Open Obituary" in the tray menu shows a notification saying it cannot find the window program, `obit-gui.exe` is not next to `obitd.exe`, which is how the ARM64 archive is laid out. On x64, put the two in the same folder.

## macOS: the app says it is not compatible with this version

The macOS window in 1.6.0 (`Obituary.app` in the DMG) needs macOS 26. On macOS 15 and earlier, opening it brings up a message that the app is not compatible with this version of macOS, and it does not start. That is a packaging mistake in 1.6.0, fixed in 1.6.1. Until then the window needs macOS 26. The problem is only in the window inside the DMG. Once you have 1.6.1 or later, the app opens, and that is how you know.

## macOS: "cannot verify the developer"

The app in the DMG has no Developer ID signature and is not notarized, so Gatekeeper stops it the first time. After checking the SHA-256, open it normally once. When macOS says it cannot verify the developer, go to System Settings, then Privacy & Security, find Obituary at the bottom of the page, choose Open Anyway and confirm once more. The steps are in [First opening an unsigned app](./desktop#first-opening-an-unsigned-app). Do not turn Gatekeeper off globally for this. If macOS says the app is damaged or contains malware, that is a different matter, and you should not allow it.

## The tray does not start at login (macOS, Ubuntu)

After you sign in there is no Obituary icon in the menu bar or the tray. Starting at login relies on a startup entry: on macOS the user LaunchAgent `site.woodfish.obituary.tray.supervised`, on Ubuntu `obituary-tray.service`. If the switch in the window's Settings is off, or the app was moved, or the switch was turned on while the app ran straight from the DMG, the path recorded in the entry is stale and nothing is found at login.

Open the window and, in Settings, turn "Start the tray at sign-in" off and on again, so that Obituary rewrites the entry. On macOS, `launchctl list | grep obituary` should now show `site.woodfish.obituary.tray.supervised`; on Ubuntu, `systemctl --user status obituary-tray.service` shows the state of the service.

## Ubuntu: the window does not open, or there is no tray icon

Clicking Obituary in the application list does nothing, which usually means GTK 3 or WebKitGTK 4.1 is missing. `apt install ./….deb` installs them for you; they are missing only if they were removed later or the installation broke halfway. On Ubuntu 24.04 the GTK 3 package is called `libgtk-3-0t64`; the name without `t64`, `libgtk-3-0`, does not exist in this release, so a command copied from an older guide fails:

```bash
sudo apt update
sudo apt install libgtk-3-0t64 libwebkit2gtk-4.1-0 xdg-utils
```

Open Obituary from the application list again; when the window appears, it is fixed. If the window opens but there is no tray icon, that is because the tray needs the desktop to provide a StatusNotifierItem host, and GNOME does not by default. You can open the same window from the application list without any extension; for the icon, add an extension that supports it to GNOME, such as AppIndicator.

## Expired items are not being buried

Check `obit daemon status` for the expiry job. Run `obit reap` to handle expired items now.

With desktop supervision enabled, the tray handles expiries as they occur and an independent job checks once an hour. Without tray supervision for this data directory, the job runs every five minutes.

`obit doctor` shows who supervises the tray, recent failures and the last exit reason. It distinguishes waiting to restart, a pause or deliberate stop, and a missing or failed supervisor.

Run `obit daemon enable` to install the job (on Windows this also installs the tray at login), and `obit sweep` if you want the cleanup now. On a Linux machine without a systemd user service, as in some containers and minimal systems, `obit daemon enable` cannot install it; use cron to run `obitd --once --home <data directory> --config-dir <config directory>` every 5 minutes instead, and `obit config` tells you both directories. When the `runner` line of `obit daemon status` no longer says `off`, it is fixed; on Linux you can also see the timer with `systemctl --user list-timers | grep obituary`.

## An item expired but was not buried

The item shows up on the window's "For you" page under "Burials that failed", with a reason next to it. Often a file is open in an editor, a sync client or another program, and it is still where it was. Burying means moving the file, and that fails when it is in use or you lack permission. Obituary does not force it: the original stays untouched, it tries again after a while, waiting longer each time, and after repeated failures it stops and waits for you.

Close whatever is holding the file and click "Retry", or run `obit sweep`. If you no longer want it dealt with, click "Stop tracking": the file stays where it is and the countdown ends. When the item disappears from "Burials that failed" and shows up in `obit graveyard`, it was buried.

## A resurrected file has a different name

`obit resurrect` reported no error, but the file came back as `report (resurrected).txt`, because something with that name was already in the original place. Resurrection never overwrites, so when the name is taken it puts the file back in the same folder under another name, and both copies are intact. Compare them, keep the one you want and rename it.

## The configuration file has a mistake

`obit config` says `config file has errors`, followed by a file path, a line number and a key name. `config.toml` contains a key Obituary does not know, or a value that is not allowed. Obituary does not quietly ignore these, because then `retention = 30` would leave the 7-day retention in force while you believed it had changed. Correct the spelling, quotes or duration on the line it names and run `obit config` again; when it stops complaining and the `config` line of `obit doctor` is `PASS`, it is fixed. How each setting is written is in the [configuration reference](./config).

## A hook has no effect

After `obit init`, an agent was not stopped from deleting a file, or nothing seemed to happen. There are three common causes: the agent was not restarted and is still on its old configuration; the integration for this agent was installed in a different scope (global or project); or `OBIT_DISABLED=1` (turns every hook off) or `OBIT_GUARD_RM=0` (turns the delete guard off) is set in the agent's environment.

First see what is installed:

```bash
obit init -g --show        # what is installed globally
obit init --show           # what is installed in the current project
obit doctor --verbose      # how each agent is connected
```

If it is not installed, install it with `obit init -g --agent <name>`, adding `--dry-run` if you want to see the plan first. Restart the agent afterwards; in Codex, also run `/hooks` and trust Obituary's hooks. To see what the hook actually receives, set `OBIT_HOOK_AUDIT=1` in the terminal you start the agent from, use the agent for a while, then run `obit hook audit`. Denied delete commands need no such switch, since they are always recorded. The hook's own errors go to `hook.log` in the data directory, and the `hook-log` line of `obit doctor` counts the errors of the last 24 hours.

When the agent's line in `obit doctor` (`claude`, for example) is `PASS` and says `integration is current`, it is installed; if you use the delete guard, a line such as `claude-guard-rm` should say `registered: yes`. Run `obit doctor` in the same terminal you start the agent from: if an environment variable has switched the hooks off, that line becomes `WARN` and names the variable.

## A Docker container proposal cannot be approved

"Approve" on a container proposal in the window is greyed out, and the card says "Docker is not running: start Docker to decide on this proposal." Registering a container means asking Docker for its full ID and creation time, which cannot be done while Docker is off. Start Docker Desktop or Docker Engine and approve again; when "Approve" can be clicked and `obit docker list` shows the container, it worked.

When a container expires, Obituary only stops it and keeps its writable layer, and resurrecting starts it again; a stopped container is deleted only after the retention period.

## The Docker volume was not confirmed as buried

Docker events are checked before and after removal. If deletion cannot be confirmed, keep the verified archive and check `obit doctor`; do not assume the volume was buried successfully. Each `docker-volume-recovery` warning identifies one entry to check. Inspect the volume on the original Docker daemon as directed. Neither `obit resurrect` nor `obit purge` accepts an uncommitted archive. [Docker containers and volumes](./docker)

## An update fails, or downloads from mainland China are slow

`obit update --check` reports an error, or the install script is slow or fails; most likely this computer cannot reach GitHub. When GitHub is slow to answer, `obit update --check` and the install script switch to the mainland China mirror on their own. To use the mirror from the start, run `bash install.sh --mirror cn` on macOS and Linux, or `.\install.ps1 -Mirror cn` on Windows. If you download by hand, take the archive and `checksums.txt` from the same release and the same source. When `obit update --check` prints a `Latest:` line, it is fixed.
