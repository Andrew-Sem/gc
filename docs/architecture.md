# Architecture

Event-driven control plane over the PR lifecycle. One process, one SQLite file, server-rendered
HTML.

## Packages

| Package | Responsibility |
|---|---|
| `internal/domain` | Pure core: the fact vocabulary and folds over history. Knows nothing about storage, GitHub, or HTTP. |
| `internal/journal` | The fact journal, in memory for now. |
| `internal/web` | HTTP: admin pages in templ, htmx fragments, static files. |
| `internal/spec` | The executable spec framework: Given facts, When an HTTP request, Expect status, facts, and HTML. |
| `cmd/gc` | `main`: reads env, wires the journal and `internal/web`, starts the server. |

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
