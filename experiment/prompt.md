This repository holds the executable specifications, the spec framework and the documentation
of garbage-collector, but not its implementation. Implement the application so that it keeps
every promise in the contracts `specs/*.md` and passes all executable specifications.

Rules:

- Write code only under `internal/` and `cmd/gc/`. Do not change anything else: `framework/`,
  `specs/`, `docs/`, `AGENTS.md`, `CLAUDE.md`, `go.mod`, `go.sum`, `justfile`, `.gitignore`.
- Do not search for or read any other copy of this project, its history, or earlier attempts
  outside this directory. Everything you need is here.
- Download htmx 2 from a public CDN if you need it.
- You are done when `just check` and `just spec` both pass and the application works in a
  real browser as the contracts describe.

Finish with a short summary of what you built.
