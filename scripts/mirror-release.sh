#!/usr/bin/env bash
# Manual fallback from a computer in China: gh can download and pre-stage
# draft archives, checksums and tagged installers. Normally the mirror pulls
# these files and updates latest.json itself after the release is published.
#
# Usage: bash scripts/mirror-release.sh VERSION [SSH_HOST]
set -euo pipefail
version=${1:?usage: mirror-release.sh VERSION [SSH_HOST]}
version=${version#v}
host=${2:-jdcloud-blog}
dest=/opt/blog-stack/sites/obituary/download
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.+-]+)?$ ]] || { echo "not a version: $version" >&2; exit 1; }

work=$(obit mktemp 1d)
if command -v cygpath >/dev/null; then work=$(cygpath -u "$work"); fi
gh release download "v$version" -R woodfishhhh/obituary -D "$work" -p 'obituary_*' -p checksums.txt
(cd "$work" && sha256sum -c checksums.txt)

# These must come from the release tag, never the mutable site root.
tag_url="https://raw.githubusercontent.com/woodfishhhh/obituary/v$version"
curl -fsS --max-time 30 "$tag_url/packaging/install.ps1" -o "$work/install.ps1"
curl -fsS --max-time 30 "$tag_url/install.sh" -o "$work/install.sh"

# The mirror host is small and shared: one low-priority stream into a staging
# folder, checked again there before the files take their public names.
tar -C "$work" -cf - . | ssh "$host" "set -e
stage=\$(mktemp -d $dest/.v$version.XXXXXX)
nice -n 19 tar -C \"\$stage\" -xf -
cd \"\$stage\"
nice -n 19 sha256sum -c --quiet checksums.txt
chmod 644 ./*
mkdir -p $dest/v$version
chown obit-deploy: $dest/v$version ./*
mv -f ./* $dest/v$version/
cd /
rmdir \"\$stage\""

# Read back over HTTPS exactly what users will download.
for f in install.ps1 install.sh checksums.txt $(awk '{print $2}' "$work/checksums.txt"); do
  curl -fsS --max-time 300 "https://cn.obituary.woodfish.site/download/v$version/$f" | cmp -s - "$work/$f" \
    || { echo "the mirror's $f differs from the release" >&2; exit 1; }
done
echo "mirror has every v$version file"
