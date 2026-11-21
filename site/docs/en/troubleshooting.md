# Troubleshooting

Start with `obit doctor`. It checks installation, integrations, background services and data without changing files. Add `--json` or `--verbose` for more detail.

## Install, PATH and checksums

**Symptom:** the shell cannot find `obit`, or a downloaded archive does not match its checksum.

**Cause:** the install directory is not on `PATH`, or the archive is incomplete or altered.

**Fix:** download `checksums.txt` from the same release and run this in the download directory:

```bash
sha256sum --check --strict checksums.txt
```

In Windows PowerShell use `Get-FileHash .\obit-*.zip -Algorithm SHA256` and compare the result with the checksum file. Do not run a mismatched archive. Open a new terminal after changing `PATH`.

## Windows desktop

**SmartScreen:** unsigned programs may show “Windows protected your PC”. Verify SHA-256 first, then choose “More info → Run anyway”. Stop when the file cannot be verified or is reported as malware.

**WebView2:** when the window cannot open or says that the runtime is missing, install Microsoft Edge WebView2 Runtime and run `obit doctor`. The Windows window is currently x64 only.

## macOS desktop

**Gatekeeper:** the DMG app is not Developer ID signed or notarized. Verify the checksum before the first open. If macOS blocks it as an unidentified developer, use “System Settings → Privacy & Security → Open Anyway”. Do not disable Gatekeeper or approve an app reported as damaged or malicious.

**launchd:** if the menu-bar tray does not start at login, turn “Start tray at login” off and on in Settings, then check:

```bash
obit daemon status
launchctl list | grep obituary
```

## Linux desktop and background work

**WebKitGTK:** the Ubuntu 24.04 window needs GTK 3 and WebKitGTK 4.1. If it still does not open:

```bash
sudo apt update
sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0
obit doctor
```

**systemd / crontab:** inspect the user timer and any legacy cron entry:

```bash
systemctl --user status obituary.timer
systemctl --user list-timers | grep obituary
crontab -l
```

Linux without the desktop package still supports the CLI. `obit daemon enable` installs the user scheduler and `obit daemon status` shows the next expiry.

## Background work and updates

**The daemon is not running:** use `obit daemon status`. Run `obit sweep` for an immediate pass, or `obit daemon enable` to restore scheduled work.

**An update or the mainland mirror fails:** run `obit update --check`, then download the archive and `checksums.txt` from the same release. The mainland installer uses:

```bash
bash install.sh --mirror cn
```

Do not combine an archive and checksum file from different releases.

## Files and configuration

**A WSL mounted drive was renamed:** after a move under `/mnt/<drive>`, confirm the real Windows path before using Obituary. Use a literal path with `obit list`, `obit graveyard` or a new `obit mark`; do not force a restore with an old path.

**Permissions, locks and retries:** when an editor or sync tool holds a file, Obituary keeps the original and schedules a retry. Close the holder and run `obit sweep`; the window also offers “Retry” and “Stop tracking”. Do not remove the failed record manually.

**Invalid TOML:** `obit config` reports the file, line and unknown key. Fix spelling, quotes or duration syntax and run it again; unknown keys are never silently ignored.

## Integrations and containers

**A hook did not run:** inspect and reinstall the integration in the current project or globally:

```powershell
obit init --show
obit init --dry-run
obit doctor --verbose
```

Check that the agent uses the installed configuration and that `OBIT_DISABLED=1` is not set. For hook audit details, set `OBIT_HOOK_AUDIT=1` and run `obit hook audit`.

**Docker is not running:** `obit docker list` and the window report the connection failure. Start Docker Desktop or the Docker Engine, then approve the container proposal. Images, volumes and networks are not removed by Obituary.

**A restore target already exists:** `obit resurrect` never overwrites. It restores the item under a non-conflicting name. Inspect the destination, move or rename the conflict, and retry.
