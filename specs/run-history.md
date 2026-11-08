# Run history

This contract details what the operator sees about agent runs. The
[open pull requests contract](open-pull-requests.md) decides when runs start and how they end.

## Why

The operator should see whether the agent is working right now and how earlier runs ended,
without reading logs. The history outlives `gc`: see the
[journal storage contract](journal-storage.md).

## Glossary

**Run** — see the [open pull requests contract](open-pull-requests.md).

**Running run** — one with `AgentRunStarted` and no `AgentRunFinished` with the same number.

**Runs page** — `/runs`: a table in `#runs` with one `tr[data-run="<number>"]` per run. Its cells
are `td[data-cell="repository"]`, `td[data-cell="started"]`, `td[data-cell="duration"]`,
`td[data-cell="tokens"]`, and `td[data-cell="status"]`.

**Runs panel** — `#runs-panel`: the part of the runs page that holds the table in `#runs` and the
"No runs yet" text.

**Event stream** — `/runs/events`: a stream of server-sent events that tells the page the journal
has changed. It says only that, and the page then asks for the panel with an ordinary request.

## Clauses

### HISTORY-1. The runs page

#### HISTORY-1.1. The page

HISTORY-1.1.1. The runs page shows one row for each run in the journal.

HISTORY-1.1.2. Rows go from the highest run number to the lowest.

HISTORY-1.1.3. With no runs, the page shows "No runs yet" and no rows.

HISTORY-1.1.4. The runs page includes the shared interface head `ui.Head`.

HISTORY-1.1.5. Opening the page writes no facts.

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

HISTORY-1.3.6. The page shows no reason for a failure. The
[open pull requests contract](open-pull-requests.md) puts the reason in the log.

HISTORY-1.3.7. The status of a run with the outcome "interrupted" is "interrupted".

#### HISTORY-1.4. The duration

HISTORY-1.4.1. The duration of a finished run is the time from its start to its end.

HISTORY-1.4.2. The duration of a running run is the time from its start until the page was
opened.

HISTORY-1.4.3. A duration is rounded down to whole minutes. Under a minute it reads "under a
minute"; under an hour, "41 min"; on a whole hour, "2 h"; otherwise, "2 h 5 min".

HISTORY-1.4.4. The duration of a run with the outcome "interrupted" reads "unknown": `gc` learns
about the end only at its next start, which can be much later.

#### HISTORY-1.5. Why there is no PR

HISTORY-1.5.1. The status cell of a run with the outcome "no PR" and an explanation has, after
"no PR", a `details[data-explanation]` whose `summary` reads "why" and whose `pre` holds the
explanation as it is, so its line breaks stay.

HISTORY-1.5.2. A run with the outcome "no PR" and an empty explanation has no
`details[data-explanation]`, and its status is "no PR" alone.

HISTORY-1.5.3. The explanation is text: markup in it is shown, not run.

#### HISTORY-1.6. Tokens

HISTORY-1.6.1. The tokens cell of a finished run reads "<input> in (<cached> cached), <output>
out", from the tokens in its `AgentRunFinished`.

HISTORY-1.6.2. A count under 1000 is written as it is; under a million, in thousands with one
decimal and "k"; otherwise, in millions with two decimals and "M". Decimals are cut, not rounded:
`1214346` reads "1.21M", `9012` reads "9.0k", and `999999` reads "999.9k".

HISTORY-1.6.3. The tokens cell of a running run, or of a finished run with no tokens, is empty.

### HISTORY-2. Getting there

#### HISTORY-2.1. Links

HISTORY-2.1.1. The repositories page links to the runs page.

HISTORY-2.1.2. The runs page links to the repositories page.

### HISTORY-3. Live updates

#### HISTORY-3.1. The panel

HISTORY-3.1.1. `GET /runs/panel` answers 200 with the runs panel alone, without `ui.Head`, showing
the same rows or the same "No runs yet" as the page.

HISTORY-3.1.2. On the page and in the `GET /runs/panel` answer, the panel has
`hx-get="/runs/panel"`, `hx-trigger="runs-changed from:body, every 60s"`, and
`hx-swap="outerHTML"`.

HISTORY-3.1.3. The panel asks for itself once a minute even with no new facts, because the duration
of a running run grows without any. This is the `every 60s` in HISTORY-3.1.2.

HISTORY-3.1.4. `GET /runs/panel` writes no facts.

#### HISTORY-3.2. The event stream

HISTORY-3.2.1. `GET /runs/events` answers 200 with `Content-Type: text/event-stream` and
`Cache-Control: no-store`, and sends its headers at once, before any fact is written.

HISTORY-3.2.2. After a fact of any kind is written to the journal, the stream sends
`data: changed` followed by an empty line.

HISTORY-3.2.3. Facts written close together may share one signal, but a signal always follows the
last of them.

HISTORY-3.2.4. Facts written before the stream was opened send nothing.

HISTORY-3.2.5. The stream sends nothing except that signal: no fact, no repository, no number.

HISTORY-3.2.6. Every open stream gets the signal.

HISTORY-3.2.7. Opening the stream writes no facts.

HISTORY-3.2.8. When the client disconnects, the stream ends and leaves nothing running.

HISTORY-3.2.9. An open stream does not hold `gc` back: on a stop signal `gc` exits without waiting
for the client to disconnect.

#### HISTORY-3.3. The page

HISTORY-3.3.1. The runs page opens `new EventSource("/runs/events")`.

HISTORY-3.3.2. On each signal, the page fires `runs-changed` on `body`.

HISTORY-3.3.3. Each time the stream connects, including when the browser reconnects it by itself,
the page also fires `runs-changed`, so a phone that slept catches up at once.

HISTORY-3.3.4. Only the runs page opens the stream: the repositories page does not.
