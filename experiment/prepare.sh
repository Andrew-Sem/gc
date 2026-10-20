#!/usr/bin/env bash
# Usage: experiment/prepare.sh <run-dir> [source-commit] [arm]
# The arm is C (default: the retained files and prompt.md) or a control group listed in
# experiment/arms/<arm>.txt with experiment/arms/prompt-<arm>.md.
# Copies the retained files of the source commit (default HEAD) into <run-dir>/workspace as a
# fresh git repository with one commit, and records protected-file hashes in
# <run-dir>/protected.sha256.
set -euo pipefail
repo=$(git rev-parse --show-toplevel)
run=$1
source=$(git -C "$repo" rev-parse "${2:-HEAD}")
arm=${3:-C}
if [ "$arm" = C ]; then files=experiment/retained.txt; prompt=experiment/prompt.md
else files=experiment/arms/$arm.txt; prompt=experiment/arms/prompt-$arm.md; fi
[ -e "$run" ] && { echo "$run already exists" >&2; exit 1; }
mkdir -p "$run/workspace"
run=$(cd "$run" && pwd)
git -C "$repo" archive --format=tar "$source" $(git -C "$repo" show "$source:$files") | tar -x -C "$run/workspace"
echo "$source" > "$run/source-commit"
git -C "$repo" show "$source:$prompt" > "$run/prompt.md"
echo "$arm" > "$run/arm"
(cd "$run/workspace" && find . -type f ! -path './.git/*' -print0 | sort -z | xargs -0 shasum -a 256) > "$run/protected.sha256"
(cd "$run/workspace" && git init -q && git add -A && git -c user.name=experiment -c user.email=experiment@localhost commit -qm "starter")
echo "prepared $run from $(cat "$run/source-commit")"
