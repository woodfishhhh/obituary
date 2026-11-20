#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
version=${1:-${GITHUB_REF_NAME:-dev}}
version=${version#v}
out=${2:-$root/bin/macos-desktop}
work=$out/.work
bundle=$out/Obituary.app

mkdir -p "$work" "$bundle/Contents/MacOS" "$bundle/Contents/Resources"

ldflags="-s -w -X github.com/woodfishhhh/obituary/internal/buildinfo.Version=$version"
for arch in amd64 arm64; do
    mkdir -p "$work/$arch"
    GOOS=darwin GOARCH="$arch" CGO_ENABLED=1 go build -trimpath -tags desktop,production -ldflags "$ldflags" -o "$work/$arch/obit-gui" ./cmd/obit-gui
    GOOS=darwin GOARCH="$arch" CGO_ENABLED=1 go build -trimpath -tags desktop -ldflags "$ldflags" -o "$work/$arch/obitd" ./cmd/obitd
    GOOS=darwin GOARCH="$arch" CGO_ENABLED=1 go build -trimpath -ldflags "$ldflags" -o "$work/$arch/obit" ./cmd/obit
done

for name in obit obitd obit-gui; do
    lipo -create "$work/amd64/$name" "$work/arm64/$name" -output "$bundle/Contents/MacOS/$name"
    chmod 755 "$bundle/Contents/MacOS/$name"
done

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
<key>LSMinimumSystemVersion</key><string>12.0</string>
<key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
PLIST

plutil -lint "$bundle/Contents/Info.plist"
for name in obit obitd obit-gui; do
    codesign --force --sign - "$bundle/Contents/MacOS/$name"
done
codesign --force --deep --sign - "$bundle"
codesign --verify --deep --strict "$bundle"

app_zip="$out/obituary_${version}_darwin_universal.app.zip"
dmg="$out/obituary_${version}_darwin_universal.dmg"
ditto -c -k --sequesterRsrc --keepParent "$bundle" "$app_zip"
hdiutil create -volname Obituary -srcfolder "$bundle" -ov -format UDZO "$dmg" >/dev/null
hdiutil verify "$dmg" >/dev/null
(cd "$out" && shasum -a 256 "$(basename "$app_zip")" "$(basename "$dmg")" > checksums.txt)

file "$bundle/Contents/MacOS/obit-gui" "$bundle/Contents/MacOS/obitd" "$bundle/Contents/MacOS/obit"
printf '%s\n' "$bundle" "$app_zip" "$dmg" "$out/checksums.txt"
