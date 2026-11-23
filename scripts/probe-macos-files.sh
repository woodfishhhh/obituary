#!/usr/bin/env bash
# Finder resolves Trash using the login identity, not just the HOME variable.
set -eu
[[ ${GITHUB_ACTIONS:-} == true && ${RUNNER_OS:-} == macOS ]]
sw_vers
user=obittrashprobe
home=$(mktemp -d /private/tmp/obit-trash-home.XXXXXXXX)
uid=561
! id "$user" >/dev/null 2>&1 || exit 1
! id "$uid" >/dev/null 2>&1 || exit 1
sudo dscl . -create /Users/$user
sudo dscl . -create /Users/$user UniqueID "$uid"
sudo dscl . -create /Users/$user PrimaryGroupID 20
sudo dscl . -create /Users/$user NFSHomeDirectory "$home"
sudo chown "$uid":20 "$home"
run=(sudo -u "$user" env HOME="$home" OBIT_HOME="$home/data" OBIT_CONFIG_DIR="$home/config" OBIT_USER_HOME="$home" OBIT_TELEMETRY=0)
"${run[@]}" mkdir -m 700 "$home/.Trash" "$home/source" "$home/locked"
for method in direct trash; do
  if [[ $method == trash && ! -x /usr/bin/trash ]]; then echo "trash: unavailable"; continue; fi
  src="$home/source/$method"
  "${run[@]}" touch "$src" "$home/locked/$method"
  "${run[@]}" chmod 500 "$home/locked"
  move=(/bin/mv); dest=("$home/.Trash/")
  if [[ $method == trash ]]; then move=(/usr/bin/trash); dest=(); fi
  if "${run[@]}" "${move[@]}" "$home/locked/$method" ${dest[@]+"${dest[@]}"}; then echo "unexpected: locked source moved"; exit 1; fi
  "${run[@]}" test -f "$home/locked/$method"
  echo "$method: failure retained source"
  "${run[@]}" chmod 700 "$home/locked"
  if "${run[@]}" "${move[@]}" "$src" ${dest[@]+"${dest[@]}"}; then
    "${run[@]}" test ! -e "$src"
    "${run[@]}" ls -la "$home/.Trash"
    "${run[@]}" test -f "$home/.Trash/$method"
    echo "$method: present in isolated OS Trash directory"
  else
    "${run[@]}" test -f "$src"
    echo "$method: command failed; original intact"
  fi
done
# A headless secondary user has no Finder to exercise Put Back; never drive
# the logged-in user’s Finder or infer Put Back from the file’s mere presence.
if sudo launchctl print "gui/$uid" >/dev/null 2>&1; then echo "secondary GUI session present"; else echo "no isolated Finder session: Finder visibility and Put Back unverified"; fi
