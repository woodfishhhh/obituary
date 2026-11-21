#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
version=${1:-${GITHUB_REF_NAME:-dev}}
version=${version#v}
version=${version//\//-}
out=${2:-$root/bin/macos-desktop}
work=$out/.work
bundle=$out/Obituary.app
tray="$bundle/Contents/Helpers/Obituary Menu.app"
export MACOSX_DEPLOYMENT_TARGET=12.0
minflag="-mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"
export CGO_CFLAGS="${CGO_CFLAGS:--O2 -g} $minflag"
export CGO_CXXFLAGS="${CGO_CXXFLAGS:--O2 -g} $minflag"
export CGO_LDFLAGS="${CGO_LDFLAGS:--O2 -g} $minflag"

mkdir -p "$work" "$bundle/Contents/MacOS" "$bundle/Contents/Resources" "$tray/Contents/MacOS" "$tray/Contents/Resources"

ldflags="-s -w -X github.com/woodfishhhh/obituary/internal/buildinfo.Version=$version"
for arch in amd64 arm64; do
    mkdir -p "$work/$arch"
    GOOS=darwin GOARCH="$arch" CGO_ENABLED=1 go build -trimpath -tags desktop,production -ldflags "$ldflags" -o "$work/$arch/obit-gui" ./cmd/obit-gui
    GOOS=darwin GOARCH="$arch" CGO_ENABLED=1 go build -trimpath -tags desktop -ldflags "$ldflags" -o "$work/$arch/obitd" ./cmd/obitd
    GOOS=darwin GOARCH="$arch" CGO_ENABLED=1 go build -trimpath -ldflags "$ldflags" -o "$work/$arch/obit" ./cmd/obit
done

for name in obit obitd obit-gui; do
    destination="$bundle/Contents/MacOS/$name"
    if [[ $name == obitd ]]; then destination="$tray/Contents/MacOS/$name"; fi
    lipo -create "$work/amd64/$name" "$work/arm64/$name" -output "$destination"
    chmod 755 "$destination"
done
ln -sfn '../Helpers/Obituary Menu.app/Contents/MacOS/obitd' "$bundle/Contents/MacOS/obitd"

iconset="$work/Obituary.iconset"
mkdir -p "$iconset"
for size in 16 32 128 256 512; do
    sips -z "$size" "$size" cmd/obit-gui/frontend/dist/icon.png --out "$iconset/icon_${size}x${size}.png"
    sips -z "$((size * 2))" "$((size * 2))" cmd/obit-gui/frontend/dist/icon.png --out "$iconset/icon_${size}x${size}@2x.png"
done
iconutil -c icns "$iconset" -o "$bundle/Contents/Resources/icon.icns"

cat > "$bundle/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
<key>CFBundleDisplayName</key><string>Obituary</string>
<key>CFBundleExecutable</key><string>obit-gui</string>
<key>CFBundleIconFile</key><string>icon.icns</string>
<key>CFBundleIdentifier</key><string>site.woodfish.obituary.gui</string>
<key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
<key>CFBundleName</key><string>Obituary</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>$version</string>
<key>CFBundleVersion</key><string>$version</string>
<key>LSMinimumSystemVersion</key><string>$MACOSX_DEPLOYMENT_TARGET</string>
<key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
PLIST

plutil -lint "$bundle/Contents/Info.plist"
# Notification Center authorizes the bundle's main executable, not a sibling tool.
cp "$bundle/Contents/Resources/icon.icns" "$tray/Contents/Resources/"
plist="$tray/Contents/Info.plist"
cp "$bundle/Contents/Info.plist" "$plist"
plutil -replace CFBundleExecutable -string obitd "$plist"
plutil -replace CFBundleIdentifier -string site.woodfish.obituary.tray "$plist"
plutil -insert LSUIElement -bool true "$plist"
bash "$root/scripts/check-macos-target.sh" "$bundle"
codesign --force --sign - "$tray"
codesign --force --sign - "$bundle/Contents/MacOS/obit"
codesign --force --sign - "$bundle"
codesign --verify --deep --strict "$bundle"

dmg="$out/obituary_${version}_darwin_universal.dmg"
hdiutil create -volname Obituary -srcfolder "$bundle" -ov -format UDZO "$dmg" >/dev/null
hdiutil verify "$dmg" >/dev/null
(cd "$out" && shasum -a 256 "$(basename "$dmg")" > checksums.txt)

file "$bundle/Contents/MacOS/obit-gui" "$bundle/Contents/MacOS/obitd" "$bundle/Contents/MacOS/obit"
printf '%s\n' "$bundle" "$dmg" "$out/checksums.txt"
