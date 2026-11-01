## Finding

The architecture document presented unimplemented storage and execution guarantees as current.

## Evidence

cmd/gc/main.go wires framework/journal; its implementation stores a slice in memory. internal/web/serve.go starts agent runs directly without a task queue or transaction.

## Selected Candidate

Correct the current-versus-planned distinction throughout docs/architecture.md; direct implementation evidence supports this documentation-only fix.

## Rejected Candidates

Webhook changes touch excluded security paths. Request-builder consistency changes would require test changes and an established contract.

## Change

One file: 11 insertions, 1 deletion. Changes remain uncommitted.

## Validation

Passed just check with GOCACHE redirected to writable temporary storage after the default cache was denied. Formatting, go vet, template checks, and all Go tests passed. git diff --check passed.

## Risk

Low; documentation only.

## Rollback

Revert the edits to docs/architecture.md.

## Follow-up

None
