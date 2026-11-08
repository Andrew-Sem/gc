# Pull requests page

This contract details the first step of scenario 6 of the [PRD](prd.md): the operator sees every
open PR of ours and what the last sweep decided about it.

## Why

A PR that does not merge should say why without anyone reading the journal: it has conflicts, its
required checks have not passed, or merging is off for its repository.

## Glossary

**Open PR** — see the [open pull requests contract](open-pull-requests.md).

**Merge decision** — the latest `MergeApproved`, `MergeDeclined`, or `MergeFailed` for the PR, from
the [merge contract](merge-pull-requests.md).

**Pull requests page** — `/pulls`: a table in `#pulls` with one `tr[data-pull="<owner>/<name>#<number>"]`
per open PR. Its cells are `td[data-cell="repository"]`, `td[data-cell="number"]`,
`td[data-cell="decision"]`, and `td[data-cell="reason"]`.

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

### PULLS-2. Getting there

#### PULLS-2.1. Links

PULLS-2.1.1. The repositories page and the runs page link to the pull requests page as "Pull
requests".

PULLS-2.1.2. The pull requests page links to the repositories page and to the runs page.
