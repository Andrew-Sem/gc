import type { DomainCommand } from "@workspace/domain"
import type { ParseContext, WebhookDelivery } from "./contract"
import {
  finiteTimestamp,
  isObject,
  nonEmptyString,
  parseCheckRun,
  parsePullRequest,
  positiveInteger,
} from "./github-values"

const PULL_REQUEST_ACTIONS = new Set([
  "opened",
  "reopened",
  "synchronize",
  "converted_to_draft",
  "ready_for_review",
  "edited",
  "closed",
])

const CHECK_RUN_ACTIONS = new Set(["created", "rerequested", "completed"])

function validContext(context: ParseContext): boolean {
  return nonEmptyString(context.tenant) && finiteTimestamp(context.at)
}

function commandsOfPullRequest(
  payload: unknown,
  context: ParseContext
): readonly DomainCommand[] {
  if (
    !isObject(payload) ||
    !nonEmptyString(payload.action) ||
    !PULL_REQUEST_ACTIONS.has(payload.action) ||
    !isObject(payload.repository) ||
    !nonEmptyString(payload.repository.full_name)
  ) {
    return []
  }

  const pullRequest = parsePullRequest(payload.pull_request)
  if (pullRequest === null) {
    return []
  }

  return [
    {
      type: "ObservePr",
      pr: {
        tenant: context.tenant,
        repo: payload.repository.full_name,
        number: pullRequest.number,
      },
      snapshot: pullRequest.snapshot,
      at: context.at,
    },
  ]
}

function commandsOfCheckRun(
  payload: unknown,
  context: ParseContext
): readonly DomainCommand[] {
  if (
    !isObject(payload) ||
    !nonEmptyString(payload.action) ||
    !CHECK_RUN_ACTIONS.has(payload.action) ||
    !isObject(payload.repository) ||
    !nonEmptyString(payload.repository.full_name)
  ) {
    return []
  }

  const parsed = parseCheckRun(payload.check_run)
  if (parsed === null || !isObject(payload.check_run)) {
    return []
  }

  const associations = payload.check_run.pull_requests
  if (!Array.isArray(associations)) {
    return []
  }

  const seen = new Set<number>()
  const commands: DomainCommand[] = []
  for (const association of associations) {
    if (
      !isObject(association) ||
      !positiveInteger(association.number) ||
      seen.has(association.number)
    ) {
      continue
    }

    seen.add(association.number)
    commands.push({
      type: "ObserveCheckRun",
      pr: {
        tenant: context.tenant,
        repo: payload.repository.full_name,
        number: association.number,
      },
      headSha: parsed.headSha,
      run: parsed.run,
      at: context.at,
    })
  }

  return commands
}

/** Чистый разбор одной доставки в ноль или несколько команд домена. */
export function commandsOf(
  delivery: WebhookDelivery,
  context: ParseContext
): readonly DomainCommand[] {
  if (
    !nonEmptyString(delivery.id) ||
    !nonEmptyString(delivery.event) ||
    !validContext(context)
  ) {
    return []
  }

  if (delivery.event === "pull_request") {
    return commandsOfPullRequest(delivery.payload, context)
  }

  if (delivery.event === "check_run") {
    return commandsOfCheckRun(delivery.payload, context)
  }

  return []
}
