import { expect, test } from "vitest"
import {
  main,
  parseReleaseSmokeTarget,
  type ReleaseSmokeDriver,
  runReleaseSmoke,
  type SmokeMode,
} from "./release-smoke"

test.each(["api_key", "subscription"] as const)(
  "release smoke %s проходит обязательный production порядок",
  async (mode: SmokeMode) => {
    const calls: string[] = []
    const step = (name: string) => async () => {
      calls.push(name)
    }
    const driver: ReleaseSmokeDriver = {
      verifyDisposableRepository: step("verify-repository"),
      startCleanStack: step("start-stack"),
      waitForHealth: step("health"),
      registerOrSignIn: step("auth"),
      reloadAndVerifySession: step("reload-session"),
      connectGitHubAndDiscoverRepository: step("github"),
      connectCodex: async (receivedMode) => {
        calls.push(`codex:${receivedMode}`)
      },
      runMinimalCodexCall: step("codex-call"),
      waitForSystemCreatedPullRequest: async () => {
        calls.push("wait-system-pr")
        return { number: 101 }
      },
      deliverSignedWebhook: async (pr) => {
        calls.push(`webhook:${pr}`)
      },
      runProductionTick: step("production-tick"),
      runSweep: step("sweep"),
      verifyWhyStuck: async (pr) => {
        calls.push(`why-stuck:${pr}`)
      },
      waitForMergedObservation: async (pr) => {
        calls.push(`wait-merged:${pr}`)
      },
      closeAgedPullRequest: step("close-aged"),
      restartControlPlaneAndWorker: step("restart"),
      verifyPersistence: step("persistence"),
      logoutAndVerifyAuthDenial: step("logout-denial"),
      saveEvidence: async ({ mode: evidenceMode, pr }) => {
        calls.push(`evidence:${evidenceMode}:${pr}`)
      },
    }

    await runReleaseSmoke(mode, driver, {
      repository: "acme/gc-disposable",
      repositoryId: 123,
      marker: "garbage-collector-disposable",
    })

    expect(calls).toEqual([
      "verify-repository",
      "start-stack",
      "health",
      "auth",
      "reload-session",
      "github",
      `codex:${mode}`,
      "codex-call",
      "production-tick",
      "wait-system-pr",
      "webhook:101",
      "sweep",
      "why-stuck:101",
      "wait-merged:101",
      "close-aged",
      "restart",
      "persistence",
      "logout-denial",
      `evidence:${mode}:101`,
    ])
  }
)

const releaseEnv = {
  SMOKE_GITHUB_REPOSITORY: "acme/gc-disposable",
  SMOKE_GITHUB_REPOSITORY_ID: "123",
  SMOKE_DISPOSABLE_MARKER: "garbage-collector-disposable",
  SMOKE_OWNER_EMAIL: "owner@example.com",
  SMOKE_OWNER_PASSWORD: "password",
  SMOKE_GITHUB_APP_ID: "7",
  SMOKE_GITHUB_INSTALLATION_ID: "42",
  SMOKE_GITHUB_PRIVATE_KEY: "private-key",
  SMOKE_GITHUB_WEBHOOK_SECRET: "webhook-secret",
  SMOKE_GITHUB_BASE_BRANCH: "main",
  SMOKE_OPENAI_API_KEY: "api-key",
  SMOKE_SUBSCRIPTION_TIMEOUT_SECONDS: "600",
}

test.each(["api_key", "subscription"] as const)(
  "CLI %s создаёт production driver и вызывает orchestrator",
  async (mode) => {
    let factoryMode: SmokeMode | null = null
    await main([mode], releaseEnv, async ({ mode: receivedMode }) => {
      factoryMode = receivedMode
      return recordingDriver([])
    })
    expect(factoryMode).toBe(mode)
  }
)

test("release env validation происходит до driver creation", async () => {
  let created = false
  await expect(
    main(
      ["api_key"],
      { ...releaseEnv, SMOKE_GITHUB_REPOSITORY_ID: "" },
      async () => {
        created = true
        return recordingDriver([])
      }
    )
  ).rejects.toThrow("SMOKE_GITHUB_REPOSITORY_ID")
  expect(created).toBe(false)
})

test("subscription не требует API key, но требует timeout", () => {
  expect(
    parseReleaseSmokeTarget("subscription", {
      ...releaseEnv,
      SMOKE_OPENAI_API_KEY: undefined,
    }).mode
  ).toBe("subscription")
  expect(() =>
    parseReleaseSmokeTarget("subscription", {
      ...releaseEnv,
      SMOKE_SUBSCRIPTION_TIMEOUT_SECONDS: undefined,
    })
  ).toThrow("SMOKE_SUBSCRIPTION_TIMEOUT_SECONDS")
})

function recordingDriver(calls: string[]): ReleaseSmokeDriver {
  const step = (name: string) => async () => {
    calls.push(name)
  }
  return {
    verifyDisposableRepository: step("verify-repository"),
    startCleanStack: step("start-stack"),
    waitForHealth: step("health"),
    registerOrSignIn: step("auth"),
    reloadAndVerifySession: step("reload-session"),
    connectGitHubAndDiscoverRepository: step("github"),
    connectCodex: step("codex"),
    runMinimalCodexCall: step("codex-call"),
    waitForSystemCreatedPullRequest: async () => ({ number: 101 }),
    deliverSignedWebhook: step("webhook"),
    runProductionTick: step("production-tick"),
    runSweep: step("sweep"),
    verifyWhyStuck: step("why-stuck"),
    waitForMergedObservation: step("merged"),
    closeAgedPullRequest: step("close"),
    restartControlPlaneAndWorker: step("restart"),
    verifyPersistence: step("persistence"),
    logoutAndVerifyAuthDenial: step("logout"),
    saveEvidence: step("evidence"),
  }
}
