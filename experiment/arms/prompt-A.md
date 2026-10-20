This repository holds the product requirements (`specs/prd.md`), the architecture (`docs/`),
the shared fact and dependency definitions (`specs/contract`), the shared interface package
(`ui`), and the in-memory journal (`framework/journal`), but no application.

Implement scenario 3 of the PRD, "Connect a repository", and only that scenario. Follow the
architecture: the application entry point is `web.New(contract.Dependencies) http.Handler` in
`internal/web`, the binary is `cmd/gc`, and every GitHub request goes through
`contract.GitHub.Transport`.

Rules:

- Write code only under `internal/` and `cmd/gc/`. You may add your own tests there. Do not
  change anything else.
- Do not search for or read any other copy of this project, its history, or earlier attempts
  outside this directory.
- htmx and its configuration are provided by the `ui` package; do not download or add another
  copy.
- Other programs may run on this machine at the same time. If you start the binary, give it a
  random free port, and stop everything you start.
- No browser or browser automation is available. You are done when `just check` passes and you
  are confident the application works for an operator in a browser.

Finish with a short summary of what you built.
