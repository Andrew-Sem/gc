## Finding

The architecture guide described existing webhook parsing as future work and presented planned storage and execution components as current.

## Evidence

internal/web/webhooks.go already writes PrStateChanged; serve.go directly schedules agent runs; framework/journal stores facts in memory; both app.go and agent_run.go import application packages.

## Selected Candidate

Stale architecture descriptions: directly verifiable against local code and safe to correct without changing behavior.

## Rejected Candidates

Webhook contract gaps involve excluded security code and product behavior. Test harness changes would require separate specification review. Other code cleanup lacked sufficient evidence.

## Change

Updated docs/architecture.md only: 17 insertions and 4 deletions. Searched documentation for related stale claims and corrected the same pattern throughout the guide.

## Validation

just check passed with GOCACHE redirected to the writable temporary directory: generation, formatting, go vet, and all tests passed. git diff --check passed.

## Risk

Low; documentation only.

## Rollback

Revert the change to docs/architecture.md.

## Follow-up

None
