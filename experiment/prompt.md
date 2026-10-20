This repository holds the executable specifications, the spec framework and the documentation
of garbage-collector, but not its implementation. Implement the application so that it keeps
every promise in the contracts `specs/*.md` and passes all executable specifications.

Rules:

- Write code only under `internal/` and `cmd/gc/`. Do not change anything else: `framework/`,
  `specs/`, `ui/`, `docs/`, `AGENTS.md`, `CLAUDE.md`, `go.mod`, `go.sum`, `justfile`,
  `.gitignore`.
- Do not search for or read any other copy of this project, its history, or earlier attempts
  outside this directory. Everything you need is here.
- htmx and its configuration are provided by the `ui` package; do not download or add another
  copy.
- Other programs may run on this machine at the same time. If you start the binary, give it a
  random free port in `GC_ADDR`, and stop everything you start.
- No browser or browser automation is available. You are done when `just check` and
  `just spec` both pass and you are confident the application works for an operator in a
  browser as the contracts describe.

Finish with a short summary of what you built.
