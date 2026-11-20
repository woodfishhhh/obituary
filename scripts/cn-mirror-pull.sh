#!/bin/bash
set -euo pipefail
export LC_ALL=C
shopt -s nullglob dotglob
stable='^v[0-9]+\.[0-9]+\.[0-9]+$'
fail() { printf '%s\n' "$*" >&2; exit 1; }

# All names must be accounted for, including hidden files. Never follow uploaded links.
regular_files() {
  local dir=$1 name
  shift
  [[ -d $dir && ! -L $dir ]] || fail 'Missing or linked upload directory'
  local entries=("$dir"/*)
  [[ ${#entries[@]} == $# ]] || fail 'Unexpected file count in upload'
  for name do
    [[ -f $dir/$name && ! -L $dir/$name ]] || fail "Missing or non-regular file: $name"
  done
}

check_sums() {
  local list=$1
  shift
  # Validate names and duplicates before sha256sum opens any path from a manifest.
  local names
  names=$(awk 'length($1) != 64 || $0 !~ /^[[:xdigit:]]+ [ *][A-Za-z0-9_.-]+$/ {exit 1}
               {print substr($0, 67)}' "$list" | sort) || fail "Invalid $list"
  [[ $names == "$(printf '%s\n' "$@" | sort)" ]] || fail "Incomplete or unexpected names in $list"
  sha256sum --check --strict --quiet "$list" || fail "SHA-256 mismatch: $list"
}

# Used at both trust boundaries: the runner download and the locked server upload.
verify() (
  local tag=$1 prefix=obituary_${1#v}_
  [[ $tag =~ $stable ]] || fail 'Expected a stable tag (vX.Y.Z)'
  [[ -d $2 && ! -L $2 ]] || fail 'Missing or linked staging directory'
  cd -- "$2"
  local archives=("$prefix"{darwin,linux}_{amd64,arm64}.tar.gz "$prefix"windows_{amd64,arm64}.zip)
  # Older releases have only the six archives; accept only the exact native package name.
  if [[ -e ${prefix}linux_amd64.deb ]]; then archives+=("${prefix}linux_amd64.deb"); fi
  regular_files . "${archives[@]}" install.sh install.ps1 checksums.txt release.json latest-tag transport.sha256
  check_sums checksums.txt "${archives[@]}"
  check_sums transport.sha256 install.sh install.ps1 checksums.txt release.json latest-tag
  jq -es --arg tag "$tag" 'length == 1 and (.[0] | .tag_name == $tag and .draft == false and .prerelease == false
    and .html_url == ("https://github.com/woodfishhhh/obituary/releases/tag/" + $tag)
    and (.published_at | fromdateiso8601 | type == "number"))' release.json >/dev/null || fail 'Invalid release metadata'
  [[ $(<latest-tag) =~ $stable ]] || fail 'Invalid latest tag'
)

# The caller holds the same lock as the manual pull and the upload receiver.
publish() (
  local tag=$1 input=$2 download=$3 dest=$3/$1 work
  verify "$tag" "$input"
  [[ $(stat -c %d "$input") == "$(stat -c %d "$download")" ]] || fail 'Staging and downloads must share a filesystem'
  work=$(mktemp -d -- "${input%/*}/.publish.XXXXXX")
  trap 'rm -rf -- "$work"' EXIT
  mkdir -- "$work/files"
  install -m 644 -- "$input"/obituary_* "$input"/install.{sh,ps1} "$input/checksums.txt" "$work/files/"
  if [[ -e $dest || -L $dest ]]; then
    [[ -d $dest && ! -L $dest ]] && diff -qr --no-dereference -- "$work/files" "$dest" || fail 'Existing release differs; refusing to replace it'
  else
    chmod 755 -- "$work/files"
    mv -T -- "$work/files" "$dest"
  fi
  if [[ $(<"$input/latest-tag") == "$tag" ]]; then
    install -m 644 -- "$input/release.json" "$work/latest.json"
    if ! cmp -s -- "$work/latest.json" "$download/latest.json"; then
      mv -fT -- "$work/latest.json" "$download/latest.json"
    fi
  fi
  printf 'Published %s from verified staging (no network fetch)\n' "$tag"
)

main() {
  local staging=/home/obit-deploy/release-staging download=/opt/blog-stack/sites/obituary/download
  local command=${SSH_ORIGINAL_COMMAND-} pattern='^publish (v[0-9]+\.[0-9]+\.[0-9]+)$'
  if [[ $command =~ $pattern ]]; then
    local tag=${BASH_REMATCH[1]}
    exec 9>>"$download/.pull.lock"; flock 9
    publish "$tag" "$staging/incoming" "$download"
    logger -t obituary-mirror -- "published $tag source=staging checksums=verified fetch=none"
  elif [[ $command == 'rsync --server '* ]]; then
    exec 9>>"$download/.pull.lock"; flock 9
    exec /usr/bin/rrsync -wo -munge "$staging"
  else
    fail 'Only a staging upload or publish vX.Y.Z is allowed'
  fi
}

if [[ ${BASH_SOURCE[0]} == "$0" ]]; then main; fi
