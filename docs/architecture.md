# Architecture

Event-driven control plane over the PR lifecycle. One Go process with server-rendered HTML.
The current repository feature stores facts in memory for one process lifetime. SQLite and
restart persistence are planned separately.

## Packages

| Package | Responsibility |
|---|---|
| `framework/journal` | Generic append-only journal in memory; knows no project event types. |
| `framework/spec` | Generic Given/When/Then scenarios, clause reporting, HTTP and HTML assertions, and exact ordered new-fact checks. |
| `specs/contract` | Project event definitions and the Journal/GitHub dependency interfaces shared by specifications and application. |
| `specs/extensions` | Project scenario methods, fake GitHub, and the adapter that supplies the application factory to the framework. |
| `specs` | Feature contracts and executable scenarios. |
| `internal/domain` | Repository-name validation and folds over project facts. |
| `internal/web` | HTTP handlers, templ pages, htmx fragments, and static files. |
| `cmd/gc` | Reads the address, wires the in-memory journal and GitHub placeholder, starts the server. |

The framework imports neither the project contract nor application packages, including in its
own checks. Each project chooses its fact type and supplies a factory to `Feature.Run`.
For this project, the factory is `web.New(contract.Dependencies) http.Handler`, connected in
`specs/extensions/app.go`. Only that integration file imports the application.

Each scenario gets a fresh journal and fake GitHub. `Given` seeds the journal, `When` sends a
request directly to the handler, and `ExpectFacts` compares the complete ordered appended
suffix. `ExpectFacts()` explicitly requires an empty suffix. Project-specific checks use
`ExpectCheck`; common HTTP and HTML checks stay in the framework. The project scenario embeds the generic scenario, so common methods are reused directly.
Project-specific setup and expectations are separate statements: common fluent methods return
the generic scenario, which does not expose GitHub methods.

Fact comparison keeps the existing Go equality semantics. Current project facts are immutable,
comparable value structs. The journal copies its slice on read, not an arbitrary nested object
graph. Supporting mutable or non-comparable facts requires a separate decision on copying and
equality; this extraction does not introduce deep copying or serialization.

When rebuilding this project, retain `framework` and `specs`, including their checks and
integration contract. Reimplement `internal` and restore `cmd/gc` from a minimal starter.
htmx remains under `internal/web/static`; it is an implementation dependency that the agent
can download again, not a retained framework asset. The reconstruction requires an implementation
of the fixed application entry point before project specs can compile; the framework's own
checks can run without the application.

Application behavior specifications live under `specs`. Repository-name validation is checked
through POST scenarios, including empty and malformed names. The scenarios also check HTML
responses. No test files remain under `internal`. Tests of the htmx asset body and its internal
response-rule configuration were removed; actual browser interaction is checked separately
at acceptance, not inferred from the fast HTTP/HTML specifications.

Planned, not built yet:

| Package | Responsibility |
|---|---|
| `internal/domain` | `Decide` and `Evolve` on top of the fact vocabulary. |
| `internal/github` | GitHub format: parsing webhooks into commands, parsing API responses. |
| `internal/store` | Journal, task queue, and settings in SQLite; replaces the in-memory journal. |
| `internal/agent` | Running Codex, working copies, `git`, creating PRs. |
| `internal/app` | Wiring: scheduler, task executor, ports, start and stop; `cmd/gc` assembles it. |

## Flow

```
GitHub ──webhooks──▶ HTTP ──▶ journal ──task──▶ executor ──▶ Codex ──▶ model API
                      ▲          │  ▲               │
                   admin ────────┘  └────result─────┘
```

## Invariants

- The journal is append-only and the single source of truth; domain state is its fold.
- Secrets live only in the environment: never in the journal, tasks, HTML, or logs.
- The domain does not read the clock: time arrives with the `TickProduction` or `TickSweep`
  command.
- `Decide` does not return a "wake me up at time T" effect; the scheduler owns the schedule.
- Facts and the task they produce are written in one transaction.
- The queue is read through an interface; the task executor does not go to the database
  directly.
- Evidence completeness belongs to the sweep: a full snapshot or `LookupIncomplete`, not both.
- The webhook signature is checked before dedupe and parsing.

## Stack

| Layer | Choice |
|---|---|
| Language, build, tests | Go, standard `testing` |
| HTTP | `net/http` |
| Templates | templ |
| Interactivity | htmx, no frontend build |
| Storage | SQLite |
| Agent | Codex as a subprocess |
