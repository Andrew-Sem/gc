# Garbage Collector

`gc` keeps a steady flow of small pull requests that pay down technical debt. Once an hour it
may ask a coding agent (Codex) to fix one thing in a connected repository and opens the result as
a PR. Every 6 hours it merges the PRs whose required checks passed, and records why the others
wait. It never closes a PR: a stuck one waits for a person.

It is a single Go binary with an admin built on htmx, behind an HTTP Basic login. Every fact
goes into a SQLite journal.

What it does and why: [specs/prd.md](specs/prd.md). Each feature has its own contract next to it
in [specs](specs), and the `*_test.go` files there are its executable specifications.

## Run

You need Go 1.26, [just](https://github.com/casey/just), the `codex` CLI on `PATH`, and a GitHub
App installed on the repositories, with its webhook pointing at `/webhooks/github`.

```sh
cp .env.example .env   # fill in the App, the OpenAI key, the webhook secret and the admin login
just run               # http://localhost:8080/repositories
just check             # lint and specs
```

`.env.example` lists every setting. `gc` refuses to start without one it needs.

## License

[MIT](LICENSE)
