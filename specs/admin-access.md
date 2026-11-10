# Admin access

This contract closes the admin of `gc` to everyone but the operator. It belongs to scenarios 3–6
of the [PRD](prd.md), whose pages it guards.

## Why

The admin and the public webhook share one address. A request without browser headers passes the
cross-site check of REPO-5.1, so without a login anyone who reaches the address could turn
merging on, raise the limit, or start runs. HTTP Basic is the login: the browser asks once and
sends the credentials with every request of the same origin, pages, panels, and the event stream
alike, and `gc` checks them in one place, with no session to sign, expire, or forge.

`gc` compares the credentials in constant time, so how long a refusal takes says nothing about
them.

## Glossary

**Operator's credentials** — the user in `GC_ADMIN_USER` and the password in `GC_ADMIN_PASSWORD`
(see the [launch contract](launch.md)).

**Admin request** — any request except `POST /webhooks/github`: pages, panels, the event stream,
forms, and `/static/`.

**Refusal** — response 401 with `WWW-Authenticate: Basic realm="gc", charset="UTF-8"` and the
text "Authentication required". The request is not handled further: it writes no fact and asks
GitHub nothing.

## Clauses

### ACCESS-1. Logging in

#### ACCESS-1.1. Admin requests

ACCESS-1.1.1. An admin request with the operator's credentials in an `Authorization: Basic`
header is handled as usual.

ACCESS-1.1.2. An admin request without an `Authorization` header gets a refusal.

ACCESS-1.1.3. An admin request with a wrong user, a wrong password, or both gets a refusal.

ACCESS-1.1.4. An admin request whose `Authorization` header is not `Basic`, or not valid Base64,
gets a refusal.

ACCESS-1.1.5. When the operator's user or password is empty, every admin request gets a refusal,
including one that sends an empty user and password: an unset login closes the admin, it never
opens it.

#### ACCESS-1.2. The webhook

ACCESS-1.2.1. `POST /webhooks/github` needs no credentials: its signature decides (HOOK-1 of the
[webhook contract](receive-webhooks.md)).

### ACCESS-2. Secrets

#### ACCESS-2.1. The credentials are never disclosed

ACCESS-2.1.1. The operator's password never reaches a response, including a refusal.

ACCESS-2.1.2. The operator's credentials never reach the journal file or the log.
