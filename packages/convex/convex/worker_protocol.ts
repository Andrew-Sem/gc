import type {
  CheckConclusion,
  CheckRun,
  CheckStatus,
  DomainCommand,
  Effect,
  LookupFailure,
  MergeFailure,
  PrRef,
  PrSnapshot,
  RequiredCheck,
} from "@workspace/domain"

type LookupRequiredChecksEffect = {
  readonly type: "LookupRequiredChecks"
  readonly sweepId: string
  readonly pr: PrRef
  readonly headSha: string
  readonly now: number
  readonly settings: unknown
}

type StartCodexDeviceLoginEffect = {
  readonly type: "StartCodexDeviceLogin"
  readonly tenant: string
}

type WorkerEffect =
  | Effect
  | LookupRequiredChecksEffect
  | StartCodexDeviceLoginEffect

type WorkerReportCommand = Extract<
  DomainCommand,
  {
    type:
      | "ReportRequiredChecks"
      | "ReportLookupFailure"
      | "ReportPrCreated"
      | "ReportAgentRun"
      | "ReportMergeFailure"
      | "ReportCodexConnected"
  }
>

export type WorkerProtocolTask = {
  readonly effect: unknown
  readonly tenant: string
  readonly repo?: string
  readonly lastCommand?: unknown
  readonly acceptedReports?: readonly unknown[]
  readonly observedHeadSha?: string
}

export type AuthorizedWorkerCommand = {
  readonly command: DomainCommand
  readonly replayed?: boolean
  readonly acceptedReports?: readonly DomainCommand[]
  readonly observedHeadSha?: string
}

export function assertWorkerTaskScope(task: WorkerProtocolTask): void {
  assertTaskScope(task, workerEffect(task.effect))
}

const CHECK_STATUSES: readonly CheckStatus[] = [
  "queued",
  "in_progress",
  "completed",
  "waiting",
  "requested",
  "pending",
]
const CHECK_CONCLUSIONS: readonly CheckConclusion[] = [
  "success",
  "failure",
  "neutral",
  "cancelled",
  "timed_out",
  "action_required",
  "stale",
  "skipped",
]
const LOOKUP_FAILURES: readonly LookupFailure[] = [
  "rate_limited",
  "forbidden",
  "upstream_error",
  "page_limit",
]
const MERGE_FAILURES: readonly MergeFailure[] = [
  "head_changed",
  "not_mergeable",
  "forbidden",
  "upstream_error",
]

export function authorizeWorkerCommand(
  taskId: string,
  task: WorkerProtocolTask,
  input: unknown
): AuthorizedWorkerCommand {
  const command = workerCommand(input)
  const effect = workerEffect(task.effect)
  assertTaskScope(task, effect)

  const acceptedReports = task.acceptedReports?.map(workerReportCommand) ?? []
  const reportId = commandReportId(command)
  if (reportId !== null) {
    const previous = acceptedReports.find(
      (candidate) => commandReportId(candidate) === reportId
    )
    if (previous) {
      if (!sameReport(previous, command)) {
        throw new Error("worker report conflicts with an accepted report")
      }
      return { command, replayed: true }
    }
  }

  switch (effect.type) {
    case "RunAgent":
      authorizeAgentCommand(taskId, effect, acceptedReports, command)
      break
    case "ReconcilePr":
      authorizeReconciliationCommand(task, effect, command)
      break
    case "LookupRequiredChecks":
      authorizeLookupCommand(taskId, task, effect, acceptedReports, command)
      break
    case "MergePr":
      authorizeMergeCommand(taskId, effect, command)
      break
    case "StartCodexDeviceLogin":
      authorizeDeviceLoginCommand(taskId, effect, command)
      break
    case "ClosePr":
      throw new Error("close task does not accept domain commands")
  }

  return {
    command,
    ...(reportId === null
      ? {}
      : { acceptedReports: [...acceptedReports, command] }),
    ...(command.type === "ObservePr"
      ? { observedHeadSha: command.snapshot.headSha }
      : {}),
  }
}

