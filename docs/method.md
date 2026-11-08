# Method

Behavior is defined by contracts (`specs/*.md`) and executable specs (`specs/*_test.go`). The
application is replaceable: an agent can rebuild `internal/` and `cmd/gc` from them.

## What the rebuild experiments showed

Each series gave ten agents, five Codex and five Claude, the same inputs and asked them to build
`internal/` and `cmd/gc` from scratch (`experiment/`).

| Series | Agent got | Result |
|---|---|---|
| October | PRD and architecture | 4–7 of 78 hidden scenarios |
| October | plus text contracts | 10 of 78 |
| October | plus executable specs | 78 of 78, ten of ten builds |
| 3, November | contracts and specs | 10 of 10 green, yet hidden probes failed 2 to 12 of 16 per build |
| 4, November | contracts and specs, with the probes' behavior written into them | 10 of 10 green; 2 of 230 hidden probe checks failed |
| 5, November | the same contracts, no executable specs | 169–211 of 295 hidden scenarios as written, 212–229 with the builds' own names; 4 of 400 probe checks failed |

What this means:

- Detailed contracts carry behavior almost as well as executable specs. Most of what series 5
  failed was names the contracts leave open: form fields, routes, HTML hooks. The October gap
  was probably mostly names too.
- Executable specs fix those names, so independent parts fit together, and they leave fewer rare
  breakages: a panic on a `null` answer and a stop that hangs on an open stream appeared without
  them.
- The contracts in series 5 grew out of work with specs and probes. The series does not show that
  such contracts can be written without them.
- A green build is not proof: series 3 passed every scenario and still broke promises. Probe
  each promise the contract makes with hidden checks the builder did not see.
- Reviewer verdicts are unreliable: two reviewers often disagreed, and a build both called a
  success failed 12 probes. Reviewers are useful for finding cases to probe, not for judging.
- A build takes 10 to 33 minutes.

## Practice

- Two model families read contracts differently; review every attempt with both.
- An ambiguity shows up as two families disagreeing; settle it in the contract.
- Decisions shared by all pages belong in `ui`, not in each feature.
- Fakes run in memory and time is virtual where possible, so specs stay fast.
- A probe that finds a real breakage becomes an ordinary scenario.

Run an experiment with `experiment/series.sh`; see the scripts for details.
