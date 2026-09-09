import type {
  AgentOutcome,
  DomainCommand,
  Effect,
  LookupFailure,
  MergeFailure,
  PrRef,
  RepositorySettings,
  TenantSettings,
  Timestamp,
} from "@workspace/domain"
import {
  commandOfRequiredChecks,
  commandsOfReconciliation,
} from "@workspace/github"

export const TASK_LEASE_MS = 60_000

export type StartCodexDeviceLoginTask = {
  readonly type: "StartCodexDeviceLogin"
  readonly tenant: string
}

export type LookupRequiredChecksTask = {
  readonly type: "LookupRequiredChecks"
  readonly sweepId: string
  readonly pr: PrRef
  readonly headSha: string
  readonly now: Timestamp
  readonly settings: RepositorySettings
}

export type WorkerTaskEffect =
  | Effect
  | StartCodexDeviceLoginTask
  | LookupRequiredChecksTask

export type DeviceLoginUpdate =
  | {
      readonly status: "pending"
      readonly deviceUrl: string
      readonly userCode: string
    }
  | { readonly status: "connected" }
  | { readonly status: "expired" }
  | { readonly status: "failed" }

export type WorkerTask = {
  readonly id: string
  readonly leaseToken: string
  readonly effect: WorkerTaskEffect
  readonly executionCount: number
}

export type GitHubCredentials = {
  readonly appId?: number
  readonly installationId?: number
  readonly token?: string | null
}

export type CodexCredentials = {
  readonly apiKey?: string | null
  readonly baseUrl?: string
  readonly codexHome?: string
}

export type WorkerCredentials = {
  readonly github: GitHubCredentials
  readonly codex: CodexCredentials
  readonly expiresAt: Timestamp
}

export type WorkerContinuation = Extract<
  Effect,
  { type: "MergePr" | "ClosePr" }
>

export type WorkerReportResult = {
  readonly replayed?: boolean
  readonly continuation?: WorkerContinuation
}

export type WorkspaceCheckout = {
  readonly workingDirectory: string
  readonly baseRef: string
  readonly branch: string
  readonly repo?: string
  readonly remoteBranchExists?: boolean
  readonly remoteBranchHead?: string
}

export type AgentRunResult = {
  readonly outcome: AgentOutcome
  readonly usage: {
    readonly inputTokens: number
    readonly outputTokens: number
    readonly reasoningTokens: number
  }
}

export type PushResult = {
  readonly branch: string
  readonly headSha: string
  readonly changed?: boolean
}

export type PullRequestResult = {
  readonly number: number
  readonly branch: string
  readonly headSha: string
}

export type WorkerRuntime = {
  readonly workerId: string
  readonly now: () => Timestamp
  readonly tasks: {
    claim(workerId: string, leaseMs: number): Promise<WorkerTask | null>
    renew(
      taskId: string,
      leaseToken: string,
      leaseMs: number,
      signal?: AbortSignal
    ): Promise<void>
    complete(
      taskId: string,
      leaseToken: string,
      signal?: AbortSignal
    ): Promise<void>
    release(taskId: string, leaseToken: string): Promise<void>
  }
  readonly credentials: {
    get(
      taskId: string,
      leaseToken: string,
      signal?: AbortSignal
    ): Promise<WorkerCredentials>
  }
  readonly reports: {
    submit(
      taskId: string,
      leaseToken: string,
      command: DomainCommand,
      signal?: AbortSignal
    ): Promise<WorkerReportResult | undefined>
    updateDeviceLogin(
      taskId: string,
      leaseToken: string,
      update: DeviceLoginUpdate,
      signal?: AbortSignal
    ): Promise<void>
  }
  readonly github: {
    createPullRequest(input: {
      readonly tenant: string
      readonly repo: string
      readonly baseRef: string
      readonly headBranch: string
      readonly headSha: string
      readonly title: string
      readonly body: string
      readonly idempotencyKey: string
      readonly credentials: GitHubCredentials
      readonly signal?: AbortSignal
    }): Promise<PullRequestResult>
    getPullRequest(input: {
      readonly tenant: string
      readonly repo: string
      readonly pr: number
      readonly credentials: GitHubCredentials
      readonly signal?: AbortSignal
    }): Promise<unknown>
    getRequiredChecks(input: {
      readonly tenant: string
      readonly repo: string
      readonly pr: number
      readonly headSha: string
      readonly baseRef: string
      readonly credentials: GitHubCredentials
      readonly signal?: AbortSignal
    }): Promise<unknown>
    listCheckRuns(input: {
      readonly tenant: string
      readonly repo: string
      readonly pr: number
      readonly headSha: string
      readonly credentials: GitHubCredentials
      readonly signal?: AbortSignal
    }): Promise<unknown>
    mergePullRequest(input: {
      readonly tenant: string
      readonly repo: string
      readonly pr: number
      readonly headSha: string
      readonly idempotencyKey: string
      readonly credentials: GitHubCredentials
      readonly signal?: AbortSignal
    }): Promise<unknown>
    closePullRequest(input: {
      readonly tenant: string
      readonly repo: string
      readonly pr: number
      readonly idempotencyKey: string
      readonly credentials: GitHubCredentials
      readonly signal?: AbortSignal
    }): Promise<unknown>
  }
  readonly workspace: {
    prepare(
      tenant: string,
      repo: string,
      credentials: GitHubCredentials,
      idempotencyKey: string,
      signal?: AbortSignal
    ): Promise<WorkspaceCheckout>
    push(
      checkout: WorkspaceCheckout,
      credentials: GitHubCredentials,
      signal?: AbortSignal
    ): Promise<PushResult>
    cleanup?(checkout: WorkspaceCheckout): Promise<void>
  }
  readonly codex: {
    run(input: {
      readonly tenant: string
      readonly repo: string
      readonly workingDirectory: string
      readonly settings: TenantSettings
      readonly credentials: CodexCredentials
      readonly signal?: AbortSignal
    }): Promise<AgentRunResult>
    startDeviceLogin?(input: {
      readonly tenant: string
      readonly codexHome: string
      readonly credentials: CodexCredentials
      readonly onUpdate: (update: DeviceLoginUpdate) => Promise<void>
      readonly signal?: AbortSignal
    }): Promise<"connected" | "expired" | "failed">
  }
}

