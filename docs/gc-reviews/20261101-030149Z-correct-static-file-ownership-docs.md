## Finding

Two package descriptions incorrectly located static files in internal/web.

## Evidence

ui/ui.go embeds static assets and defines Static(); internal/web/web.go mounts ui.Static(). REPO-1.1.2 documents this same boundary.

## Selected Candidate

Correct both stale static-file ownership descriptions: directly verifiable, complete, and behavior-neutral.

## Rejected Candidates

The architecture guide's exclusive application-import claim is also stale but is a separate documentation issue. Potential framework request-copying changes lacked an established contract.

## Change

Changed one line in each of docs/architecture.md and docs/project-structure.md. Left changes uncommitted.

## Validation

just check passed with a temporary GOCACHE: generation, formatting, go vet, and all tests. git diff --check passed. The initial attempt was blocked by sandbox access to the default Go cache.

## Risk

Minimal; documentation only.

## Rollback

Revert the two documentation lines.

## Follow-up

None
