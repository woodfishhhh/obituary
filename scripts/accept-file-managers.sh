#!/usr/bin/env bash
# Only the hosted disposable macOS login or a private Ubuntu X11 session may be used.
set -euo pipefail
[[ ${GITHUB_ACTIONS:-} == true ]]
login_home=$HOME
export OBIT_USER_HOME=$(mktemp -d "$login_home/obit-acceptance.XXXXXXXX")
export HOME=$OBIT_USER_HOME OBIT_HOME=$OBIT_USER_HOME/data OBIT_CONFIG_DIR=$OBIT_USER_HOME/config
export CODEX_HOME=$HOME/.codex XDG_DATA_HOME=$HOME/.local/share XDG_CONFIG_HOME=$HOME/.config
export OBIT_TELEMETRY=0 OBIT_LANG=en LANG=en_US.UTF-8
export PATH="$RUNNER_TEMP/tools:$PATH" EVIDENCE="$RUNNER_TEMP/evidence"
exec > >(tee "$EVIDENCE/session.log") 2>&1
mkdir -p "$HOME/Downloads" "$HOME/Desktop" "$OBIT_CONFIG_DIR"
trap "obit daemon stop || true" EXIT
obit init -g --agent codex --context-menu --yes
mkdir -p "$HOME/Downloads/Nautilus" "$HOME/Desktop/Dolphin"
if [[ $RUNNER_OS == Linux ]]; then
  export GDK_BACKEND=x11 QT_QPA_PLATFORM=xcb LC_ALL=C.UTF-8
  openbox >"$EVIDENCE/openbox.log" 2>&1 &
  for manager in Nautilus Dolphin; do
    folder="$HOME/Downloads/Nautilus"; [[ $manager != Dolphin ]] || folder="$HOME/Desktop/Dolphin"
    sample="$folder/Sample.txt"; touch "$sample"
    python3 -c "import os,sys;os.setxattr(sys.argv[1], b\"user.xdg.tags\", b\"work\")" "$sample"
    python3 scripts/file-manager-menu.py "$manager" "$folder"
    python3 -c "import os,sys,time; f=sys.argv[1]; end=time.monotonic()+10
while time.monotonic()<end and b\"burial\" not in os.getxattr(f,b\"user.xdg.tags\"): time.sleep(.2)
v=os.getxattr(f,b\"user.xdg.tags\");print(f,v);assert v.startswith(b\"work,\") and b\"burial\" in v" "$sample"
  done
  g++ -fPIC scripts/read-kde-tags.cpp -I/usr/include/KF5/KFileMetaData -o "$RUNNER_TEMP/tools/read-kde-tags" $(pkg-config --cflags --libs Qt5Core) -lKF5FileMetaData
  read-kde-tags "$sample"
else
  sample="$HOME/Downloads/Sample.txt"; touch "$sample"
  tag=com.apple.metadata:_kMDItemUserTags
  python3 -c "import os,plistlib,sys;os.setxattr(sys.argv[1],sys.argv[2],plistlib.dumps([\"work\n6\"],fmt=plistlib.FMT_BINARY))" "$sample" "$tag"
  workflow="$HOME/Library/Services/Add to Obituary · 7 days.workflow"
  plutil -lint "$workflow/Contents/Info.plist" "$workflow/Contents/document.wflow"
  for key in OBIT_USER_HOME OBIT_CONFIG_DIR OBIT_HOME CODEX_HOME OBIT_LANG OBIT_TELEMETRY; do launchctl setenv "$key" "${!key}"; done
  # Services lookup uses the login home, even when the app under test has its own home.
  mkdir -p "$login_home/Library/Services"; ln -s "$workflow" "$login_home/Library/Services/${workflow##*/}"
  /System/Library/CoreServices/pbs -update || true
  /System/Library/CoreServices/pbs -dump_pboard >"$EVIDENCE/services.txt" || true
  /usr/bin/automator -i "$sample" "$workflow"
  xattr -px "$tag" "$sample" | tee "$EVIDENCE/xattr.txt"
  mdls -name kMDItemUserTags "$sample" | tee "$EVIDENCE/spotlight.txt"
  python3 -c "import os,plistlib,sys;v=plistlib.loads(os.getxattr(sys.argv[1],sys.argv[2]));print(v);assert v[0]==\"work\n6\" and any(\"burial\" in t for t in v)" "$sample" "$tag"
  open -R "$sample"; sleep 2; screencapture -x "$EVIDENCE/finder.png" || true
fi
obit list --json | tee "$EVIDENCE/tracked.json"
obit init -g --agent codex --uninstall --yes
obit uninstall --yes
if [[ $RUNNER_OS == Linux ]]; then
  python3 -c "import os,sys;assert os.getxattr(sys.argv[1], b\"user.xdg.tags\")==b\"work\"" "$sample"
else
  python3 -c "import os,plistlib,sys;assert plistlib.loads(os.getxattr(sys.argv[1],sys.argv[2]))==[\"work\n6\"]" "$sample" "$tag"
fi
