# Run the agent

This contract details how `gc` runs Codex for one run of the
[open pull requests contract](open-pull-requests.md). That contract decides when a run starts and
what happens to its result; this one covers the run itself.

## Why

The agent reads and edits code it did not write, and that code can contain instructions aimed at
it. So it works in a sandbox on a throwaway copy of the repository and never holds the GitHub
token. Everything that touches GitHub is done by `gc` once the agent has finished.

## Glossary

**Working copy** — a shallow clone of the repository's default branch in a temporary folder of
its own.

**Answer** — the agent's final message: JSON with the fields `slug`, `title`, `description`,
`finding`, `evidence`, `selected_candidate`, `rejected_candidates`, `change`, `validation`,
`risk`, `rollback`, and `follow_up`, as described by a schema `gc` passes to Codex. The prompt
asks for a `description` of a few sentences; the full explanation goes in the other fields.

**Slug** — a short name for what the run fixed: 1 to 60 lowercase Latin letters, digits, and
hyphens.

**Report** — a Markdown file in the repository that explains why a PR was made:
`docs/gc-reviews/<YYYYMMDD-HHMMSSZ>-<slug>.md`, with the time of the commit in UTC.

**Agent's environment** — the environment variables the Codex process receives.

## Clauses

### AGENT-1. The working copy

#### AGENT-1.1. Preparing it

AGENT-1.1.1. Each run works in its own working copy with only the latest commit of the default
branch.

AGENT-1.1.2. If the repository cannot be cloned, the run fails and Codex does not start.

AGENT-1.1.3. `gc` clones the repository with the GitHub token.

#### AGENT-1.2. Removing it

AGENT-1.2.1. The working copy is removed when the run ends, however it ended.

### AGENT-2. Starting Codex

#### AGENT-2.1. The command

AGENT-2.1.1. `gc` runs `codex exec` in the working copy with the prompt, the answer schema, and
the `workspace-write` sandbox.

AGENT-2.1.2. Network access is on inside the sandbox, so the agent can install dependencies and
run tests.

AGENT-2.1.3. If `OPENAI_BASE_URL` is set, `gc` tells Codex to use it as `openai_base_url`, so a
local proxy can stand in for OpenAI. Without it, `gc` says nothing about the address and Codex
uses OpenAI.

AGENT-2.1.4. Codex gets an empty standard input, so it does not wait for one.

AGENT-2.1.5. Each run gives Codex an empty `CODEX_HOME` of its own, so the operator's Codex
settings and login are not used. It is removed when the run ends.

#### AGENT-2.2. Secrets

AGENT-2.2.1. The agent's environment has no `GITHUB_TOKEN` and no `GC_WEBHOOK_SECRET`.

AGENT-2.2.2. The GitHub token is not stored anywhere in the working copy.

AGENT-2.2.3. The agent's environment has the value of `OPENAI_API_KEY` as `CODEX_API_KEY`, the
name `codex exec` reads its key from.

AGENT-2.2.4. The sandbox gives no write access to the shared temporary folders, `TMPDIR` and
`/tmp`, because another run's working copy can be there.

AGENT-2.2.5. Each run gives the agent a temporary folder of its own as `TMPDIR`, writable in the
sandbox and outside the working copy. It is removed when the run ends.

### AGENT-3. Finishing

#### AGENT-3.1. The agent changed files

AGENT-3.1.1. `gc` commits every changed file and pushes the commit to a new branch.

AGENT-3.1.2. The run returns that branch, the default branch as the base, the title from the
answer, and the description of AGENT-3.5.3.

AGENT-3.1.3. Each run gets a branch of its own, named `gc/` followed by a unique part.

AGENT-3.1.4. `gc` pushes the branch with the GitHub token.

#### AGENT-3.2. The agent changed nothing

AGENT-3.2.1. The run returns no change, whatever the answer says, and nothing is pushed.

#### AGENT-3.3. The run failed

AGENT-3.3.1. If Codex exits with an error, the run fails and nothing is pushed.

AGENT-3.3.2. If the agent changed files but the answer is not valid JSON or its title is empty,
the run fails and nothing is pushed.

#### AGENT-3.4. Stopping

AGENT-3.4.1. When a run is stopped, Codex is killed together with the processes it started.

#### AGENT-3.5. The report

AGENT-3.5.1. `gc`, not the agent, writes the report and commits it in the same commit as the
agent's changes. The base branch does not get it.

AGENT-3.5.2. The report has these headings in this order, each followed by a blank line and the
matching answer field: `## Finding`, `## Evidence`, `## Selected Candidate`,
`## Rejected Candidates`, `## Change`, `## Validation`, `## Risk`, `## Rollback`, `## Follow-up`.
A blank line separates the sections, and the file ends with one newline.

AGENT-3.5.3. The PR description is the answer's `description`, a blank line, and the line
`Report: <the report's path>`. The other answer fields appear only in the report.

AGENT-3.5.4. If the agent changed files but the slug is not valid, the run fails and nothing is
pushed.

AGENT-3.5.5. If the agent changed files but any answer field is missing or empty, the run fails
and nothing is pushed. A field with nothing to say says "None".

AGENT-3.5.6. If the agent changed nothing, there is no report.
