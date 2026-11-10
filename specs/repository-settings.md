# Repository settings

This contract details the repository level of scenario 5 of the [PRD](prd.md): how often
garbage-collector opens PRs in a repository and how many may be open at once.

## Why

One repository wants a PR every hour, another once a day. The operator sets that per repository
without restarting `gc`, and the hourly decision follows it from the next wake-up.

## Glossary

**Pace** — how often a repository gets a decision at a wake-up, as a whole number of hours. By
default 1: every wake-up. A pace of 0 means never: the repository gets a decision only when the
operator presses "Run now".

**Maximum open** — the limit of the [open pull requests contract](open-pull-requests.md) for one
repository. By default 12.

**Settings** — the pace, the maximum open, merging (MERGE-1 of the
[merge contract](merge-pull-requests.md)), and where the report goes of one repository. Until the operator saves them, a
repository has the defaults.

**Interval start** — the start of the clock hour in UTC in which the settings were last saved.
A repository whose settings were never saved has none.

**Due** — a repository is due at a wake-up if it has no interval start, or if the whole hours from
its interval start to the wake-up are a multiple of its pace. A repository with a pace of 0 is
never due.

**Settings page** — `/repositories/<owner>/<name>/settings`: the settings form of one repository
in `#settings-panel`. The form submits through htmx; the response is the updated
`#settings-panel` with, if something went wrong, an error message.

## Clauses

### SET-1. Saving settings

#### SET-1.1. The settings page

SET-1.1.1. Each repository in the list on the repositories page has a "Settings" link to its
settings page.

SET-1.1.2. The settings page of a connected repository answers 200, includes the shared interface
head `ui.Head`, shows the repository's `owner/name` in `h1`, and links to `/repositories`.

SET-1.1.3. The form has the fields `pace` and `limit`, filled with the current settings, and a
"Save" button. It sends `POST /repositories/<owner>/<name>/settings`.

SET-1.1.4. The settings page of a repository that is not connected answers 404 with the text
"This repository is not connected".

SET-1.1.5. Opening the settings page writes no facts.

#### SET-1.2. Saving

SET-1.2.1. For a connected repository with valid values, `RepositorySettingsChanged` with the
repository, the pace, the maximum open, and the time of saving is written to the journal. The
answer is 200 with the updated `#settings-panel`, the new values in the form, and the text
"Saved".

SET-1.2.2. Saving the values the repository already has writes no fact and answers like SET-1.2.1.

SET-1.2.3. Saving makes no decision by itself.

SET-1.2.4. The repository name in the request is normalized like a submitted name (REPO-1.7.1).

SET-1.2.5. A save from another site is refused like a connect request (REPO-5.1).

#### SET-1.3. Invalid values

SET-1.3.1. A pace that is not a whole number from 0 to 168 writes no fact, and the block shows
the error "Pace must be a whole number of hours from 0 to 168" (response 422).

SET-1.3.2. A maximum open that is not a whole number from 1 to 100 writes no fact, and the block
shows the error "Maximum open must be a whole number from 1 to 100" (response 422).

SET-1.3.3. Spaces around a value are ignored.

SET-1.3.4. With an invalid value, the form keeps what the operator entered, so it can be
corrected.

SET-1.3.5. For a repository that is not connected, no fact is written, and the block shows the
error "This repository is not connected" (response 422).

### SET-2. Following the settings

#### SET-2.1. Pace

SET-2.1.1. At a wake-up, only connected repositories that are due get a decision. One that is not
due gets none, and no fact is written for it.

SET-2.1.2. Saving a new pace starts a new interval: a repository saved at 14:37 with a pace of 3
is due at 17:00, 20:00, and so on, whatever its earlier pace was.

SET-2.1.3. A run request (OPEN-3.2.1) makes a decision whether the repository is due or not.

SET-2.1.4. A run request does not move the interval: the repository is due at the same wake-ups
as without it.

SET-2.1.5. A repository with a pace of 0 gets no decision at any wake-up; a run request still
makes one (SET-2.1.3).

#### SET-2.2. Maximum open

SET-2.2.1. Decisions compare the open PRs with the repository's maximum open, and
`AdmissionDeclined` carries that maximum as its limit.

SET-2.2.2. Lowering the maximum below the open PRs closes nothing: decisions decline until enough
PRs close.

#### SET-2.3. Lifetime

SET-2.3.1. Settings stay when the repository is disconnected, and apply again when it is
connected again.

SET-2.3.2. The repositories list shows each repository's settings after its name, as
"every <pace> h, at most <maximum> open".

SET-2.3.3. With a pace of 0, the list shows "only on Run now, at most <maximum> open" instead.

#### SET-2.4. Where the report goes

SET-2.4.1. The settings form has a checkbox `report_in_description` labelled "Put the report in
the PR description, not in a file", checked when that is on. It is not checked for a repository
whose settings were never saved, so by default the report is a file (AGENT-3.5 of the
[agent contract](run-agent.md)).

SET-2.4.2. Saving the form writes `RepositorySettingsChanged` with `ReportInDescription` true when
the checkbox is checked and false when it is not. Saving what the repository already has writes
nothing (SET-1.2.2).

SET-2.4.3. `AdmissionApproved` carries the repository's `ReportInDescription` at that decision,
and the run it starts gives that value to the agent. A save during the run changes nothing for it.
