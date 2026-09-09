import type {
  Effect,
  PrRef,
  RepositorySettings,
  Timestamp,
} from "@workspace/domain"
import type { DocumentByName, GenericQueryCtx } from "convex/server"
import type { GenericId } from "convex/values"
import { v } from "convex/values"
import type { DataModel } from "../src/data-model"
import { runtimeConfiguration, tenantCodexHome } from "../src/runtime-ports"
import { decryptSecretValue } from "../src/secret-crypto"
import { internalMutation, internalQuery } from "./_generated/server"
import { dispatchCommand } from "./control_plane"
import {
  assertWorkerTaskScope,
  authorizeWorkerCommand,
} from "./worker_protocol"

type TaskDocument = DocumentByName<DataModel, "tasks">
type TaskReadContext = Pick<GenericQueryCtx<DataModel>, "db">
type LookupRequiredChecksEffect = {
  readonly type: "LookupRequiredChecks"
  readonly sweepId: string
  readonly pr: PrRef
  readonly now: Timestamp
  readonly settings: RepositorySettings
}
type StartCodexDeviceLoginEffect = {
  readonly type: "StartCodexDeviceLogin"
  readonly tenant: string
}

function isLookupRequiredChecksEffect(
  effect: unknown
): effect is LookupRequiredChecksEffect {
  return (
    typeof effect === "object" &&
    effect !== null &&
    "type" in effect &&
    effect.type === "LookupRequiredChecks"
  )
}

function isStartCodexDeviceLoginEffect(
  effect: unknown
): effect is StartCodexDeviceLoginEffect {
  return (
    typeof effect === "object" &&
    effect !== null &&
    "type" in effect &&
    effect.type === "StartCodexDeviceLogin" &&
    "tenant" in effect &&
    typeof effect.tenant === "string"
  )
}

type SweepContinuation = Extract<Effect, { type: "MergePr" | "ClosePr" }>

function isSweepContinuation(
  effect: Effect,
  lookup: LookupRequiredChecksEffect
): effect is SweepContinuation {
  return (
    (effect.type === "MergePr" || effect.type === "ClosePr") &&
    effect.sweepId === lookup.sweepId &&
    effect.pr.tenant === lookup.pr.tenant &&
    effect.pr.repo === lookup.pr.repo &&
    effect.pr.number === lookup.pr.number
  )
}

function replayedContinuation(
  task: TaskDocument,
  command: unknown
): SweepContinuation | undefined {
  if (
    !isLookupTerminalReport(command) ||
    !isPersistedSweepContinuation(task.effect)
  ) {
    return undefined
  }
  const scope =
    command.type === "ReportRequiredChecks"
      ? { sweepId: command.sweepId, pr: command.pr }
      : command.scope.kind === "checks"
        ? { sweepId: command.scope.sweepId, pr: command.scope.pr }
        : null
  return scope &&
    task.effect.sweepId === scope.sweepId &&
    task.effect.pr.tenant === scope.pr.tenant &&
    task.effect.pr.repo === scope.pr.repo &&
    task.effect.pr.number === scope.pr.number
    ? task.effect
    : undefined
}

function isLookupTerminalReport(command: unknown): command is
  | {
      readonly type: "ReportRequiredChecks"
      readonly sweepId: string
      readonly pr: PrRef
    }
  | {
      readonly type: "ReportLookupFailure"
      readonly scope: {
        readonly kind: string
        readonly sweepId: string
        readonly pr: PrRef
      }
    } {
  return (
    typeof command === "object" &&
    command !== null &&
    "type" in command &&
    (command.type === "ReportRequiredChecks" ||
      command.type === "ReportLookupFailure")
  )
}

function isPersistedSweepContinuation(
  effect: unknown
): effect is SweepContinuation {
  return (
    typeof effect === "object" &&
    effect !== null &&
    "type" in effect &&
    (effect.type === "MergePr" || effect.type === "ClosePr") &&
    "sweepId" in effect &&
    typeof effect.sweepId === "string" &&
    "pr" in effect &&
    typeof effect.pr === "object" &&
    effect.pr !== null &&
    "tenant" in effect.pr &&
    typeof effect.pr.tenant === "string" &&
    "repo" in effect.pr &&
    typeof effect.pr.repo === "string" &&
    "number" in effect.pr &&
    typeof effect.pr.number === "number"
  )
}

async function taskById(
  ctx: TaskReadContext,
  taskId: string
): Promise<TaskDocument> {
  const task = await ctx.db.get(taskId as GenericId<"tasks">)
  if (!task) throw new Error("task not found")
  return task
}

function activeLease(
  task: Pick<TaskDocument, "leaseToken" | "leaseUntil" | "status">,
  leaseToken: string,
  now: number
): boolean {
  return (
    task.leaseToken === leaseToken &&
    task.status === "leased" &&
    (task.leaseUntil ?? 0) >= now
  )
}

async function leaseTokenHash(leaseToken: string): Promise<string> {
  const digest = new Uint8Array(
    await crypto.subtle.digest("SHA-256", new TextEncoder().encode(leaseToken))
  )
  return [...digest].map((byte) => byte.toString(16).padStart(2, "0")).join("")
}

