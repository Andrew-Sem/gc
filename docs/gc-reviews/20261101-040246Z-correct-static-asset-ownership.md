## Finding

Package maps must reflect actual ownership. Both documentation maps incorrectly assigned static files to internal/web.

## Evidence

ui/ui.go embeds static assets and defines Static(); internal/web/web.go mounts ui.Static() at /static/. specs/connect-repository.md establishes this boundary, and existing scenarios check the served htmx file.

## Selected Candidate

Correct static asset ownership across both affected documents: directly verifiable, complete, and behavior preserving.

## Rejected Candidates

Considered repeated fact scans in domain.WakeUp, but found no demonstrated workload warranting optimization. Considered Request.WithForm map sharing versus WithHeader cloning, but compatibility intent was unestablished and regression coverage would require separately approved test edits.

## Change

Updated three lines across docs/architecture.md and docs/project-structure.md. Left changes uncommitted.

## Validation

just check passed with GOCACHE redirected to a writable temporary directory: generation, gofmt, go vet, templ formatting, and all tests. git diff --check passed. Initial check encountered sandbox restrictions on the default cache.

## Risk

Low; documentation only.

## Rollback

Revert the edits to the two documentation files.

## Follow-up

None
