#!/bin/bash
set -euo pipefail
export LC_ALL=C
work=$(mktemp -d); trap 'rm -rf -- "$work"' EXIT
export staging="$work/release-staging"
mkdir -p "$staging/incoming" "$work/source"
dd if=/dev/urandom of="$work/source/archive" bs=1M count=8 status=none
# Replace only SSH: rsync still sends its real server command through the write-only receiver.
printf '%s\n' '#!/bin/sh' 'shift; export SSH_ORIGINAL_COMMAND="$*"; exec /usr/bin/rrsync -wo -munge "$staging"' > "$work/transport"
chmod 700 "$work/transport"
upload=(rsync -rc --delete --timeout=120 --partial -e "$work/transport" "$work/source/" fake:incoming/)
rc=0
timeout --foreground --signal=INT 3 "${upload[@]}" --bwlimit=256 >"$work/log" 2>&1 || rc=$?
[[ $rc == 124 ]] || { cat "$work/log" >&2; exit 1; }
flock -w 5 "$staging" true
partial=$(stat -c %s "$staging/incoming/archive")
size=$(stat -c %s "$work/source/archive")
(( partial > 0 && partial < size ))
"${upload[@]}" --stats --no-human-readable >"$work/stats"
matched=$(awk '/^Matched data:/ {print $3}' "$work/stats")
literal=$(awk '/^Literal data:/ {print $3}' "$work/stats")
(( matched > 0 && matched <= partial && matched + literal == size ))
cmp "$work/source/archive" "$staging/incoming/archive"
printf 'PASS: %s bytes preserved, %s reused, %s transmitted on retry\n' "$partial" "$matched" "$literal"
