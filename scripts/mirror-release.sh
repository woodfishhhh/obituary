#!/usr/bin/env bash
# Copies a release's archives and checksums.txt to the mainland China mirror
# through a computer in China. GitHub's runners reach the mirror across the
# border at about 25 KB/s in the evening (40 minutes for one release), while a
# computer in China downloads from GitHub and uploads to the mirror at about
# 1 MB/s. gh downloads draft assets too, so the mirror is complete before the
# release is published. The release workflow's own upload stays as a fallback.
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
for f in checksums.txt $(awk '{print $2}' "$work/checksums.txt"); do
  curl -fsS --max-time 300 "https://cn.obituary.woodfish.site/download/v$version/$f" | cmp -s - "$work/$f" \
    || { echo "the mirror's $f differs from the release" >&2; exit 1; }
done
echo "mirror has every v$version file"
