#!/usr/bin/env bash
# Download a versioned release and verify it before extracting any bytes.
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: bash install.sh [options]
  --version VERSION   Release to install (default: the latest release)
  --install-dir DIR   Install obit and obitd (default: ~/.local/bin)
  --architecture ARCH  amd64 or arm64 (default: native architecture)
  --mirror cn        Download directly from the mainland China mirror
  --base-url URL     HTTPS release directory, or loopback HTTP for snapshots
  --dry-run          Print the source and destination without writing files
The script never edits PATH or shell startup files. Rerun it to upgrade.
USAGE
}

fail() { printf 'Error: %s\n' "$*" >&2; exit 1; }
version='' architecture='' mirror='' base_url='' dry_run=false
install_dir="${HOME:?HOME is required}/.local/bin"
while [ "$#" -gt 0 ]; do
  case "$1" in
    --version|--architecture|--install-dir|--mirror|--base-url)
      [ "$#" -ge 2 ] && [ -n "$2" ] || fail "$1 requires a value"
      case "$1" in
        --version) version=${2#v} ;;
        --architecture) architecture=$2 ;;
        --install-dir) install_dir=$2 ;;
        --mirror) mirror=$2 ;;
        --base-url) base_url=$2 ;;
      esac
      shift 2 ;;
    --dry-run) dry_run=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) fail "Unknown option: $1" ;;
  esac
done
case "$mirror" in ''|cn) ;; *) fail 'Mirror must be cn' ;; esac
[ -z "$mirror" ] || [ -z "$base_url" ] || fail 'Use either --mirror or --base-url'
# GitHub's API and the mirror's latest.json carry the same release record.
latest_version() {
  curl -fsSL --max-time 10 "$1" | grep -o '"tag_name": *"v[^"]*"' | head -n 1 | sed 's/.*"v//; s/"$//'
}
if [ -z "$version" ]; then
  [ -z "$base_url" ] || fail '--base-url needs --version'
  mirror_latest=https://cn.obituary.woodfish.site/download/latest.json
  if [ "$mirror" = cn ]; then version=$(latest_version "$mirror_latest") || version=''
  else version=$(latest_version https://api.github.com/repos/woodfishhhh/obituary/releases/latest) || version=$(latest_version "$mirror_latest") || version=''
  fi
  [ -n "$version" ] || fail 'Could not look up the latest release; pass --version'
  printf 'Latest release: %s\n' "$version"
fi
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.+-]+)?$ ]] || fail 'A valid --version is required'
case "$(uname -s)" in
  Darwin) platform=darwin ;;
  Linux) platform=linux ;;
  *) fail 'Use install.ps1 on Windows; install.sh supports macOS and Linux' ;;
esac
if [ -z "$architecture" ]; then
  case "$(uname -m)" in
    x86_64|amd64) architecture=amd64 ;;
    arm64|aarch64) architecture=arm64 ;;
    *) fail 'Unsupported architecture; expected x86_64 or arm64' ;;
  esac
fi
case "$architecture" in amd64|arm64) ;; *) fail 'Architecture must be amd64 or arm64' ;; esac
mirror_url="https://cn.obituary.woodfish.site/download/v$version"
fallback='' download_source=custom
if [ -z "$base_url" ]; then
  download_source=github
  if [ "$mirror" = cn ]; then base_url=$mirror_url download_source=mirror
  else
    base_url="https://github.com/woodfishhhh/obituary/releases/download/v$version"
    fallback=$mirror_url
  fi
