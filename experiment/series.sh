#!/usr/bin/env bash
# Usage: experiment/series.sh <series-name> <agent> [<agent> ...]
# Runs one attempt per listed agent (codex or claude), in order, in
# ~/gc-rebuilds/<series-name>-<n>, all from the same source commit. Each attempt is verified,
# then reviewed by a Codex and a Claude reviewer while the next attempt runs.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
name=$1; shift
source=$(git -C "$here" rev-parse HEAD)
echo "series $name from $source: $*"
n=0
for agent in "$@"; do
  n=$((n + 1))
  run=$HOME/gc-rebuilds/$name-$n
  "$here/prepare.sh" "$run" "$source" || exit 1
  AGENT=$agent "$here/run.sh" "$run"
  "$here/verify.sh" "$run" > "$run/verify.txt" 2>&1
  echo "$name-$n ($agent): verify exit $? ($(cat "$run/started") .. $(cat "$run/finished"))"
  ("$here/review.sh" "$run" codex; "$here/review.sh" "$run" claude) &
done
wait
echo "series $name done"
