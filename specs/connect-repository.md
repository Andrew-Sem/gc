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
and the page shows the error "Enter the repository as owner/name" (response 422). Both parts
must be nonempty, separated by exactly one slash, with no spaces inside either part or next
to the slash. Invalid examples: `acmeapi`, `/api`, `acme/`, `acme/api/extra`, `ac me/api`,
`acme/ap i`, `acme /api`, `acme/ api`, and the empty string.

REPO-1.5. If GitHub does not respond, no fact is written, and the page shows the error "Could not
check access: GitHub did not respond" (response 502).

REPO-1.6. Before validating a submitted repository name, checking whether it is already
connected, or asking GitHub, remove surrounding spaces and lowercase the name. For example,
` Acme/API ` becomes `acme/api`. Use this normalized name in the access check, journal fact,
and displayed repository name. Submitting another casing or surrounding spaces for an
already connected repository follows REPO-1.3.

REPO-2. Connected repositories are shown on the page as a list.

REPO-2.1. The page and updated list responses show the most recently connected repository
first, following the order of connection facts in the journal rather than alphabetical order.

REPO-2.2. Disconnecting and reconnecting a repository moves it to the top of the list. The
remaining repositories keep their relative order.

REPO-2.3. Connecting an already connected repository leaves the list order unchanged,
including when the submitted name differs only in casing or surrounding spaces.

REPO-3. Each repository in the list has a disconnect button next to it.

REPO-3.1. Disconnecting writes `RepositoryDisconnected`, and the repository disappears from the
list (response 200).

REPO-3.2. Disconnecting a repository that is not connected writes no facts (response 200).

REPO-3.3. Disconnecting and reconnecting do not rewrite history: new facts are appended after
the old ones.

REPO-4. The PAT never reaches the journal, the HTML, or the logs — even if GitHub returns it in
an error response.