async function completedByLease(
  task: Pick<TaskDocument, "completionLeaseTokenHash" | "status">,
  leaseToken: string
): Promise<boolean> {
  return (
    task.status === "completed" &&
    task.completionLeaseTokenHash === (await leaseTokenHash(leaseToken))
  )
}

export const claimTask = internalMutation({
  args: {
    workerId: v.string(),
    leaseMs: v.number(),
    readiness: v.optional(v.boolean()),
  },
  handler: async (ctx, { workerId, leaseMs, readiness }) => {
    const now = runtimeConfiguration().ports.now()
    const tasks = await ctx.db.query("tasks").collect()
    if (readiness) return null
    const task = tasks.find(
      (candidate) =>
        candidate.status === "pending" ||
        (candidate.status === "leased" && (candidate.leaseUntil ?? 0) <= now)
    )
    if (!task) return null
    const leaseToken = crypto.randomUUID()
    await ctx.db.patch(task._id, {
      status: "leased",
      workerId,
      leaseToken,
      leaseUntil: now + leaseMs,
      executionCount: task.executionCount + 1,
    })
    return {
      id: task._id,
      leaseToken,
      effect: task.effect,
      executionCount: task.executionCount + 1,
    }
  },
})

export const renewTask = internalMutation({
  args: { taskId: v.string(), leaseToken: v.string(), leaseMs: v.number() },
  handler: async (ctx, { taskId, leaseToken, leaseMs }) => {
    const task = await taskById(ctx, taskId)
    const now = runtimeConfiguration().ports.now()
    if (!activeLease(task, leaseToken, now)) {
      throw new Error("invalid task lease")
    }
    await ctx.db.patch(task._id, { leaseUntil: now + leaseMs })
    runtimeConfiguration().ports.recordControlPlaneOperation?.({
      operation: "worker.renewLease",
      taskId,
      leaseToken,
    })
  },
})

export const completeTask = internalMutation({
  args: { taskId: v.string(), leaseToken: v.string() },
  handler: async (ctx, { taskId, leaseToken }) => {
    const task = await taskById(ctx, taskId)
    const now = runtimeConfiguration().ports.now()
    if (await completedByLease(task, leaseToken)) return
    if (!activeLease(task, leaseToken, now))
      throw new Error("invalid task lease")
    await ctx.db.patch(task._id, {
      status: "completed",
      completedAt: now,
      workerId: undefined,
      leaseToken: undefined,
      leaseUntil: undefined,
      completionLeaseTokenHash: await leaseTokenHash(leaseToken),
    })
  },
})

export const releaseTask = internalMutation({
  args: { taskId: v.string(), leaseToken: v.string() },
  handler: async (ctx, { taskId, leaseToken }) => {
    const task = await taskById(ctx, taskId)
    const now = runtimeConfiguration().ports.now()
    if (!activeLease(task, leaseToken, now)) {
      throw new Error("invalid task lease")
    }
    await ctx.db.patch(task._id, {
      status: "pending",
      workerId: undefined,
      leaseToken: undefined,
      leaseUntil: undefined,
    })
  },
})

export const submitReport = internalMutation({
  args: { taskId: v.string(), leaseToken: v.string(), command: v.any() },
  handler: async (ctx, { taskId, leaseToken, command }) => {
    const task = await taskById(ctx, taskId)
    const now = runtimeConfiguration().ports.now()
    if (!activeLease(task, leaseToken, now))
      throw new Error("invalid task lease")
    const authorized = authorizeWorkerCommand(taskId, task, command)
    if (
      isStartCodexDeviceLoginEffect(task.effect) &&
      authorized.command.type === "ReportCodexConnected"
    ) {
      const login = await ctx.db
        .query("codexDeviceLogins")
        .filter((query) => query.eq(query.field("tenant"), task.tenant))
        .first()
      if (!login || login.taskId !== taskId || login.status !== "connected") {
        throw new Error("Codex device login is not connected")
      }
    }
    if (authorized.replayed) {
      const continuation = replayedContinuation(task, authorized.command)
      await ctx.db.patch(task._id, {
        reportSubmissions: task.reportSubmissions + 1,
      })
      runtimeConfiguration().ports.recordControlPlaneOperation?.({
        operation: "worker.submitReport",
        taskId,
        leaseToken,
        command: authorized.command,
      })
      return {
        replayed: true,
        ...(continuation ? { continuation } : {}),
      }
    }
    await ctx.db.patch(task._id, {
      reportSubmissions: task.reportSubmissions + 1,
      lastCommand: authorized.command,
      ...(authorized.acceptedReports
        ? { acceptedReports: authorized.acceptedReports }
        : {}),
      ...(authorized.observedHeadSha
        ? { observedHeadSha: authorized.observedHeadSha }
        : {}),
    })
    runtimeConfiguration().ports.recordControlPlaneOperation?.({
      operation: "worker.submitReport",
      taskId,
      leaseToken,
      command: authorized.command,
    })
    const domainCommand = authorized.command
    await dispatchCommand(ctx.db, domainCommand)

    const effect: unknown = task.effect
    if (
      isLookupRequiredChecksEffect(effect) &&
      (domainCommand.type === "ReportRequiredChecks" ||
        domainCommand.type === "ReportLookupFailure")
    ) {
      let continuation: SweepContinuation | undefined
      await dispatchCommand(
        ctx.db,
        {
          type: "TickSweep",
          sweepId: effect.sweepId,
          tenant: effect.pr.tenant,
          repo: effect.pr.repo,
          settings: effect.settings,
          now: effect.now,
        },
        {
          handleEffect: async (generated) => {
            if (!isSweepContinuation(generated, effect)) return false
            if (continuation) {
              throw new Error("checks lookup produced multiple continuations")
            }
            continuation = generated
            await ctx.db.patch(task._id, { effect: generated })
            return true
          },
        }
      )
      return continuation
        ? { replayed: false, continuation }
        : { replayed: false }
    }
    return { replayed: false }
  },
})

