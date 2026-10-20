You are reviewing one finished attempt of an experiment in which an AI agent rebuilt an
application from executable specifications.

Run directory: RUN_DIR

- `workspace/` is the rebuilt project. The implementation is in `workspace/internal/` and
  `workspace/cmd/gc/`; contracts are `workspace/specs/*.md`, executable specs are
  `workspace/specs/*_test.go`, docs are in `workspace/docs/`.
- `verify.txt` holds the automatic checks already run by the harness.
- `trace.jsonl` is the implementer's full JSONL trace. It is large; parse it with a script
  (for example `python3 -I`) instead of reading it whole.
- `prompt.md` is the prompt the implementer got; `last-message.md` is its final message.
- `review-checklist.md` is the checklist. Follow it exactly.

Rules:

- Do not modify anything in the run directory. If you need to build or run the project, copy
  `workspace/` into your current working directory first and work on the copy.
- Do not read anything under `/Users/andrew/Projects/my/garbage-collector` and do not read
  other run directories under `/Users/andrew/gc-rebuilds`.
- Other attempts may run at the same time on this machine. If you start the binary, use a
  random free port (never 8080) and a small local fake GitHub; stop everything you start.
- No browser is available.
- Judge against the contracts and the checklist, not style preferences.

Return the report in the format the checklist asks for. Start with the line
`Verdict: <verdict>`.
