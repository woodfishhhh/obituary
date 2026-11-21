#!/usr/bin/env bash
# Only run on a fresh GitHub-hosted Mac, never on a developer's desktop.
set -euo pipefail
[[ ${GITHUB_ACTIONS:-} == true && ${RUNNER_ENVIRONMENT:-} == github-hosted ]]
ev=$RUNNER_TEMP/evidence
mkdir "$ev"
exec > >(tee "$ev/session.log") 2>&1
set -x
app=/Applications/Obituary.app
cli=$app/Contents/MacOS/obit
data="$HOME/Library/Application Support/obituary"
mnt=$RUNNER_TEMP/obit-volume
snap() { screencapture -x "$ev/$1.png" || echo "screencapture failed: $1"; }
ui() { python3 -c 'import subprocess,sys; sys.exit(subprocess.run(["osascript", *sys.argv[1:]],timeout=12).returncode)' "$@"; }
sw_vers
[[ $(uname -m) == "$EXPECTED_ARCH" ]]
cd "$RUNNER_TEMP/release"
file=obituary_1.6.0_darwin_universal.dmg
awk -v f="$file" '$2==f {print; n++} END {if(n!=1)exit 1}' checksums.txt > "$ev/dmg.sha256"
shasum -a 256 -c "$ev/dmg.sha256"
[[ ! -e $app && ! -e $data ]]
mkdir "$mnt"
hdiutil attach "$file" -readonly -nobrowse -mountpoint "$mnt"
ls -la "$mnt/"
snap mounted
ditto "$mnt/Obituary.app" "$app"
hdiutil detach "$mnt"
! mount | grep -F " on $mnt " || exit 1
codesign --verify --deep --strict "$app"
codesign -dv "$app"
plutil -extract LSMinimumSystemVersion raw "$app/Contents/Info.plist"
for exe in "$app"/Contents/MacOS/*; do xcrun vtool -show-build "$exe"; done
snap installed
if [[ $LANE == gatekeeper ]]; then
  xattr -w com.apple.quarantine "0083;$(printf '%x' "$(date +%s)");Safari;$(uuidgen)" "$app"
  xattr -p com.apple.quarantine "$app"
  spctl --status
  spctl -a -t execute -vv "$app" || echo "assessment exit=$?"
  ui -e 'return system version of (system info)' || true
  open "$app" > "$ev/first-open.log" 2>&1 &
  sleep 4
  pgrep -lx obit-gui || true
  snap first-open
  open 'x-apple.systempreferences:com.apple.preference.security?Privacy' || true
  sleep 3
  snap privacy-security
  echo 'Open Anyway and confirmation are intentionally left to the user.'
  exit 0
fi

# Separate pristine VM: this measures desktop behavior, not Gatekeeper approval.
xattr -l "$app"
open "$app"
sleep 6
pgrep -lx obit-gui
pgrep -lx obitd || echo 'No tray process on first launch'
snap cold-open
before=$(pgrep -x obit-gui)
open "$app"
sleep 2
after=$(pgrep -x obit-gui)
[[ $before == "$after" && $after != *$'\n'* ]] || echo 'FAIL: second open changed GUI PID set'

"$cli" config telemetry off
printf 'calendar fixture\n' > "$HOME/obit-calendar.txt"
printf 'grave-retention fixture\n' > "$HOME/obit-retention.txt"
printf 'notification fixture\n' > "$HOME/obit-notify.txt"
"$cli" mark "$HOME/obit-calendar.txt" 2d
"$cli" bury "$HOME/obit-retention.txt" "$HOME/obit-notify.txt"
"$cli" graveyard --json
pkill -TERM -x obit-gui
sleep 2
if ! pgrep -x obitd; then "$app/Contents/MacOS/obitd" --tray > "$ev/tray.log" 2>&1 & sleep 3; fi
tray='tell application "System Events" to tell (first application process whose unix id is '
if traypid=$(pgrep -xo obitd); then
  tray="$tray$traypid) to "
  ui -e "${tray}click menu bar item 1 of last menu bar" || true
  sleep 1
  snap tray-menu
  ui -e "${tray}click menu item \"Open Obituary\" of menu 1 of menu bar item 1 of last menu bar" || true
  sleep 2
  pgrep -lx obit-gui || echo 'No window process after tray menu action'
  snap tray-opened-window
fi
for style in light dark; do
  flag=false; [[ $style == light ]] || flag=true
  ui -e "tell application \"System Events\" to tell appearance preferences to set dark mode to $flag" || true
  ui -e 'tell application "System Events" to get dark mode of appearance preferences' || true
  open "$app"
  sleep 2
  snap "$style-calendar-and-menu-bar"
done

svc="gui/$(id -u)/site.woodfish.obituary.tray"
plist="$HOME/Library/LaunchAgents/site.woodfish.obituary.tray.plist"
launchctl print "$svc" >/dev/null || echo "initial launchctl exit=$?"
pkill -TERM -x obit-gui
sleep 2
open "$app"
sleep 5
ui -e "tell application \"System Events\" to set frontmost of (first application process whose unix id is $(pgrep -xo obit-gui)) to true" || true
swiftc "$GITHUB_WORKSPACE/scripts/accept-macos-controls.swift" -o "$RUNNER_TEMP/controls"
click() {
  if [[ $1 == Settings ]]; then
    ui "$GITHUB_WORKSPACE/scripts/accept-macos-click.applescript" "$(pgrep -xo obit-gui)" "$1" && return 0
  fi
  for i in 1 2 3 4; do
    snap "settings-$i"
    "$RUNNER_TEMP/controls" "$ev/settings-$i.png" "$1" && return 0
    [[ $1 != Settings ]] || break
    sleep 1
  done
  return 1
}
click Settings || true
sleep 2
click 'Start the tray at sign-in' || true
sleep 2
if [[ -f $plist ]]; then
  plutil -p "$plist"
  launchctl print "$svc" | grep -E 'state =|program =|pid =|/Obituary.app/' || true
  snap login-on
  click 'Start the tray at sign-in' || true
  sleep 2
  [[ ! -e $plist ]] || echo 'FAIL: plist still present'
  launchctl print "$svc" >/dev/null || echo "after toggle off: launchctl exit=$?"
else
  echo 'No login plist after attempted UI toggle'
  launchctl print "$svc" >/dev/null || echo "after attempted enable: launchctl exit=$?"
fi
snap login-off
if ! pgrep -x obitd; then "$app/Contents/MacOS/obitd" --tray >> "$ev/tray.log" 2>&1 & sleep 3; fi
if traypid=$(pgrep -xo obitd); then
  path="of menu 1 of menu bar item 1 of last menu bar"
  tray="tell application \"System Events\" to tell (first application process whose unix id is $traypid) to "
  ui -e "${tray}click menu bar item 1 of last menu bar" || true
  parent="menu item \"Recently buried (click one to resurrect)\" $path"
  ui -e "${tray}click $parent" || true
  ui -e "${tray}click (first menu item of menu 1 of $parent whose name contains \"obit-notify\")" || true
fi
sleep 3
[[ -f "$HOME/obit-notify.txt" ]] && echo 'Tray restored notification fixture' || echo 'No tray restore observed'
snap notification
/usr/bin/log show --last 3m --style compact --predicate '(process == "usernoted" OR subsystem == "com.apple.UserNotifications") AND eventMessage CONTAINS[c] "obituary"' > "$ev/notification.log" || true

if traypid=$(pgrep -xo obitd); then
  tray="tell application \"System Events\" to tell (first application process whose unix id is $traypid) to "
  ui -e "${tray}click menu bar item 1 of last menu bar" || true
  ui -e "${tray}click menu item \"Quit\" of menu 1 of menu bar item 1 of last menu bar" || true
  sleep 2
fi
"$cli" daemon stop || true
pkill -TERM -x obitd || true
pkill -TERM -x obit-gui || true
sleep 2
! pgrep -lx obit-gui || exit 1
! pgrep -lx obitd || exit 1
find "$data" -type f | sort
payload=$(find "$data/graveyard" -type f -name obit-retention.txt)
[[ -n $payload ]]
grep -Fx 'grave-retention fixture' "$payload"
shasum -a 256 "$data/store.json" "$data/config.toml" "$payload" > "$ev/preserved.sha256"
# A different ledger removes only the test installation, not the data under test.
cp "$cli" "$RUNNER_TEMP/obit"
OBIT_HOME="$RUNNER_TEMP/uninstall-data" OBIT_CONFIG_DIR="$RUNNER_TEMP/uninstall-config" "$RUNNER_TEMP/obit" bury "$app"
[[ ! -e $app ]]
shasum -a 256 -c "$ev/preserved.sha256"
snap uninstalled