function authorizeAgentCommand(
  taskId: string,
  effect: Extract<Effect, { type: "RunAgent" }>,
  accepted: readonly WorkerReportCommand[],
  command: DomainCommand
): void {
  const created = accepted.find(
    (
      candidate
    ): candidate is Extract<WorkerReportCommand, { type: "ReportPrCreated" }> =>
      candidate.type === "ReportPrCreated"
  )
  const finished = accepted.some(
    (candidate) => candidate.type === "ReportAgentRun"
  )

  if (command.type === "ReportPrCreated") {
    if (
      command.reportId !== stableReportId(taskId, "pr-created") ||
      !prBelongsTo(command.pr, effect.tenant, effect.repo) ||
      finished
    ) {
      throw new Error("report is not authorized for the agent task")
    }
    return
  }

  if (command.type === "ReportAgentRun") {
    if (
      command.reportId !== stableReportId(taskId, "agent-run") ||
      command.tenant !== effect.tenant ||
      command.repo !== effect.repo ||
      (created === undefined
        ? command.pr !== null
        : command.pr === null || !samePr(command.pr, created.pr))
    ) {
      throw new Error("report is not authorized for the agent task")
    }
    return
  }

  throw new Error("command is not authorized for the agent task")
}

function authorizeReconciliationCommand(
  task: WorkerProtocolTask,
  effect: Extract<Effect, { type: "ReconcilePr" }>,
  command: DomainCommand
): void {
  if (command.type === "ObservePr") {
    if (
      !samePr(command.pr, effect.pr) ||
      (task.observedHeadSha !== undefined &&
        task.observedHeadSha !== command.snapshot.headSha)
    ) {
      throw new Error("observation is not authorized for reconciliation")
    }
    return
  }
  if (
    command.type === "ObserveCheckRun" &&
    samePr(command.pr, effect.pr) &&
    task.observedHeadSha !== undefined &&
    command.headSha === task.observedHeadSha
  ) {
    return
  }
  throw new Error("command is not authorized for reconciliation")
}

function authorizeLookupCommand(
  taskId: string,
  task: WorkerProtocolTask,
  effect: LookupRequiredChecksEffect,
  accepted: readonly WorkerReportCommand[],
  command: DomainCommand
): void {
  const terminalReported = accepted.some(
    (candidate) =>
      candidate.type === "ReportRequiredChecks" ||
      candidate.type === "ReportLookupFailure"
  )
  if (terminalReported) {
    throw new Error("checks lookup already has a terminal report")
  }
  if (command.type === "ObservePr") {
    if (
      !samePr(command.pr, effect.pr) ||
      (task.observedHeadSha !== undefined &&
        task.observedHeadSha !== command.snapshot.headSha)
    ) {
      throw new Error("observation is not authorized for checks lookup")
    }
    return
  }

  const reportId = stableReportId(taskId, "required-checks")
  if (
    command.type === "ReportRequiredChecks" &&
    command.reportId === reportId &&
    command.sweepId === effect.sweepId &&
    samePr(command.pr, effect.pr) &&
    command.headSha === (task.observedHeadSha ?? effect.headSha)
  ) {
    return
  }
  if (
    command.type === "ReportLookupFailure" &&
    command.reportId === reportId &&
    command.scope.kind === "checks" &&
    command.scope.sweepId === effect.sweepId &&
    samePr(command.scope.pr, effect.pr) &&
    command.scope.headSha === effect.headSha
  ) {
    return
  }
  throw new Error("report is not authorized for checks lookup")
}

function authorizeMergeCommand(
  taskId: string,
  effect: Extract<Effect, { type: "MergePr" }>,
  command: DomainCommand
): void {
  if (
    command.type !== "ReportMergeFailure" ||
    command.reportId !== stableReportId(taskId, "merge-failure") ||
    command.sweepId !== effect.sweepId ||
    !samePr(command.pr, effect.pr) ||
    command.headSha !== effect.headSha
  ) {
    throw new Error("report is not authorized for merge")
  }
}

