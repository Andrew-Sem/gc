This repository holds the product requirements (`specs/prd.md`), the contracts (`specs/*.md`), the
architecture (`docs/`), the shared fact and dependency definitions (`specs/contract`), the shared
interface package (`ui`), and the in-memory journal (`framework/journal`), but no application. The
docs mention executable scenarios and a spec framework; they are not part of this repository.

Implement the application so that it keeps every promise in the contracts `specs/*.md`.

Other code will connect to your application through exactly these names, so they must exist with
these signatures:

- `internal/web`: `New(contract.Dependencies) http.Handler`;
  `Serve(context.Context, net.Listener, contract.Dependencies) error` serves until the context is
  cancelled and wakes the system up at the start of every hour;
  `WakeUp(context.Context, contract.Dependencies, *sync.WaitGroup)` makes one wake-up and starts
  an agent run for each approval, adding each run to the wait group.
- `internal/domain`: `WakeUp(contract.Journal) []string` writes the admission decisions and returns
  the approved repositories.
- `internal/agent`: `type Codex struct { Binary string; Environ []string; GitHub contract.GitHub }`
  implementing `contract.Agent`, where `Binary` is the path to `codex` and `Environ` the whole
  environment of the commands it starts; `const Prompt string`, the prompt given to Codex as its
  last argument.
- `internal/store`: `Open(path string, onWriteFailure func(error)) (*Journal, error)`, where
  `*Journal` implements `contract.Journal` and has `Close() error`.
- `cmd/gc`: the binary.

Every GitHub request goes through `contract.GitHub.Transport`.

Rules:

- Write code only under `internal/` and `cmd/gc/`. You may add your own tests there. Do not change
  anything else: `framework/`, `specs/`, `ui/`, `docs/`, `AGENTS.md`, `CLAUDE.md`, `go.mod`,
  `go.sum`, `justfile`, `.gitignore`.
- Do not search for or read any other copy of this project, its history, or earlier attempts
  outside this directory. Everything you need is here.
- htmx and its configuration are provided by the `ui` package; do not download or add another
  copy.
- Other programs may run on this machine at the same time. If you start the binary, give it a
  random free port in `GC_ADDR`, and stop everything you start.
- No browser or browser automation is available. You are done when `just check` passes and you
  are confident the application works for an operator in a browser as the contracts describe.

Finish with a short summary of what you built.
