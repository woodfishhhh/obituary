#!/bin/bash
set -euo pipefail
export LC_ALL=C
script=$(realpath "${1:-$(dirname "$0")/cn-mirror-pull.sh}")
work=$(mktemp -d); trap 'rm -rf -- "$work"' EXIT
input=$work/incoming; public=$work/download; tag=v1.2.3
mkdir "$input" "$public"
cd "$input"
for name in obituary_1.2.3_{darwin,linux}_{amd64,arm64}.tar.gz obituary_1.2.3_windows_{amd64,arm64}.zip install.sh install.ps1; do
  printf '%s\n' "$name" > "$name"
done
sha256sum obituary_* > checksums.txt
printf '%s\n' "$tag" > latest-tag
jq -n --arg tag "$tag" '{tag_name:$tag,html_url:("https://github.com/woodfishhhh/obituary/releases/tag/"+$tag),
  published_at:"2026-01-01T00:00:00Z",draft:false,prerelease:false}' > release.json
transport() { sha256sum install.sh install.ps1 checksums.txt release.json latest-tag > transport.sha256; }
transport
run() { bash -c 'source "$1"; publish "$2" "$3" "$4"' _ "$script" "${1:-$tag}" "$input" "$public"; }
snapshot() { (cd "$public" && find . -printf '%P %i %T@\n' | sort && sha256sum "$tag"/* latest.json); }
reject() {
  local message=$1; shift
  if run "$@" >"$work/log" 2>&1; then echo 'Unexpected publish success' >&2; exit 1; fi
  grep -Fq -- "$message" "$work/log" || { cat "$work/log" >&2; exit 1; }
  [[ $(snapshot) == "$before" ]] || { echo 'Rejected upload changed public files' >&2; exit 1; }
}
run
cmp release.json "$public/latest.json"
(cd "$public/$tag" && sha256sum --quiet --strict -c checksums.txt)
cmp install.sh "$public/$tag/install.sh"; cmp install.ps1 "$public/$tag/install.ps1"
before=$(snapshot)
run
[[ $(snapshot) == "$before" ]]
(
  desktop=obituary_1.2.3_linux_amd64.deb
  cp -a "$input" "$work/with-desktop"
  cd "$work/with-desktop"
  printf 'native desktop\n' > "$desktop"
  sha256sum "$desktop" >> checksums.txt; transport
  source "$script"; verify "$tag" .
  printf 'corrupt' >> "$desktop"
  if (verify "$tag" .) >/dev/null 2>&1; then exit 1; fi
)
reject 'Expected a stable tag' v1.2.3-rc.1
touch .extra; reject 'Unexpected file count'; mv .extra "$work/extra"
archive=obituary_1.2.3_linux_amd64.tar.gz
mv "$archive" "$work/original"; ln -s "$work/original" "$archive"; reject 'non-regular file'
mv "$archive" "$work/link"; reject 'Unexpected file count'
printf 'corrupt\n' > "$archive"; reject 'SHA-256 mismatch: checksums.txt'
mv "$work/original" "$archive"
printf 'corrupt installer\n' >> install.sh; reject 'SHA-256 mismatch: transport.sha256'
transport; reject 'Existing release differs'
cp "$public/$tag/install.sh" install.sh
sed -i '1d' checksums.txt; transport; reject 'Incomplete or unexpected names'
printf 'PASS: publish, idempotence, invalid tag, file list, link, missing file, corrupt archive/installer, immutable tag and incomplete checksums\n'
