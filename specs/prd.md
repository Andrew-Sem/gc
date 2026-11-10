# Garbage Collector — PRD

Event-driven control plane over the PR lifecycle: ingesting facts from GitHub, an event journal,
a decision layer, effects.

Six scenarios in two groups. The system runs two on its own; a human runs four in the admin.
Human scenarios are carried out through admin pages: a form is submitted over HTTP, and the
server responds with HTML. Calling internal functions directly does not count as implementing a
scenario.

Each feature's contract details its scenario. Where the PRD and a contract differ, the contract
is right.

## The system works by two rules

1. **Hourly wake-up** — for repositories whose configured pace has come due: if there are fewer
   open PRs than the limit, open one PR.
2. **Every 6 hours** — for each open PR: if it can be merged, merge it; if not, record the
   reason.

**We do not fix or close PRs.** A PR that cannot be merged waits for a human: they fix it, merge
it, or close it, and the screen shows why it is stuck. We do not rerun CI: a flaky test will
sometimes eat good work, and reruns would bring attempt counters into the system.

The volume threshold is the only limit on production. A stuck PR keeps its place until a human
closes it, so a repository full of stuck PRs gets no new ones: the system becomes quieter, not
noisier.

---

## 1. Start new work

**Trigger:** the due tick after the hourly wake-up. The pace is set as a whole number of hours.
Saving a new pace starts a new interval; saving does not itself trigger a decision.

**Admission checks:** there are fewer open PRs than the limit from this tick's settings
snapshot. The same tick carries a snapshot of the agent settings and the OpenAI Base URL for the
future run; a task already created does not reread the model, reasoning, prompt, or Base URL
before running.

The number of open PRs comes from **our own journal**: we created those PRs and we see when they
are closed. We do not ask GitHub for it. A pleasant side effect: if a close webhook gets lost,
the count ends up too high — the system becomes more cautious, not more reckless.

**Approval does not reserve a slot.** A slot is taken only by an actually created PR. The time
Codex works passes between approval and the PR appearing, so the next tick may approve again:
the 13:00 tick approves, Codex works for 70 minutes, the 14:00 tick approves again. There will be
13 open with a threshold of 12.

This is deliberate: 12 is a health indicator, not a hard limit, and reservation would bring into
the domain a lifetime the domain cannot end on its own — a crashed process would hold the slot
forever. Protection against two concurrent runs on the same repository lives in the task
executor, not in the domain.

**If allowed:** `AdmissionApproved`, the executor launches Codex, which opens a PR.
**If not:** `AdmissionDeclined` with the numbers. A decline is always recorded — otherwise the
screen cannot answer why an hour went by for nothing.

If the agent fails before creating a PR, `AgentRunFinished` is recorded with the repository and
no PR.

**What the agent looks for:** for now a general prompt; the agent decides on its own what to
clean up. The prompt lives in the settings.

## 2. Bring a PR to merge

**Trigger:** a tick every 6 hours.

Before deciding, the executor obtains one terminal result of the current `sweepId` for each open
PR: either a full evidence snapshot or a decline because the lookup was incomplete.

Before the required-check lookup it rereads the current PR. If GitHub is still computing
`mergeable` or the head has changed, the fresh snapshot goes into the journal, and checks are
requested for the head from that response.

**The merge gate checks four conditions:**

1. The PR is open and not a draft
2. The head is the same one CI went green on
3. Required checks have passed
4. GitHub says `mergeable`

**Required checks are the ones from GitHub branch protection**, not a list of our own in the
config. Required identity is `context + appId`. Evidence contains the full required inventory
and all check-run attempts of the current head. For each identity, the latest attempt by
`createdAt`, then `id`, is normative. `success`, `neutral`, `skipped` pass; other completed
conclusions fail; incomplete statuses and a missing attempt wait.

