## Finding

Two package descriptions incorrectly placed static files in internal/web.

## Evidence

ui/ui.go embeds static assets and implements ui.Static; internal/web/web.go mounts it. specs/connect-repository.md specifies this arrangement.

## Selected Candidate

Correct misleading package ownership documentation: directly verifiable, complete, and low risk.

## Rejected Candidates

Storage wording mixes current and planned architecture, requiring broader interpretation. Request-builder map aliasing would need an explicitly authorized regression test.

## Change

Three documentation replacements across docs/architecture.md and docs/project-structure.md. Repository-wide search found no other instances of the same incorrect ownership claim. Changes remain uncommitted.

## Validation

just check passed with GOCACHE="$TMPDIR/gc-cleanup-go-cache": generation, formatting, vet, and all tests. The initial attempt encountered a sandbox restriction on the default cache. git diff --check passed.

## Risk

Minimal: documentation only.

## Rollback

Revert the three documentation replacements.

## Follow-up

None
