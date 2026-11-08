# Connect a repository

This contract details scenario 3 of the [PRD](prd.md).

## Why

The operator chooses which repositories garbage-collector works with. The rest of the system
looks only at connected ones: it opens PRs and accepts webhooks for them. Those rules are
described in their own features.

## Glossary

**Repository** — an `owner/name` string, as in its GitHub URL.

**Access check** — a request to GitHub signed with the app's private key: the app is installed
on the repository.

**Connected repository** — one whose latest journal fact about that repository is
`RepositoryConnected`, not `RepositoryDisconnected`.

**Repositories page** — `/repositories`: the connect form and the list of connected
repositories. The form and buttons submit through htmx; the response is the updated block with
the list and, if something went wrong, an error message.

## Clauses

### REPO-1. Connecting a repository

#### REPO-1.1. Connect form

REPO-1.1.1. The repositories page shows a connect form with a single `owner/name` field.

REPO-1.1.2. Pages include the shared interface head `ui.Head` (htmx and its response rules),
and the application serves the files of `ui.Static` under `/static/`.

#### REPO-1.2. The access check passes

REPO-1.2.1. If the access check passes, `RepositoryConnected` with this `owner/name` is written
to the journal (response 200).

REPO-1.2.2. If the access check passes, the response shows the repository in the list.

#### REPO-1.3. The app is not installed

REPO-1.3.1. If the app is not installed on the repository or the repository does not exist, no
fact is written, and the page shows the error "The app is not installed on this repository"
(response 422).

REPO-1.3.2. GitHub reports that the app is not installed when it answers 404, or 403 that is not
a rate limit (REPO-1.10.2).

#### REPO-1.12. GitHub moved the repository

REPO-1.12.1. If GitHub answers with a redirect (301, 302, 307, or 308), for example because the
repository was renamed or transferred, the application does not follow it. No fact is written,
and the page shows the error "GitHub moved this repository. Enter its current name." (response
422).

#### REPO-1.4. The repository is already connected

REPO-1.4.1. Connecting an already connected repository writes no facts (response 200).

REPO-1.4.2. Connecting an already connected repository does not ask GitHub.

#### REPO-1.5. The name is malformed

REPO-1.5.1. If the string is not of the form `owner/name`, no fact is written, GitHub is not
asked, and the page shows the error "Enter the repository as owner/name" (response 422).

REPO-1.5.2. A string is of the form `owner/name` when it is two nonempty parts separated by
exactly one slash. The owner has only Latin letters, digits, and hyphens. The name has only
Latin letters, digits, hyphens, underscores, and dots, and is not `.` or `..`. Valid examples:
`acme/api`, `acme-co/api_v2.js`. Invalid examples: `acmeapi`, `/api`, `acme/`,
`acme/api/extra`, `ac me/api`, `acme/ap i`, `acme /api`, `acme/ api`, `acme/api?x`, `acme/a#b`,
`acme/a%20b`, `acme_co/api`, `acmé/api`, `acme/..`, and the empty string.

#### REPO-1.6. GitHub does not respond

REPO-1.6.1. If GitHub does not respond, no fact is written, and the page shows the error "Could
not check access: GitHub did not respond" (response 502).

REPO-1.6.2. The application waits 5 seconds for GitHub to answer. If no answer arrives in that
time, GitHub does not respond.

REPO-1.6.3. GitHub also does not respond when the connection to it cannot be made or breaks
before an answer.

#### REPO-1.7. The name is normalized

REPO-1.7.1. Before validating a submitted repository name, checking whether it is already
connected, or asking GitHub, remove surrounding whitespace (spaces, tabs, and line breaks) and
lowercase the name. For example, ` Acme/API ` becomes `acme/api`, and so does `acme/api` with a
tab before it and a line break after it.

REPO-1.7.2. The normalized name is used in the access check, the journal fact, and the
displayed repository name.

REPO-1.7.3. Submitting another casing or surrounding spaces for an already connected
repository follows REPO-1.4.

#### REPO-1.8. GitHub rejects the app

