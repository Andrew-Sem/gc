# Garbage Collector

Event-driven control plane over the PR lifecycle: ingesting facts from GitHub, an event journal,
a decision layer, effects. A single Go binary with a server-rendered HTML admin built on htmx.

## Code

Comments only as a last resort: when neither names nor specs explain why the code is the way it
is.

HTML lives in `.templ` files. Pages include `ui.Head`; interface-wide decisions live in `ui`.

Reach the outside world only through `contract.Dependencies`; every GitHub request goes through
`contract.GitHub.Transport`. Leave no goroutines running after a request.

## Specs

- Tests are executable specifications. Keep application behavior checks in `specs`, not
  `internal`; framework checks belong beside the framework. Never change specs to fit the
  implementation.
- Contract clauses have three levels: an area heading (`REPO-1`), a requirement heading
  (`REPO-1.6`), and clauses (`REPO-1.6.1`). Each clause is one promise; headings promise
  nothing. `Covers` names clauses only, never headings. A new promise gets the next free number
  under its requirement and is placed next to related clauses. Never renumber or reuse ids.
- `Covers` only links a scenario to a clause; the report counts links, not how well they check.
  When you link a clause, make sure the `Expect…` lines check its whole promise. If they cannot,
  split the clause instead of linking it.
- The scenario name in `Run` is the short gist of the case. Name opposite cases as a pair so they
  read side by side in the output: `connect a repository the token can see` / `cannot see`.
- `Then` says what is true after the action, not the action itself: `the repository is
  disconnected`. The `Expect…` lines below check it. If the request is rejected: `the request is
  rejected and nothing is connected`.
- If two reasonable implementations could differ, the contract decides: name the case, status,
  and text.
- Pin in specs every route, form field, and HTML hook that other parts rely on.
- Grow the spec framework in `framework/spec` only when a spec needs something it cannot express
  yet. Write that spec first, then add the smallest framework change, covered by its own test in
  `framework/spec`.
