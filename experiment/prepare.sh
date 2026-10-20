#!/usr/bin/env bash
# Usage: experiment/prepare.sh <run-dir>
# Copies the retained files of the current commit into <run-dir>/workspace as a fresh git
# repository with one commit, and records protected-file hashes in <run-dir>/protected.sha256.
set -euo pipefail
repo=$(git rev-parse --show-toplevel)
run=$1
[ -e "$run" ] && { echo "$run already exists" >&2; exit 1; }
mkdir -p "$run/workspace"
run=$(cd "$run" && pwd)
git -C "$repo" archive --format=tar HEAD $(cat "$repo/experiment/retained.txt") | tar -x -C "$run/workspace"
git -C "$repo" rev-parse HEAD > "$run/source-commit"
cp "$repo/experiment/prompt.md" "$run/prompt.md"
(cd "$run/workspace" && find . -type f ! -path './.git/*' -print0 | sort -z | xargs -0 shasum -a 256) > "$run/protected.sha256"
(cd "$run/workspace" && git init -q && git add -A && git -c user.name=experiment -c user.email=experiment@localhost commit -qm "starter")
echo "prepared $run from $(cat "$run/source-commit")"
