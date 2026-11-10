# Run the agent

This contract details how `gc` runs Codex for one run of the
[open pull requests contract](open-pull-requests.md). That contract decides when a run starts and
what happens to its result; this one covers the run itself.

## Why

The agent reads and edits code it did not write, and that code can contain instructions aimed at
it. So it works in a sandbox on a throwaway copy of the repository and never holds a GitHub
credential. Everything that touches GitHub is done by `gc` once the agent has finished.

## Glossary

**Installation token** — a token of the GitHub App that `gc` asks GitHub for, signing the request
with the app's private key. GitHub makes it valid for an hour.

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

AGENT-1.1.3. `gc` clones the repository with an installation token.

AGENT-1.1.4. Every installation token `gc` asks for is limited to the run's repository and to
writing its contents and pull requests.

#### AGENT-1.2. Removing it

AGENT-1.2.1. The working copy is removed when the run ends, however it ended.

AGENT-1.2.2. Removal does not depend on what the agent left: a folder it made read-only or closed
to everyone goes too, in the working copy, in `CODEX_HOME`, and in its temporary folder.

### AGENT-2. Starting Codex

#### AGENT-2.1. The command

AGENT-2.1.1. `gc` runs `codex exec` in the working copy with the prompt, the answer schema, and
the `workspace-write` sandbox.

AGENT-2.1.2. Network access is on inside the sandbox, so the agent can install dependencies and
run tests.

AGENT-2.1.3. If the run's task has an OpenAI Base URL (INSTANCE-2.1 of the
[instance settings contract](instance-settings.md)), `gc` tells Codex to use it as
`openai_base_url`, so a local proxy can stand in for OpenAI. With an empty one, `gc` says nothing
about the address and Codex uses OpenAI, even if `OPENAI_BASE_URL` is set in `gc`'s environment.

AGENT-2.1.4. Codex gets an empty standard input, so it does not wait for one.

AGENT-2.1.5. Each run gives Codex an empty `CODEX_HOME` of its own, so the operator's Codex
settings and login are not used. It is removed when the run ends.

AGENT-2.1.6. The prompt tells the agent not to change anything in `.github` or `.gitmodules` and
not to create a git repository inside the working copy, because a run that does fails and its
work is lost (AGENT-4.2.1, AGENT-4.3.1, AGENT-4.3.2).

AGENT-2.1.7. Before Codex starts, `gc` asks GitHub for the repository's open PRs whose branch
starts with `gc/`. The prompt lists the title and the changed files of each and asks the agent
not to fix the same problem again, so one finding does not become a PR every hour. Other open PRs
are not listed, and with none of ours open the prompt has no such list.

AGENT-2.1.8. If GitHub cannot list the open PRs, the run fails and Codex does not start.

AGENT-2.1.9. Each request of that listing, the list itself and the files of each PR, waits up to
5 seconds for GitHub on its own, so a repository with many of our PRs open does not fail the run.

#### AGENT-2.2. Secrets

AGENT-2.2.1. The agent's environment has no `GITHUB_APP_PRIVATE_KEY_FILE` and no
`GC_WEBHOOK_SECRET`.

AGENT-2.2.2. No installation token is stored anywhere in the working copy.

AGENT-2.2.3. The agent's environment has the value of `OPENAI_API_KEY` as `CODEX_API_KEY`, the
name `codex exec` reads its key from.

AGENT-2.2.4. The sandbox gives no write access to the shared temporary folders, `TMPDIR` and
`/tmp`, because another run's working copy can be there.

AGENT-2.2.5. Each run gives the agent a temporary folder of its own as `TMPDIR`, writable in the
sandbox and outside the working copy. It is removed when the run ends.

AGENT-2.2.6. If the clone URL that GitHub gives holds credentials, the run fails and Codex does not
start, because `git` would store that URL in the working copy.

### AGENT-3. Finishing

#### AGENT-3.1. The agent changed files

AGENT-3.1.1. `gc` commits every changed file and pushes the commit to a new branch.

AGENT-3.1.2. The run returns that branch, the default branch as the base, the title from the
answer, and the description of AGENT-3.5.3.

AGENT-3.1.3. Each run gets a branch of its own, named `gc/` followed by a unique part.