function authorizeDeviceLoginCommand(
  taskId: string,
  effect: StartCodexDeviceLoginEffect,
  command: DomainCommand
): void {
  if (
    command.type !== "ReportCodexConnected" ||
    command.reportId !== stableReportId(taskId, "codex-subscription") ||
    command.tenant !== effect.tenant ||
    command.method !== "subscription"
  ) {
    throw new Error("report is not authorized for device login")
  }
}

function assertTaskScope(task: WorkerProtocolTask, effect: WorkerEffect): void {
  const tenant =
    effect.type === "RunAgent"
      ? effect.tenant
      : effect.type === "StartCodexDeviceLogin"
        ? effect.tenant
        : effect.pr.tenant
  const repo =
    effect.type === "RunAgent"
      ? effect.repo
      : effect.type === "StartCodexDeviceLogin"
        ? undefined
        : effect.pr.repo
  if (task.tenant !== tenant || task.repo !== repo) {
    throw new Error("task scope does not match its effect")
  }
}

function workerEffect(input: unknown): WorkerEffect {
  if (!isRecord(input) || typeof input.type !== "string") {
    throw new Error("invalid worker task effect")
  }
  switch (input.type) {
    case "RunAgent":
      if (
        exactKeys(input, ["type", "tenant", "repo", "tenantSettings"]) &&
        typeof input.tenant === "string" &&
        typeof input.repo === "string" &&
        isRecord(input.tenantSettings)
      ) {
        return input as WorkerEffect
      }
      break
    case "ReconcilePr":
      if (
        exactKeys(input, ["type", "pr", "headSha"]) &&
        isPrRef(input.pr) &&
        typeof input.headSha === "string"
      ) {
        return input as WorkerEffect
      }
      break
    case "LookupRequiredChecks":
      if (
        exactKeys(input, [
          "type",
          "sweepId",
          "pr",
          "headSha",
          "now",
          "settings",
        ]) &&
        typeof input.sweepId === "string" &&
        isPrRef(input.pr) &&
        typeof input.headSha === "string" &&
        isTimestamp(input.now) &&
        isRecord(input.settings)
      ) {
        return input as WorkerEffect
      }
      break
    case "MergePr":
      if (
        exactKeys(input, ["type", "sweepId", "pr", "headSha"]) &&
        typeof input.sweepId === "string" &&
        isPrRef(input.pr) &&
        typeof input.headSha === "string"
      ) {
        return input as WorkerEffect
      }
      break
    case "ClosePr":
      if (
        exactKeys(input, ["type", "sweepId", "pr"]) &&
        typeof input.sweepId === "string" &&
        isPrRef(input.pr)
      ) {
        return input as WorkerEffect
      }
      break
    case "StartCodexDeviceLogin":
      if (
        exactKeys(input, ["type", "tenant"]) &&
        typeof input.tenant === "string"
      ) {
        return input as WorkerEffect
      }
      break
  }
  throw new Error("invalid worker task effect")
}

function workerReportCommand(input: unknown): WorkerReportCommand {
  const command = workerCommand(input)
  if (commandReportId(command) === null) {
    throw new Error("invalid accepted worker report")
  }
  return command as WorkerReportCommand
}