If branch protection is not configured and there are no required checks, we do not merge.
Partial pagination is never treated as a full snapshot. An incomplete lookup blocks merging for
the current sweep only: the next sweep gathers its own evidence afresh, and until a new snapshot
is complete the screen keeps showing the last full one.

**If everything checks out:** `MergeApproved`, the effect calls GitHub. Success will arrive as an
observation. Failure — `MergeFailed`.
**If not:** `MergeDeclined` with the reason.

## 3. Connect a repository

garbage-collector acts on GitHub as a GitHub App, so its PRs and commits come from the app, not
from a person. The operator registers the app, installs it on the repositories it may work on,
and gives `gc` the app's ID and private key. The app needs to write contents and pull requests and
to read metadata; the merge sweep will also need to read checks and branch protection. It is not
given the workflows permission, so it cannot push workflow files even by mistake.

The operator chooses which of those repositories garbage-collector works with: a form checks that
the app is installed on the repository and writes `RepositoryConnected`; a button next to each
repository writes `RepositoryDisconnected`. Past facts stay in the journal; nobody touches or
closes open PRs. Details: the [connect contract](connect-repository.md) and the
[launch contract](launch.md).

The app's webhook, for `pull_request` and `check_run` events, points at one path and is signed
with the instance's webhook secret, so a repository needs no webhook of its own. Webhooks keep the "why is the PR stuck" screen fresh; a missing webhook does not break
merging, since the sweep requests evidence itself. Details: the
[webhook contract](receive-webhooks.md).

## 4. Configure the model and key

The OpenAI key comes from the environment, like the app's private key: it is not entered or shown in
the admin. Codex runs as a process with the key in its own environment and its own `CODEX_HOME`.

Only the fact that settings changed goes into the journal, never the key or the contents of
`auth.json` — the journal is forever.

The OpenAI Base URL is an optional setting. A non-empty value is normalized as an absolute
HTTP(S) URL without credentials, query, or fragment. An empty value means the official OpenAI
endpoint. The value in effect at the due tick is saved in the task's immutable snapshot and passed
to Codex; a later change affects only future tasks.

## 5. Configure pace and thresholds

Settings live on two levels:

| Level | What is configured |
|---|---|
| Instance | model and reasoning, agent prompt, optional OpenAI Base URL |
| Repository | how often we open PRs, maximum open, whether we merge |

Changes take effect **from the next wake-up or sweep**; nothing is recalculated retroactively.
Every domain tick carries the repository settings snapshot it used. Lowered the threshold from 12
to 5 while 9 are open — no new ones are opened until it drops to five, but nobody touches the
ones already open.

There are no roles: the instance has one owner.

## 6. Understand why a PR is stuck

This scenario is the reason for having our own screen. It shows the chain:

```
what is known  →  what was decided  →  why exactly  →  what next
```

The verdict is **recomputed on the fly**: the latest full required inventory for the current head
is overlaid with newer webhook observations of check runs. An interrupted new sweep does not
erase the screen's latest full snapshot. "It was red six hours ago" is useless if the test has
already been fixed. The cost: the verdict shown is a forecast, not a fact.

The screen refreshes on its own, without reloading the page by hand: the list polls the server
and replaces its HTML fragment.

---

## Facts

Seventeen facts in four groups. The formal types are in `specs/contract`, which is the source of
truth. Here are meaning and boundaries.

### Observations from GitHub

| Fact | When it occurs |
|---|---|
| `PrStateChanged` | We saw a PR snapshot, not older than the known one, that differs from it. Carries head, target branch, draft, open, mergeable, creation and update dates. |
| `CheckRunStateChanged` | We saw a newer check run state. Carries raw data: id, required identity, status, conclusion, and times. Whether it is required is not derived from the webhook. |
| `RequiredChecksObserved` | We fully obtained the required inventory and all attempts for one PR, head, and `sweepId`. A partial response cannot be this fact. |

