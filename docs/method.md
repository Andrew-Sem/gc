# Method

Behavior is defined by contracts (`specs/*.md`) and executable specs (`specs/*_test.go`). The
application is replaceable: an agent can rebuild `internal/` and `cmd/gc` from them.

Why executable specs, from the rebuild experiment (`experiment/`, October 2026):

| Agent got | Hidden specs passed |
|---|---|
| PRD and architecture | 4–7 of 78 |
| plus text contracts | 10 of 78 |
| plus executable specs | 78 of 78, ten of ten attempts |

Text contracts fix behavior; agents then invent their own routes, fields, and HTML hooks.
Executable specs also fix the interface, so independent builds fit together. Time to build was
similar, about three minutes.

What the experiment taught:

- Two model families read contracts differently; review every attempt with both.
- An ambiguity shows up as two families disagreeing; settle it in the contract.
- Decisions shared by all pages belong in `ui`, not in each feature.
- Fakes run in memory and time is virtual, so specs stay fast.

Run an experiment with `experiment/series.sh`; see the scripts for details.
