# Instance settings

This contract details the instance level of scenarios 4 and 5 of the [PRD](prd.md). For now it
holds one setting: the OpenAI Base URL. Model, reasoning, and prompt come later.

## Why

A local proxy can stand in for OpenAI. The operator points `gc` at it from the admin, without a
restart, and a run keeps the address it started with even if the operator changes it meanwhile.

## Glossary

**OpenAI Base URL** — the address Codex talks to instead of OpenAI's own. Empty means OpenAI's
official endpoint. It is not a secret.

**Normalized URL** — a submitted value with surrounding whitespace removed, its scheme and host
lowercased, and the slashes at the end of its path removed: ` HTTPS://Proxy.Example:8317/v1/ `
becomes `https://proxy.example:8317/v1`. An empty value stays empty.

**Valid URL** — empty, or an absolute `http` or `https` URL with a host and without credentials,
a query, or a fragment. A `?` or a `#` with nothing after it counts as a query or a fragment.

**Default** — the normalized value of `OPENAI_BASE_URL` when `gc` started (see the
[launch contract](launch.md)), or empty when it is unset.

**Value in effect** — the OpenAI Base URL in the latest `SettingsChanged`, or the default when
the journal has none. So an existing setup keeps working, and once the operator saves the
setting, the environment no longer matters.

**Settings page** — `/settings`: the form in `#instance-settings-panel`. It submits through htmx;
the response is the updated `#instance-settings-panel` with, if something went wrong, an error
message.

## Clauses

### INSTANCE-1. The settings page

#### INSTANCE-1.1. The page

INSTANCE-1.1.1. The repositories page links to the settings page as "Instance settings".

INSTANCE-1.1.2. The settings page answers 200, includes the shared interface head `ui.Head`, has
"Settings" in `h1`, and links to `/repositories`.

INSTANCE-1.1.3. The form has the field `openai_base_url`, filled with the value in effect, and a
"Save" button. It sends `POST /settings`.

INSTANCE-1.1.4. Opening the page writes no facts.

#### INSTANCE-1.2. Saving

INSTANCE-1.2.1. Saving a valid value writes `SettingsChanged` with the normalized value and the
time of saving. The answer is 200 with the updated `#instance-settings-panel`, the normalized
value in the form, and the text "Saved".

INSTANCE-1.2.2. Saving a value whose normalized form is the value in effect writes no fact and
answers like INSTANCE-1.2.1.

INSTANCE-1.2.3. Saving an empty value when the value in effect is not empty writes
`SettingsChanged` with an empty value: Codex then uses OpenAI's official endpoint, whatever the
environment says.

INSTANCE-1.2.4. Saving a value that is not a valid URL writes no fact, and the block shows the
error "Enter an absolute http or https URL without credentials, query, or fragment"
(response 422). The form keeps what the operator entered.

INSTANCE-1.2.5. A save from another site is refused like a connect request (REPO-5.1).

### INSTANCE-2. Using the setting

#### INSTANCE-2.1. The snapshot

INSTANCE-2.1.1. `AdmissionApproved` carries the value in effect at that decision, at a wake-up and
on a run request alike.

INSTANCE-2.1.2. The run that an approval starts gives the agent the value from that approval.
A setting saved while the run works changes nothing for it; the next decision carries the new
value.
