## Finding

Both documentation maps incorrectly assigned static files to internal/web.

## Evidence

ui/ui.go embeds static assets and implements Static(); internal/web/web.go registers GET /static/ using ui.Static().

## Selected Candidate

Stale static asset ownership documentation: directly contradicted by code and safely corrected across both instances.

## Rejected Candidates

The architecture document's exclusive application-import claim is also stale but is a separate cleanup. Request-builder mutation and experiment-script failure handling would require further contract and test review.

## Change

Changed three lines across docs/architecture.md and docs/project-structure.md. Left changes uncommitted.

## Validation

just check passed with GOCACHE redirected to a writable temporary directory: generation, formatting, go vet, and all tests passed. git diff --check passed.

## Risk

Minimal; documentation only.

## Rollback

Revert the edits in the two documentation files.

## Follow-up

None
