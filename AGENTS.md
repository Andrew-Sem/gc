# Garbage Collector

Event-driven control plane over the PR lifecycle: ingesting facts from GitHub, an event journal,
a decision layer, effects. A single Go binary with a server-rendered HTML admin built on htmx.

## Code

Comments only as a last resort: when neither names nor specs explain why the code is the way it
is.

HTML lives in `.templ` files.

## Specs

- The scenario name in `Run` is the short gist of the case. Name opposite cases as a pair so they
  read side by side in the output: `connect a repository the token can see` / `cannot see`.
- `Then` says what is true after the action, not the action itself: `the repository is
  disconnected`. The `Expect…` lines below check it. If the request is rejected: `the request is
  rejected and nothing is connected`.
- Grow the spec framework in `internal/spec` only when a spec needs something it cannot express
  yet. Write that spec first, then add the smallest framework change, covered by its own test in
  `internal/spec`.
