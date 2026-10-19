# Connect a repository

This contract details scenario 3 of the [PRD](prd.md).

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

### REPO-1. Connecting a repository

#### REPO-1.1. Connect form

REPO-1.1.1. The repositories page shows a connect form with a single `owner/name` field.

#### REPO-1.2. The access check passes

REPO-1.2.1. If the access check passes, `RepositoryConnected` with this `owner/name` is written
to the journal (response 200).

REPO-1.2.2. If the access check passes, the response shows the repository in the list.

#### REPO-1.3. The repository is inaccessible

REPO-1.3.1. If the repository is inaccessible or does not exist, no fact is written, and the
page shows the error "No access to this repository" (response 422).

#### REPO-1.4. The repository is already connected

REPO-1.4.1. Connecting an already connected repository writes no facts (response 200).

REPO-1.4.2. Connecting an already connected repository does not ask GitHub.

#### REPO-1.5. The name is malformed

REPO-1.5.1. If the string is not of the form `owner/name`, no fact is written, GitHub is not
asked, and the page shows the error "Enter the repository as owner/name" (response 422).

REPO-1.5.2. A string is of the form `owner/name` when both parts are nonempty, separated by
exactly one slash, with no spaces inside either part or next to the slash. Invalid examples:
`acmeapi`, `/api`, `acme/`, `acme/api/extra`, `ac me/api`, `acme/ap i`, `acme /api`,
`acme/ api`, and the empty string.

#### REPO-1.6. GitHub does not respond

REPO-1.6.1. If GitHub does not respond, no fact is written, and the page shows the error "Could
not check access: GitHub did not respond" (response 502).

#### REPO-1.7. The name is normalized

REPO-1.7.1. Before validating a submitted repository name, checking whether it is already
connected, or asking GitHub, remove surrounding spaces and lowercase the name. For example,
` Acme/API ` becomes `acme/api`.

REPO-1.7.2. The normalized name is used in the access check, the journal fact, and the
displayed repository name.

REPO-1.7.3. Submitting another casing or surrounding spaces for an already connected
repository follows REPO-1.4.

#### REPO-1.8. GitHub rejects the token

REPO-1.8.1. If GitHub rejects the token, no fact is written, and the page shows the error
"GitHub rejected the token. Check GITHUB_TOKEN." (response 502).

### REPO-2. Repository list

#### REPO-2.1. Listing

REPO-2.1.1. Connected repositories are shown on the page as a list.

REPO-2.1.2. Disconnected repositories are not shown.

#### REPO-2.2. Order

REPO-2.2.1. The page and updated list responses show the most recently connected repository
first, following the order of connection facts in the journal rather than alphabetical order.

REPO-2.2.2. Disconnecting and reconnecting a repository moves it to the top of the list. The
remaining repositories keep their relative order.

REPO-2.2.3. Connecting an already connected repository leaves the list order unchanged,
including when the submitted name differs only in casing or surrounding spaces.

### REPO-3. Disconnecting

#### REPO-3.1. Disconnect button

REPO-3.1.1. Each repository in the list has a disconnect button next to it.

#### REPO-3.2. Disconnecting a connected repository

REPO-3.2.1. Disconnecting writes `RepositoryDisconnected` (response 200).

REPO-3.2.2. The response no longer lists the disconnected repository; other repositories stay
listed.

#### REPO-3.3. Disconnecting a repository that is not connected

REPO-3.3.1. Disconnecting a repository that is not connected writes no facts (response 200).

#### REPO-3.4. History

REPO-3.4.1. Disconnecting and reconnecting do not rewrite history: new facts are appended
after the old ones.

### REPO-4. Secrets

#### REPO-4.1. The PAT is never disclosed

REPO-4.1.1. The PAT never reaches the journal, even if GitHub returns it in an error response.

REPO-4.1.2. The PAT never reaches the HTML, even if GitHub returns it in an error response.

REPO-4.1.3. The PAT never reaches the logs, even if GitHub returns it in an error response.
