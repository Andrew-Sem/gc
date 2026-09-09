import assert from "node:assert/strict"
import {
  configureConvexRuntime,
  convexModules,
  schema,
} from "@workspace/convex"
import { convexTest } from "convex-test"
import { test } from "vitest"

const workerToken = "probe-worker-token"
const tenantSettings = {
  model: "gpt-5.6-luna",
  reasoning: "high" as const,
  prompt: "probe",
}
const repositorySettings = {
  productionEveryHours: 1,
  openPrLimit: 5,
  closeAfterHours: 24,
}
const pr = { tenant: "acme", repo: "acme/app", number: 7 }
const snapshot = {
  headSha: "fresh-head",
  baseRef: "main",
  draft: false,
  open: true,
  mergeable: true,
  createdAt: 1,
  updatedAt: 2,
}
const run = {
  id: 11,
  check: { context: "build", appId: 1001 },
  status: "completed" as const,
  conclusion: "success" as const,
  createdAt: 1,
  updatedAt: 2,
  startedAt: 1,
  completedAt: 2,
}

const now = 1_000
let callbacks = 0
configureConvexRuntime({
  masterKey: "probe-master-key",
  workerToken,
  codexHomeRoot: "/probe/codex-home",
  ports: {
    now: () => now,
    discoverInstallationRepositories: async () => [],
    recordControlPlaneOperation: () => {
      callbacks += 1
    },
  },
})

type Backend = ReturnType<typeof convexTest>

async function protocol(
  backend: Backend,
  input: Record<string, unknown>
): Promise<Response> {
  return backend.fetch("/worker/protocol", {
    method: "POST",
    headers: {
      authorization: `Bearer ${workerToken}`,
      "content-type": "application/json",
    },
    body: JSON.stringify(input),
  })
}

async function expectStatus(
  response: Response,
  expected: number
): Promise<unknown> {
  const body = await response.json().catch(() => null)
  assert.equal(response.status, expected, JSON.stringify(body))
  return body
}

async function insertTask(
  backend: Backend,
  effect: unknown,
  tenant = "acme",
  repo?: string
): Promise<string> {
  return backend.run(async (ctx) =>
    ctx.db.insert("tasks", {
      status: "pending",
      executionCount: 0,
      effect,
      tenant,
      ...(repo === undefined ? {} : { repo }),
      reportSubmissions: 0,
    })
  )
}

async function claim(backend: Backend) {
  return (await expectStatus(
    await protocol(backend, {
      operation: "claim",
      workerId: "probe-worker",
      leaseMs: 60_000,
    }),
    200
  )) as { id: string; leaseToken: string }
}

async function taskState(backend: Backend, taskId: string) {
  return backend.run((ctx) => ctx.db.get(taskId as never))
}

async function journal(backend: Backend) {
  return backend.run(async (ctx) =>
    (await ctx.db.query("journal").collect()).map((row) => row.event)
  )
}

async function report(
  backend: Backend,
  task: { id: string; leaseToken: string },
  command: unknown
): Promise<Response> {
  return protocol(backend, {
    operation: "report",
    taskId: task.id,
    leaseToken: task.leaseToken,
    command,
  })
}

async function completedTaskProbe(): Promise<void> {
  const backend = convexTest(schema, convexModules)
  await insertTask(
    backend,
    { type: "RunAgent", tenant: "acme", repo: "acme/app", tenantSettings },
    "acme",
    "acme/app"
  )
  const task = await claim(backend)
  await expectStatus(
    await protocol(backend, {
      operation: "credentials",
      taskId: task.id,
      leaseToken: task.leaseToken,
    }),
    200
  )
  await expectStatus(
    await protocol(backend, {
      operation: "complete",
      taskId: task.id,
      leaseToken: task.leaseToken,
    }),
    200
  )
  const completed = await taskState(backend, task.id)
  assert.equal(completed?.status, "completed")
  assert.equal(completed?.workerId, undefined)
  assert.equal(completed?.leaseToken, undefined)
  assert.equal(completed?.leaseUntil, undefined)
  assert.notEqual(completed?.completionLeaseTokenHash, task.leaseToken)
  await expectStatus(
    await protocol(backend, {
      operation: "complete",
      taskId: task.id,
      leaseToken: task.leaseToken,
    }),
    200
  )

  for (const input of [
    { operation: "credentials" },
    {
      operation: "report",
      command: {
        type: "ReportAgentRun",
        reportId: `worker:${task.id}:agent-run`,
        tenant: "acme",
        repo: "acme/app",
        pr: null,
        outcome: "failed",
        usage: { inputTokens: 0, outputTokens: 0, reasoningTokens: 0 },
        at: now,
      },
    },
    { operation: "deviceLogin", update: { status: "connected" } },
    { operation: "renew", leaseMs: 60_000 },
    { operation: "release" },
  ]) {
    await expectStatus(
      await protocol(backend, {
        ...input,
        taskId: task.id,
        leaseToken: task.leaseToken,
      }),
      409
    )
  }
}

