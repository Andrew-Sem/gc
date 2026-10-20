#!/usr/bin/env bash
# Usage: experiment/acceptance.sh <run-dir>
# Runs the hidden executable specifications against a copy of the attempt: copies the workspace
# to <run-dir>/acceptance, overlays the full retained set of the source commit, runs the specs,
# and writes <run-dir>/acceptance.txt.
set -uo pipefail
repo=$(git rev-parse --show-toplevel)
run=$(cd "$1" && pwd)
source=$(cat "$run/source-commit")
acc=$run/acceptance
rm -rf "$acc"
cp -R "$run/workspace" "$acc"
git -C "$repo" archive --format=tar "$source" $(git -C "$repo" show "$source:experiment/retained.txt") | tar -x -C "$acc"
(cd "$acc" && go tool templ generate >/dev/null 2>&1; go test -count=1 -v ./specs/ > "$run/acceptance.log" 2>&1)
{
  if grep -qE '\[build failed\]|\[setup failed\]' "$run/acceptance.log"; then
    echo "build: FAILED"
    grep -E '^\S+\.go:[0-9]+' "$run/acceptance.log" | head -5
  else
    echo "build: ok"
  fi
  pass=$(grep -cE '^\s+--- PASS' "$run/acceptance.log")
  fail=$(grep -cE '^\s+--- FAIL' "$run/acceptance.log")
  echo "scenarios: $pass passed, $fail failed"
  grep -E 'feature.go:[0-9]+: (failing|uncovered):' "$run/acceptance.log" | sed 's/^ *feature.go:[0-9]*: //'
} > "$run/acceptance.txt"
cat "$run/acceptance.txt"
