## Finding

Both guides incorrectly assigned static files to internal/web, violating their purpose as accurate package maps.

## Evidence

ui/ui.go embeds static assets and defines Static(); internal/web/web.go mounts ui.Static(). specs/connect-repository.md documents this same boundary.

## Selected Candidate

Stale static asset ownership documentation: directly verifiable, complete, and safe to fix.

## Rejected Candidates

Request form cloning asymmetry lacked an established immutability contract. Other architecture documentation discrepancies require separate scope and interpretation.

## Change

Updated three lines across docs/architecture.md and docs/project-structure.md. Left changes uncommitted.

## Validation

just check passed with GOCACHE redirected to a writable temporary directory: generation, formatting, go vet, and all tests. git diff --check passed.

## Risk

Minimal; documentation only.

## Rollback

Revert the edits in the two documentation files.

## Follow-up

None
