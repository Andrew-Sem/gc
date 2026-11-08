#!/usr/bin/env bash
# Usage: experiment/review.sh <run-dir> codex|claude
# Runs one reviewer in a scratch directory and writes <run-dir>/review-<family>.md.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
run=$(cd "$1" && pwd)
family=$2
cp "$here/review-checklist.md" "$run/review-checklist.md"
scratch=$(mktemp -d "${TMPDIR:-/tmp}/gc-review-XXXXXX")
prompt=$(sed "s#RUN_DIR#$run#" "$here/review-prompt.md")
arm=$(cat "$run/arm" 2>/dev/null || echo C)
if [ "$arm" = D ]; then
  prompt="$prompt

This attempt belongs to control group D. The implementer got the contracts but no executable
specifications, and neither do you: there are no \`*_test.go\` files in \`workspace/specs/\`, and
the checklist items about \`just spec\` and spec constants do not apply. Judge every contract
promise in \`workspace/specs/*.md\` by reading the code and by running the application as an
operator would, with a local fake GitHub. Go through the clauses one by one.

The contracts do not name every HTML id, attribute, or data hook. A name the contracts leave open
is the implementer's choice: judge what it does, not what it is called. A name, route, field,
status, or text the contracts do state must match exactly.

End the report with these lines; the word limit does not apply to them:
\`Contract clauses: <kept> kept, <broken> broken, <unchecked> unchecked\`
\`Broken: <clause id> — <what happens instead>\`, one line per broken clause
\`Unchecked: <clause ids you could not check>\`"
elif [ "$arm" != C ]; then
  prompt="$prompt

This attempt belongs to control group $arm. The implementer did not see the executable
specifications; \`prompt.md\` shows what it was told and \`protected.sha256\` lists the files it
had. \`acceptance.txt\` and \`acceptance.log\` show the hidden executable specifications run
afterwards on a copy in \`acceptance/\`. Use the contracts in \`acceptance/specs/*.md\` as the
yardstick. Mark a broken promise the implementer could not know from its inputs (for example
an exact error text or status that only the contracts state) as \`unknowable\` instead of
\`fail\`. In that case the verdict is success if nothing else fails. Add a final line
\`Contract promises: <kept> kept, <unknowable> unknowable, <broken> broken\`."
fi
case $family in
codex)
  codex exec --skip-git-repo-check --ephemeral --ignore-user-config --disable memories \
    ${CODEX_PROXY_URL:+-c "openai_base_url=\"$CODEX_PROXY_URL\""} \
    -m gpt-6.1-sol -c model_reasoning_effort=medium \
    -s workspace-write -c sandbox_workspace_write.network_access=true \
    --add-dir "$(go env GOCACHE)" --add-dir "$(go env GOMODCACHE)" \
    -C "$scratch" --json -o "$run/review-codex.md" \
    "$prompt" < /dev/null > "$run/review-codex.trace.jsonl" 2> "$run/review-codex.stderr.txt"
  ;;
claude)
  (cd "$scratch" && env -u CODEX_API_KEY -u CODEX_PROXY_URL claude -p "$prompt" \
    --model claude-opus-5-5 --effort medium \
    --setting-sources project --strict-mcp-config --no-session-persistence \
    --dangerously-skip-permissions --output-format stream-json --verbose \
    < /dev/null > "$run/review-claude.trace.jsonl" 2> "$run/review-claude.stderr.txt")
  python3 -I -c 'import json,sys
report = ""
for l in open(sys.argv[1]):
    try: e = json.loads(l)
    except ValueError: continue
    for b in e.get("message", {}).get("content", []) if e.get("type") == "assistant" else []:
        text = b.get("text", "") if b.get("type") == "text" else ""
        if "Verdict:" in text: report = text[text.index("Verdict:"):]
open(sys.argv[2], "w").write(report)' "$run/review-claude.trace.jsonl" "$run/review-claude.md"
  ;;
*) echo "unknown reviewer $family" >&2; exit 2 ;;
esac
rm -rf "$scratch"
echo "$(basename "$run") $family: $(head -1 "$run/review-$family.md" 2>/dev/null)"
