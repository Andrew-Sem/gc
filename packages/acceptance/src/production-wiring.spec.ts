import { createHmac } from "node:crypto"
import {
  convexModules,
  decryptSecret,
  productionCrons,
  publicApi,
  schema,
  systemApi,
} from "@workspace/convex"
import { runWorkerOnce, TASK_LEASE_MS } from "@workspace/worker"
import { createProductionWorkerRuntime } from "@workspace/worker/runtime"
import { convexTest } from "convex-test"
import { expect, test } from "vitest"
import { productionScreens } from "web/screens"
import { createProductionScreenSource } from "web/source"
import type { AcceptanceCase, ProductionApp } from "./contract"
import {
  checkRunPayload,
  checkRunResponse,
  HEAD,
  HOUR,
  NOW,
  OTHER_REPO,
  OTHER_TENANT,
  OTHER_WEBHOOK_SECRET,
  pullRequestPayload,
  pullRequestResponse,
  REPO,
  repositorySettings,
  TENANT,
  tenantSettings,
  WEBHOOK_SECRET,
} from "./fixtures"
import { createAcceptanceHarness } from "./harness"

const MASTER_KEY = "acceptance-master-key"

async function createOwnedPr(
  harness: Parameters<AcceptanceCase["run"]>[0],
  app: Parameters<AcceptanceCase["run"]>[1],
  worker: Parameters<AcceptanceCase["run"]>[2]
): Promise<number> {
  await app.connectGitHubApp({
    tenant: TENANT,
    appId: 7,
    installationId: 42,
    privateKey: "github-private-key",
    webhookSecret: WEBHOOK_SECRET,
  })
  await app.selectRepositories(TENANT, [REPO])
  await app.connectCodexWithApiKey(TENANT, "openai-api-key")
  await app.saveTenantSettings(TENANT, tenantSettings)
  harness.clock.set(NOW)
  await app.saveRepositorySettings(TENANT, REPO, {
    ...repositorySettings,
    productionEveryHours: 1,
    openPrLimit: 12,
    closeAfterHours: 24,
  })
  await app.deliver(
    "check_run",
    "early-unassociated-check",
    checkRunPayload({ conclusion: "success", pull_requests: [] })
  )
  expect(
    (await app.journal()).filter(
      (event) => event.type === "CheckRunStateChanged"
    )
  ).toEqual([])
  const readyAt = NOW + HOUR
  harness.worker.setNextCreatedHeadSha("stale-head")
  harness.clock.set(readyAt)
  await app.productionWakeUp(readyAt)
  harness.github.setReconciliation(
    TENANT,
    REPO,
    1,
    pullRequestResponse({
      created_at: "2026-08-05T07:00:00Z",
      updated_at: "2026-08-05T07:00:00Z",
    }),
    [
      checkRunResponse({
        conclusion: "success",
        pull_requests: [],
        created_at: "2026-08-05T06:55:00Z",
        updated_at: "2026-08-05T07:00:00Z",
        started_at: "2026-08-05T06:55:00Z",
        completed_at: "2026-08-05T07:00:00Z",
      }),
    ]
  )
  await worker.drain()
  expect(harness.observed.calls()).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ operation: "github.getPullRequest", pr: 1 }),
      expect.objectContaining({
        operation: "github.listCheckRuns",
        pr: 1,
        headSha: HEAD,
      }),
    ])
  )
  expect(await app.journal()).toEqual(
    expect.arrayContaining([
      expect.objectContaining({
        type: "CheckRunStateChanged",
        pr: { tenant: TENANT, repo: REPO, number: 1 },
        headSha: HEAD,
        run: expect.objectContaining({ id: 777, conclusion: "success" }),
      }),
    ])
  )
  return readyAt
}

