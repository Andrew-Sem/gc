## Finding

Package documentation contradicted the implemented static asset ownership and rebuild boundary.

## Evidence

ui/ui.go embeds static files and defines Static(); internal/web/web.go mounts ui.Static under /static/. specs/connect-repository.md explicitly specifies this arrangement.

## Selected Candidate

Stale static asset ownership descriptions: directly verifiable, complete, and safe to correct without behavior or test changes.

## Rejected Candidates

Framework request-builder consistency and experiment harness error handling were considered; changes would require behavioral decisions and test edits. No sufficiently grounded query optimization was found.

## Change

Updated three lines across docs/architecture.md and docs/project-structure.md, covering both stale ownership descriptions.

## Validation

GOCACHE="$TMPDIR/gc-go-build" just check passed generation, formatting, go vet, and all tests. git diff --check passed. The initial check encountered a sandbox restriction on the default Go cache.

## Risk

Minimal: documentation only. Changes remain uncommitted.

## Rollback

Revert the documentation diff in the two changed files.

## Follow-up

None