export async function runWorkerOnce(
  runtime: WorkerRuntime,
  isStopping: () => boolean = () => false,
  leaseRenewalMs = Math.floor(TASK_LEASE_MS / 3)
): Promise<"worked" | "idle"> {
  const task = await claimTask(runtime, isStopping)
  if (task === null) return "idle"
  await executeClaimedTask(runtime, task, isStopping, leaseRenewalMs)
  return "worked"
}

async function claimTask(
  runtime: WorkerRuntime,
  isStopping: () => boolean
): Promise<WorkerTask | null> {
  if (isStopping()) return null
  const task = await runtime.tasks.claim(runtime.workerId, TASK_LEASE_MS)
  if (task !== null && isStopping()) {
    await runtime.tasks.release(task.id, task.leaseToken)
    return null
  }
  return task
}

type LeaseFence = {
  readonly signal: AbortSignal
  readonly failure: Promise<never>
  readonly failed: () => boolean
  readonly stop: () => Promise<void>
}

async function executeClaimedTask(
  runtime: WorkerRuntime,
  task: WorkerTask,
  isStopping: () => boolean,
  leaseRenewalMs: number
): Promise<void> {
  let fence: LeaseFence | null = null
  let releaseOnFailure = false
  try {
    fence = await startLeaseFence(runtime, task, leaseRenewalMs)
    const execution = (async () => {
      let credentials: WorkerCredentials
      try {
        credentials = await runtime.credentials.get(
          task.id,
          task.leaseToken,
          fence.signal
        )
      } catch (error) {
        if (!fence.failed()) releaseOnFailure = true
        throw error
      }
      if (credentials.expiresAt <= runtime.now()) {
        releaseOnFailure = true
        throw new Error("worker credentials expired before task execution")
      }
      let currentTask = task
      while (true) {
        const continuation = await executeTask(
          runtime,
          currentTask,
          credentials,
          fence.signal
        )
        if (!continuation) break
        currentTask = { ...task, effect: continuation }
      }
      await fence.stop()
      throwIfAborted(fence.signal)
      await runtime.tasks.complete(task.id, task.leaseToken, fence.signal)
      throwIfAborted(fence.signal)
    })()
    try {
      await Promise.race([execution, fence.failure])
    } catch (error) {
      if (fence.failed()) await execution.catch(() => undefined)
      throw error
    }
  } catch (error) {
    if ((releaseOnFailure || isStopping()) && !fence?.failed()) {
      await runtime.tasks.release(task.id, task.leaseToken)
    }
    throw error
  } finally {
    await fence?.stop()
  }
}

