#!/usr/bin/env bash
# Usage: [ARM=A|B|C] experiment/series.sh <series-name> <agent> [<agent> ...]
# Runs one attempt per listed agent (codex or claude), in order, in
# ~/gc-rebuilds/<series-name>-<n>, all from the same source commit. Each attempt is verified,
# then reviewed by a Codex and a Claude reviewer while the next attempt runs. SOURCE names
# another source commit than HEAD.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
name=$1; shift
source=$(git -C "$here" rev-parse "${SOURCE:-HEAD}")
echo "series $name from $source: $*"
n=0
for agent in "$@"; do
  n=$((n + 1))
  run=$HOME/gc-rebuilds/$name-$n
  "$here/prepare.sh" "$run" "$source" "${ARM:-C}" || exit 1
  AGENT=$agent "$here/run.sh" "$run"
  env -u CODEX_API_KEY -u CODEX_PROXY_URL "$here/verify.sh" "$run" > "$run/verify.txt" 2>&1
  echo "$name-$n ($agent): verify exit $? ($(cat "$run/started") .. $(cat "$run/finished"))"
  if [ "${ARM:-C}" != C ]; then "$here/acceptance.sh" "$run" | sed "s/^/  /"; fi
  ("$here/review.sh" "$run" codex; "$here/review.sh" "$run" claude) &
done
wait
echo "series $name done"
