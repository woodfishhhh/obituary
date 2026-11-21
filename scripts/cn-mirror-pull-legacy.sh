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

stage=$(mktemp -d "$download/.$tag.XXXXXX")
proxy=127.0.0.1:31761
tunnel= transfer=
cleanup() {
  local pid
  for pid in "$transfer" "$tunnel"; do
    [[ -n $pid ]] || continue
    kill "$pid" 2>/dev/null || :; wait "$pid" 2>/dev/null || :
  done
  rm -rf -- "$stage"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
files=$stage/files
mkdir -- "$files"
# LocalCommand signals after authentication and binding. Keep SSH as a child, without the lock.
mkfifo -- "$stage/ready"
ssh -F /etc/obituary-mirror/attvps.conf -nNT -D "$proxy" -o ExitOnForwardFailure=yes -o BatchMode=yes \
  -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=12 \
  -o ServerAliveInterval=15 -o ServerAliveCountMax=2 -o PermitLocalCommand=yes -o 'LocalCommand=echo ready' \
  attvps 9>&- > "$stage/ready" &
tunnel=$!
if ! read -rt 20 ready < "$stage/ready" || [[ $ready != ready ]]; then fail 'Could not open the attvps SOCKS tunnel'; fi

get() {
  local out=$1; shift
  # wait is interruptible even when a large transfer is stalled. Never honor an inherited NO_PROXY.
  curl -qfsSL --proto '=https' --proto-redir '=https' --proxy "socks5h://$proxy" --noproxy '' \
    --connect-timeout 15 --max-time 60 --retry 3 --retry-max-time 180 --remove-on-error -o "$out" "$@" 9>&- &
  transfer=$!
  wait "$transfer"
  transfer=
}
get "$stage/release.json" "$api/releases/tags/$tag"
jq -er --arg tag "$tag" --arg assets "$api/releases/assets/" '
  if .tag_name != $tag or .draft != false or .prerelease != false then
    error("Release must be published and stable")
  else .assets[] | select(.name == "checksums.txt" or (.name | startswith("obituary_"))) |
    if (.name | test("[^A-Za-z0-9._-]") | not) and .url == ($assets + (.id | tostring)) then
      [.name, .url] | @tsv
    else error("Invalid release asset name or URL") end
  end' "$stage/release.json" > "$stage/assets"

while IFS=$'\t' read -r name url; do
  get "$files/$name" --max-time 300 --retry-all-errors --retry-max-time 480 -H 'Accept: application/octet-stream' "$url"
done < "$stage/assets"

# Validate the list before asking sha256sum to open any paths from it.
awk -F '\t' '$1 != "checksums.txt" {print $1}' "$stage/assets" | sort > "$stage/expected"
[[ -s $stage/expected && -f $files/checksums.txt ]] || fail 'Missing archives or checksums.txt'
awk 'length($1) != 64 || $0 !~ /^[a-fA-F0-9]+ [ *]obituary_[A-Za-z0-9._-]+$/ {exit 1}
     {print substr($0, 67)}' "$files/checksums.txt" | sort > "$stage/listed" || fail 'Invalid checksums.txt'
cmp -s "$stage/expected" "$stage/listed" || fail 'Release assets and checksums.txt differ'
(cd "$files" && nice -n 19 sha256sum --check --strict --quiet checksums.txt)

# The tagged Git bytes are the contract, not a checkout's line endings.
get "$files/install.sh" -H 'Accept: application/vnd.github.raw' "$api/contents/install.sh?ref=$tag"
get "$files/install.ps1" -H 'Accept: application/vnd.github.raw' "$api/contents/packaging/install.ps1?ref=$tag"
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
get "$stage/github-latest.json" "$api/releases/latest"
latest=$(jq -er '.tag_name | strings | select(length > 0)' "$stage/github-latest.json")
if [[ $latest == "$tag" ]]; then
  jq '{tag_name, name, body, html_url, published_at, draft, prerelease}' "$stage/release.json" > "$stage/latest.json"
  chmod 644 -- "$stage/latest.json"
  replace_if_changed "$stage/latest.json" "$download/latest.json"
fi

printf 'Mirror %s: %d files, %s bytes, %d s\n' "$tag" "$((count + 2))" "$bytes" "$SECONDS"