async function maliciousAgentProbe(): Promise<void> {
  const backend = convexTest(schema, convexModules)
  await insertTask(
    backend,
    { type: "RunAgent", tenant: "acme", repo: "acme/app", tenantSettings },
    "acme",
    "acme/app"
  )
  const otherTaskId = await insertTask(
    backend,
    { type: "RunAgent", tenant: "globex", repo: "globex/app", tenantSettings },
    "globex",
    "globex/app"
  )
  const task = await claim(backend)
  const callbackStart = callbacks
  for (const command of [
    {
      type: "ReportAgentRun",
      reportId: `worker:${task.id}:agent-run`,
      tenant: "globex",
      repo: "globex/app",
      pr: null,
      outcome: "failed",
      usage: { inputTokens: 0, outputTokens: 0, reasoningTokens: 0 },
      at: now,
    },
    {
      type: "ReportAgentRun",
      reportId: `worker:${otherTaskId}:agent-run`,
      tenant: "acme",
      repo: "acme/app",
      pr: null,
      outcome: "failed",
      usage: { inputTokens: 0, outputTokens: 0, reasoningTokens: 0 },
      at: now,
    },
    {
      type: "ReportTenantSettingsChanged",
      reportId: `worker:${task.id}:agent-run`,
      tenant: "acme",
      settings: tenantSettings,
      at: now,
    },
  ]) {
    await expectStatus(await report(backend, task, command), 409)
  }
  const state = await taskState(backend, task.id)
  assert.equal(state?.reportSubmissions, 0)
  assert.equal(state?.lastCommand, undefined)
  assert.equal((await journal(backend)).length, 0)
  assert.equal(callbacks, callbackStart)

  const created = {
    type: "ReportPrCreated",
    reportId: `worker:${task.id}:pr-created`,
    pr,
    headSha: "created-head",
    branch: "gc/probe",
    at: now,
  }
  await expectStatus(await report(backend, task, created), 200)
  const finished = {
    type: "ReportAgentRun",
    reportId: `worker:${task.id}:agent-run`,
    tenant: "acme",
    repo: "acme/app",
    pr,
    outcome: "finished",
    usage: { inputTokens: 1, outputTokens: 2, reasoningTokens: 3 },
    at: now,
  }
  await expectStatus(await report(backend, task, finished), 200)
  await expectStatus(
    await report(backend, task, { ...finished, at: now + 1 }),
    200
  )
  await expectStatus(
    await report(backend, task, {
      ...finished,
      usage: { inputTokens: 999, outputTokens: 2, reasoningTokens: 3 },
    }),
    409
  )
  assert.equal(
    (await journal(backend)).filter(
      (event) => event.type === "PrCreated" || event.type === "AgentRunFinished"
    ).length,
    2
  )
}

async function reconciliationProbe(): Promise<void> {
  const backend = convexTest(schema, convexModules)
  await insertTask(
    backend,
    { type: "ReconcilePr", pr, headSha: "created-head" },
    pr.tenant,
    pr.repo
  )
  const task = await claim(backend)
  await expectStatus(
    await report(backend, task, {
      type: "ObserveCheckRun",
      pr,
      headSha: snapshot.headSha,
      run,
      at: now,
    }),
    409
  )
  await expectStatus(
    await report(backend, task, { type: "ObservePr", pr, snapshot, at: now }),
    200
  )
  await expectStatus(
    await report(backend, task, {
      type: "ObserveCheckRun",
      pr,
      headSha: snapshot.headSha,
      run,
      at: now,
    }),
    200
  )
  await expectStatus(
    await report(backend, task, {
      type: "ObservePr",
      pr: { ...pr, tenant: "globex" },
      snapshot,
      at: now,
    }),
    409
  )
}

