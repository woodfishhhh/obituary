#!/usr/bin/env bash
# Run as root in a disposable Ubuntu 24.04 container, with one .deb argument.
set -euo pipefail
. /etc/os-release
[[ $ID == ubuntu && $VERSION_ID == 24.04 && $(dpkg --print-architecture) == amd64 ]]
! dpkg-query -W obituary-desktop >/dev/null 2>&1 || exit 1
export DEBIAN_FRONTEND=noninteractive OBIT_TELEMETRY=0
export HOME=/tmp/desktop-user OBIT_HOME=/tmp/desktop-user/data OBIT_CONFIG_DIR=/tmp/desktop-user/config
mkdir -p "$OBIT_HOME" "$OBIT_CONFIG_DIR" "$HOME/.local/bin"
echo preserve > "$OBIT_CONFIG_DIR/user-file"
echo preserve > "$HOME/.local/bin/obit"
apt-get update -qq
apt-get install -y --no-install-recommends "$1" desktop-file-utils
dpkg-query -W obituary-desktop libgtk-3-0t64 libwebkit2gtk-4.1-0
desktop=/usr/share/applications/obituary.desktop
icon=/usr/share/icons/hicolor/128x128/apps/obituary.png
desktop-file-validate "$desktop"
bin=/usr/libexec/obituary
grep -Fx "Exec=$bin/obit-gui" "$desktop"
test -s "$icon"
"$bin/obit" --version
"$bin/obit-gui" --smoke
"$bin/obitd" --once
OBIT_START_MENU_DIR="$HOME/.local/share/applications" "$bin/obit" --help >/dev/null
apt-get remove -y obituary-desktop
for path in "$desktop" "$icon" "$bin/obit-gui" "$bin/obitd" "$bin/obit"; do test ! -e "$path"; done
grep -Fx preserve "$OBIT_CONFIG_DIR/user-file" "$HOME/.local/bin/obit"
echo 'PASS: install, desktop entry, runtime, removal and user data preservation'