REPO-1.8.1. If GitHub rejects the app's credentials, no fact is written, and the page shows the
error "GitHub rejected the app. Check GITHUB_APP_ID and GITHUB_APP_PRIVATE_KEY_FILE."
(response 502).

REPO-1.8.2. GitHub rejects the app's credentials when it answers 401.

#### REPO-1.9. GitHub fails

REPO-1.9.1. If GitHub answers with any other error, for example 400, 500, 502, or 503, no fact
is written, and the page shows the error "Could not check access: GitHub failed. Try again
later." (response 502).

#### REPO-1.10. GitHub limits requests

REPO-1.10.1. If GitHub limits requests, no fact is written, and the page shows the error
"GitHub rate limit reached. Try again later." (response 502).

REPO-1.10.2. GitHub limits requests when it answers 429, or 403 with the header
`x-ratelimit-remaining: 0` or with a `retry-after` header.

#### REPO-1.11. One attempt

REPO-1.11.1. A connect request asks GitHub at most once. When GitHub rejects the app, fails,
limits requests, moves the repository, or does not respond, the error is shown without
retrying. This includes the repeat an HTTP client makes by itself when a reused connection breaks.

### REPO-2. Repository list

#### REPO-2.1. Listing

REPO-2.1.1. Connected repositories are shown on the page as a list.

REPO-2.1.2. Disconnected repositories are not shown.

REPO-2.1.3. Each row of the list reads as the repository name followed by its "Run now" button
(OPEN-3.1.1 of the [open pull requests contract](open-pull-requests.md)) and its "Disconnect"
button.

#### REPO-2.2. Order

REPO-2.2.1. The page and updated list responses show the most recently connected repository
first, following the order of connection facts in the journal rather than alphabetical order.

REPO-2.2.2. Disconnecting and reconnecting a repository moves it to the top of the list. The
remaining repositories keep their relative order.

REPO-2.2.3. Connecting an already connected repository leaves the list order unchanged,
including when the submitted name differs only in casing or surrounding spaces.

#### REPO-2.3. Updated block

REPO-2.3.1. The response to the connect form or a disconnect button is the updated block, not
a whole page: the connect form, the list, and the error if there is one.

### REPO-3. Disconnecting

#### REPO-3.1. Disconnect button

REPO-3.1.1. Each repository in the list has a disconnect button next to it.

#### REPO-3.2. Disconnecting a connected repository

REPO-3.2.1. Disconnecting writes `RepositoryDisconnected` (response 200).

REPO-3.2.2. The response no longer lists the disconnected repository; other repositories stay
listed.

REPO-3.2.3. The repository name in a disconnect request is normalized like a submitted name
(REPO-1.7.1), so disconnecting `Acme/API` disconnects `acme/api`.

#### REPO-3.3. Disconnecting a repository that is not connected

REPO-3.3.1. Disconnecting a repository that is not connected writes no facts (response 200).

#### REPO-3.4. History

REPO-3.4.1. Disconnecting and reconnecting do not rewrite history: new facts are appended
after the old ones.

### REPO-4. Secrets

#### REPO-4.1. GitHub credentials are never disclosed

GitHub credentials here are the app's private key and every token made from it.

REPO-4.1.1. GitHub credentials never reach the journal, even if GitHub returns them in an error
response.

REPO-4.1.2. GitHub credentials never reach the HTML, even if GitHub returns them in an error
response.

REPO-4.1.3. GitHub credentials never reach the logs, even if GitHub returns them in an error
response or quotes them in a malformed answer.

### REPO-5. Requests from other sites

#### REPO-5.1. Connecting and disconnecting

REPO-5.1.1. A connect or disconnect request with `Sec-Fetch-Site` other than `same-origin` or
`none` is refused (response 403). It writes no fact and does not ask GitHub, because a page the
operator visits could otherwise connect a repository and have the agent run on it.

REPO-5.1.2. A connect or disconnect request with no `Sec-Fetch-Site` and an `Origin` whose host is
not the request's host is refused the same way.

REPO-5.1.3. A connect request is handled as usual if its `Sec-Fetch-Site` is `same-origin` or
`none`, if it has no `Sec-Fetch-Site` and its `Origin` host is the request's host, or if it has
neither header.
