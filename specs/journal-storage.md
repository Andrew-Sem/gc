# Journal storage

This contract details how the journal outlives the `gc` process. The
[launch contract](launch.md) decides how `gc` is configured.

## Why

The operator restarts `gc` to update it, the machine reboots, the process crashes. Connected
repositories, known PRs, and run history must still be there afterwards. A `gc` that forgets its
PRs would count zero open PRs and keep opening new ones past the limit.

## Glossary

**Journal file** — the SQLite database at the path in `GC_DB`. It has a table `facts` with the
columns `seq` (the order of writing), `kind` (the name of the fact, such as
`RepositoryConnected`), and `data` (the fields of the fact as JSON).

**Restart** — `gc` stops, by a signal or by being killed, and starts again with the same `GC_DB`.

**Running run** — see the [run history contract](run-history.md).

## Clauses

### STORE-1. Surviving a restart

#### STORE-1.1. What survives

STORE-1.1.1. After a restart, the journal holds every fact it held before, of every kind, in the
same order and with equal fields.

STORE-1.1.2. A repository connected before a restart is still on the repositories page, in the
same place in the list.

STORE-1.1.3. A repository disconnected before a restart is still not on the repositories page.

STORE-1.1.4. A run recorded before a restart is still on the runs page.

#### STORE-1.2. When a fact is written

STORE-1.2.1. A fact is in the journal file before the request that wrote it is answered: if `gc`
is killed right after a 200 response, the fact is there after the restart.

STORE-1.2.2. If a fact cannot be written, `gc` does not answer the request with success. It exits
with a non-zero status and prints "cannot write to the journal at" followed by the path. It does
not carry on with a journal that has lost a fact.

STORE-1.2.3. If the journal file was removed or replaced while `gc` runs, a fact counts as one that
cannot be written: writing it to a file nobody can reach any more would lose it at the next start.

#### STORE-1.3. Time missed while stopped

STORE-1.3.1. Wake-ups missed while `gc` was stopped are not made up: a start writes no
`AdmissionApproved` and no `AdmissionDeclined`.

### STORE-2. Runs a restart cut off

#### STORE-2.1. Closing them

STORE-2.1.1. At start, before serving requests, `gc` writes for each running run an
`AgentRunFinished` with its number and repository, the outcome "interrupted", and the time of
this start. This covers a stop by a signal and a kill alike.

STORE-2.1.2. An interrupted run is not resumed or repeated. Its repository gets its next chance at
the next wake-up.

STORE-2.1.3. With no running runs, a start writes no facts.

STORE-2.1.4. A start that fails, because of the address or the journal file, writes no facts.

### STORE-3. The journal file

#### STORE-3.1. Opening it

STORE-3.1.1. If the journal file does not exist, `gc` creates it with an empty journal, readable
and writable only by its owner.

STORE-3.1.2. If the journal file cannot be opened, because its folder does not exist or the file
is not a SQLite database, `gc` exits with a non-zero status and prints "cannot open the journal
at" followed by the path. An existing file stays byte for byte as it was.

STORE-3.1.3. If the journal file is in use by another running `gc`, the second `gc` exits with a
non-zero status and prints "cannot open the journal at" followed by the path. The first keeps
working.

#### STORE-3.2. Reading it

STORE-3.2.1. If the journal file has a fact of a kind this `gc` does not know, or a fact it cannot
parse, `gc` exits with a non-zero status and prints "cannot read the journal at" followed by the
path. It does not skip the fact: state built from part of the journal would be wrong without
anyone noticing.

STORE-3.2.2. A fact whose data is JSON `null` is a fact `gc` cannot parse.

### STORE-4. Secrets

#### STORE-4.1. The journal file

STORE-4.1.1. GitHub credentials, the OpenAI key, and the webhook secret never reach the journal
file, even if GitHub returns a credential in an error response.
