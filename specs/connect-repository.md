# Connect a repository

This contract details scenario 3 of the [PRD](prd.md). Its specs live in
`connect_repository_test.go` and refer to clauses through `Covers`. `just spec` prints the
clauses that have no spec yet.

## Why

The operator chooses which repositories garbage-collector works with. The rest of the system
looks only at connected ones: it opens PRs and accepts webhooks for them. Those rules are
described in their own features.

## Glossary

**Repository** — an `owner/name` string, as in its GitHub URL.

**Access check** — a request to GitHub with the PAT from the environment: the repository exists
and the token can see it.

**Connected repository** — one whose latest journal fact is `RepositoryConnected`, not
`RepositoryDisconnected`.

**Repositories page** — `/repositories`: the connect form and the list of connected
repositories. The form and buttons submit through htmx; the response is the updated block with
the list and, if something went wrong, an error message.

## Clauses

REPO-1. The repositories page shows a connect form with a single `owner/name` field.

REPO-1.1. If the access check passes, `RepositoryConnected` with this `owner/name` is written to
the journal, and the repository appears in the list (response 200).

REPO-1.2. If the repository is inaccessible or does not exist, no fact is written, and the page
shows the error "No access to this repository" (response 422).

REPO-1.3. Connecting an already connected repository writes no facts and does not ask GitHub
(response 200).

REPO-1.4. If the string is not of the form `owner/name`, no fact is written, GitHub is not asked,
and the page shows the error "Enter the repository as owner/name" (response 422).

REPO-1.5. If GitHub does not respond, no fact is written, and the page shows the error "Could not
check access: GitHub did not respond" (response 502).

REPO-2. Connected repositories are shown on the page as a list.

REPO-3. Each repository in the list has a disconnect button next to it.

REPO-3.1. Disconnecting writes `RepositoryDisconnected`, and the repository disappears from the
list (response 200).

REPO-3.2. Disconnecting a repository that is not connected writes no facts (response 200).

REPO-3.3. Disconnecting and reconnecting do not rewrite history: new facts are appended after
the old ones.

REPO-4. The PAT never reaches the journal, the HTML, or the logs — even if GitHub returns it in
an error response.