`PrStateChanged` and `CheckRunStateChanged` are emitted only for our PRs and only if the
observation is not older than the known one. The delivery ID drops an exact duplicate at intake,
`updatedAt` protects against late delivery of an old state, and comparing snapshots protects
against a duplicate in content.

Webhook observations keep the live screen up to date but do not prove required checks are
complete. Merging is allowed only on the current sweep's `RequiredChecksObserved`.

### Our decisions and work results

| Fact | When it occurs |
|---|---|
| `PrCreated` | The agent opened a PR. Carries `reportId`, repository, number, branch, and first commit. Triggers reconciliation. |
| `AgentRunStarted` | Admission started an agent run. Carries the run number, repository, and start time. |
| `AgentRunFinished` | An agent run ended. Carries `reportId`, repository, optional PR, outcome, token usage, and, when there is no PR, the agent's explanation. A failure before the PR was created does not invent a number. |
| `RunRequested` | The operator pressed "Run now" for a repository; its decision is made at once instead of at the wake-up. |
| `SweepRequested` | The operator pressed "Sweep now" for a repository; its sweep runs at once instead of at the 6-hour tick. |
| `AdmissionApproved` | Admission allowed starting new work. |
| `MergeApproved` | The merge gate of the current `sweepId` allowed the merge. This is a decision, not an accomplished fact. |

A successful merge is not written as a separate fact: it will arrive as an observation. A human
closed our PR by hand — that is also `PrStateChanged` with `open: false`.

`AdmissionApproved` carries the agent settings from the due tick. It is a snapshot of a specific
attempt, not a reference to the mutable current settings.

### Failures and declines

| Fact | When it occurs |
|---|---|
| `MergeFailed` | The effect of a specific `sweepId` called GitHub, and the merge did not go through. Carries `reportId`. |
| `AdmissionDeclined` | The due tick passed, no PR was opened. Carries the reason and the numbers. |
| `MergeDeclined` | The sweep passed, the PR was not merged. Carries `sweepId` and the reason. |

### Connections and settings

| Fact | When it occurs |
|---|---|
| `RepositoryConnected` | The operator added `owner/name` in the admin, and the token access check passed. |
| `RepositoryDisconnected` | The operator removed a repository. No new ticks run for it; past facts remain. |
| `SettingsChanged` | Model, reasoning, prompt, and optional OpenAI Base URL were saved. |
| `RepositorySettingsChanged` | Production pace, maximum open, and merging were saved. |

Every report command and its corresponding fact carry a `reportId`. Repeating the same
`type + reportId` produces nothing. Different report IDs mean different real attempts, even if the
payload matches, but they do not cancel a command's semantic inapplicability: a second
`PrCreated` for an already known PR still produces nothing and does not trigger reconciliation
again.

<!-- FLAG: "There are no secrets in the journal" is fully covered by the next sentence ("never reach the journal, tasks, the interface, or logs"). Not cut: it may be a deliberate emphasis. -->

There are no secrets in the journal. The app's private key and the tokens made from it, the
OpenAI key, and the webhook secret come
from the environment, belong to the operator, and never reach the journal, tasks, the interface,
or logs. The OpenAI Base URL is not a secret: it goes into the task's immutable snapshot and into
`SettingsChanged`.

Connecting an already connected repository and disconnecting one that is not connected create no
facts. Disconnecting and reconnecting the same `owner/name` do not rewrite history: PRs opened
earlier stay in the journal and count again once the repository is connected.

---

## Delivery and recovery of observations

A single `check_run` webhook can relate to 0..N PRs and turns into as many commands as there are
unique associations. Lifecycle actions for PRs and `created`, `rerequested`, `completed` for check
runs are supported; other actions are ignored.

An early check webhook may arrive without a PR association or before `PrCreated`. That is why the
first accepted `PrCreated` always creates a `ReconcilePr` effect: the system requests the current
PR and all runs of its head, without relying on the old association.
