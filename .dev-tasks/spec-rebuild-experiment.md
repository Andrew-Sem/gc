# Specification-driven rebuild experiment

Status: Stage A implemented for user review. The user authorized the structural refactor and mechanical specification migration; later feature requirements and experiments remain separate stages.

## Change contract

Prepare the repository connect/disconnect feature for ten independent rebuilds from the same approved specifications and reusable framework. The experiment is a learning exercise on this feature, not proof of reliability for arbitrary commercial applications.

Work in `claude/go-htmx-bdd-project-f905f1` (inspected at `a0cdf05`). Preserve the unrelated dirty main checkout. Keep project documentation and UI text in English.

The user owns the contract and executable specifications. Agents may create or change tests only with direct user permission; all such changes require user review before they become the implementation acceptance baseline. There is no separate category of freely authored implementation tests. Rebuild agents must not modify the approved framework, its checks, project contract, or specifications.

Scope: the existing repositories feature, generic specification infrastructure, GitHub adapter, and local experiment tooling. Exclude PR production/merging, scheduling, authentication, deployment, framework publication, automatic mutation testing, and a general differential-testing system. No commits or pushes are included.

## 1. Minimal Reasonable Solution

### Stage A — make the boundary concrete

Prepare a reviewable file/dependency map and API sketch before editing executable specifications:

- Keep generic scenario execution, HTTP requests, HTML assertions, clause tracking, and an in-memory fact journal in reusable framework packages. They must not import project event types, GitHub, or application packages.
- Move project event definitions from `internal/domain/facts.go` into a `specs/contract` package. Keep folds and validation in replaceable application code.
- Place GitHub ports and project dependency types in the project contract; place GitHub scenario setup, fake behavior, and project-specific assertions in `specs/extensions`. Move the GitHub-specific portions of `internal/spec/scenario.go`, `github.go`, and `run.go` there.
- Replace the hard-coded `web.New` call in `internal/spec/feature.go` with a factory supplied by the project. Project support connects the framework to an application entry point with a fixed signature. The factory implementation belongs to replaceable application code; its signature belongs to the retained contract.
- Preserve exact ordered comparison of newly appended facts, explicit empty expectations, isolated scenarios, immutable given history, and rejection of malformed scenarios. Define fact equality/copy semantics explicitly rather than assuming all future project facts are comparable Go values.
- Keep the framework in `framework/spec` and `framework/journal` inside this repository initially. No new published module or broad plugin architecture.
- Keep htmx in `internal/web/static` as an implementation dependency. It is deleted with the application and downloaded by the implementation agent; do not introduce a retained assets package.

Output: the agreed layout and `extensions.NewFeature`/`extensions.Scenario` API were approved. The implementation diff preserves feature assertions and migrates the existing framework checks to local fixtures. User review of the resulting diff is still required. Existing application checks have also been moved into retained HTTP specifications following the user request for a complete migration. Their existing behavior is documented for user review.

### Stage B — complete and approve the feature contract

Present concrete proposed behavior and examples to the user for the remaining choices: valid repository names, whitespace, case/identity, list ordering, GitHub response/error mapping, timeout behavior, launch command, address and configuration variables. The first experiment explicitly uses in-memory storage within one process lifetime. Persistence across restarts remains a product requirement, but SQLite and restart acceptance are deferred to a separate feature.

Use current code as evidence of implicit choices, not as the authority for requirements. Mark intentionally unconstrained behavior explicitly. Specify `GITHUB_TOKEN` and a configurable GitHub API endpoint for the local fixture, and define PAT non-disclosure including reflected error bodies.

After direct permission, draft the missing executable specifications and any required framework checks. The user reviews them before an implementation agent receives a frozen baseline. Review clause coverage semantically; `Covers` alone does not prove a clause is fully checked.

#### Approved behavior decisions (2026-10-08)

