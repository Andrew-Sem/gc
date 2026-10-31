## Finding

Repository navigation docs omitted existing packages and assigned static files to the wrong package.

## Evidence

ui/ui.go embeds and serves static files; internal/web/web.go mounts ui.Static(). internal/web/serve.go owns scheduling and orchestration. Existing agent, GitHub, and spec harness packages substantiate the missing entries.

## Selected Candidate

Stale package inventories: directly verifiable against source, low risk, and completely fixable within two documentation files.

## Rejected Candidates

Repeated journal scans lack measured performance impact. Launch harness address allocation warrants separate investigation and test review.

## Change

Updated docs/architecture.md and docs/project-structure.md: 7 insertions, 5 deletions. Changes remain uncommitted.

## Validation

just check passed with GOCACHE redirected to a writable temporary directory: generation, formatting, go vet, and all tests. git diff --check passed.

## Risk

Low; documentation only.

## Rollback

Revert the two documentation diffs.

## Follow-up

None
