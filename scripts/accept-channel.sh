#!/usr/bin/env bash
set -euxo pipefail

root=$(cd "$RUNNER_TEMP" && pwd -P)/channel-work
if [[ "$RUNNER_OS" == Windows ]]; then
  root=$(cygpath -am "$root")
fi
export OBIT_HOME="$root/data" OBIT_CONFIG_DIR="$root/config"
fixture="$root/sample.txt"

case "$1" in
  exercise)
    command -v obit
    command -v obitd
    version=$(obit --version)
    printf '%s\n' "$version"
    [[ "$version" == "obit version $EXPECTED_VERSION" ]]
    obitd --help
    mkdir -p "$root"
    printf 'channel acceptance fixture\n' > "$fixture"
    cp "$fixture" "$root/expected.txt"
    obit mark "$fixture" 1d --no-daemon
    obit list --json | tee "$RUNNER_TEMP/channel-evidence/list.json" |
      jq -e 'any(.[]; .path | endswith("sample.txt"))'
    obit bury "$fixture"
    test ! -e "$fixture"
    buried=$(obit graveyard --json)
    printf '%s\n' "$buried"
    id=$(jq -er '.burials[] | select(.path | endswith("sample.txt")) | .id' <<< "$buried")
    obit resurrect "$id"
    cmp "$fixture" "$root/expected.txt"

    # Leave a real payload in the graveyard to check uninstall, not just an empty directory.
    obit bury "$fixture"
    test ! -e "$fixture"
    find "$OBIT_HOME/graveyard" -type f -name sample.txt -print | grep -F sample.txt
    cp "$OBIT_HOME/store.json" "$root/store-before.json"
    cp -R "$OBIT_HOME/graveyard" "$root/graveyard-before"
    ;;
  verify)
    for executable in obit obitd; do
      if command -v "$executable"; then
        echo "Still on PATH after uninstall: $executable" >&2
        exit 1
      fi
    done
    cmp "$root/store-before.json" "$OBIT_HOME/store.json"
    diff -r "$root/graveyard-before" "$OBIT_HOME/graveyard"
    echo "Uninstalled both executables; ledger and non-empty graveyard are unchanged."
    ;;
  *) exit 2 ;;
esac