function workerCommand(input: unknown): DomainCommand {
  if (!isRecord(input) || typeof input.type !== "string") {
    throw new Error("invalid worker command")
  }
  switch (input.type) {
    case "ObservePr":
      if (
        exactKeys(input, ["type", "pr", "snapshot", "at"]) &&
        isPrRef(input.pr) &&
        isPrSnapshot(input.snapshot) &&
        isTimestamp(input.at)
      ) {
        return input as DomainCommand
      }
      break
    case "ObserveCheckRun":
      if (
        exactKeys(input, ["type", "pr", "headSha", "run", "at"]) &&
        isPrRef(input.pr) &&
        typeof input.headSha === "string" &&
        isCheckRun(input.run) &&
        isTimestamp(input.at)
      ) {
        return input as DomainCommand
      }
      break
    case "ReportRequiredChecks":
      if (
        exactKeys(input, [
          "type",
          "reportId",
          "sweepId",
          "pr",
          "headSha",
          "snapshot",
          "at",
        ]) &&
        typeof input.reportId === "string" &&
        typeof input.sweepId === "string" &&
        isPrRef(input.pr) &&
        typeof input.headSha === "string" &&
        isRequiredChecksSnapshot(input.snapshot) &&
        isTimestamp(input.at)
      ) {
        return input as DomainCommand
      }
      break
    case "ReportLookupFailure":
      if (
        exactKeys(input, ["type", "reportId", "scope", "failure", "at"]) &&
        typeof input.reportId === "string" &&
        isChecksLookupScope(input.scope) &&
        oneOf(input.failure, LOOKUP_FAILURES) &&
        isTimestamp(input.at)
      ) {
        return input as DomainCommand
      }
      break
    case "ReportPrCreated":
      if (
        exactKeys(input, [
          "type",
          "reportId",
          "pr",
          "headSha",
          "branch",
          "at",
        ]) &&
        typeof input.reportId === "string" &&
        isPrRef(input.pr) &&
        typeof input.headSha === "string" &&
        typeof input.branch === "string" &&
        isTimestamp(input.at)
      ) {
        return input as DomainCommand
      }
      break
    case "ReportAgentRun":
      if (
        exactKeys(input, [
          "type",
          "reportId",
          "tenant",
          "repo",
          "pr",
          "outcome",
          "usage",
          "at",
        ]) &&
        typeof input.reportId === "string" &&
        typeof input.tenant === "string" &&
        typeof input.repo === "string" &&
        (input.pr === null || isPrRef(input.pr)) &&
        oneOf(input.outcome, ["finished", "failed", "timed_out"] as const) &&
        isAgentUsage(input.usage) &&
        isTimestamp(input.at)
      ) {
        return input as DomainCommand
      }
      break
    case "ReportMergeFailure":
      if (
        exactKeys(input, [
          "type",
          "reportId",
          "sweepId",
          "pr",
          "headSha",
          "failure",
          "at",
        ]) &&
        typeof input.reportId === "string" &&
        typeof input.sweepId === "string" &&
        isPrRef(input.pr) &&
        typeof input.headSha === "string" &&
        oneOf(input.failure, MERGE_FAILURES) &&
        isTimestamp(input.at)
      ) {
        return input as DomainCommand
      }
      break
    case "ReportCodexConnected":
      if (
        exactKeys(input, ["type", "reportId", "tenant", "method", "at"]) &&
        typeof input.reportId === "string" &&
        typeof input.tenant === "string" &&
        oneOf(input.method, ["api_key", "subscription"] as const) &&
        isTimestamp(input.at)
      ) {
        return input as DomainCommand
      }
      break
  }
  throw new Error("invalid worker command")
}

function commandReportId(command: DomainCommand): string | null {
  return "reportId" in command ? command.reportId : null
}

function stableReportId(taskId: string, result: string): string {
  return `worker:${taskId}:${result}`
}

function prBelongsTo(pr: PrRef, tenant: string, repo: string): boolean {
  return pr.tenant === tenant && pr.repo === repo
}

function samePr(left: PrRef, right: PrRef): boolean {
  return (
    left.tenant === right.tenant &&
    left.repo === right.repo &&
    left.number === right.number
  )
}

function isPrRef(input: unknown): input is PrRef {
  return (
    isRecord(input) &&
    exactKeys(input, ["tenant", "repo", "number"]) &&
    typeof input.tenant === "string" &&
    typeof input.repo === "string" &&
    typeof input.number === "number" &&
    Number.isInteger(input.number) &&
    input.number > 0
  )
}

function isPrSnapshot(input: unknown): input is PrSnapshot {
  return (
    isRecord(input) &&
    exactKeys(input, [
      "headSha",
      "baseRef",
      "draft",
      "open",
      "mergeable",
      "createdAt",
      "updatedAt",
    ]) &&
    typeof input.headSha === "string" &&
    typeof input.baseRef === "string" &&
    typeof input.draft === "boolean" &&
    typeof input.open === "boolean" &&
    (typeof input.mergeable === "boolean" || input.mergeable === null) &&
    isTimestamp(input.createdAt) &&
    isTimestamp(input.updatedAt)
  )
}