AGENT-3.1.4. `gc` pushes the branch with an installation token.

AGENT-3.1.5. `gc` asks for that token after the agent has finished, so a run longer than the hour
of the token it cloned with still pushes.

AGENT-3.1.6. The commit's author and committer are the app's bot user, `<app>[bot]` with the
email `<user id>+<app>[bot]@users.noreply.github.com`, so GitHub shows the commit as the app's.
If `gc` cannot learn them from GitHub, the run fails and nothing is pushed.

#### AGENT-3.2. The agent changed nothing

AGENT-3.2.1. The run returns no change, whatever the answer says, and nothing is pushed.

AGENT-3.2.2. The run returns an explanation made of the answer's `finding`,
`selected_candidate`, and `rejected_candidates`, each under its heading and laid out as in the
report: `## Finding`, `## Selected Candidate`, `## Rejected Candidates`. The explanation has no
newline at the end.

AGENT-3.2.3. A field that is missing or empty is left out of the explanation together with its
heading. If the answer is not valid JSON or no field is left, the explanation is empty; the run
still returns no change and does not fail.

AGENT-3.2.4. An explanation longer than 4000 bytes keeps its first 4000 bytes, cut back to the
last whole character, so the journal does not grow with whatever the agent writes.

AGENT-3.2.5. The explanation has no installation token and no OpenAI key, even if the agent wrote
them in its answer: each is replaced by `[secret]`, and the cut of AGENT-3.2.4 comes after that.

#### AGENT-3.3. The run failed

AGENT-3.3.1. If Codex exits with an error, the run fails and nothing is pushed.

AGENT-3.3.2. If the agent changed files but the answer is not valid JSON or its title is empty,
the run fails and nothing is pushed.

AGENT-3.3.3. The reason a failed Codex gives is the end of its error output: the last 1000 bytes,
with line breaks turned into spaces, so that it stays one line in the log.

AGENT-3.3.4. That reason has no installation token and no OpenAI key, even if Codex printed
them.

#### AGENT-3.4. Stopping

AGENT-3.4.1. When a run is stopped, Codex is killed together with the processes it started.

AGENT-3.4.2. A stopped run ends within 30 seconds, even if one of its `git` commands is waiting on
a connection that never answers.

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

AGENT-3.5.7. The report reaches the branch even if the repository's ignore rules cover it.

AGENT-3.5.8. Line breaks at the end of an answer field are dropped, so they do not change the
layout of AGENT-3.5.2.

AGENT-3.5.9. If a file or a link is already at the report's path, or a folder on the way to it is
a link, the run fails and nothing is pushed. `gc` writes nothing through a link.

#### AGENT-3.6. Tokens

AGENT-3.6.1. `gc` runs Codex with `--json`, so Codex prints its events to standard output.

AGENT-3.6.2. The run returns the tokens the agent used: the sums of `input_tokens`,
`cached_input_tokens`, and `output_tokens` over every `turn.completed` event Codex printed.

AGENT-3.6.3. A run that fails after Codex printed such events returns those tokens with its
failure.

AGENT-3.6.4. Other events, and output that is not JSON, add nothing and do not fail the run.
Without any `turn.completed` event, the tokens are zero.

### AGENT-4. What the agent leaves behind

The agent controls the working copy, and `gc` then runs `git` on it outside the sandbox with an
installation token. Nothing the agent left may change what those commands do.

#### AGENT-4.1. Git settings

AGENT-4.1.1. Settings and hooks written to the working copy's `.git` after the clone have no
effect on `gc`'s own `git` commands. The run goes on as if they were not there: no command they
name runs, and the branch is pushed to the repository that was cloned.

#### AGENT-4.2. Nested repositories

AGENT-4.2.1. If the working copy has a `.git` anywhere below its root after the agent finishes,
the run fails and nothing is pushed.

AGENT-4.2.2. No command named in the settings of a nested repository runs.

#### AGENT-4.3. Paths the agent may not change

AGENT-4.3.1. If the change touches anything under `.github/`, the run fails and nothing is pushed,
because pushing a workflow can run it with the repository's secrets before anyone reviews the PR.

AGENT-4.3.2. If the change touches `.gitmodules`, the run fails and nothing is pushed.