- Repository input is lowercased and stripped of surrounding spaces before validation, duplicate detection, and access checks. Internal spaces are rejected. This part has approved executable specifications and is implemented.
- A rejected GitHub token produces a distinct operator-facing error directing the operator to check `GITHUB_TOKEN`. Do not connect the repository or append a fact. Never display the token or the raw GitHub error response.
- GitHub unavailability or a server error produces an access-check failure asking the operator to try later. Do not retry automatically or append a fact.
- Wait at most five seconds for GitHub; timeout has the same failure behavior, with no automatic retry or connection fact.
- List repositories by most recent connection first. Disconnecting and reconnecting moves the repository to the top. Repeating a connection that is already active leaves its position unchanged.
- Without `GITHUB_TOKEN`, startup fails with a clear error that does not disclose a token.
- The listen address defaults to `localhost:8080` and is configurable through `GC_ADDR`. An invalid address or occupied port causes startup to fail with a clear error. Superseded on 2026-10-20: `GC_ADDR` is required (RUN-1.2.4), because every rebuild agent smoke-tested the default port and concurrent attempts collided on it.
- Use the standard GitHub API by default, with `GITHUB_API_URL` overriding the endpoint for a local HTTP fixture.

Name normalization and list ordering now have user-reviewed executable specifications and are implemented. All 31 repository scenarios pass; lint and build pass. The remaining decisions still need contract wording and executable-specification drafts, explicit permission to write those tests, and user review. Exact English error wording, application response statuses for the new failure cases, and any remaining GitHub protocol mappings are not yet approved.

A draft of GitHub interaction and token-protection specifications was written without user
review and deleted at the user's request on 2026-10-09. GitHub specifications will be written
again step by step with user review. REPO-1.5 keeps its earlier wording until then.

#### Behavior chosen by the agent, pending user review (2026-10-20)

