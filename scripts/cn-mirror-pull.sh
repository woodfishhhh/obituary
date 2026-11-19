#!/bin/bash
set -euo pipefail
export LC_ALL=C
fail() { printf '%s\n' "$*" >&2; exit 1; }

command_pattern='^pull (v[0-9]+\.[0-9]+\.[0-9]+)$'
[[ ${SSH_ORIGINAL_COMMAND-} =~ $command_pattern ]] || fail 'Expected: pull vX.Y.Z'
tag=${BASH_REMATCH[1]}

download=/opt/blog-stack/sites/obituary/download
repo=woodfishhhh/obituary
api=https://api.github.com/repos/$repo
SECONDS=0
exec 9>> "$download/.pull.lock"
flock 9

get() {
  curl -qfsSL --proto '=https' --proto-redir '=https' --retry 3 --max-time 60 "$@"
}

stage=$(mktemp -d "$download/.$tag.XXXXXX")
trap 'rm -rf -- "$stage"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
files=$stage/files
mkdir -- "$files"
get "$api/releases/tags/$tag" > "$stage/release.json"
prefix="https://github.com/$repo/releases/download/$tag/"
jq -er --arg tag "$tag" --arg prefix "$prefix" '
  if .tag_name != $tag or .draft != false or .prerelease != false then
    error("Release must be published and stable")
  else .assets[] | select(.name == "checksums.txt" or (.name | startswith("obituary_"))) |
    if (.name | test("[^A-Za-z0-9._-]") | not) and
       .browser_download_url == ($prefix + .name) then
      [.name, .browser_download_url] | @tsv
    else error("Invalid release asset name or URL") end
  end' "$stage/release.json" > "$stage/assets"

aria_opts=(--max-tries=5 --retry-wait=5 --timeout=45 --connect-timeout=15 --no-conf
           --console-log-level=error --summary-interval=0 --show-console-readout=false --download-result=hide)
while IFS=$'\t' read -r name url; do
  nice -n 19 aria2c -x16 -s16 -k1M "${aria_opts[@]}" --dir="$files" --out="$name" "$url"
done < "$stage/assets"

# Validate the list before asking sha256sum to open any paths from it.
awk -F '\t' '$1 != "checksums.txt" {print $1}' "$stage/assets" | sort > "$stage/expected"
[[ -s $stage/expected && -f $files/checksums.txt ]] || fail 'Missing archives or checksums.txt'
awk 'length($1) != 64 || $0 !~ /^[a-fA-F0-9]+ [ *]obituary_[A-Za-z0-9._-]+$/ {exit 1}
     {print substr($0, 67)}' "$files/checksums.txt" | sort > "$stage/listed" || fail 'Invalid checksums.txt'
cmp -s "$stage/expected" "$stage/listed" || fail 'Release assets and checksums.txt differ'
(cd "$files" && nice -n 19 sha256sum --check --strict --quiet checksums.txt)

# The tagged Git bytes are the contract, not a checkout's line endings.
get -H 'Accept: application/vnd.github.raw' "$api/contents/install.sh?ref=$tag" > "$files/install.sh"
get -H 'Accept: application/vnd.github.raw' "$api/contents/packaging/install.ps1?ref=$tag" > "$files/install.ps1"
chmod 644 -- "$files"/*
count=$(wc -l < "$stage/assets")
bytes=$(stat -c %s -- "$files"/* | awk '{total += $1} END {printf "%.0f", total}')

replace_if_changed() {
  if [[ -f $2 && ! -L $2 ]]; then
    cmp -s -- "$1" "$2" && return 0
    [[ $? == 1 ]] || fail "Cannot compare $2"
  fi
  mv -fT -- "$1" "$2"
}

dest=$download/$tag
[[ ! -L $dest ]] || fail 'Release directory must not be a symlink'
if [[ ! -e $dest ]]; then
  chmod 755 -- "$files"
  mv -T -- "$files" "$dest"
else
  for file in "$files"/*; do
    replace_if_changed "$file" "$dest/${file##*/}"
  done
fi

# Read latest after the downloads, so replaying an older tag cannot roll it back.
latest=$(get "$api/releases/latest" | jq -er '.tag_name | strings | select(length > 0)')
if [[ $latest == "$tag" ]]; then
  jq '{tag_name, name, body, html_url, published_at, draft, prerelease}' "$stage/release.json" > "$stage/latest.json"
  chmod 644 -- "$stage/latest.json"
  replace_if_changed "$stage/latest.json" "$download/latest.json"
fi

printf 'Mirror %s: %d files, %s bytes, %d s\n' "$tag" "$((count + 2))" "$bytes" "$SECONDS"
