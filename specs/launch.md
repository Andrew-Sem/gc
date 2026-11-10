# Launch

This contract details how the operator starts garbage-collector for the scenarios of the
[PRD](prd.md).

## Why

garbage-collector is a single binary, `gc`, configured only through the environment. The
operator should learn about a wrong configuration when starting it, not later from a failing
page.

## Glossary

**Environment** — the variables `gc` reads at start: `GITHUB_APP_ID`,
`GITHUB_APP_PRIVATE_KEY_FILE`, `GC_WEBHOOK_SECRET`,
`OPENAI_API_KEY`, `OPENAI_BASE_URL`, `GC_ADMIN_USER`, `GC_ADMIN_PASSWORD`, `GC_ADDR`, `GC_DB`, `GITHUB_API_URL`, and the standard proxy variables `HTTPS_PROXY`, `HTTP_PROXY`, and
`NO_PROXY`.

**Output** — everything `gc` writes to stdout and stderr, including its logs.

## Clauses

### RUN-1. Starting

#### RUN-1.1. GitHub App

RUN-1.1.1. Without `GITHUB_APP_ID`, `gc` does not start: it exits with a non-zero status and
prints "GITHUB_APP_ID is not set".

RUN-1.1.2. Without `GITHUB_APP_PRIVATE_KEY_FILE`, `gc` does not start: it exits with a non-zero
status and prints "GITHUB_APP_PRIVATE_KEY_FILE is not set". The variable holds the path to the
app's private key, the `.pem` file GitHub gives when the key is made.

RUN-1.1.3. If that file cannot be read or does not hold an RSA private key in PEM, `gc` does not
start: it exits with a non-zero status and prints "cannot read the private key at" followed by
the path. The output has nothing of the file's contents.

#### RUN-1.2. Address

RUN-1.2.1. `gc` serves the repositories page at the address in `GC_ADDR`.

RUN-1.2.3. If `gc` cannot listen on the address, because it is malformed or the port is taken,
it exits with a non-zero status and prints "cannot listen on" followed by the address.

RUN-1.2.4. Without `GC_ADDR`, `gc` does not start: it exits with a non-zero status and prints
"GC_ADDR is not set". There is no default address.

#### RUN-1.3. GitHub API

RUN-1.3.1. `gc` checks repository access through the GitHub API at `GITHUB_API_URL`.

RUN-1.3.2. Without `GITHUB_API_URL`, `gc` uses `https://api.github.com`.

RUN-1.3.3. When `HTTPS_PROXY` is set, `gc` reaches an `https` GitHub API through that proxy.

#### RUN-1.4. Webhook secret

RUN-1.4.1. Without `GC_WEBHOOK_SECRET`, `gc` does not start: it exits with a non-zero status
and prints "GC_WEBHOOK_SECRET is not set".

RUN-1.4.2. `gc` checks webhook signatures with the value of `GC_WEBHOOK_SECRET`.

#### RUN-1.5. OpenAI key

RUN-1.5.1. Without `OPENAI_API_KEY`, `gc` does not start: it exits with a non-zero status and
prints "OPENAI_API_KEY is not set".

#### RUN-1.6. Journal file

RUN-1.6.1. Without `GC_DB`, `gc` does not start: it exits with a non-zero status and prints
"GC_DB is not set". There is no default path. The
[journal storage contract](journal-storage.md) decides what happens to the file.

#### RUN-1.7. Admin login

RUN-1.7.1. Without `GC_ADMIN_USER`, `gc` does not start: it exits with a non-zero status and
prints "GC_ADMIN_USER is not set".

RUN-1.7.2. Without `GC_ADMIN_PASSWORD`, `gc` does not start: it exits with a non-zero status and
prints "GC_ADMIN_PASSWORD is not set".

RUN-1.7.3. A running `gc` refuses an admin request without the operator's credentials (ACCESS-1.1
of the [admin access contract](admin-access.md)) and handles one with them.

#### RUN-1.8. OpenAI Base URL

RUN-1.8.1. A set `OPENAI_BASE_URL` that is not a valid URL (see the
[instance settings contract](instance-settings.md)) stops `gc` from starting: it exits with a
non-zero status and prints "OPENAI_BASE_URL is not an absolute http or https URL without
credentials, query, or fragment".

RUN-1.8.2. A valid `OPENAI_BASE_URL` becomes, normalized, the default of the OpenAI Base URL.
