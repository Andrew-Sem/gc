# Pull requests page

This contract details scenario 6 of the [PRD](prd.md): the operator sees every open PR of ours,
what the last sweep decided about it, and where its required checks stand now.

## Why

A PR that does not merge should say why without anyone reading the journal: it has conflicts, its
required checks have not passed, or merging is off for its repository.

## Glossary

**Open PR** — see the [open pull requests contract](open-pull-requests.md).

**Merge decision** — the latest `MergeApproved`, `MergeDeclined`, or `MergeFailed` for the PR, from
the [merge contract](merge-pull-requests.md).

**Pull requests page** — `/pulls`: a table in `#pulls` with one `tr[data-pull="<owner>/<name>#<number>"]`
per open PR. Its cells are `td[data-cell="repository"]`, `td[data-cell="number"]`,
`td[data-cell="checks"]`, `td[data-cell="decision"]`, and `td[data-cell="reason"]`.

**Pull requests panel** — `#pulls-panel`: the part of the page that holds the table in `#pulls` and
the "No open PRs" text.

**Current head** — the head commit in the latest `PrStateChanged` of the PR.

**Full observation** — the latest `RequiredChecksObserved` of the PR whose head is the current
head (MERGE-2.3 of the [merge contract](merge-pull-requests.md)).

**Attempt** — a check run of the PR's current head, in its latest `CheckRunStateChanged`, whether
a sweep or a webhook wrote it, and whether before or after the full observation. An attempt
matches a required check if its name is the required name and, when the required check names an
app, its app is that app.

**Passing conclusion** — `success`, `neutral`, or `skipped`.

## Clauses

### PULLS-1. The page

#### PULLS-1.1. Rows

PULLS-1.1.1. The page shows one row for each open PR of each connected repository.

PULLS-1.1.2. A PR of a repository that is not connected has no row, and neither has a PR that is
not open.

PULLS-1.1.3. Rows go from the most recently created PR to the oldest, by the order of `PrCreated`
in the journal.

PULLS-1.1.4. With no open PRs, the page shows "No open PRs" and no rows.

PULLS-1.1.5. The page includes the shared interface head `ui.Head`.

PULLS-1.1.6. Opening the page writes no facts.

#### PULLS-1.2. What a row shows

PULLS-1.2.1. The repository cell shows the repository's `owner/name`.

PULLS-1.2.2. The number cell shows "#<number>", a link to
`https://github.com/<owner>/<name>/pull/<number>`.

#### PULLS-1.3. The decision

PULLS-1.3.1. With merging off for the repository, the decision is "merging is off" and the reason
is empty, whatever was decided before it was turned off.

PULLS-1.3.2. With merging on and no merge decision yet, the decision is "no sweep yet" and the
reason is empty.

PULLS-1.3.3. With the merge decision `MergeDeclined`, the decision is "not merged" and the reason
is its reason.

PULLS-1.3.4. With the merge decision `MergeFailed`, the decision is "merge failed" and the reason
is its reason.

PULLS-1.3.5. With the merge decision `MergeApproved`, the decision is "merging" and the reason is
empty: GitHub has not yet told `gc` that the PR closed.

PULLS-1.3.6. Only the latest merge decision of the PR counts: a PR declined at one sweep and
approved at the next shows "merging".

PULLS-1.3.7. A reason is text: markup in it is shown, not run.

#### PULLS-1.4. The checks

PULLS-1.4.1. The checks cell is worked out each time the page or the panel is shown, from the full
observation and the attempts, so a check run state from a webhook changes it without waiting for
the next sweep.

PULLS-1.4.2. Without a current head or a full observation, the checks cell reads "unknown".

PULLS-1.4.3. For each required check of the full observation, the attempt that counts is the
matching attempt that started last; with equal starts, the one with the larger id.

PULLS-1.4.4. A full observation without required checks reads "no required checks".

PULLS-1.4.5. When a counted attempt is completed with a conclusion that is not passing, the cell
reads "failed: " followed by the names of those required checks.

PULLS-1.4.6. Otherwise, when a required check has no matching attempt, or its counted attempt is
not completed, the cell reads "pending: " followed by the names of those required checks.

PULLS-1.4.7. Otherwise every counted attempt has a passing conclusion, and the cell reads
"passed".

PULLS-1.4.8. Names in the cell are separated by ", " and follow the order of the full
observation.

### PULLS-2. Getting there

#### PULLS-2.1. Links

PULLS-2.1.1. The repositories page and the runs page link to the pull requests page as "Pull
requests".

PULLS-2.1.2. The pull requests page links to the repositories page and to the runs page.

### PULLS-3. Live updates

#### PULLS-3.1. The panel

PULLS-3.1.1. `GET /pulls/panel` answers 200 with the pull requests panel alone, without `ui.Head`,
showing the same rows or the same "No open PRs" as the page.

PULLS-3.1.2. On the page and in the `GET /pulls/panel` answer, the panel has
`hx-get="/pulls/panel"`, `hx-trigger="pulls-changed from:body, every 60s"`, and
`hx-swap="outerHTML"`.

PULLS-3.1.3. `GET /pulls/panel` writes no facts.

#### PULLS-3.2. The page

PULLS-3.2.1. The pull requests page opens `new EventSource("/runs/events")`, the event stream of
the [run history contract](run-history.md), which signals every new fact.

PULLS-3.2.2. On each signal, and each time the stream connects, the page fires `pulls-changed` on
`body`.

