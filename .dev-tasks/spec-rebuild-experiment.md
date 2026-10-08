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
- User chose embedding the generic scenario instead of forwarding its methods; project-specific setup and expectations are separate statements around the common fluent chain.
- User requested complete migration: validation cases now exercise HTTP; htmx checks consume public HTTP responses from `specs`, and `internal` contains no test files.
- User approved removing `specs/htmx_test.go` and the technical contract added for it: retain feature HTML assertions and verify actual browser behavior during acceptance instead of testing htmx configuration details.
