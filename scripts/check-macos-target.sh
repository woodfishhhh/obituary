#!/usr/bin/env bash
set -euo pipefail
app=$1
min=$(plutil -extract LSMinimumSystemVersion raw "$app/Contents/Info.plist")
for name in obit obitd obit-gui; do test -x "$app/Contents/MacOS/$name"; done
while IFS= read -r -d '' binary; do
  case $(file -b "$binary") in *Mach-O*) ;; *) continue ;; esac
  lipo -verify_arch arm64 x86_64 "$binary"
  for arch in $(lipo -archs "$binary"); do
    actual=$(xcrun vtool -arch "$arch" -show-build "$binary" | awk '$1 == "minos" {print $2}')
    printf '%s %s minos=%s Info.plist=%s\n' "$binary" "$arch" "$actual" "$min"
    [[ $actual == "$min" ]] || exit 1
  done
done < <(find "$app/Contents" -type f -print0)