function isRequiredChecksSnapshot(input: unknown): boolean {
  return (
    isRecord(input) &&
    exactKeys(input, ["required", "runs"]) &&
    Array.isArray(input.required) &&
    input.required.every(isRequiredCheck) &&
    Array.isArray(input.runs) &&
    input.runs.every(isCheckRun)
  )
}

function isRequiredCheck(input: unknown): input is RequiredCheck {
  return (
    isRecord(input) &&
    exactKeys(input, ["context", "appId"]) &&
    typeof input.context === "string" &&
    Number.isInteger(input.appId)
  )
}

function isCheckRun(input: unknown): input is CheckRun {
  return (
    isRecord(input) &&
    exactKeys(input, [
      "id",
      "check",
      "status",
      "conclusion",
      "createdAt",
      "updatedAt",
      "startedAt",
      "completedAt",
    ]) &&
    Number.isInteger(input.id) &&
    isRequiredCheck(input.check) &&
    oneOf(input.status, CHECK_STATUSES) &&
    (input.conclusion === null || oneOf(input.conclusion, CHECK_CONCLUSIONS)) &&
    isTimestamp(input.createdAt) &&
    isTimestamp(input.updatedAt) &&
    (input.startedAt === null || isTimestamp(input.startedAt)) &&
    (input.completedAt === null || isTimestamp(input.completedAt))
  )
}

function isChecksLookupScope(input: unknown): boolean {
  return (
    isRecord(input) &&
    exactKeys(input, ["kind", "sweepId", "pr", "headSha"]) &&
    input.kind === "checks" &&
    typeof input.sweepId === "string" &&
    isPrRef(input.pr) &&
    typeof input.headSha === "string"
  )
}

function isAgentUsage(input: unknown): boolean {
  return (
    isRecord(input) &&
    exactKeys(input, ["inputTokens", "outputTokens", "reasoningTokens"]) &&
    isNonNegativeNumber(input.inputTokens) &&
    isNonNegativeNumber(input.outputTokens) &&
    isNonNegativeNumber(input.reasoningTokens)
  )
}

function isNonNegativeNumber(input: unknown): boolean {
  return typeof input === "number" && Number.isFinite(input) && input >= 0
}

function isTimestamp(input: unknown): input is number {
  return typeof input === "number" && Number.isFinite(input)
}

function oneOf<T extends string>(
  input: unknown,
  values: readonly T[]
): input is T {
  return typeof input === "string" && values.includes(input as T)
}

function exactKeys(
  input: Record<string, unknown>,
  expected: readonly string[]
): boolean {
  const keys = Object.keys(input)
  return (
    keys.length === expected.length &&
    expected.every((key) => Object.hasOwn(input, key))
  )
}

function isRecord(input: unknown): input is Record<string, unknown> {
  return typeof input === "object" && input !== null && !Array.isArray(input)
}

function deepEqual(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) return true
  if (Array.isArray(left) || Array.isArray(right)) {
    return (
      Array.isArray(left) &&
      Array.isArray(right) &&
      left.length === right.length &&
      left.every((value, index) => deepEqual(value, right[index]))
    )
  }
  if (!isRecord(left) || !isRecord(right)) return false
  const leftKeys = Object.keys(left)
  const rightKeys = Object.keys(right)
  return (
    leftKeys.length === rightKeys.length &&
    leftKeys.every(
      (key) => Object.hasOwn(right, key) && deepEqual(left[key], right[key])
    )
  )
}

function sameReport(left: DomainCommand, right: DomainCommand): boolean {
  if (!("reportId" in left) || !("reportId" in right)) return false
  const { at: _leftAt, ...leftWithoutTime } = left
  const { at: _rightAt, ...rightWithoutTime } = right
  return deepEqual(leftWithoutTime, rightWithoutTime)
}
