#!/usr/bin/env bash
# Usage: experiment/prepare.sh <run-dir> [source-commit]
# Copies the retained files of the source commit (default HEAD) into <run-dir>/workspace as a
# fresh git repository with one commit, and records protected-file hashes in
# <run-dir>/protected.sha256.
set -euo pipefail
repo=$(git rev-parse --show-toplevel)
run=$1
source=$(git -C "$repo" rev-parse "${2:-HEAD}")
[ -e "$run" ] && { echo "$run already exists" >&2; exit 1; }
mkdir -p "$run/workspace"
run=$(cd "$run" && pwd)
git -C "$repo" archive --format=tar "$source" $(git -C "$repo" show "$source:experiment/retained.txt") | tar -x -C "$run/workspace"
echo "$source" > "$run/source-commit"
git -C "$repo" show "$source:experiment/prompt.md" > "$run/prompt.md"
(cd "$run/workspace" && find . -type f ! -path './.git/*' -print0 | sort -z | xargs -0 shasum -a 256) > "$run/protected.sha256"
(cd "$run/workspace" && git init -q && git add -A && git -c user.name=experiment -c user.email=experiment@localhost commit -qm "starter")
echo "prepared $run from $(cat "$run/source-commit")"
