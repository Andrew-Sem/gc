# Architecture

Event-driven control plane over the PR lifecycle. One Go process with server-rendered HTML.
The current repository feature stores facts in memory for one process lifetime. SQLite and
restart persistence are planned separately.

## Packages

| Package | Responsibility |
|---|---|
| `framework/journal` | Generic append-only journal in memory, safe for concurrent requests; knows no project event types. |
| `framework/spec` | Generic Given/When/Then scenarios, clause reporting, HTTP and HTML assertions, exact ordered new-fact checks, and coverage for custom scenarios. |
| `specs/contract` | Project event definitions, the Journal interface, and GitHub settings (API URL, token, and the transport every GitHub request must use) shared by specifications and application. |
| `specs/extensions` | Project scenario methods, a fake GitHub HTTP server, the adapter that supplies the application factory to the framework, and the launch harness that builds and runs `cmd/gc`. |
| `specs` | Feature contracts and executable scenarios. |
| `ui` | Shared interface platform: the htmx file, its response rules, the `Head` component every page includes, and the static file handler. |
| `internal/domain` | Repository-name validation, folds over project facts, and the wake-up decision. |
| `internal/web` | HTTP handlers, templ pages, and htmx fragments; mounts `ui.Static` under `/static/`. |
| `internal/github` | GitHub API client: repository access, the clone URL, and creating PRs. |
| `internal/agent` | One agent run: a throwaway clone, Codex as a subprocess in its sandbox, then `git` commit and push. `git` talks to GitHub directly, outside `contract.GitHub.Transport`. |
| `cmd/gc` | Reads the environment, refuses to start without a token or a usable address, wires the in-memory journal, starts the server. |

The framework imports neither the project contract nor application packages, including in its
own checks. Each project chooses its fact type and supplies a factory to `Feature.Run`.
For this project, the factory is `web.New(contract.Dependencies) http.Handler`, connected in
`specs/extensions/app.go`. Only that integration file imports the application. A scenario that
no HTTP request starts, such as a wake-up, uses `WhenAction`: it acts on the journal directly and
checks the new facts. The second
entry point is the `cmd/gc` binary. It runs `web.Serve`, which serves the same `web.New` handler, never a separate
one, wakes the system up at the start of every hour, and starts an agent run for each approval, so the fast scenarios and the binary
check one application. Launch scenarios in
`specs/extensions/launch.go` build it with `go build`, run it with a controlled environment,
and check its exit, output, and HTTP answers. The environment points `HTTPS_PROXY` at a fake
proxy that records which host `gc` asks for and refuses to forward, so the default GitHub API
is checked without leaving the machine.

Each scenario gets a fresh journal and its own fake GitHub. HTTP scenarios run inside a
`testing/synctest` bubble, so timeouts pass in virtual time: a 5-second wait costs nothing.
Inside the bubble the fake GitHub lives in memory and is reached only through
`contract.GitHub.Transport`; the application must send every GitHub request through that
transport (`cmd/gc` leaves it nil, which means the real network). The `APIURL` given to
scenarios points at a closed local port, so a client that ignores the transport fails with an
ordinary connection error. The bubble waits for every goroutine the scenario starts, so
`web.New` and request handlers must not leave background goroutines running; one that never
ends makes the scenario hang or fail as a deadlock. Launch scenarios run outside the bubble and
serve the same fake over real HTTP. The application talks to the fake through its real GitHub client, so specs check
the client too; the fake answers like
GitHub: 200 for a visible repository, 404 for an invisible one or a request without a token,
401 for a wrong token. Scenarios can also make it reject the token, answer any status with
headers, drop the connection, hang until the client gives up, or echo the token in error
bodies. `Given` seeds the journal, `When` sends a
request directly to the handler, and `ExpectFacts` compares the complete ordered appended
suffix. `ExpectFacts()` explicitly requires an empty suffix. Project-specific checks are
values passed to `Expect`, such as `Expect(s.GitHubNotAsked())`; common HTTP and HTML checks,
including `ExpectBodyWithout` for secrets, stay in the framework. The project scenario embeds the generic scenario, so common methods are
reused directly. Common fluent methods return the generic scenario, so a scenario chain starts
with project setup such as `GivenGitHubSees` and ends with project checks passed to `Expect`.

Fact comparison keeps the existing Go equality semantics. Current project facts are immutable,
comparable value structs. The journal copies its slice on read, not an arbitrary nested object
graph. Supporting mutable or non-comparable facts requires a separate decision on copying and
equality; this extraction does not introduce deep copying or serialization.

When rebuilding this project, retain `framework`, `specs`, and `ui`, including their checks and
integration contract. Reimplement `internal` and `cmd/gc`. htmx and its response rules live in
`ui`, not in the application: they are interface-wide decisions, and specs compare the page
head and the served htmx file with `ui`. The reconstruction requires an implementation
of the fixed application entry point before project specs can compile, and a `cmd/gc` that
builds before launch scenarios can run; the framework's own checks can run without the
application.

Agent scenarios in `specs/extensions/agent_run.go` run `internal/agent` outside the bubble against a fake `codex` shell script that records what it was given, and a local `git http-backend` that requires the GitHub token, so clone, commit, and push are real.

Application behavior specifications live under `specs`. Repository-name validation is checked
through POST scenarios, including empty and malformed names. The scenarios also check HTML
responses. No test files remain under `internal`. Specs check that pages use `ui`'s htmx and response rules;
actual browser interaction is checked separately at acceptance, not inferred from the fast
HTTP/HTML specifications.

Planned, not built yet:

| Package | Responsibility |
|---|---|
| `internal/domain` | `Decide` and `Evolve` on top of the fact vocabulary. |
| `internal/github` | Parsing webhooks into commands. |
| `internal/store` | Journal, task queue, and settings in SQLite; replaces the in-memory journal. |
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
