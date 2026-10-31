# Receive webhooks

This contract details the webhook part of scenario 3 of the [PRD](prd.md). This slice covers
`pull_request` events; `check_run` events come in a later slice.

## Why

GitHub tells us about changes to our PRs as they happen, so the "why is the PR stuck" screen
does not wait six hours for the next sweep. Anyone can send a request to a public path, so the
boundary trusts only requests signed with the instance's webhook secret.

## Glossary

**Webhook path** — `POST /webhooks/github`. The operator points every repository's webhook at
it.

**Webhook secret** — the value of `GC_WEBHOOK_SECRET` (see the [launch contract](launch.md)).
The operator sets the same value in each repository's webhook settings.

**Valid signature** — the `X-Hub-Signature-256` header equals `sha256=` followed by the
lowercase hex HMAC-SHA256 of the raw request body, keyed with the webhook secret.

**Delivery ID** — the `X-GitHub-Delivery` header. GitHub keeps it when it redelivers the same
event.

**Our PR** — a PR in a connected repository for which the journal has `PrCreated` with that
repository and number.

**PR snapshot** — what `PrStateChanged` carries, read from these fields of GitHub's event:
repository (`repository.full_name`), number (`pull_request.number`), head commit
(`pull_request.head.sha`), target branch (`pull_request.base.ref`), draft
(`pull_request.draft`), open (`pull_request.state`), mergeable (`pull_request.mergeable`: yes,
no, or unknown), creation date (`pull_request.created_at`), and update date
(`pull_request.updated_at`).

**Known snapshot** — the snapshot in the latest `PrStateChanged` for that PR.

## Clauses

### HOOK-1. Signature

#### HOOK-1.1. A valid signature

HOOK-1.1.1. A request with a valid signature is accepted (response 204), whatever happens to the
event afterwards.

#### HOOK-1.2. A missing or wrong signature

HOOK-1.2.1. A request without `X-Hub-Signature-256`, or with a value that is not a valid
signature, is rejected: no fact is written, and the response is 401 with the text "Invalid
signature".

HOOK-1.2.2. The signature is checked before anything else: a request with a wrong signature
gets 401 even if its body is not JSON or its delivery ID was seen before.

### HOOK-2. Deliveries

#### HOOK-2.1. A malformed delivery

HOOK-2.1.1. A request with a valid signature but without `X-GitHub-Delivery` or
`X-GitHub-Event` writes no fact (response 400).

HOOK-2.1.2. A `pull_request` request with a valid signature whose body is not JSON or lacks a
field of the PR snapshot writes no fact (response 400).

HOOK-2.1.3. A body over 25 MiB, GitHub's own limit, is refused without reading it to the end: no
fact is written (response 413). This is the one check that comes before the signature, because
the signature covers the whole body.

#### HOOK-2.2. A repeated delivery

HOOK-2.2.1. A delivery ID that was already accepted writes no fact (response 204), even if the
body differs.

HOOK-2.2.2. Delivery IDs are remembered while the application runs. After a restart, a repeat
is dropped by HOOK-3.3.

HOOK-2.2.3. The ID of a request that got 400 or 401 is not remembered, so a forged request cannot
block a real delivery, and a redelivery of a malformed one is processed.

#### HOOK-2.3. Events we do not use

HOOK-2.3.1. Events other than `pull_request`, for example `ping` or `push`, write no fact
(response 204).

HOOK-2.3.2. `pull_request` actions other than `opened`, `reopened`, `closed`, `synchronize`,
`edited`, `ready_for_review`, and `converted_to_draft` write no fact (response 204).

### HOOK-3. PR state

#### HOOK-3.1. A change to our PR

HOOK-3.1.1. A `pull_request` event about our PR whose snapshot is not older than the known one
and differs from it writes `PrStateChanged` with that snapshot (response 204).

HOOK-3.1.2. If the journal has no `PrStateChanged` for the PR yet, any snapshot of it is new.

HOOK-3.1.3. The repository name from the event is normalized like a submitted name
(REPO-1.7.1), so `Acme/API` matches the connected `acme/api`.

HOOK-3.1.4. GitHub's `mergeable: null` is recorded as unknown.

HOOK-3.1.5. GitHub's `state` decides whether the snapshot says the PR is open: `open` means
open, and `closed`, which also covers a merged PR, means closed.

#### HOOK-3.2. Not our PR

HOOK-3.2.1. An event about a repository that is not connected writes no fact (response 204).
This includes a repository that was disconnected after its PR was created.

HOOK-3.2.2. An event about a PR in a connected repository without `PrCreated` writes no fact
(response 204).

#### HOOK-3.3. An old or identical snapshot

HOOK-3.3.1. A snapshot with an update date earlier than the known one writes no fact (response
204).

HOOK-3.3.2. A snapshot that matches the known one in everything except the update date writes
no fact (response 204).

### HOOK-4. Secrets

#### HOOK-4.1. The webhook secret is never disclosed

HOOK-4.1.1. The webhook secret never reaches the journal.

HOOK-4.1.2. The webhook secret never reaches a response.

HOOK-4.1.3. The webhook secret never reaches the logs, including when a signature is rejected.