async function lookupProbe(): Promise<void> {
  const backend = convexTest(schema, convexModules)
  await insertTask(
    backend,
    {
      type: "LookupRequiredChecks",
      sweepId: "sweep-1",
      pr,
      headSha: "created-head",
      now,
      settings: repositorySettings,
    },
    pr.tenant,
    pr.repo
  )
  const task = await claim(backend)
  await expectStatus(
    await report(backend, task, { type: "ObservePr", pr, snapshot, at: now }),
    200
  )
  await expectStatus(
    await report(backend, task, {
      type: "ReportRequiredChecks",
      reportId: `worker:${task.id}:required-checks`,
      sweepId: "sweep-other",
      pr,
      headSha: snapshot.headSha,
      snapshot: { required: [], runs: [] },
      at: now,
    }),
    409
  )
  await expectStatus(
    await report(backend, task, {
      type: "ReportRequiredChecks",
      reportId: `worker:${task.id}:required-checks`,
      sweepId: "sweep-1",
      pr,
      headSha: snapshot.headSha,
      snapshot: { required: [run.check], runs: [run] },
      at: now,
    }),
    200
  )
  await expectStatus(
    await report(backend, task, { type: "ObservePr", pr, snapshot, at: now }),
    409
  )
  await expectStatus(
    await report(backend, task, {
      type: "ReportLookupFailure",
      reportId: `worker:${task.id}:required-checks`,
      scope: {
        kind: "checks",
        sweepId: "sweep-1",
        pr,
        headSha: "created-head",
      },
      failure: "upstream_error",
      at: now,
    }),
    409
  )

  const failureBackend = convexTest(schema, convexModules)
  await insertTask(
    failureBackend,
    {
      type: "LookupRequiredChecks",
      sweepId: "sweep-2",
      pr,
      headSha: "created-head",
      now,
      settings: repositorySettings,
    },
    pr.tenant,
    pr.repo
  )
  const failureTask = await claim(failureBackend)
  await expectStatus(
    await report(failureBackend, failureTask, {
      type: "ReportLookupFailure",
      reportId: `worker:${failureTask.id}:required-checks`,
      scope: {
        kind: "checks",
        sweepId: "sweep-2",
        pr,
        headSha: "created-head",
      },
      failure: "rate_limited",
      at: now,
    }),
    200
  )

  const continuationBackend = convexTest(schema, convexModules)
  await continuationBackend.run((ctx) =>
    ctx.db.insert("journal", {
      event: {
        type: "PrCreated",
        reportId: "continuation-pr-created",
        pr,
        headSha: "created-head",
        branch: "gc/continuation",
        at: 1,
      },
    })
  )
  await insertTask(
    continuationBackend,
    {
      type: "LookupRequiredChecks",
      sweepId: "sweep-continuation",
      pr,
      headSha: "created-head",
      now: 100_000_000,
      settings: repositorySettings,
    },
    pr.tenant,
    pr.repo
  )
  const continuationTask = await claim(continuationBackend)
  const staleDraft = {
    ...snapshot,
    draft: true,
    createdAt: 1,
    updatedAt: 2,
  }
  await expectStatus(
    await report(continuationBackend, continuationTask, {
      type: "ObservePr",
      pr,
      snapshot: staleDraft,
      at: now,
    }),
    200
  )
  const terminalReport = {
    type: "ReportRequiredChecks",
    reportId: `worker:${continuationTask.id}:required-checks`,
    sweepId: "sweep-continuation",
    pr,
    headSha: staleDraft.headSha,
    snapshot: { required: [run.check], runs: [run] },
    at: now,
  }
  const continued = (await expectStatus(
    await report(continuationBackend, continuationTask, terminalReport),
    200
  )) as { continuation?: unknown }
  assert.deepEqual(continued.continuation, {
    type: "ClosePr",
    sweepId: "sweep-continuation",
    pr,
  })
  assert.deepEqual(
    (await taskState(continuationBackend, continuationTask.id))?.effect,
    continued.continuation
  )
  assert.equal(
    (await continuationBackend.run((ctx) => ctx.db.query("tasks").collect()))
      .length,
    1
  )
  const replayed = (await expectStatus(
    await report(continuationBackend, continuationTask, terminalReport),
    200
  )) as { replayed?: boolean; continuation?: unknown }
  assert.equal(replayed.replayed, true)
  assert.deepEqual(replayed.continuation, continued.continuation)
}