async function startLeaseFence(
  runtime: WorkerRuntime,
  task: WorkerTask,
  leaseRenewalMs: number
): Promise<LeaseFence> {
  const controller = new AbortController()
  let failed = false
  let timer: ReturnType<typeof setTimeout> | null = null
  let renewal: Promise<void> | null = null
  let stopped = false
  let rejectFailure: (error: unknown) => void = () => undefined
  const failure = new Promise<never>((_resolve, reject) => {
    rejectFailure = reject
  })
  const fail = (error: unknown) => {
    if (failed) return
    failed = true
    const reason = asError(error, "task lease renewal failed")
    controller.abort(reason)
    rejectFailure(reason)
  }
  const schedule = () => {
    timer = setTimeout(() => {
      renewal = runtime.tasks
        .renew(task.id, task.leaseToken, TASK_LEASE_MS, controller.signal)
        .then(() => {
          renewal = null
          if (!failed && !stopped) schedule()
        })
        .catch(fail)
    }, leaseRenewalMs)
    timer.unref?.()
  }

  await runtime.tasks.renew(
    task.id,
    task.leaseToken,
    TASK_LEASE_MS,
    controller.signal
  )
  schedule()
  return {
    signal: controller.signal,
    failure,
    failed: () => failed,
    stop: async () => {
      stopped = true
      if (timer) clearTimeout(timer)
      await renewal
    },
  }
}

async function executeTask(
  runtime: WorkerRuntime,
  task: WorkerTask,
  credentials: WorkerCredentials,
  signal: AbortSignal
): Promise<WorkerContinuation | null> {
  throwIfAborted(signal)
  switch (task.effect.type) {
    case "RunAgent":
      await runAgent(runtime, task, task.effect, credentials, signal)
      return null
    case "ReconcilePr":
      await reconcilePr(runtime, task, task.effect, credentials, signal)
      return null
    case "LookupRequiredChecks":
      return lookupRequiredChecks(
        runtime,
        task,
        task.effect,
        credentials,
        signal
      )
    case "MergePr":
      await mergePr(runtime, task, task.effect, credentials, signal)
      return null
    case "ClosePr":
      if (
        isPullRequestClosed(
          await runtime.github.getPullRequest({
            tenant: task.effect.pr.tenant,
            repo: task.effect.pr.repo,
            pr: task.effect.pr.number,
            credentials: credentials.github,
            signal,
          })
        )
      ) {
        return null
      }
      await runtime.github.closePullRequest({
        ...task.effect.pr,
        pr: task.effect.pr.number,
        idempotencyKey: idempotencyKey(task, "close-pr"),
        credentials: credentials.github,
        signal,
      })
      return null
    case "StartCodexDeviceLogin": {
      if (!runtime.codex.startDeviceLogin) {
        throw new Error("Codex device login port is required")
      }
      const result = await runtime.codex.startDeviceLogin({
        tenant: task.effect.tenant,
        codexHome: requiredCodexHome(credentials.codex.codexHome),
        credentials: credentials.codex,
        onUpdate: (update) =>
          runtime.reports.updateDeviceLogin(
            task.id,
            task.leaseToken,
            update,
            signal
          ),
        signal,
      })
      if (result === "connected") {
        await submit(
          runtime,
          task,
          {
            type: "ReportCodexConnected",
            reportId: reportId(task, "codex-subscription"),
            tenant: task.effect.tenant,
            method: "subscription",
            at: runtime.now(),
          },
          signal
        )
      }
      return null
    }
  }
}

