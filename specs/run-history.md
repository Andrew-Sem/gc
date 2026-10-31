# Run history

This contract details what the operator sees about agent runs. The
[open pull requests contract](open-pull-requests.md) decides when runs start and how they end.

## Why

The operator should see whether the agent is working right now and how earlier runs ended,
without reading logs. The journal lives in memory for now, so the history lasts until `gc` stops.

## Glossary

**Run** — see the [open pull requests contract](open-pull-requests.md).

**Running run** — one with `AgentRunStarted` and no `AgentRunFinished` with the same number.

**Runs page** — `/runs`: a table in `#runs` with one `tr[data-run="<number>"]` per run. Its cells
are `td[data-cell="repository"]`, `td[data-cell="started"]`, `td[data-cell="duration"]`, and
`td[data-cell="status"]`.

## Clauses

### HISTORY-1. The runs page

#### HISTORY-1.1. The page

HISTORY-1.1.1. The runs page shows one row for each run in the journal.

HISTORY-1.1.2. Rows go from the highest run number to the lowest.

HISTORY-1.1.3. With no runs, the page shows "No runs yet" and no rows.

HISTORY-1.1.4. The runs page includes the shared interface head `ui.Head`.

HISTORY-1.1.5. Opening the page writes no facts.

HISTORY-1.1.6. The page does not refresh itself: the operator reloads it.

#### HISTORY-1.2. What a row shows

HISTORY-1.2.1. The repository cell shows the repository's `owner/name`.

HISTORY-1.2.2. The started cell shows the time the run started in UTC, as `2006-01-02 15:04 UTC`.

HISTORY-1.2.3. A run of a repository that was disconnected stays on the page.

HISTORY-1.2.4. Two runs of one repository working at the same time have a row each.

#### HISTORY-1.3. The status

HISTORY-1.3.1. The status of a running run is "running".

HISTORY-1.3.2. The status of a run with the outcome "PR" is "PR #<number>", a link to
`https://github.com/<owner>/<name>/pull/<number>`.

HISTORY-1.3.3. The status of a run with the outcome "no PR" is "no PR".

HISTORY-1.3.4. The status of a run with the outcome "failed" is "failed".

HISTORY-1.3.5. The status of a run with the outcome "timed out" is "timed out".

HISTORY-1.3.6. The page shows no reason for a failure: the reason stays in the log.

#### HISTORY-1.4. The duration

HISTORY-1.4.1. The duration of a finished run is the time from its start to its end.

HISTORY-1.4.2. The duration of a running run is the time from its start until the page was
opened.

HISTORY-1.4.3. A duration is rounded down to whole minutes. Under a minute it reads "under a
minute"; under an hour, "41 min"; on a whole hour, "2 h"; otherwise, "2 h 5 min".

### HISTORY-2. Getting there

#### HISTORY-2.1. Links

HISTORY-2.1.1. The repositories page links to the runs page.

HISTORY-2.1.2. The runs page links to the repositories page.
