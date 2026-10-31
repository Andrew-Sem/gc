## Finding

Both package maps incorrectly assigned static files to internal/web, misleading repository navigation.

## Evidence

ui/ui.go embeds static files and implements Static(); internal/web/web.go mounts ui.Static() at GET /static/.

## Selected Candidate

Stale static-file ownership documentation: directly verifiable, repeated in two documents, and safe to fix completely.

## Rejected Candidates

Repeated GitHub request construction: no demonstrated defect and involves credential handling. Spec harness changes: no established regression and tests require explicit review. Other architecture statements: deferred where intended future scope was ambiguous.

## Change

Updated three lines across docs/architecture.md and docs/project-structure.md. Changes remain uncommitted.

## Validation

just check passed with GOCACHE redirected to writable temporary storage: generation, formatting, go vet, and all tests passed. git diff --check passed. The initial run encountered a sandbox restriction on the default cache.

## Risk

Minimal; documentation only.

## Rollback

Revert the three documentation line changes.

## Follow-up

None