const cases: readonly AcceptanceCase[] = [
  {
    name: "завести новую работу: тик проходит весь путь до PR и экрана",
    usesWorker: true,
    async run(harness, app, worker) {
      await app.connectGitHubApp({
        tenant: TENANT,
        appId: 7,
        installationId: 42,
        privateKey: "github-private-key",
        webhookSecret: WEBHOOK_SECRET,
      })
      await app.selectRepositories(TENANT, [REPO])
      await app.connectCodexWithApiKey(TENANT, "openai-api-key")
      await app.saveTenantSettings(TENANT, tenantSettings)
      harness.clock.set(NOW)
      await app.saveRepositorySettings(TENANT, REPO, {
        ...repositorySettings,
        productionEveryHours: 1,
        openPrLimit: 12,
      })

      harness.clock.set(NOW + HOUR)
      await app.productionWakeUp(NOW + HOUR)
      harness.clock.set(NOW + 2 * HOUR)
      await app.productionWakeUp(NOW + 2 * HOUR)
      harness.github.setReconciliation(
        TENANT,
        REPO,
        1,
        pullRequestResponse({
          created_at: "2026-08-05T08:00:00Z",
          updated_at: "2026-08-05T08:00:00Z",
        }),
        [
          checkRunResponse({
            conclusion: "success",
            pull_requests: [],
            created_at: "2026-08-05T08:00:00Z",
            updated_at: "2026-08-05T08:00:00Z",
            started_at: "2026-08-05T08:00:00Z",
            completed_at: "2026-08-05T08:00:00Z",
          }),
        ]
      )
      harness.github.setReconciliation(
        TENANT,
        REPO,
        2,
        pullRequestResponse({
          number: 2,
          created_at: "2026-08-05T08:00:00Z",
          updated_at: "2026-08-05T08:00:00Z",
        }),
        []
      )
      harness.worker.holdAgents()
      const firstWorker = worker.runOnce("worker-a")
      await harness.worker.waitForAgentStarts(1)
      harness.clock.set(NOW + 2 * HOUR + TASK_LEASE_MS / 2)
      await harness.worker.waitForLeaseRenewals(1)
      const secondWorker = worker.runOnce("worker-b")
      await harness.worker.waitForRepositoryContentions(1)
      expect(harness.observed.maxConcurrentAgents(TENANT, REPO)).toBe(1)
      harness.worker.releaseAgents()
      await Promise.all([firstWorker, secondWorker])
      await worker.drain()

      expect(harness.observed.maxConcurrentAgents(TENANT, REPO)).toBe(1)
      const taskExecutionCounts = await app.taskExecutionCounts()
      expect(taskExecutionCounts.length).toBeGreaterThanOrEqual(2)
      expect(taskExecutionCounts.every((count) => count === 1)).toBe(true)
      expect(await app.pendingTasks()).toBe(0)
      expect(harness.observed.calls().map((call) => call.operation)).toEqual(
        expect.arrayContaining([
          "workspace.prepare",
          "codex.run",
          "workspace.push",
          "github.createPullRequest",
        ])
      )
      expect(await app.journal()).toEqual(
        expect.arrayContaining([
          expect.objectContaining({ type: "PrCreated" }),
          expect.objectContaining({ type: "AgentRunFinished" }),
        ])
      )
      const beforeRetry = await app.journal()
      const submissionsBeforeRetry = harness.observed.reportSubmissionCounts()
      await harness.worker.retryLastReport()
      expect(await app.journal()).toEqual(beforeRetry)
      expect(harness.observed.reportSubmissionCounts()).toEqual([
        ...submissionsBeforeRetry.slice(0, -1),
        (submissionsBeforeRetry.at(-1) ?? 0) + 1,
      ])

      expect(await app.prStatus(TENANT, REPO, NOW + 2 * HOUR)).toHaveLength(2)

      harness.github.setSweepLookup(
        TENANT,
        REPO,
        1,
        pullRequestResponse({
          draft: true,
          created_at: "2026-08-05T08:00:00Z",
          updated_at: "2026-08-06T08:00:00Z",
        }),
        [{ context: "build", app_id: 1001 }],
        [
          checkRunResponse({
            conclusion: "success",
            created_at: "2026-08-05T08:00:00Z",
            updated_at: "2026-08-05T08:00:00Z",
            started_at: "2026-08-05T08:00:00Z",
            completed_at: "2026-08-05T08:00:00Z",
          }),
        ]
      )
      harness.github.setSweepLookup(
        TENANT,
        REPO,
        2,
        pullRequestResponse({
          number: 2,
          draft: true,
          created_at: "2026-08-05T08:00:00Z",
          updated_at: "2026-08-06T08:00:00Z",
        }),
        [{ context: "build", app_id: 1001 }],
        [
          checkRunResponse({
            conclusion: "success",
            created_at: "2026-08-05T08:00:00Z",
            updated_at: "2026-08-05T08:00:00Z",
            started_at: "2026-08-05T08:00:00Z",
            completed_at: "2026-08-05T08:00:00Z",
          }),
        ]
      )
      harness.clock.set(NOW + 26 * HOUR)
      await app.sweep(NOW + 26 * HOUR)
      const completedBeforeClose = await app.completedTasks()
      harness.worker.failNextAfterExternalEffect()
      await expect(worker.runOnce("close-crash")).rejects.toThrow()
      expect(await app.completedTasks()).toBe(completedBeforeClose)
      const closeCallsAfterCrash = harness.observed
        .calls()
        .filter((call) => call.operation === "github.closePullRequest")
      expect(closeCallsAfterCrash).toHaveLength(1)
      expect(closeCallsAfterCrash[0]?.idempotencyKey).toBeTruthy()
      harness.clock.set(NOW + 26 * HOUR + TASK_LEASE_MS)
      await worker.drain()
      expect(await app.completedTasks()).toBeGreaterThan(completedBeforeClose)
      const closeCallsAfterRetry = harness.observed
        .calls()
        .filter((call) => call.operation === "github.closePullRequest")
      expect(
        new Set(closeCallsAfterRetry.map((call) => call.idempotencyKey)).size
      ).toBe(1)

      harness.clock.set(NOW + 27 * HOUR)
      await app.productionWakeUp(NOW + 27 * HOUR)
      harness.worker.failNextAgentBeforePr()
      await worker.drain()
      expect(await app.journal()).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            type: "AgentRunFinished",
            pr: null,
            outcome: "failed",
          }),
        ])
      )
    },
  },
  {
    name: "довести PR до мержа: свип вызывает GitHub и ждёт наблюдение успеха",
    usesWorker: true,
    async run(harness, app, worker) {
      const readyAt = await createOwnedPr(harness, app, worker)
      await app.deliver(
        "check_run",
        "check-success",
        checkRunPayload({ conclusion: "success" })
      )
      harness.github.setSweepLookup(
        TENANT,
        REPO,
        1,
        pullRequestResponse({
          created_at: "2026-08-05T07:00:00Z",
          updated_at: "2026-08-05T07:00:00Z",
        }),
        [{ context: "build", app_id: 1001 }],
        [
          checkRunResponse({
            conclusion: "success",
            created_at: "2026-08-05T06:55:00Z",
            updated_at: "2026-08-05T07:00:00Z",
            started_at: "2026-08-05T06:55:00Z",
            completed_at: "2026-08-05T07:00:00Z",
          }),
        ],
        { requiredChecksComplete: false, checkRunsComplete: true }
      )
      harness.clock.set(readyAt)
      await app.sweep(readyAt)
      await worker.drain()

      expect(await app.journal()).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            type: "LookupIncomplete",
            failure: "page_limit",
          }),
        ])
      )
      expect(harness.observed.calls()).not.toEqual(
        expect.arrayContaining([
          expect.objectContaining({ operation: "github.mergePullRequest" }),
        ])
      )

      harness.github.setSweepLookup(
        TENANT,
        REPO,
        1,
        pullRequestResponse({
          head: { sha: "fresh-head" },
          created_at: "2026-08-05T07:00:00Z",
          updated_at: "2026-08-05T13:00:00Z",
        }),
        [{ context: "build", app_id: 1001 }],
        [
          checkRunResponse({
            conclusion: "success",
            head_sha: "fresh-head",
          }),
        ]
      )
      harness.github.failNextMerge()
      harness.clock.set(readyAt + 6 * HOUR)
      await app.sweep(readyAt + 6 * HOUR)
      await worker.drain()

      expect(harness.observed.calls()).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            operation: "github.getPullRequest",
            pr: 1,
            headSha: HEAD,
          }),
          expect.objectContaining({
            operation: "github.getRequiredChecks",
            pr: 1,
            headSha: "fresh-head",
          }),
          expect.objectContaining({
            operation: "github.listCheckRuns",
            pr: 1,
            headSha: "fresh-head",
          }),
        ])
      )
      expect(await app.journal()).toEqual(
        expect.arrayContaining([
          expect.objectContaining({ type: "MergeFailed" }),
        ])
      )
      harness.github.setSweepLookup(
        TENANT,
        REPO,
        1,
        pullRequestResponse({
          head: { sha: "fresh-head" },
          created_at: "2026-08-05T07:00:00Z",
          updated_at: "2026-08-05T19:00:00Z",
        }),
        [{ context: "build", app_id: 1001 }],
        [
          checkRunResponse({
            conclusion: "success",
            head_sha: "fresh-head",
          }),
        ]
      )
      harness.clock.set(readyAt + 12 * HOUR)
      await app.sweep(readyAt + 12 * HOUR)
      await worker.drain()

      const journal = await app.journal()
      const evidenceIndex = journal.findIndex(
        (event) => event.type === "RequiredChecksObserved"
      )
      const approvalIndex = journal.findIndex(
        (event) => event.type === "MergeApproved"
      )
      expect(evidenceIndex).toBeGreaterThanOrEqual(0)
      expect(approvalIndex).toBeGreaterThan(evidenceIndex)
      const evidence = journal[evidenceIndex]
      const approval = journal[approvalIndex]
      expect(evidence?.type).toBe("RequiredChecksObserved")
      expect(approval?.type).toBe("MergeApproved")
      if (
        evidence?.type !== "RequiredChecksObserved" ||
        approval?.type !== "MergeApproved"
      ) {
        throw new Error("sweep evidence or approval missing")
      }
      expect(approval.sweepId).toBe(evidence.sweepId)
      expect(approval.headSha).toBe("fresh-head")

      const approvals = journal.filter(
        (event) => event.type === "MergeApproved"
      )
      expect(approvals).toHaveLength(2)

      expect(harness.observed.calls()).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            operation: "github.mergePullRequest",
            repo: REPO,
            pr: 1,
            headSha: "fresh-head",
          }),
        ])
      )
      expect(await app.journal()).toEqual(
        expect.arrayContaining([
          expect.objectContaining({ type: "PrStateChanged" }),
          expect.objectContaining({ type: "CheckRunStateChanged" }),
        ])
      )
      expect(await app.journal()).not.toEqual(
        expect.arrayContaining([
          expect.objectContaining({ type: "MergeSucceeded" }),
        ])
      )

      harness.clock.set(readyAt + 12 * HOUR + 10 * 60_000)
      await app.deliver("pull_request", "pr-closed", {
        ...pullRequestPayload({
          state: "closed",
          created_at: "2026-08-05T07:00:00Z",
          updated_at: "2026-08-05T19:10:00Z",
          head: { sha: "fresh-head" },
        }),
        action: "closed",
      })
      expect(await app.prStatus(TENANT, REPO, readyAt + 13 * HOUR)).toEqual([])
    },
  },
  {
    name: "подключить репозиторий: discovery и выбор не раскрывают credentials",
    usesWorker: false,
    async run(harness, app, _worker) {
      harness.github.setInstallationRepositories(TENANT, [REPO, OTHER_REPO])
      await app.connectGitHubApp({
        tenant: TENANT,
        appId: 7,
        installationId: 42,
        privateKey: "github-private-key",
        webhookSecret: WEBHOOK_SECRET,
      })
      await app.selectRepositories(TENANT, [REPO])
      await app.connectCodexWithApiKey(TENANT, "openai-api-key")
      await app.saveTenantSettings(TENANT, tenantSettings)
      harness.clock.set(NOW)
      await app.saveRepositorySettings(TENANT, OTHER_REPO, repositorySettings)

      await app.connectGitHubApp({
        tenant: OTHER_TENANT,
        appId: 8,
        installationId: 84,
        privateKey: "globex-private-key",
        webhookSecret: OTHER_WEBHOOK_SECRET,
      })

      const journalBeforeForgery = await app.journal()
      const forged = await app.deliverWithInvalidSignature(
        "pull_request",
        "forged-delivery",
        { malformed: true }
      )
      expect(forged.status).toBe(401)
      expect(await app.journal()).toEqual(journalBeforeForgery)
      expect(await app.deliveryRecorded("forged-delivery")).toBe(false)
      expect(await app.installationTenant(42)).toBe(TENANT)
      expect(await app.installationTenant(84)).toBe(OTHER_TENANT)
      const globexDelivery = await app.deliver(
        "pull_request",
        "globex-delivery",
        {
          ...pullRequestPayload(),
          installation: { id: 84 },
          repository: { full_name: "globex/app" },
        }
      )
      expect(globexDelivery.status).toBeLessThan(300)
      expect(await app.deliveryTenant("globex-delivery")).toBe(OTHER_TENANT)

      const setup = await app.setup(TENANT)
      await expect(app.unauthenticatedSetup(TENANT)).rejects.toThrow()
      await expect(app.intruderSetup(TENANT)).rejects.toThrow()
      await expect(
        app.intruderSaveTenantSettings(TENANT, tenantSettings)
      ).rejects.toThrow()
      await expect(
        app.intruderConnectGitHubApp({
          tenant: TENANT,
          appId: 9,
          installationId: 99,
          privateKey: "intruder-key",
          webhookSecret: "intruder-secret",
        })
      ).rejects.toThrow()
      await expect(
        app.intruderSelectRepositories(TENANT, [OTHER_REPO])
      ).rejects.toThrow()
      await expect(
        app.intruderConnectCodexWithApiKey(TENANT, "intruder-api-key")
      ).rejects.toThrow()
      await expect(app.intruderStartCodexDeviceLogin(TENANT)).rejects.toThrow()
      await expect(
        app.intruderSaveRepositorySettings(TENANT, REPO, repositorySettings)
      ).rejects.toThrow()
      await expect(
        app.unauthenticatedPrStatus(TENANT, REPO, NOW)
      ).rejects.toThrow()
      await expect(app.intruderPrStatus(TENANT, REPO, NOW)).rejects.toThrow()
      await expect(
        app.intruderRunProductionTick(TENANT, REPO)
      ).rejects.toThrow()
      await expect(app.intruderRunSweep(TENANT, REPO)).rejects.toThrow()
      expect(setup.availableRepositories).toEqual([REPO, OTHER_REPO])
      expect(setup.connectedRepositories).toEqual([REPO])
      expect(setup.connectedRepositories).not.toContain(OTHER_REPO)
      expect(JSON.stringify(setup)).not.toContain("github-private-key")
      expect(JSON.stringify(setup)).not.toContain(WEBHOOK_SECRET)
      const setupHtml = await productionScreens.renderFrom(
        createProductionScreenSource(
          {
            query: (reference, args) => app.productionQuery(reference, args),
          },
          { tenant: TENANT, repo: REPO, now: NOW }
        )
      )
      expect(setupHtml).toContain(REPO)
      expect(setupHtml).toContain(OTHER_REPO)
      expect(setupHtml).toContain(TENANT)
      expect(setupHtml).not.toContain("github-private-key")
      expect(setupHtml).not.toContain(WEBHOOK_SECRET)
      expect(JSON.stringify(await app.journal())).not.toContain(
        "github-private-key"
      )
      const secretStorage = JSON.stringify(await app.secretStorage(TENANT))
      expect(secretStorage).not.toContain("github-private-key")
      expect(secretStorage).not.toContain(WEBHOOK_SECRET)
      expect(secretStorage).not.toContain("openai-api-key")
      const encrypted = (await app.secretStorage(TENANT)) as readonly {
        readonly ciphertext: string
      }[]
      expect(
        encrypted.map((row) => decryptSecret(MASTER_KEY, row.ciphertext))
      ).toEqual(
        expect.arrayContaining([
          "github-private-key",
          WEBHOOK_SECRET,
          "openai-api-key",
        ])
      )
      expect(() =>
        decryptSecret("wrong-master-key", encrypted[0]?.ciphertext ?? "")
      ).toThrow()

      await app.productionWakeUp(NOW)
      expect(harness.observed.calls()).not.toEqual(
        expect.arrayContaining([
          expect.objectContaining({ operation: "codex.run", repo: OTHER_REPO }),
        ])
      )
    },
  },
  {
    name: "настроить модель и ключ: оба входа изолированы по CODEX_HOME",
    usesWorker: true,
    async run(harness, app, worker) {
      await app.connectCodexWithApiKey(TENANT, "openai-api-key")
      await app.startCodexDeviceLogin(OTHER_TENANT)

      const pending = await app.setup(OTHER_TENANT)
      expect(pending.codex.deviceUrl).toBeTruthy()
      expect(pending.codex.userCode).toBeTruthy()

      harness.github.authorizeDeviceCode(OTHER_TENANT)
      await worker.drain()

      const apiKeySetup = await app.setup(TENANT)
      const subscriptionSetup = await app.setup(OTHER_TENANT)
      expect(apiKeySetup.codex.connected).toBe(true)
      expect(subscriptionSetup.codex.connected).toBe(true)
      const publicSetup = JSON.stringify([apiKeySetup, subscriptionSetup])
      expect(publicSetup).not.toContain("openai-api-key")
      expect(publicSetup).not.toContain("access_token")
      expect(publicSetup).not.toContain("refresh_token")
      expect(publicSetup).not.toContain("auth.json")

      const homes = harness.observed
        .calls()
        .filter((call) => call.codexHome)
        .map((call) => call.codexHome)
      expect(new Set(homes).size).toBeGreaterThanOrEqual(2)

      const journal = JSON.stringify(await app.journal())
      expect(journal).not.toContain("openai-api-key")
      expect(journal).not.toContain("access_token")
      expect(journal).not.toContain("refresh_token")
      expect(await app.journal()).toEqual(
        expect.arrayContaining([
          expect.objectContaining({ type: "CodexConnected", tenant: TENANT }),
          expect.objectContaining({
            type: "CodexConnected",
            tenant: OTHER_TENANT,
          }),
        ])
      )
    },
  },
  {
    name: "настроить темп и пороги: изменения применяются на следующем wake-up",
    usesWorker: true,
    async run(harness, app, worker) {
      await app.connectGitHubApp({
        tenant: TENANT,
        appId: 7,
        installationId: 42,
        privateKey: "github-private-key",
        webhookSecret: WEBHOOK_SECRET,
      })
      await app.selectRepositories(TENANT, [REPO])
      await app.connectCodexWithApiKey(TENANT, "openai-api-key")
      const before = await app.journal()
      const callsBefore = harness.observed.calls().length
      await app.saveTenantSettings(TENANT, tenantSettings)
      harness.clock.set(NOW)
      await app.saveRepositorySettings(TENANT, REPO, repositorySettings)

      expect(harness.observed.calls()).toHaveLength(callsBefore)
      expect(await app.pendingTasks()).toBe(0)
      harness.clock.set(NOW + HOUR)
      await app.productionWakeUp(NOW + HOUR)
      expect(harness.observed.calls()).toHaveLength(callsBefore)
      expect(await app.pendingTasks()).toBe(0)
      await worker.drain()
      expect(harness.observed.calls()).toHaveLength(callsBefore)

      harness.clock.set(NOW + 2 * HOUR)
      await app.productionWakeUp(NOW + 2 * HOUR)
      expect(await app.pendingTasks()).toBe(1)
      await app.saveTenantSettings(TENANT, {
        model: "changed-after-admission",
        reasoning: "low",
        prompt: "Changed after the due wake-up",
      })
      harness.github.setReconciliation(
        TENANT,
        REPO,
        1,
        pullRequestResponse({
          created_at: "2026-08-05T08:00:00Z",
          updated_at: "2026-08-05T08:00:00Z",
        }),
        []
      )
      await worker.drain()
      expect(
        harness.observed
          .calls()
          .slice(callsBefore)
          .filter((call) => call.operation === "codex.run")
      ).toEqual([
        expect.objectContaining({
          operation: "codex.run",
          model: tenantSettings.model,
          reasoning: tenantSettings.reasoning,
          prompt: tenantSettings.prompt,
        }),
      ])
      expect((await app.setup(TENANT)).repositorySettings[REPO]).toEqual(
        repositorySettings
      )
      expect((await app.journal()).slice(0, before.length)).toEqual(before)

      harness.clock.set(NOW + 3 * HOUR)
      await app.productionWakeUp(NOW + 3 * HOUR)
      expect(await app.pendingTasks()).toBe(0)
      harness.clock.set(NOW + 4 * HOUR)
      await app.productionWakeUp(NOW + 4 * HOUR)
      expect(await app.pendingTasks()).toBe(1)
      harness.github.setReconciliation(
        TENANT,
        REPO,
        2,
        pullRequestResponse({
          number: 2,
          created_at: "2026-08-05T10:00:00Z",
          updated_at: "2026-08-05T10:00:00Z",
        }),
        []
      )
      await worker.drain()

      harness.clock.set(NOW + 6 * HOUR)
      await app.productionWakeUp(NOW + 6 * HOUR)
      expect(await app.pendingTasks()).toBe(1)
      harness.github.setReconciliation(
        TENANT,
        REPO,
        3,
        pullRequestResponse({
          number: 3,
          created_at: "2026-08-05T12:00:00Z",
          updated_at: "2026-08-05T12:00:00Z",
        }),
        []
      )
      harness.worker.failNextAfterLease()
      await expect(worker.runOnce("crashed-worker")).rejects.toThrow()
      expect(await app.pendingTasks()).toBe(0)
      harness.clock.set(NOW + 6 * HOUR + TASK_LEASE_MS)
      await expect(worker.runOnce("recovery-worker")).resolves.toBe("worked")
      await worker.drain()
      expect(await app.pendingTasks()).toBe(0)
      expect(harness.observed.calls().map((call) => call.operation)).toContain(
        "worker.renewLease"
      )

      harness.clock.set(NOW + 6 * HOUR + 30 * 60_000)
      await app.saveRepositorySettings(TENANT, REPO, {
        ...repositorySettings,
        productionEveryHours: 1,
      })
      harness.clock.set(NOW + 7 * HOUR)
      await app.productionWakeUp(NOW + 7 * HOUR)
      expect(await app.pendingTasks()).toBe(0)
      harness.clock.set(NOW + 7 * HOUR + 30 * 60_000)
      await app.productionWakeUp(NOW + 7 * HOUR + 30 * 60_000)
      expect(await app.pendingTasks()).toBe(1)
    },
  },
  {
    name: "понять, почему PR висит: delivery dedupe доходит до production UI",
    usesWorker: true,
    async run(harness, app, worker) {
      const readyAt = await createOwnedPr(harness, app, worker)
      harness.github.setSweepLookup(
        TENANT,
        REPO,
        1,
        pullRequestResponse({
          created_at: "2026-08-05T07:00:00Z",
          updated_at: "2026-08-05T07:00:00Z",
        }),
        [{ context: "build", app_id: 1001 }],
        [
          checkRunResponse({
            conclusion: "success",
            created_at: "2026-08-05T06:55:00Z",
            updated_at: "2026-08-05T07:00:00Z",
            started_at: "2026-08-05T06:55:00Z",
            completed_at: "2026-08-05T07:00:00Z",
          }),
        ]
      )
      harness.clock.set(readyAt)
      await app.sweep(readyAt)
      await worker.drain()
      expect(await app.journal()).toEqual(
        expect.arrayContaining([
          expect.objectContaining({ type: "RequiredChecksObserved" }),
        ])
      )
      const failedCheck = checkRunPayload({
        id: 778,
        created_at: "2026-08-05T07:06:00Z",
        updated_at: "2026-08-05T07:10:00Z",
        started_at: "2026-08-05T07:06:00Z",
        completed_at: "2026-08-05T07:10:00Z",
      })
      harness.clock.set(readyAt + 10 * 60_000)
      await app.deliver("check_run", "check-1", failedCheck)
      await app.deliver("check_run", "check-1", failedCheck)
      const journalBeforeForgedDuplicate = await app.journal()
      const forgedDuplicate = await app.deliverWithInvalidSignature(
        "check_run",
        "check-1",
        failedCheck
      )
      expect(forgedDuplicate.status).toBe(401)
      expect(await app.journal()).toEqual(journalBeforeForgedDuplicate)

      const failedCheckFacts = (await app.journal()).filter(
        (event) =>
          event.type === "CheckRunStateChanged" &&
          event.run.conclusion === "failure"
      )
      expect(failedCheckFacts).toHaveLength(1)

      const lines = await app.prStatus(TENANT, REPO, readyAt + 5 * HOUR)
      expect(lines).toHaveLength(1)
      expect(lines[0]?.reason).toEqual({
        kind: "required_check_failed",
        check: { context: "build", appId: 1001 },
      })
      const html = await productionScreens.renderFrom(
        createProductionScreenSource(
          {
            query: (reference, args) => app.productionQuery(reference, args),
          },
          { tenant: TENANT, repo: REPO, now: readyAt + 5 * HOUR }
        )
      )
      expect(html).toContain("#1")
      expect(html).toContain("build")
      expect(html).toMatch(/failure|failed|провер/i)
      expect(html).toMatch(/waiting|next|дальше|ожид/i)
    },
  },
]

