#!/usr/bin/env bash
# Usage: experiment/series.sh <series-name> <count>
# Runs <count> sequential attempts in ~/gc-rebuilds/<series-name>-<n>, verifying each.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
for n in $(seq 1 "$2"); do
  run=$HOME/gc-rebuilds/$1-$n
  "$here/prepare.sh" "$run" || exit 1
  "$here/run.sh" "$run"
  "$here/verify.sh" "$run" > "$run/verify.txt" 2>&1
  echo "$1-$n: verify exit $? ($(cat "$run/started") .. $(cat "$run/finished"))"
done