async function mergeAndCloseProbe(): Promise<void> {
  const backend = convexTest(schema, convexModules)
  await insertTask(
    backend,
    { type: "MergePr", sweepId: "sweep-merge", pr, headSha: "merge-head" },
    pr.tenant,
    pr.repo
  )
  const task = await claim(backend)
  await expectStatus(
    await report(backend, task, {
      type: "ReportMergeFailure",
      reportId: `worker:${task.id}:merge-failure`,
      sweepId: "sweep-other",
      pr,
      headSha: "merge-head",
      failure: "upstream_error",
      at: now,
    }),
    409
  )
  await expectStatus(
    await report(backend, task, {
      type: "ReportMergeFailure",
      reportId: `worker:${task.id}:merge-failure`,
      sweepId: "sweep-merge",
      pr,
      headSha: "merge-head",
      failure: "upstream_error",
      at: now,
    }),
    200
  )

  const closeBackend = convexTest(schema, convexModules)
  await insertTask(
    closeBackend,
    { type: "ClosePr", sweepId: "sweep-close", pr },
    pr.tenant,
    pr.repo
  )
  const closeTask = await claim(closeBackend)
  await expectStatus(
    await report(closeBackend, closeTask, {
      type: "ReportMergeFailure",
      reportId: `worker:${closeTask.id}:merge-failure`,
      sweepId: "sweep-close",
      pr,
      headSha: "head",
      failure: "upstream_error",
      at: now,
    }),
    409
  )
}

async function deviceLoginProbe(): Promise<void> {
  const backend = convexTest(schema, convexModules)
  const taskId = await insertTask(
    backend,
    { type: "StartCodexDeviceLogin", tenant: "acme" },
    "acme"
  )
  await backend.run((ctx) =>
    ctx.db.insert("codexDeviceLogins", {
      tenant: "acme",
      taskId,
      status: "pending",
      startedAt: now,
    })
  )
  const task = await claim(backend)
  const connectedReport = {
    type: "ReportCodexConnected",
    reportId: `worker:${task.id}:codex-subscription`,
    tenant: "acme",
    method: "subscription",
    at: now,
  }
  await expectStatus(await report(backend, task, connectedReport), 409)
  await expectStatus(
    await protocol(backend, {
      operation: "deviceLogin",
      taskId: task.id,
      leaseToken: task.leaseToken,
      update: {
        status: "pending",
        deviceUrl: "https://auth.example/device",
        userCode: "PROBE-CODE",
      },
    }),
    200
  )
  await expectStatus(
    await protocol(backend, {
      operation: "deviceLogin",
      taskId: task.id,
      leaseToken: task.leaseToken,
      update: { status: "connected" },
    }),
    200
  )
  await expectStatus(
    await protocol(backend, {
      operation: "deviceLogin",
      taskId: task.id,
      leaseToken: task.leaseToken,
      update: { status: "connected" },
    }),
    200
  )
  await expectStatus(
    await protocol(backend, {
      operation: "deviceLogin",
      taskId: task.id,
      leaseToken: task.leaseToken,
      update: { status: "failed" },
    }),
    409
  )
  await expectStatus(
    await report(backend, task, { ...connectedReport, tenant: "globex" }),
    409
  )
  await expectStatus(await report(backend, task, connectedReport), 200)
}

test("malicious worker protocol commands are fenced before journal dispatch", async () => {
  try {
    await completedTaskProbe()
    await maliciousAgentProbe()
    await reconciliationProbe()
    await lookupProbe()
    await mergeAndCloseProbe()
    await deviceLoginProbe()
  } finally {
    configureConvexRuntime(null)
  }
})
