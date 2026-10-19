# Launch

This contract details how the operator starts garbage-collector for the scenarios of the
[PRD](prd.md).

## Why

garbage-collector is a single binary, `gc`, configured only through the environment. The
operator should learn about a wrong configuration when starting it, not later from a failing
page.

## Glossary

**Environment** — the variables `gc` reads at start: `GITHUB_TOKEN`, `GC_ADDR`, and
`GITHUB_API_URL`.

**Output** — everything `gc` writes to stdout and stderr, including its logs.

## Clauses

### RUN-1. Starting

#### RUN-1.1. GitHub token

RUN-1.1.1. Without `GITHUB_TOKEN`, `gc` does not start: it exits with a non-zero status and
prints "GITHUB_TOKEN is not set".

#### RUN-1.2. Address

RUN-1.2.1. `gc` serves the repositories page at the address in `GC_ADDR`.

RUN-1.2.2. Without `GC_ADDR`, `gc` serves at `localhost:8080`.

RUN-1.2.3. If `gc` cannot listen on the address, because it is malformed or the port is taken,
it exits with a non-zero status and prints "cannot listen on" followed by the address.

#### RUN-1.3. GitHub API

RUN-1.3.1. `gc` checks repository access through the GitHub API at `GITHUB_API_URL`.

RUN-1.3.2. Without `GITHUB_API_URL`, `gc` uses `https://api.github.com`.
