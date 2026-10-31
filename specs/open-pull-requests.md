# Open pull requests

This contract details scenario 1 of the [PRD](prd.md). This slice covers the decision to open a
PR; running the agent comes in a later slice. Merging and closing PRs is not garbage-collector's
job for now.

## Why

garbage-collector keeps a steady flow of small PRs in each connected repository, as part of an
automated pipeline that keeps cleaning up technical debt. Once an hour it decides, for each
repository, whether to open one more.

## Glossary

**Wake-up** — the start of each clock hour in UTC, when the decision is made.

**Connected repository** — see the [connect contract](connect-repository.md).

**Open PR** — a PR of the repository with `PrCreated` in the journal, unless the latest
`PrStateChanged` for it says it is closed.

**Limit** — the maximum number of open PRs in a repository: 12.

## Clauses

### OPEN-1. Deciding to open a PR

#### OPEN-1.1. Which repositories

OPEN-1.1.1. At a wake-up, each connected repository gets exactly one decision.

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
