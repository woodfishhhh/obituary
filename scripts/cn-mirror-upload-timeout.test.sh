#!/bin/bash
# Keep the sending socket open and silent, as with a lost runner behind SSH.
set -euo pipefail
export LC_ALL=C
timeout=${1:-4}
[[ $timeout =~ ^[1-9][0-9]*$ ]] || exit 2
script=$(realpath "${2:-$(dirname "$0")/cn-mirror-pull.sh}")
work=$(mktemp -d); sender=
cleanup() {
  if [[ -n $sender ]]; then kill -CONT "$sender" 2>/dev/null || :; kill "$sender" 2>/dev/null || :; wait "$sender" 2>/dev/null || :; fi
  rm -rf -- "$work"
}
trap cleanup EXIT
export work
mkdir -p "$work"/{download,release-staging/incoming,sites/site,src}
truncate -s 8M "$work/src/archive"
# Keep the production receiver, including its real .pull.lock, confined to this directory.
sed -e "s@/home/obit-deploy/release-staging@$work/release-staging@" \
    -e "s@/opt/blog-stack/sites/obituary/download@$work/download@" "$script" > "$work/publish"
grep -Fq "local staging=$work/release-staging download=$work/download" "$work/publish"
printf '%s\n' '#!/bin/bash' 'shift; export SSH_ORIGINAL_COMMAND="$*"' \
  'if [[ $mode == release ]]; then bash "$work/publish"; else /usr/bin/rrsync -wo "$work/sites"; fi' \
  'rc=$?; echo "$rc" > "$work/$mode.rc"; exit "$rc"' > "$work/transport"
chmod 700 "$work/transport"

modes=(site release)
if (( $# > 2 )); then modes=("${@:3}"); fi
for mode in "${modes[@]}"; do
  export mode
  if [[ $mode == release ]]; then
    root=$work/release-staging; dest=incoming; opts=(-rc --partial)
  else
    root=$work/sites; dest=site; opts=(-rlt --chmod=D755,F644)
  fi
  rsync "${opts[@]}" --delete --timeout="$timeout" --bwlimit=256 -e "$work/transport" \
    "$work/src/" "fake:$dest/" 2>"$work/log" & sender=$!
  until [[ -n $(find "$root/$dest" -type f -size +0c -print -quit) ]]; do
    kill -0 "$sender" || { cat "$work/log" >&2; exit 1; }; sleep 0.05
  done
  kill -STOP "$sender"; start=$SECONDS
  if flock -n "$root" true; then echo 'Missing rrsync lock' >&2; exit 1; fi
  if [[ $mode == release ]] && flock -n "$work/download/.pull.lock" true; then echo 'Missing release lock' >&2; exit 1; fi
  until [[ -s $work/$mode.rc ]]; do
    (( SECONDS - start < timeout * 2 + 30 )) || { echo "$mode receiver still running" >&2; cat "$work/log" >&2; exit 1; }; sleep 0.2
  done
  [[ $(<"$work/$mode.rc") == 30 ]] || { cat "$work/log" >&2; exit 1; }
  flock -n "$root" true
  if [[ $mode == release ]]; then
    flock -n "$work/download/.pull.lock" true
    [[ -s $root/$dest/archive ]] || { echo 'Receiver did not preserve the partial upload' >&2; ls -la "$root/$dest" >&2; exit 1; }
  fi
  printf 'PASS %s: receiver timed out and unlocked after %ds (limit %ds)\n' "$mode" "$((SECONDS-start))" "$timeout"
  kill -CONT "$sender"; wait "$sender" || :; sender=
done