test.each(cases)("$name", async ({ run, usesWorker }) => {
  expect(productionCrons.crons).toEqual({
    production: {
      name: "wakeUp:production",
      args: [{}],
      schedule: { type: "interval", hours: 1 },
    },
    sweep: {
      name: "wakeUp:sweep",
      args: [{}],
      schedule: { type: "interval", hours: 6 },
    },
  })
  const backend = convexTest(schema, convexModules)
  const intruderBackend = backend.withIdentity({
    subject: "intruder",
    tokenIdentifier: "gc|intruder",
  })
  function tenantBackend(tenant: string) {
    const owner = tenant === TENANT ? "account-a" : "account-b"
    return backend.withIdentity({
      subject: owner,
      tokenIdentifier: `gc|${owner}`,
    })
  }
  function signature(payload: unknown): string {
    const installationId =
      typeof payload === "object" &&
      payload !== null &&
      "installation" in payload &&
      typeof payload.installation === "object" &&
      payload.installation !== null &&
      "id" in payload.installation
        ? payload.installation.id
        : null
    const secret = installationId === 84 ? OTHER_WEBHOOK_SECRET : WEBHOOK_SECRET
    return `sha256=${createHmac("sha256", secret)
      .update(JSON.stringify(payload))
      .digest("hex")}`
  }
  const app: ProductionApp = {
    deliver: (event, deliveryId, payload) =>
      backend.fetch("/github/webhook", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          "x-github-delivery": deliveryId,
          "x-github-event": event,
          "x-hub-signature-256": signature(payload),
        },
        body: JSON.stringify(payload),
      }),
    deliverWithInvalidSignature: (event, deliveryId, payload) =>
      backend.fetch("/github/webhook", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          "x-github-delivery": deliveryId,
          "x-github-event": event,
          "x-hub-signature-256": "sha256=invalid",
        },
        body: JSON.stringify(payload),
      }),
    connectGitHubApp: (input) =>
      tenantBackend(input.tenant).action(publicApi.connectGitHubApp, input),
    selectRepositories: (tenant, repos) =>
      tenantBackend(tenant).mutation(publicApi.selectRepositories, {
        tenant,
        repos,
      }),
    connectCodexWithApiKey: (tenant, apiKey) =>
      tenantBackend(tenant).mutation(publicApi.connectCodexWithApiKey, {
        tenant,
        apiKey,
      }),
    startCodexDeviceLogin: (tenant) =>
      tenantBackend(tenant).action(publicApi.startCodexDeviceLogin, {
        tenant,
      }),
    saveTenantSettings: (tenant, settings) =>
      tenantBackend(tenant).mutation(publicApi.saveTenantSettings, {
        tenant,
        settings,
      }),
    saveRepositorySettings: (tenant, repo, settings) =>
      tenantBackend(tenant).mutation(publicApi.saveRepositorySettings, {
        tenant,
        repo,
        settings,
      }),
    productionWakeUp: (_now) => backend.mutation(systemApi.productionCron, {}),
    sweep: (_now) => backend.mutation(systemApi.sweepCron, {}),
    productionRepositoryTick: (now) =>
      backend.mutation(systemApi.productionWakeUp, {
        tenant: TENANT,
        repo: REPO,
        now,
      }),
    sweepRepositoryTick: (now) =>
      backend.mutation(systemApi.sweep, {
        tenant: TENANT,
        repo: REPO,
        now,
      }),
    ownerRunProductionTick: (tenant, repo) =>
      tenantBackend(tenant).action(publicApi.runProductionTick, {
        tenant,
        repo,
      }),
    ownerRunSweep: (tenant, repo) =>
      tenantBackend(tenant).action(publicApi.runSweep, { tenant, repo }),
    intruderRunProductionTick: (tenant, repo) =>
      intruderBackend.action(publicApi.runProductionTick, { tenant, repo }),
    intruderRunSweep: (tenant, repo) =>
      intruderBackend.action(publicApi.runSweep, { tenant, repo }),
    intruderSaveTenantSettings: (tenant, settings) =>
      intruderBackend.mutation(publicApi.saveTenantSettings, {
        tenant,
        settings,
      }),
    intruderConnectGitHubApp: (input) =>
      intruderBackend.action(publicApi.connectGitHubApp, input),
    intruderSelectRepositories: (tenant, repos) =>
      intruderBackend.mutation(publicApi.selectRepositories, { tenant, repos }),
    intruderConnectCodexWithApiKey: (tenant, apiKey) =>
      intruderBackend.mutation(publicApi.connectCodexWithApiKey, {
        tenant,
        apiKey,
      }),
    intruderStartCodexDeviceLogin: (tenant) =>
      intruderBackend.action(publicApi.startCodexDeviceLogin, { tenant }),
    intruderSaveRepositorySettings: (tenant, repo, settings) =>
      intruderBackend.mutation(publicApi.saveRepositorySettings, {
        tenant,
        repo,
        settings,
      }),
    intruderPrStatus: (tenant, repo, now) =>
      intruderBackend.query(publicApi.prStatus, { tenant, repo, now }),
    unauthenticatedPrStatus: (tenant, repo, now) =>
      backend.query(publicApi.prStatus, { tenant, repo, now }),
    journal: () =>
      backend.run(async (ctx) => {
        const rows = await ctx.db.query("journal").collect()
        return rows.map((row) => row.event)
      }),
    operatorAudit: () =>
      backend.run(async (ctx) => ctx.db.query("operatorAudit").collect()),
    taskExecutionCounts: () =>
      backend.run(async (ctx) => {
        const rows = await ctx.db.query("tasks").collect()
        return rows.map((row) => row.executionCount)
      }),
    completedTasks: () =>
      backend.run(async (ctx) => {
        const rows = await ctx.db.query("tasks").collect()
        return rows.filter((row) => row.status === "completed").length
      }),
    pendingTasks: () =>
      backend.run(async (ctx) => {
        const rows = await ctx.db.query("tasks").collect()
        return rows.filter((row) => row.status === "pending").length
      }),
    secretStorage: (tenant) =>
      backend.run(async (ctx) => {
        return ctx.db
          .query("secrets")
          .filter((q) => q.eq(q.field("tenant"), tenant))
          .collect()
      }),
    deliveryRecorded: (deliveryId) =>
      backend.run(async (ctx) => {
        const rows = await ctx.db
          .query("deliveries")
          .filter((q) => q.eq(q.field("deliveryId"), deliveryId))
          .collect()
        return rows.length > 0
      }),
    deliveryTenant: (deliveryId) =>
      backend.run(async (ctx) => {
        const row = await ctx.db
          .query("deliveries")
          .filter((q) => q.eq(q.field("deliveryId"), deliveryId))
          .first()
        return row?.tenant ?? null
      }),
    installationTenant: (installationId) =>
      backend.run(async (ctx) => {
        const row = await ctx.db
          .query("installations")
          .filter((q) => q.eq(q.field("installationId"), installationId))
          .first()
        return row?.tenant ?? null
      }),
    unauthenticatedSetup: (tenant) =>
      backend.query(publicApi.setup, { tenant }),
    intruderSetup: (tenant) =>
      intruderBackend.query(publicApi.setup, { tenant }),
    setup: (tenant) => tenantBackend(tenant).query(publicApi.setup, { tenant }),
    prStatus: (tenant, repo, now) =>
      tenantBackend(tenant).query(publicApi.prStatus, { tenant, repo, now }),
    productionQuery: (reference, args) =>
      tenantBackend(TENANT).query(
        reference as typeof publicApi.setup | typeof publicApi.prStatus,
        args
      ),
  }
  const harness = await createAcceptanceHarness({
    backend,
    masterKey: MASTER_KEY,
  })
  for (const operation of [
    "claim",
    "renew",
    "credentials",
    "complete",
    "release",
    "report",
  ]) {
    const rejectedWorker = await backend.fetch("/worker/protocol", {
      method: "POST",
      headers: {
        authorization: "Bearer invalid-worker-token",
        "content-type": "application/json",
      },
      body: JSON.stringify({ operation, workerId: "intruder" }),
    })
    expect(rejectedWorker.status).toBe(401)
  }
  let workerInvocations = 0
  const worker = {
    async runOnce(workerId?: string) {
      workerInvocations += 1
      return runWorkerOnce(
        createProductionWorkerRuntime(
          harness.worker.ports(workerId),
          {
            request: async ({ credential, ...request }) => {
              const response = await backend.fetch("/worker/protocol", {
                method: "POST",
                headers: {
                  authorization: `Bearer ${credential}`,
                  "content-type": "application/json",
                },
                body: JSON.stringify(request),
              })
              if (!response.ok) {
                throw new Error(`worker protocol failed: ${response.status}`)
              }
              return response.json()
            },
          },
          workerId ?? "acceptance-worker",
          "acceptance-worker-token"
        )
      )
    },
    async drain() {
      while ((await this.runOnce()) === "worked") {
        // Drain production tasks without sleeps or polling an external system.
      }
    },
  }
  try {
    await run(harness, app, worker)
    const smokeNow = Date.parse("2026-08-08T00:00:00Z")
    const journalOnlyRepo = `${TENANT}/journal-only`
    await backend.run(async (ctx) => {
      await ctx.db.insert("journal", {
        event: {
          type: "RepositorySettingsChanged",
          reportId: "journal-only-settings",
          tenant: TENANT,
          repo: journalOnlyRepo,
          settings: repositorySettings,
          at: smokeNow,
        },
      })
    })
    expect(
      (await app.setup(TENANT)).repositorySettings[journalOnlyRepo]
    ).toEqual(repositorySettings)
    harness.clock.set(smokeNow)
    await app.productionRepositoryTick(smokeNow)
    await app.sweepRepositoryTick(smokeNow)
    await app.ownerRunProductionTick(TENANT, REPO)
    await app.ownerRunSweep(TENANT, REPO)
    expect(await app.operatorAudit()).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ tenant: TENANT, repo: REPO }),
      ])
    )
    expect(workerInvocations > 0).toBe(usesWorker)
  } finally {
    await harness.dispose()
  }
})