fi
if ! [[ "$base_url" =~ ^https://[^/@?#[:space:]]+(/[^?#[:space:]]*)?$ || "$base_url" =~ ^http://(localhost|127\.0\.0\.1)(:[0-9]+)?(/[^?#[:space:]]*)?$ ]]; then
  fail 'Base URL must use HTTPS (or loopback HTTP for local snapshots), without credentials, query or fragment'
fi
base_url=${base_url%/}
archive="obituary_${version}_${platform}_${architecture}.tar.gz"
printf 'Install %s from %s into %s\n' "$archive" "$base_url" "$install_dir"
if "$dry_run"; then exit 0; fi
for tool in curl tar mktemp; do command -v "$tool" >/dev/null || fail "$tool is required"; done
if command -v sha256sum >/dev/null; then hash=(sha256sum)
elif command -v shasum >/dev/null; then hash=(shasum -a 256)
else fail 'sha256sum or shasum is required'; fi

work=$(mktemp -d "${TMPDIR:-/tmp}/obituary-install.XXXXXX")
transaction=''
cleanup() {
  # Only our exclusive scratch files are removed; failed transactions stay recoverable.
  rm -f -- "$work/checksums.txt" "$work/archive.tar.gz" "$work"/archive.tar.gz.part[0-3] "$work/entries" "$work/obit" "$work/obitd"
  rmdir -- "$work"
}
trap cleanup EXIT
fetch() { curl --fail --silent --show-error --location --proto '=https,http' --proto-redir '=https,http' "$@"; }
# Some routes cap each connection (about 4 Mbit/s into mainland China), so a
# large archive comes as four byte ranges at once. A server that ignores
# ranges, a failed part or a wrong total size falls back to one stream; the
# SHA-256 check below covers either way.
fetch_parallel() { # url destination
  local url=$1 dest=$2 size i ok=true pids=()
  size=$(fetch --head --max-time 15 "$url" | tr -d '\r' | awk 'tolower($1) == "content-length:" { n = $2 } END { print n }') || size=''
  if [[ "$size" =~ ^[0-9]+$ ]] && [ "$size" -ge 4194304 ]; then
    for i in 0 1 2 3; do
      fetch --range "$(( size * i / 4 ))-$(( size * (i + 1) / 4 - 1 ))" "$url" -o "$dest.part$i" &
      pids+=("$!")
    done
    for i in "${pids[@]}"; do wait "$i" || ok=false; done
    if "$ok" && cat "$dest.part0" "$dest.part1" "$dest.part2" "$dest.part3" > "$dest" && [ "$(wc -c < "$dest" | tr -d ' ')" = "$size" ]; then
      rm -f -- "$dest".part[0-3]
      return 0
    fi
    rm -f -- "$dest" "$dest".part[0-3]
    printf 'Parallel download did not work; using one connection\n'
  fi
  fetch "$url" -o "$dest"
}
if [ -n "$fallback" ]; then
  if fetch --max-time 10 "$base_url/checksums.txt" -o "$work/checksums.txt"; then :
  else
    status=$?
    case "$status" in
      5|6|7|28) base_url=$fallback download_source=mirror; printf 'GitHub did not answer; using %s\n' "$base_url" ;;
      *) fail "Checksum download failed (curl $status)" ;;
    esac
    fetch "$base_url/checksums.txt" -o "$work/checksums.txt"
  fi
else fetch "$base_url/checksums.txt" -o "$work/checksums.txt"; fi
fetch_parallel "$base_url/$archive" "$work/archive.tar.gz"
expected=$(awk -v file="$archive" '
  { sub(/\r$/, ""); name=$2; sub(/^\*/, "", name) }
  name == file { count++; if (length($1) != 64 || $1 ~ /[^0-9a-fA-F]/ || NF != 2) bad=1; value=tolower($1) }
  END { if (count != 1 || bad) exit 1; print value }
' "$work/checksums.txt") || fail "Expected exactly one SHA-256 entry for $archive"
actual=$("${hash[@]}" "$work/archive.tar.gz")
[ "${actual%% *}" = "$expected" ] || fail "SHA-256 mismatch for $archive"
printf 'SHA-256 verified: %s\n' "$archive"
tar -tzf "$work/archive.tar.gz" > "$work/entries"
awk '
  $0 != "obit" && $0 != "obitd" && $0 != "LICENSE" && $0 != "README.txt" { exit 1 }
  ++seen[$0] > 1 { exit 1 }
  END { if (seen["obit"] != 1 || seen["obitd"] != 1) exit 1 }
' "$work/entries" || fail 'Unexpected or missing files in release archive'
for name in obit obitd; do
  # Stream only the named payload: archive paths and link targets are never extracted.
  tar -xOzf "$work/archive.tar.gz" "$name" > "$work/$name"
  [ -s "$work/$name" ] || fail "Release archive is missing executable content for $name"
  chmod 755 "$work/$name"
done
mkdir -p -- "$install_dir"
install_dir=$(cd -- "$install_dir" && pwd -P)
for name in obit obitd .obituary-install.json; do
  [ ! -L "$install_dir/$name" ] || fail "Refusing symlink: $install_dir/$name"
  [ ! -e "$install_dir/$name" ] || [ -f "$install_dir/$name" ] || fail "Not a file: $install_dir/$name"
done
transaction=$(mktemp -d "$install_dir/.obituary-install.XXXXXX")
for name in obit obitd; do cp -- "$work/$name" "$transaction/$name.new"; done
for name in obit obitd; do
  if [ -e "$install_dir/$name" ]; then cp -p -- "$install_dir/$name" "$transaction/$name.old"; fi
done
for name in obit obitd; do
  if ! mv -f -- "$transaction/$name.new" "$install_dir/$name"; then
    fail "Install failed; previous binaries are retained in $transaction; restore them before retrying"
  fi
done
# The receipt lets `obit update` name this script and its download source.
printf '{"owner":"obituary-install-sh-v1","source":"%s"}\n' "$download_source" > "$transaction/receipt"
mv -f -- "$transaction/receipt" "$install_dir/.obituary-install.json"
rm -f -- "$transaction/obit.old" "$transaction/obitd.old"
rmdir -- "$transaction"
printf 'Installed obit and obitd in %s\n' "$install_dir"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) printf 'Add this directory to PATH in your shell configuration:\n  export PATH=%q:"$PATH"\n' "$install_dir" ;;
esac
printf 'Next: obit --version; obit init --dry-run; obit daemon enable --dry-run; obit doctor\n'
