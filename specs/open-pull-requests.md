# Open pull requests

This contract details scenario 1 of the [PRD](prd.md). Merging PRs is in the
[merge contract](merge-pull-requests.md).

## Why

garbage-collector keeps a steady flow of small PRs in each connected repository, as part of an
automated pipeline that keeps cleaning up technical debt. Once an hour it decides, for each
repository, whether to open one more.

## Glossary

**Wake-up** — the start of each clock hour in UTC, when the decision is made.

**Connected repository** — see the [connect contract](connect-repository.md).

**Open PR** — a PR of the repository with `PrCreated` in the journal, unless the latest
`PrStateChanged` for it says it is closed.

**Limit** — the maximum number of open PRs in a repository: its maximum open from the
[repository settings contract](repository-settings.md), 12 by default.

**Agent** — the program that looks for technical debt in one repository and changes its code.
It edits files only: `gc` opens the PR. When it changed something, it also gives the PR's title
and description and the branch its work was pushed to.

**Run** — one launch of the agent for one approval.

**Run request** — the operator asking for a decision about one repository now, without waiting
for the wake-up.

**Run limit** — the longest a run may last: 2 hours.

## Clauses

### OPEN-1. Deciding to open a PR

#### OPEN-1.1. Which repositories

OPEN-1.1.1. At a wake-up, each connected repository that is due (SET-2.1.1 of the
[repository settings contract](repository-settings.md)) gets exactly one decision. With the
default pace, every connected repository is due.

OPEN-1.1.2. A repository that is not connected gets no decision. This includes one that was
disconnected.

OPEN-1.1.3. Starting `gc` makes no decision by itself: a wake-up missed while it was not running
is not made up.

OPEN-1.1.4. Decisions are written in the order of the repositories list (REPO-2.2.1).

OPEN-1.1.5. While `gc` runs, a wake-up happens at the start of every clock hour.

OPEN-1.1.6. When `gc` starts, it prints "next wake-up at" followed by the time of the next
wake-up.

#### OPEN-1.2. There is room

OPEN-1.2.1. If the repository has fewer open PRs than the limit, `AdmissionApproved` with this
repository is written to the journal.

OPEN-1.2.2. An approval does not take a place: until a PR is actually created, the next wake-up
counts the same open PRs and can approve again.

#### OPEN-1.3. There is no room

OPEN-1.3.1. If the repository has as many open PRs as the limit or more, `AdmissionDeclined`
with this repository, the number of open PRs, and the limit is written to the journal.

#### OPEN-1.4. Counting open PRs

OPEN-1.4.1. Only PRs of this repository count.

OPEN-1.4.2. A PR that was closed, whether merged or not, no longer counts. A PR that was reopened
counts again.

OPEN-1.4.3. PRs opened before the repository was disconnected count again once it is connected
again.

OPEN-1.4.4. A PR without `PrCreated` is not ours: whatever `PrStateChanged` says about it, it
neither takes nor frees a place.

### OPEN-2. Running the agent

#### OPEN-2.1. Starting runs

OPEN-2.1.1. Each `AdmissionApproved` starts one run for that repository.

OPEN-2.1.2. A decline starts no run.

OPEN-2.1.3. A new run does not wait for earlier ones: if a run of the same repository is still
working, the new one works alongside it.

OPEN-2.1.4. When a run starts, `AgentRunStarted` with the run's number, the repository, and the
time is written to the journal.

OPEN-2.1.5. Run numbers start at 1 and go up by one in the order runs start, across all
repositories.

#### OPEN-2.2. The agent changed code

OPEN-2.2.1. If the agent changed code, `gc` asks GitHub to create a PR from its branch with its
title and description. Then `PrCreated` with the repository and the PR's number and
`AgentRunFinished` with the outcome "PR" and the same number are written to the journal.

OPEN-2.2.2. A PR created by a run takes a place from the next wake-up on.

OPEN-2.2.3. `gc` waits up to 60 seconds for GitHub's answer to creating a PR, because a PR that
GitHub creates after `gc` gave up would never be counted.

#### OPEN-2.3. The agent changed nothing

OPEN-2.3.1. If the agent changed nothing, `gc` does not ask GitHub to create a PR, and
`AgentRunFinished` with the outcome "no PR" is written to the journal.

OPEN-2.3.2. That `AgentRunFinished` carries the explanation the run returned, so the operator can
see why there is no PR. Facts with other outcomes carry no explanation.

#### OPEN-2.4. The run failed

OPEN-2.4.1. If the agent fails, `gc` does not ask GitHub to create a PR, and `AgentRunFinished`
with the outcome "failed" is written to the journal.

OPEN-2.4.2. If GitHub does not create the PR, no `PrCreated` is written, and `AgentRunFinished`
with the outcome "failed" is written to the journal.

OPEN-2.4.3. If the agent fails, the log has a line with the repository and the reason the agent
gave.

OPEN-2.4.4. If GitHub does not create the PR, the log has a line with the repository and the status
GitHub answered.

#### OPEN-2.5. The run limit

OPEN-2.5.1. A run that reaches the run limit is stopped, `gc` does not ask GitHub to create a PR,
and `AgentRunFinished` with the outcome "timed out" is written to the journal.

OPEN-2.5.2. A run that finishes before the run limit is not affected by it.

#### OPEN-2.6. Recording the end

OPEN-2.6.1. `AgentRunFinished` carries the run's number and the time the run ended.

OPEN-2.6.2. `AgentRunFinished` carries the tokens the run returned, whatever its outcome.

#### OPEN-2.7. Secrets in the log

OPEN-2.7.1. No GitHub credential reaches the log during a run, on the way to the clone URL or to
creating the PR, even if GitHub quotes one in a malformed answer or a broken redirect.

### OPEN-3. Running now

#### OPEN-3.1. The button

OPEN-3.1.1. Each repository in the list on the repositories page has a "Run now" button, which
sends `POST /repositories/<owner>/<name>/run`.

OPEN-3.1.2. For a connected repository, the request writes `RunRequested` with the repository
and answers 200 with the updated block of the repositories page and the text "Run requested.
Follow it on the runs page.", where "runs page" links to `/runs`.

OPEN-3.1.3. For a repository that is not connected, no fact is written, and the block shows the
error "This repository is not connected" (response 422).

OPEN-3.1.4. The repository name in the request is normalized like a submitted name (REPO-1.7.1).

OPEN-3.1.5. A run request from another site is refused like a connect request (REPO-5.1).

OPEN-3.1.6. If the repository has as many open PRs as its limit or more, the request still writes
`RunRequested`, but the block shows instead "No room: <open> of <limit> PRs are open. Merge or
close some, or raise the limit in the settings.", where "settings" links to the repository's
settings page, so the operator knows at once why no run will start.

#### OPEN-3.2. The decision

OPEN-3.2.1. Right after `RunRequested`, `gc` makes the decision of OPEN-1.2 and OPEN-1.3 for that
repository alone, and an approval starts a run as at a wake-up (OPEN-2).

OPEN-3.2.2. A `RunRequested` written before `gc` started makes no decision.
