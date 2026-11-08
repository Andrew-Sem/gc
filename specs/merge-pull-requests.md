# Merge pull requests

This contract details the merging half of scenario 2 of the [PRD](prd.md). Closing PRs by age is
not here yet.

## Why

A PR that passed its required checks should not wait for a person: garbage-collector merges it
itself. Merging is a choice of the operator per repository, off until they turn it on. The same
pass also reads the state of every open PR from GitHub, so `gc` sees closed PRs without webhooks
and frees their places.

## Glossary

**Sweep** — a pass over the open PRs of every connected repository, at the wake-ups of 00:00,
06:00, 12:00, and 18:00 UTC.

**Open PR** — see the [open pull requests contract](open-pull-requests.md).

**Merging** — a repository setting, next to the pace and the maximum open of the
[repository settings contract](repository-settings.md). Off by default.

**PR read** — `GET /repos/<owner>/<name>/pulls/<number>` with an installation token.

**Required checks** — the status checks that the branch protection of the PR's base branch
requires, as GitHub gives them in `protection.required_status_checks` of
`GET /repos/<owner>/<name>/branches/<base>`: its `contexts` and `checks`.

**Merge state** — GitHub's `mergeable` and `mergeable_state` of the PR read.

## Clauses

### MERGE-1. Turning merging on

#### MERGE-1.1. The setting

MERGE-1.1.1. The settings form has a checkbox `merge` labelled "Merge green PRs", checked when
merging is on. It is not checked for a repository whose settings were never saved.

MERGE-1.1.2. Saving the form writes `RepositorySettingsChanged` with `Merge` true when the
checkbox is checked and false when it is not. Saving the merging the repository already has,
together with its pace and maximum, writes nothing (SET-1.2.2).

MERGE-1.1.3. When merging is on, the repositories list shows ", merges green PRs" right after the
repository's settings (SET-2.3.2). When it is off, the list shows nothing more.

### MERGE-2. The sweep

#### MERGE-2.1. When

MERGE-2.1.1. A sweep happens at the wake-ups of 00:00, 06:00, 12:00, and 18:00 UTC, before that
wake-up's decisions of the open pull requests contract, so that a PR merged or closed in the sweep
no longer takes a place at that decision.

MERGE-2.1.2. Starting `gc` makes no sweep by itself: a sweep missed while it was not running is
not made up.

MERGE-2.1.3. Each connected repository is swept, whatever its pace. A repository that is not
connected is not.

#### MERGE-2.2. Reading the PRs

MERGE-2.2.1. The sweep reads each open PR of the repository, whether merging is on or not.

MERGE-2.2.2. If the PR read differs from the latest known state of the PR, `PrStateChanged` with
it is written to the journal, as for a webhook (HOOK-2 of the
[webhook contract](receive-webhooks.md)). A PR closed on GitHub then no longer counts as open.

MERGE-2.2.3. A PR read that fails, or that lacks a field of `PrStateChanged`, writes no
`PrStateChanged`. With merging on, it writes `MergeDeclined` with the reason "GitHub did not give
the PR".

MERGE-2.2.4. A closed PR gets no merge decision.

### MERGE-3. Deciding to merge

#### MERGE-3.1. The gate

MERGE-3.1.1. With merging on, each PR that the read shows open gets exactly one decision:
`MergeApproved` or `MergeDeclined`. With merging off, no PR gets one.

MERGE-3.1.2. The conditions are checked in this order, and the first that fails gives the reason
of `MergeDeclined` with the repository and the PR's number:

1. the PR is not a draft — "draft";
2. the base branch has at least one required check — "no required checks";
3. GitHub has computed the merge state, `mergeable` is not null — "GitHub is still checking";
4. `mergeable` is true — "conflicts";
5. `mergeable_state` is not "blocked" — "required checks have not passed";
6. `mergeable_state` is not "behind" — "behind the base branch";
7. `mergeable_state` is "clean", "unstable", or "has_hooks" — "not mergeable: " followed by the
   state.

MERGE-3.1.3. If the base branch cannot be read, the reason is "GitHub did not give the base
branch".

MERGE-3.1.4. If every condition holds, `MergeApproved` with the repository, the PR's number, and
the head SHA of the read is written.

#### MERGE-3.2. Merging

MERGE-3.2.1. After `MergeApproved`, `gc` asks GitHub to merge the PR with
`PUT /repos/<owner>/<name>/pulls/<number>/merge`, `merge_method` "squash", and `sha` the head SHA
of the approval, so that a head pushed after the read is not merged.

MERGE-3.2.2. When GitHub answers 200, `gc` reads the PR again, and a state that differs from the
known one is written as `PrStateChanged` (MERGE-2.2.2), so the merged PR no longer counts as open.

MERGE-3.2.3. When GitHub answers anything else, or does not answer within 60 seconds,
`MergeFailed` with the repository, the PR's number, and the reason is written. The reason is
"GitHub answered " followed by the status, or "GitHub did not answer".

MERGE-3.2.4. A reason is only one of the texts of this contract, with at most GitHub's status in
it: nothing GitHub writes in an answer, credentials included, reaches the journal.
