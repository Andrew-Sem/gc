import type {
  CheckConclusion,
  CheckRun,
  CheckStatus,
  PrSnapshot,
  RequiredCheck,
  Timestamp,
} from "@workspace/domain"

type JsonObject = Record<string, unknown>

const CHECK_STATUSES = new Set<CheckStatus>([
  "queued",
  "in_progress",
  "completed",
  "waiting",
  "requested",
  "pending",
])

const CHECK_CONCLUSIONS = new Set<CheckConclusion>([
  "success",
  "failure",
  "neutral",
  "cancelled",
  "timed_out",
  "action_required",
  "stale",
  "skipped",
])

export type ParsedPullRequest = {
  readonly number: number
  readonly snapshot: PrSnapshot
}

export type ParsedCheckRun = {
  readonly headSha: string
  readonly run: CheckRun
}

export function isObject(value: unknown): value is JsonObject {
  return typeof value === "object" && value !== null && !Array.isArray(value)
}

export function nonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.length > 0
}

export function positiveInteger(value: unknown): value is number {
  return Number.isSafeInteger(value) && typeof value === "number" && value > 0
}

export function finiteTimestamp(value: unknown): value is Timestamp {
  return typeof value === "number" && Number.isFinite(value)
}

function parseTimestamp(value: unknown): Timestamp | undefined {
  if (typeof value !== "string") {
    return undefined
  }

  const timestamp = Date.parse(value)
  return Number.isFinite(timestamp) ? timestamp : undefined
}

function parseNullableTimestamp(value: unknown): Timestamp | null | undefined {
  return value === null ? null : parseTimestamp(value)
}

function parseStatus(value: unknown): CheckStatus | undefined {
  return typeof value === "string" && CHECK_STATUSES.has(value as CheckStatus)
    ? (value as CheckStatus)
    : undefined
}

function parseConclusion(value: unknown): CheckConclusion | null | undefined {
  if (value === null) {
    return null
  }

  return typeof value === "string" &&
    CHECK_CONCLUSIONS.has(value as CheckConclusion)
    ? (value as CheckConclusion)
    : undefined
}

export function parsePullRequest(value: unknown): ParsedPullRequest | null {
  if (!isObject(value)) {
    return null
  }

  const head = value.head
  const base = value.base
  const createdAt = parseTimestamp(value.created_at)
  const updatedAt = parseTimestamp(value.updated_at)

  if (
    !positiveInteger(value.number) ||
    typeof value.draft !== "boolean" ||
    (value.state !== "open" && value.state !== "closed") ||
    (value.mergeable !== null && typeof value.mergeable !== "boolean") ||
    !isObject(head) ||
    !nonEmptyString(head.sha) ||
    !isObject(base) ||
    !nonEmptyString(base.ref) ||
    createdAt === undefined ||
    updatedAt === undefined
  ) {
    return null
  }

  return {
    number: value.number,
    snapshot: {
      headSha: head.sha,
      baseRef: base.ref,
      draft: value.draft,
      open: value.state === "open",
      mergeable: value.mergeable,
      createdAt,
      updatedAt,
    },
  }
}

export function checkRunHead(value: unknown): string | null {
  return isObject(value) && nonEmptyString(value.head_sha)
    ? value.head_sha
    : null
}

export function parseCheckRun(value: unknown): ParsedCheckRun | null {
  if (!isObject(value)) {
    return null
  }

  const app = value.app
  const status = parseStatus(value.status)
  const conclusion = parseConclusion(value.conclusion)
  const createdAt = parseTimestamp(value.created_at)
  const updatedAt = parseTimestamp(value.updated_at)
  const startedAt = parseNullableTimestamp(value.started_at)
  const completedAt = parseNullableTimestamp(value.completed_at)

  if (
    !positiveInteger(value.id) ||
    !nonEmptyString(value.name) ||
    !isObject(app) ||
    !positiveInteger(app.id) ||
    status === undefined ||
    conclusion === undefined ||
    createdAt === undefined ||
    updatedAt === undefined ||
    startedAt === undefined ||
    completedAt === undefined ||
    !nonEmptyString(value.head_sha)
  ) {
    return null
  }

  return {
    headSha: value.head_sha,
    run: {
      id: value.id,
      check: { context: value.name, appId: app.id },
      status,
      conclusion,
      createdAt,
      updatedAt,
      startedAt,
      completedAt,
    },
  }
}

export function parseRequiredCheck(value: unknown): RequiredCheck | null {
  if (
    !isObject(value) ||
    !nonEmptyString(value.context) ||
    !positiveInteger(value.app_id)
  ) {
    return null
  }

  return { context: value.context, appId: value.app_id }
}
