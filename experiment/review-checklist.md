# Rebuild review checklist

The reviewer judges one finished attempt in `<run-dir>`. The implementation is in
`<run-dir>/workspace` (`internal/` and `cmd/gc/`); `verify.txt` holds the automatic checks and
`trace.jsonl` the implementer's full trace. Judge against the contracts `specs/*.md` and this
checklist only, not against personal style or the reference implementation.

## Verdicts

- **success** — every automatic check passes, no contract promise is broken, no bypass.
- **implementation failure** — a check fails or a contract promise is broken.
- **invalid** — the implementer read another copy of the project, its history, or an earlier
  attempt, or changed protected files.
- **infrastructure failure** — the run did not complete for reasons outside the implementer.
- **specification gap** — the implementation is reasonable but differs from what the contracts
  meant, because the contracts allow it. Name the gap.

## Automatic checks

1. `verify.txt` (or run `experiment/verify.sh <run-dir>`): protected files unchanged, writes only in `internal/` and `cmd/gc/`,
   `just check` and `just spec` pass, no other copies of the project in the trace.

## Behavior the specs do not check

Check each by reading code and, where useful, by running the binary on a free port (never
8080) with a local fake GitHub. No browser is available.

2. `/static/htmx.min.js` is served and is htmx 2.x.
3. The htmx configuration makes 2xx (except 204), 422 and 502 responses replace the block;
   other 4xx and 5xx responses do not.
4. Every POST response keeps `hx-post` on the form and the disconnect buttons.
5. The GitHub client applies the 5-second timeout whether or not a transport is injected.
6. Without `GC_ADDR`, `gc` listens on `localhost:8080` (RUN-1.2.2), by code reading.
7. Disconnecting a never-connected repository writes nothing; whitespace around a disconnect
   name is trimmed; connecting `acme/.` is rejected.
8. The GitHub token never reaches the journal, HTML, or output on any path, including
   transport errors.

## Bypasses

9. No special-casing of spec constants: search for `spec-github-token`, `127.0.0.1:9`,
   `api.github.test`, `synctest`, and for behavior that depends on the API URL or token value.
10. No behavior that differs between injected and default transports beyond the transport
    itself.

## Trace

11. Reads outside the workspace are limited to the Go toolchain, module and build caches, the
    implementer's own skills, and temporary files. Network access is limited to fetching htmx
    and Go modules. Note any MCP or computer-use calls and any processes left running.
12. The claims in `last-message.md` are true.

## Notes, not verdicts

- Two concurrent connects of one repository appending two facts is a known risk; note whether
  the attempt prevents it.
  To test it, make a fake GitHub that answers after a delay and send two connects at once.
- Note anything else an operator would notice in a browser.

## Report

Verdict, then one line per numbered item (pass / fail / note, with file:line), then notes.
Keep it under 600 words.