async function runAgent(
  runtime: WorkerRuntime,
  task: WorkerTask,
  effect: Extract<Effect, { type: "RunAgent" }>,
  credentials: WorkerCredentials,
  signal: AbortSignal
): Promise<void> {
  let checkout: WorkspaceCheckout | null = null
  try {
    checkout = await runtime.workspace.prepare(
      effect.tenant,
      effect.repo,
      credentials.github,
      idempotencyKey(task, "agent-workspace"),
      signal
    )
    let agent: AgentRunResult
    try {
      agent = await runtime.codex.run({
        tenant: effect.tenant,
        repo: effect.repo,
        workingDirectory: checkout.workingDirectory,
        settings: effect.tenantSettings,
        credentials: credentials.codex,
        signal,
      })
    } catch {
      throwIfAborted(signal)
      agent = { outcome: "failed", usage: emptyUsage() }
    }

    if (agent.outcome !== "finished") {
      await reportAgentRun(runtime, task, effect, agent, null, signal)
      return
    }

    const pushed = await runtime.workspace.push(
      checkout,
      credentials.github,
      signal
    )
    if (pushed.changed === false) {
      await reportAgentRun(runtime, task, effect, agent, null, signal)
      return
    }

    const created = await runtime.github.createPullRequest({
      tenant: effect.tenant,
      repo: effect.repo,
      baseRef: checkout.baseRef,
      headBranch: pushed.branch,
      headSha: pushed.headSha,
      title: "Garbage collector maintenance",
      body: "Automated maintenance produced by garbage-collector.",
      idempotencyKey: idempotencyKey(task, "create-pr"),
      credentials: credentials.github,
      signal,
    })
    const pr = {
      tenant: effect.tenant,
      repo: effect.repo,
      number: created.number,
    }
    await submit(
      runtime,
      task,
      {
        type: "ReportPrCreated",
        reportId: reportId(task, "pr-created"),
        pr,
        headSha: created.headSha,
        branch: created.branch,
        at: runtime.now(),
      },
      signal
    )
    await reportAgentRun(runtime, task, effect, agent, pr, signal)
  } finally {
    if (checkout) await runtime.workspace.cleanup?.(checkout)
  }
}

async function reportAgentRun(
  runtime: WorkerRuntime,
  task: WorkerTask,
  effect: Extract<Effect, { type: "RunAgent" }>,
  result: AgentRunResult,
  pr: PrRef | null,
  signal: AbortSignal
): Promise<void> {
  await submit(
    runtime,
    task,
    {
      type: "ReportAgentRun",
      reportId: reportId(task, "agent-run"),
      tenant: effect.tenant,
      repo: effect.repo,
      pr,
      outcome: result.outcome,
      usage: result.usage,
      at: runtime.now(),
    },
    signal
  )
}

async function reconcilePr(
  runtime: WorkerRuntime,
  task: WorkerTask,
  effect: Extract<Effect, { type: "ReconcilePr" }>,
  credentials: WorkerCredentials,
  signal: AbortSignal
): Promise<void> {
  const pullRequest = await runtime.github.getPullRequest({
    tenant: effect.pr.tenant,
    repo: effect.pr.repo,
    pr: effect.pr.number,
    credentials: credentials.github,
    signal,
  })
  const checkRuns = await runtime.github.listCheckRuns({
    tenant: effect.pr.tenant,
    repo: effect.pr.repo,
    pr: effect.pr.number,
    headSha: pullRequestHead(pullRequest) ?? effect.headSha,
    credentials: credentials.github,
    signal,
  })
  for (const command of commandsOfReconciliation({
    pr: effect.pr,
    headSha: effect.headSha,
    pullRequest,
    checkRuns: pageItems(checkRuns),
    at: runtime.now(),
  })) {
    await submit(runtime, task, command, signal)
  }
}

async function lookupRequiredChecks(
  runtime: WorkerRuntime,
  task: WorkerTask,
  effect: LookupRequiredChecksTask,
  credentials: WorkerCredentials,
  signal: AbortSignal
): Promise<WorkerContinuation | null> {
  let command: DomainCommand
  try {
    const pullRequest = await runtime.github.getPullRequest({
      tenant: effect.pr.tenant,
      repo: effect.pr.repo,
      pr: effect.pr.number,
      credentials: credentials.github,
      signal,
    })
    const headSha = pullRequestHead(pullRequest) ?? effect.headSha
    const baseRef = pullRequestBase(pullRequest) ?? "main"
    const [observePr] = commandsOfReconciliation({
      pr: effect.pr,
      headSha,
      pullRequest,
      checkRuns: [],
      at: runtime.now(),
    })
    if (observePr) await submit(runtime, task, observePr, signal)
    const [requiredChecks, checkRuns] = await Promise.all([
      runtime.github.getRequiredChecks({
        tenant: effect.pr.tenant,
        repo: effect.pr.repo,
        pr: effect.pr.number,
        headSha,
        baseRef,
        credentials: credentials.github,
        signal,
      }),
      runtime.github.listCheckRuns({
        tenant: effect.pr.tenant,
        repo: effect.pr.repo,
        pr: effect.pr.number,
        headSha,
        credentials: credentials.github,
        signal,
      }),
    ])
    command = commandOfRequiredChecks({
      reportId: reportId(task, "required-checks"),
      sweepId: effect.sweepId,
      pr: effect.pr,
      headSha,
      requiredChecks: pageItems(requiredChecks),
      checkRuns: pageItems(checkRuns),
      requiredChecksComplete: pageComplete(requiredChecks),
      checkRunsComplete: pageComplete(checkRuns),
      at: runtime.now(),
    })
  } catch (error) {
    throwIfAborted(signal)
    command = {
      type: "ReportLookupFailure",
      reportId: reportId(task, "required-checks"),
      scope: {
        kind: "checks",
        sweepId: effect.sweepId,
        pr: effect.pr,
        headSha: effect.headSha,
      },
      failure: lookupFailure(error),
      at: runtime.now(),
    }
  }
  return (await submit(runtime, task, command, signal))?.continuation ?? null
}