export const updateDeviceLogin = internalMutation({
  args: {
    taskId: v.string(),
    leaseToken: v.string(),
    update: v.union(
      v.object({
        status: v.literal("pending"),
        deviceUrl: v.string(),
        userCode: v.string(),
      }),
      v.object({ status: v.literal("connected") }),
      v.object({ status: v.literal("expired") }),
      v.object({ status: v.literal("failed") })
    ),
  },
  handler: async (ctx, { taskId, leaseToken, update }) => {
    const task = await taskById(ctx, taskId)
    const now = runtimeConfiguration().ports.now()
    if (!activeLease(task, leaseToken, now)) {
      throw new Error("invalid task lease")
    }
    assertWorkerTaskScope(task)
    if (!isStartCodexDeviceLoginEffect(task.effect)) {
      throw new Error("task does not own a Codex device login")
    }
    const login = await ctx.db
      .query("codexDeviceLogins")
      .filter((query) => query.eq(query.field("tenant"), task.tenant))
      .first()
    if (!login || login.taskId !== taskId) return { applied: false }
    if (login.status === update.status && update.status !== "pending") {
      return { applied: false, replayed: true }
    }
    if (login.status !== "pending") {
      throw new Error("Codex device login is already terminal")
    }
    await ctx.db.patch(login._id, {
      status: update.status,
      deviceUrl: update.status === "pending" ? update.deviceUrl : undefined,
      userCode: update.status === "pending" ? update.userCode : undefined,
      finishedAt: update.status === "pending" ? undefined : now,
    })
    return { applied: true }
  },
})

export const credentials = internalQuery({
  args: { taskId: v.string(), leaseToken: v.string() },
  handler: async (ctx, { taskId, leaseToken }) => {
    const task = await taskById(ctx, taskId)
    const now = runtimeConfiguration().ports.now()
    if (!activeLease(task, leaseToken, now))
      throw new Error("invalid task lease")
    assertWorkerTaskScope(task)
    const effect: unknown = task.effect
    const effectType =
      typeof effect === "object" &&
      effect !== null &&
      "type" in effect &&
      typeof effect.type === "string"
        ? effect.type
        : ""
    const needsGitHub = [
      "RunAgent",
      "ReconcilePr",
      "LookupRequiredChecks",
      "MergePr",
      "ClosePr",
    ].includes(effectType)
    const needsCodex = effectType === "RunAgent"
    const codexApiKey = needsCodex
      ? await taskSecret(ctx, task.tenant, "codex_api_key")
      : null
    const installation = needsGitHub
      ? await ctx.db
          .query("installations")
          .withIndex("by_tenant", (query) => query.eq("tenant", task.tenant))
          .unique()
      : null
    const installationEligible = Boolean(
      installation &&
        !installation.suspended &&
        !installation.deleted &&
        (!task.repo || installation.availableRepositories.includes(task.repo))
    )
    return {
      github:
        needsGitHub && installationEligible && installation
          ? {
              appId: installation.appId,
              installationId: installation.installationId,
            }
          : {},
      githubRequired: needsGitHub,
      githubRepo: task.repo,
      tenant: task.tenant,
      codex:
        needsCodex || isStartCodexDeviceLoginEffect(effect)
          ? {
              ...(needsCodex ? { apiKey: codexApiKey } : {}),
              ...(needsCodex && task.runAgentSnapshot?.openaiBaseUrl
                ? { baseUrl: task.runAgentSnapshot.openaiBaseUrl }
                : {}),
              codexHome: tenantCodexHome(task.tenant),
            }
          : {},
      expiresAt: now + 60 * 60_000,
    }
  },
})

async function taskSecret(
  ctx: TaskReadContext,
  tenant: string,
  kind: string
): Promise<string | null> {
  const secret = await ctx.db
    .query("secrets")
    .withIndex("by_tenant_kind", (query) =>
      query.eq("tenant", tenant).eq("kind", kind)
    )
    .first()
  return secret
    ? decryptSecretValue(runtimeConfiguration().masterKey, secret.ciphertext)
    : null
}