The user delegated the remaining Stage B slices ("finish up to acceptance, commit along the
way") and reviews them together before browser acceptance:

- REPO-1.8: a rejected token (401) answers 502 with "GitHub rejected the token. Check
  GITHUB_TOKEN." (wording approved by the user).
- REPO-1.3.2: 403 that is not a rate limit counts as no access (422), like 404; GitHub uses it
  for SSO-protected organizations.
- REPO-1.10: 429, or 403 with `x-ratelimit-remaining: 0` or `retry-after`, answers 502 with
  "GitHub rate limit reached. Try again later."
- REPO-1.9: any other error status answers 502 with "Could not check access: GitHub failed. Try
  again later."
- REPO-1.6.2: the 5-second wait is checked in virtual time (`testing/synctest`), chosen by
  the user over a configurable timeout or an injected clock. The cost is a `Transport` field
  in `contract.GitHub` that the application must use. REPO-1.6.1 keeps its wording without
  "try again later".
- REPO-4.1.1–4.1.3: GitHub echoes the token in error bodies; the journal, the page, and the
  process output must not contain it. Logs are checked through the running binary's output.
- `specs/launch.md` (RUN-1): `gc` exits with "GITHUB_TOKEN is not set" when the token is
  missing or empty, and with "cannot listen on <address>" for a malformed or taken address.
  RUN-1.2.2 (default `localhost:8080`) was intentionally uncovered, then removed on
  2026-10-20 in favor of a required `GC_ADDR` (RUN-1.2.4). RUN-1.3.2 (default `https://api.github.com`) is checked through a fake proxy,
  which required RUN-1.3.3: `gc` honors `HTTPS_PROXY`.
- Redirects from GitHub (renamed or transferred repositories) are deferred: they raise a naming
  question for connected repositories that is outside this experiment.
  Resolved on 2026-10-20 after series 2 showed the ambiguity: a redirect is not followed and
  shows "GitHub moved this repository. Enter its current name." (REPO-1.12.1, response 422).

### Stage C — finish and accept the reference feature

Implement the approved specifications, including a real GitHub HTTP adapter replacing `cmd/gc`'s `noGitHub`. Use the project fake for application scenarios and a local HTTP server for adapter specifications. The fixture checks outbound requests and supplies controlled responses; it is part of the visible specification support.

Build and launch the actual executable. Independently inspect the implementation and exercise connect, duplicate connect, errors, disconnect, reconnect, and retained neighboring entries through the browser. Check PAT non-disclosure. Restart recovery is outside this experiment. Keep all acceptance requirements visible; no hidden test suite. If exploratory validation exposes a missing requirement, bring it to the user rather than silently turning it into a test.

### Stage D — prepare a repeatable workspace

Create an explicit allowlist manifest of retained files and writable implementation paths. Include approved specifications and support, event/port definitions, generic framework and its checks, fixed dependency/build inputs and a minimal `cmd/gc` starter. Contract declaration files must contain no preserved business logic. Document generation commands and allowed generated files separately from protected inputs.

Create a standalone scratch directory with no Git history or previous implementation. Run each attempt with a fresh agent context and the same starter snapshot. Do not attach the prior conversation containing implementation details. Use a fixed prompt forbidding edits to protected inputs and access to prior implementations. Protect retained material with before/after content hashes and check changes outside the writable manifest.

This is procedural isolation, not a filesystem security boundary. Record available tool traces and inspect them for reads of the source checkout, history, previous attempts, or remote copies. Do not claim traces prove the absence of every possible access. If contamination is observed, classify the attempt as invalid, preserve it, strengthen isolation and rerun; if conditions materially change, begin a new series.

Archive each complete result and available trace outside the scratch directory before resetting it. Reset only the dedicated experiment workspace. No container is required initially.

### Stage E — run the experiment

Run as many pilots as necessary to validate the harness. Before the counted series, freeze the source snapshot, specifications, framework, prompt, model/settings, tools/environment, time/token limits, writable paths, and independent review checklist. Select the remaining execution parameters explicitly before starting; do not invent a budget now.

Run ten sequential attempts from the identical starter snapshot. The implementer sees all normal development materials, as in real use. A separate agent validates each finished result using the same published acceptance checklist, approved specifications, protected-input hashes, code inspection, trace inspection, and browser exercise.

The reviewer checks actual behavior, bypasses/hardcoding, and correctness risks, not personal style preferences or unpublished acceptance conditions. No reviewer feedback or manual repairs are fed into a completed counted attempt. Record success, implementation failure, invalid/contaminated run, infrastructure failure, or specification gap distinctly; never hide unsuccessful attempts. Any change to the frozen inputs requires a new series. Keep all artifacts and report the complete results, not only successful reconstructions.

Measure scenario execution separately from build/generation/tool startup and browser validation. Fast feedback is a target to measure, not evidence that complete implementation time will be short.

## 2. Matrix Decisions on Complications

| Complication | Options | Decision | Rationale and Cost |
| --- | --- | --- | --- |
| Reuse across projects | Framework imports project; project supplies integration | Project supplies factory and support | Keeps HTTP/HTML/fact machinery reusable; needs a small stable integration contract |
| Event vocabulary | Agent invents types; approved project definitions | Retain approved definitions | Specifications and application share their language; behavior remains replaceable |
| Test authorship | Agent freely adds tests; explicit permission and review | Explicit permission and user review | Prevents implementation from defining its own acceptance conditions |
| GitHub verification | Live GitHub; fake only; fake plus local HTTP fixture | Fake plus visible local HTTP fixture | Fast behavior checks and real adapter coverage without upstream availability dependence |
| Hidden acceptance | Hidden suite; visible specifications plus independent inspection | Visible materials and independent inspection | Matches intended real development conditions; reviewer must not invent requirements |
| Isolation | Instruction only; separate reset directory and traces; container | Separate directory and traces initially | Accepted procedural compromise; filesystem access remains possible |
| Persistent storage in first slice | Memory-only boundary; include durable storage | Memory-only for this experiment; durable storage deferred | User chose to avoid the database scope now; restart persistence remains a future requirement |
| Series preparation | Fixed three pilots; pilots as needed | Pilots as needed, then frozen ten-run series | Avoids arbitrary pilot count; changes invalidate comparability |

## 3. Refactoring Options

| Option | Value | Cost and Risk | Decision |
| --- | --- | --- | --- |
| Separate framework from project facts, GitHub and app factory | Enables actual reuse and reconstruction | API/import migration touches specifications and requires user review | Include |
| Separate event definitions from folds and validation | Preserves the specification language without preserving the answer | Small file/package migration | Include |
| Publish framework as independent library | Future distribution | Versioning and release overhead without a second consumer | Defer |
| General plugin system, mutation engine, differential runner | Potential later scale tools | Premature complexity | Defer |
| Add unrelated GC features | Broader product functionality | Obscures the experiment and expands acceptance scope | Reject |

## 4. Test Coverage Plan

This section describes proposed evidence, not permission to author tests. New or changed checks require direct permission and user review.

| Behavior or Risk | Test or Check | Level |
| --- | --- | --- |
| Framework accidentally retains project coupling | Import/dependency inspection and review of factory signature | Static |
| Scenario/fact semantics change during extraction | Existing framework checks; review proposed changes for independent fixtures, exact fact tail and given-history preservation | Unit |
| Requirements survive deletion | Clause-to-spec review and inspection of retained-file manifest; review requirements currently encoded in application tests | Static |
| Feature outcomes, invalid inputs, idempotency, history and HTML | User-approved executable feature specifications | Integration |
| GitHub protocol, failure mapping and PAT disclosure | User-approved adapter specifications against local HTTP fixture | Integration |
| Persistence boundary | Verify that experiment scope excludes restart recovery; add its specifications only with the later persistence feature | Static |
| Binary wiring, static assets, htmx and visible errors | Build/start plus independent browser exercise using controlled GitHub fixture | Runtime / E2E |
| Specifications/framework modified or bypassed | Protected-input hashes, full diff and code inspection after every attempt | Static |
| Previous implementation accessed | Fresh context/starter and available tool-trace review, with stated visibility limits | Static |
| Results lost or mixed across attempts | Archive result, snapshot identity, logs, trace and reviewer verdict before reset | Runtime |
| Fast feedback target | Measure uncached specification execution separately from build/startup and browser work | Runtime |
| Normal repository checks | Generation, all approved specifications, Go tests, formatting/vet, templ formatting and build | Static / Integration |

## Deviation log

- User resolved the persistence scope after the initial plan: use in-memory storage for the first experiment and defer SQLite/restart verification.
- User placed the project vocabulary under `specs/contract` and named project support `specs/extensions`.
- User chose to leave htmx in the replaceable application; rebuild agents may download it.
- Stage A preserves current comparable value-fact equality and shallow slice copying rather than introducing a generalized serialization or deep-copy system. See `docs/architecture.md`.
- User chose embedding the generic scenario instead of forwarding its methods. Project setup starts the fluent chain, and project checks join it as values passed to `Expect` (2026-10-09), so a scenario reads as one chain.
- User requested complete migration: validation cases now exercise HTTP; htmx checks consume public HTTP responses from `specs`, and `internal` contains no test files.
- User approved removing `specs/htmx_test.go` and the technical contract added for it: retain feature HTML assertions and verify actual browser behavior during acceptance instead of testing htmx configuration details.
- User rejected browser-driven specifications (headless Chrome through `chromedp`, or a
  lighter headless browser) on 2026-10-20. htmx configuration, the served htmx asset and its
  version stay unchecked by specs; the independent reviewer and manual acceptance cover them.
- User kept 422/502 error responses (which need the htmx response-handling configuration)
  rather than switching errors to 200, accepting that the configuration stays unchecked and
  relying on the rebuild agent and reviewer (2026-10-20).
- REPO-2.3.1 checks the shape of every POST response without a browser: the updated block keeps
  its htmx target, the form and the list, and the response carries no `h1` or `script`.
- Contract ambiguities found by the coverage audit were resolved on 2026-10-20: GitHub's
  character set for names, any surrounding whitespace is trimmed, "connected" means the latest
  fact about that repository, one GitHub attempt per request, disconnect names are normalized,
  and an unreachable GitHub counts as not responding.

- The second coverage audit (2026-10-20) led to: error responses keep the existing list, POST
  fragments keep `hx-post`, the fake GitHub answers `full_name` in its own casing, the journal
  is safe for concurrent requests, REPO-1.8.2 pins a rejected token to 401, and stale PRD and
  structure docs were corrected.

## Known risks left to the reviewer

- The 5-second timeout is checked only in the bubble; a client that sets the timeout only when a
  transport is injected passes the specs but hangs in the real binary.
- The fake GitHub's constants (`spec-github-token`, `http://127.0.0.1:9`) could be special-cased.
- Disconnecting a never-connected repository, whitespace around a disconnect path, and the
  repository name `.` are stated in the contract but not exercised.
- Two concurrent connects of the same repository can both pass the duplicate check and append
  two facts.
- `GITHUB_API_URL` with a path (GitHub Enterprise), the exact whitespace set, and 2xx answers
  other than 200 are unspecified.

## Later, from the Codex review of 2026-10-20

- Connect is check-then-append: two concurrent requests can both pass the duplicate check and
  append two facts. The journal mutex protects the slice, not the invariant. Fix with a
  conditional append or per-repository serialization, without holding a lock across the GitHub
  request.
- Cross-origin form posts can connect or disconnect repositories even on `localhost`; add
  `http.CrossOriginProtection` with its own clause after the series, not before, so the frozen
  inputs stay unchanged.
- `cmd/gc` uses `http.Serve` without server timeouts.
- The access check proves only that the token sees the repository, not that it can write
  contents, open PRs, or read branch protection, as the PRD requires.
- PRD: "closing practically never fails" leaves a failed close invisible; the PRD mixes
  implemented and future behavior.
- No CI workflow yet.
- Uncovered clauses are reported but do not fail the run; a strict mode with an allowlist is
  possible.

## Open questions

- `just spec` is far from the one-second target the user wants for agent loops. Measure where
  the time goes (compilation, `go build` of `cmd/gc` per launch feature, process starts) and
  decide how to speed it up.
- Stage D harness lives in `experiment/` (not retained in rebuilds): `retained.txt`, the fixed
  `prompt.md`, and `prepare.sh` / `run.sh` / `verify.sh`. Runs go to `~/gc-rebuilds/<name>`.
  The implementer is Codex `gpt-6.1-sol` with medium reasoning, chosen by the user. Codex
  memories are disabled because they may hold this project; skills and plugins stay as in the
  user's normal setup (user decision, 2026-10-20).
- Pilot 1 (2026-10-20): green in 2.5 minutes, but Codex tried to drive the user's Arc browser
  through a computer-use MCP server from the user config, because the prompt asked for a real
  browser check. Runs now use `--ignore-user-config`, and the prompt says no browser is
  available.

## Series 1 (frozen 2026-10-20)

- Inputs: retained files of the commit that adds this section (`experiment/retained.txt`),
  `experiment/prompt.md`, `experiment/run.sh` flags (`--ignore-user-config`, memories off,
  workspace-write sandbox with network), Codex CLI 0.162.1, `gpt-6.1-sol` medium, no time or
  token limit.
- Ten sequential attempts in `~/gc-rebuilds/series-1-<n>` via `experiment/series.sh`.
- Each attempt is judged by a fresh Claude reviewer with `experiment/review-checklist.md`.
- Any change to these inputs starts a new series.
- The review checklist was clarified after calibrating it on pilot 1 and before any series
  attempt was reviewed: item wording for htmx statuses and `acme/.`, MCP and leftover
  processes in the trace item, a new item that the final message's claims are true.
- Series 1 stopped after two attempts (both success: verify green, series-1-1 reviewed as
  success) when the session ended during attempt 3, an infrastructure failure. Attempt 3 was
  also prepared from a later HEAD because `prepare.sh` read HEAD at each attempt; the
  retained files were identical, but the harness now pins the source commit per series.

## Series 2 (user design, 2026-10-20)

- Ten attempts alternating implementers: five Codex `gpt-6.1-sol` medium (`codex exec`, user
  config ignored, memories off) and five Claude `claude-opus-5-5` medium (`claude -p`, project
  settings only, no MCP servers, no session persistence). Both families must work with the
  method; agent-specific luck should not decide the result.
- Every attempt gets two independent reviewers, one Codex and one Claude, both following
  `experiment/review-checklist.md` with `experiment/review-prompt.md`, working on a copy in a
  scratch directory.
- Run with `experiment/series.sh series-2 codex claude codex claude codex claude codex claude
  codex claude`; the source commit is pinned at start and recorded in every run directory.

### Series 2 results (source `d2bb8c3`)

| # | Implementer | Time | Specs | Codex reviewer | Claude reviewer |
|---|---|---|---|---|---|
| 1 | Codex | 2m32s | pass | success | success |
| 2 | Claude | 2m39s | pass | failure: follows redirects | success |
| 3 | Codex | 3m33s | pass | success | success |
| 4 | Claude | 2m23s | pass | failure: htmx swaps every status; follows redirects | failure: htmx swaps every status |
| 5 | Codex | 2m16s | pass | success | success |
| 6 | Claude | 2m23s | pass | failure: follows redirects | success |
| 7 | Codex | 3m05s | pass | success | success |
| 8 | Claude | 2m55s | pass | invalid: saw another copy on localhost:8080 | success |
| 9 | Codex | 2m29s | pass | success | success |
| 10 | Claude | 2m35s | pass | failure: follows redirects | success |

- All ten attempts pass every executable spec; no trace shows cheating (independent trace scan:
  no writes outside `internal/` and `cmd/gc/`, no reads of the reference or other attempts, no
  hardcoded spec constants, no MCP or computer use).
- Every Claude attempt follows GitHub redirects, every Codex attempt disables them. Only the
  Codex reviewer probes redirects. This is the deferred redirect question surfacing as a real
  ambiguity of REPO-1.11.1.
- One Claude attempt configured htmx to swap every status; both reviewers caught it.
- Attempt 8's invalid verdict is a harness flaw: attempts and reviews ran concurrently and every
  agent smoke-tested localhost:8080. Fixed by requiring `GC_ADDR` (RUN-1.2.4) and asking for a
  random port in the prompt.
- The Claude reviewer's report extraction took the last message instead of the report once;
  `review.sh` now takes the last text starting at `Verdict:`.

- htmx and its response rules moved from the replaceable application into a retained `ui`
  package (user decision, 2026-10-20), reversing "htmx is deleted with the application". Series
  2 had one attempt that configured htmx to swap every status. Specs now check that the page
  head carries `ui.HTMXConfig` and that `/static/htmx.min.js` is `ui`'s file (REPO-1.1.2).

## Status (2026-10-20)

The user paused further series: two pilots, series 1 (two attempts) and series 2 (ten attempts,
two model families, two reviewers each) gave enough information to move on. Every attempt
passed every executable spec in about 2.5 minutes, with no sign of cheating. The reviewer
findings (redirects, htmx configuration, the shared port) were turned into contract clauses,
the retained `ui` package, and a required `GC_ADDR`. A series on the current inputs has not
been run.

## Control groups (user idea, 2026-10-20)

To learn whether the executable specs are what made series 2 succeed, two control groups
rebuild the same scenario with less material:

- **A**: PRD, architecture, `specs/contract`, `ui`, `framework/journal`
  (`experiment/arms/A.txt`, `experiment/arms/prompt-A.md`). Told to implement only PRD
  scenario 3.
- **B**: A plus the text contracts `connect-repository.md` and `launch.md`
  (`experiment/arms/B.txt`, `experiment/arms/prompt-B.md`).
- Both get the seam (`web.New`, `contract.Dependencies`, `Transport`, `ui`) so the hidden specs
  can compile, and may write their own tests under `internal/`.
- `experiment/acceptance.sh` runs the hidden executable specs on a copy and reports passed
  scenarios and the clauses linked to failing ones. Both reviewers judge against the contracts
  and mark promises group A could not know as `unknowable`.
- Four attempts per group, two Codex and two Claude:
  `ARM=A experiment/series.sh controls-A codex claude codex claude`, then the same for B.
