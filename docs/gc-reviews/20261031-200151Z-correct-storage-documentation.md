## Finding

The stack table incorrectly presented SQLite as implemented storage.

## Evidence

cmd/gc/main.go constructs journal.Journal[contract.Fact]; framework/journal/journal.go stores facts in a slice. The architecture introduction already describes SQLite as planned.

## Selected Candidate

Correct the storage claim: directly verifiable, complete, and limited to documentation.

## Rejected Candidates

GitHub client request duplication touches excluded token handling. Broader target-architecture descriptions do not establish a sufficiently clear defect.

## Change

Changed one row in docs/architecture.md. Searched repository-wide for SQLite and persistence references; other SQLite references already describe planned work. Changes remain uncommitted.

## Validation

just check passed with GOCACHE redirected to a writable temporary directory: generation, formatting, go vet, and all tests. git diff --check passed. The initial run was blocked by sandbox access to the default Go cache.

## Risk

Minimal; documentation only.

## Rollback

Restore the previous storage table row.

## Follow-up

None
