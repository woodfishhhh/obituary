#!/usr/bin/env bash
# Extract a snapshot into temporary storage; never inherit the user's display or bus.
set -euo pipefail
deb=$(realpath "$1")
export EVIDENCE=$(realpath "$2")
root=$(mktemp -d)
export HOME=$root/user OBIT_HOME=$root/user/data OBIT_CONFIG_DIR=$root/user/config
export OBIT_USER_HOME=$HOME XDG_CONFIG_HOME=$HOME/.config XDG_RUNTIME_DIR=$root/runtime
export OBIT_LANG=en OBIT_TELEMETRY=0 GDK_BACKEND=x11 LIBGL_ALWAYS_SOFTWARE=1
unset WAYLAND_DISPLAY DISPLAY DBUS_SESSION_BUS_ADDRESS
mkdir -m 700 -p "$XDG_RUNTIME_DIR" "$OBIT_CONFIG_DIR" "$HOME/.config"
printf '[update]\ncheck=false\n' > "$OBIT_CONFIG_DIR/config.toml"
dpkg-deb -x "$deb" "$root/pkg"
export BIN=$root/pkg/usr/libexec/obituary
xvfb-run -a -s '-screen 0 1440x960x24 -nolisten tcp' dbus-run-session -- bash -eu <<'SESSION'
trap '"$BIN/obit" daemon stop; kill ${gui:-} ${other:-} ${host:-} 2>/dev/null || true' EXIT
echo "Private X11 display $DISPLAY; runtime $XDG_RUNTIME_DIR"
if [[ ${NO_WATCHER:-0} != 1 ]]; then
  /usr/bin/python3 scripts/test-linux-desktop-session.py host & host=$!
  gdbus wait --session --timeout 10 org.kde.StatusNotifierWatcher
fi
if [[ ${NO_WATCHER:-0} == 1 ]]; then
  "$BIN/obitd" --tray & daemon=$!
  for _ in $(seq 1 30); do
    [[ -e "$HOME/data/obitd.lock" ]] && break
    sleep .1
  done
  test -e "$HOME/data/obitd.lock"
  touch "$HOME/data/obitd.stop"
  timeout 10 tail --pid="$daemon" -f /dev/null
  wait "$daemon"
  echo 'PASS: no watcher keeps headless cleanup alive and honors stop'
  exit 0
fi
echo 'recoverable fixture' > "$HOME/restore-note"
echo 'calendar fixture' > "$HOME/calendar-note"
"$BIN/obitd" --tray & daemon=$!
"$BIN/obit" mark "$HOME/calendar-note" 1d
"$BIN/obit" bury "$HOME/restore-note"
test ! -e "$HOME/restore-note"
"$BIN/obit" list
"$BIN/obit-gui" & gui=$!
window_for() { timeout 20 xdotool search --sync --all --onlyvisible --pid "$1" --name '^Obituary' | head -1; }
win=$(window_for "$gui")
test "$(xdotool getwindowname "$win")" = "Obituary ($OBIT_HOME)"
for _ in $(seq 1 80); do
  import -window "$win" "$EVIDENCE/calendar-live.png"
  pixels=$(identify -format '%[standard_deviation]' "$EVIDENCE/calendar-live.png" 2>/dev/null || true)
  if [[ -n $pixels && $pixels != 0 && $pixels != 0.0 ]]; then
    break
  fi
  sleep .25
done
test -s "$EVIDENCE/calendar-live.png"
# Wails v2 exits 1 after forwarding a second launch to the running instance.
OBIT_HOME="$HOME/other" "$BIN/obit-gui" & other=$!
otherwin=$(window_for "$other")
test "$win" != "$otherwin"
test "$(xdotool getwindowname "$otherwin")" = "Obituary ($HOME/other)"
for home in "$OBIT_HOME" "$HOME/other"; do
  rc=0; OBIT_HOME="$home" timeout 15 "$BIN/obit-gui" || rc=$?; test "$rc" = 1
done
test "$(window_for "$gui")" = "$win"
test "$(window_for "$other")" = "$otherwin"
echo "PASS: two ledger windows ($win, $otherwin); both repeat launches forwarded"
kill -INT "$other"; wait "$other" || true; other=
xdotool mousemove --window "$win" 449 25 click 1
sleep 1
import -window "$win" "$EVIDENCE/settings-live.png"
xdotool mousemove --window "$win" 900 650
xdotool click --repeat 12 --delay 80 5
sleep 1
import -window "$win" "$EVIDENCE/settings-bottom.png"
xdotool mousemove --window "$win" 870 475 click 1
sleep 2
xdotool mousemove --window "$win" 185 25 click 1
sleep 1
import -window "$win" "$EVIDENCE/night-dark.png"
kill -INT "$gui"; wait "$gui" || true; gui=
timeout 30 /usr/bin/python3 scripts/test-linux-desktop-session.py probe
timeout 10 tail --pid="$daemon" -f /dev/null
wait "$daemon"
"$BIN/obitd" --once
echo 'PASS: Quit exited the tray; headless maintenance still works'
SESSION