async function mergePr(
  runtime: WorkerRuntime,
  task: WorkerTask,
  effect: Extract<Effect, { type: "MergePr" }>,
  credentials: WorkerCredentials,
  signal: AbortSignal
): Promise<void> {
  try {
    const current = await runtime.github.getPullRequest({
      tenant: effect.pr.tenant,
      repo: effect.pr.repo,
      pr: effect.pr.number,
      credentials: credentials.github,
      signal,
    })
    if (isPullRequestMerged(current)) return
    await runtime.github.mergePullRequest({
      tenant: effect.pr.tenant,
      repo: effect.pr.repo,
      pr: effect.pr.number,
      headSha: effect.headSha,
      idempotencyKey: idempotencyKey(task, "merge-pr"),
      credentials: credentials.github,
      signal,
    })
  } catch (error) {
    throwIfAborted(signal)
    await submit(
      runtime,
      task,
      {
        type: "ReportMergeFailure",
        reportId: reportId(task, "merge-failure"),
        sweepId: effect.sweepId,
        pr: effect.pr,
        headSha: effect.headSha,
        failure: mergeFailure(error),
        at: runtime.now(),
      },
      signal
    )
  }
}

async function submit(
  runtime: WorkerRuntime,
  task: WorkerTask,
  command: DomainCommand,
  signal: AbortSignal
): Promise<WorkerReportResult | undefined> {
  throwIfAborted(signal)
  const result = await runtime.reports.submit(
    task.id,
    task.leaseToken,
    command,
    signal
  )
  throwIfAborted(signal)
  return result
}

function reportId(task: WorkerTask, result: string): string {
  return `worker:${task.id}:${result}`
}

function idempotencyKey(task: WorkerTask, operation: string): string {
  return `worker:${task.id}:${operation}`
}

function emptyUsage(): AgentRunResult["usage"] {
  return { inputTokens: 0, outputTokens: 0, reasoningTokens: 0 }
}

function pageItems(value: unknown): unknown {
  if (Array.isArray(value)) return value
  if (isRecord(value) && Array.isArray(value.items)) return value.items
  return value
}

function pageComplete(value: unknown): boolean {
  return !isRecord(value) || value.complete !== false
}

function pullRequestHead(value: unknown): string | null {
  if (!isRecord(value) || !isRecord(value.head)) return null
  return typeof value.head.sha === "string" ? value.head.sha : null
}

function pullRequestBase(value: unknown): string | null {
  if (!isRecord(value) || !isRecord(value.base)) return null
  return typeof value.base.ref === "string" ? value.base.ref : null
}

function isPullRequestClosed(value: unknown): boolean {
  return isRecord(value) && value.state === "closed"
}

function isPullRequestMerged(value: unknown): boolean {
  return isRecord(value) && value.merged === true
}

function lookupFailure(error: unknown): LookupFailure {
  const status = errorStatus(error)
  if (status === 403) return "forbidden"
  if (status === 429) return "rate_limited"
  return "upstream_error"
}

function mergeFailure(error: unknown): MergeFailure {
  const status = errorStatus(error)
  const message = error instanceof Error ? error.message.toLowerCase() : ""
  if (status === 403) return "forbidden"
  if (status === 409 || message.includes("head")) return "head_changed"
  if (status === 405 || message.includes("mergeable")) return "not_mergeable"
  return "upstream_error"
}

function errorStatus(error: unknown): number | null {
  return isRecord(error) && typeof error.status === "number"
    ? error.status
    : null
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null
}

function requiredCodexHome(value: string | undefined): string {
  if (!value) throw new Error("tenant CODEX_HOME is required")
  return value
}

function throwIfAborted(signal: AbortSignal): void {
  if (!signal.aborted) return
  throw asError(
    signal.reason,
    "task execution fenced after lease renewal failure"
  )
}

function asError(value: unknown, fallback: string): Error {
  return value instanceof Error ? value : new Error(fallback)
}
