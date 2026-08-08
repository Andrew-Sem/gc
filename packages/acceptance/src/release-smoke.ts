import { chromium } from "@playwright/test"

export type SmokeMode = "api_key" | "subscription"

export type ReleaseSmokeTarget = {
  readonly repository: string
  readonly repositoryId: number
  readonly marker: string
}

export type ReleaseSmokeDriver = {
  verifyDisposableRepository(input: {
    readonly repository: string
    readonly repositoryId: number
    readonly marker: string
  }): Promise<void>
  startCleanStack(): Promise<void>
  waitForHealth(): Promise<void>
  registerOrSignIn(): Promise<void>
  reloadAndVerifySession(): Promise<void>
  connectGitHubAndDiscoverRepository(): Promise<void>
  connectCodex(mode: SmokeMode): Promise<void>
  runMinimalCodexCall(): Promise<void>
  waitForSystemCreatedPullRequest(): Promise<{ readonly number: number }>
  deliverSignedWebhook(pr: number): Promise<void>
  runProductionTick(): Promise<void>
  runSweep(): Promise<void>
  verifyWhyStuck(pr: number): Promise<void>
  waitForMergedObservation(pr: number): Promise<void>
  closeAgedPullRequest(): Promise<void>
  restartControlPlaneAndWorker(): Promise<void>
  verifyPersistence(): Promise<void>
  logoutAndVerifyAuthDenial(): Promise<void>
  saveEvidence(input: {
    readonly mode: SmokeMode
    readonly pr: number
  }): Promise<void>
}

export async function runReleaseSmoke(
  mode: SmokeMode,
  driver: ReleaseSmokeDriver,
  target: ReleaseSmokeTarget
): Promise<void> {
  await driver.verifyDisposableRepository(target)
  await driver.startCleanStack()
  await driver.waitForHealth()
  await driver.registerOrSignIn()
  await driver.reloadAndVerifySession()
  await driver.connectGitHubAndDiscoverRepository()
  await driver.connectCodex(mode)
  await driver.runMinimalCodexCall()
  await driver.runProductionTick()
  const pr = await driver.waitForSystemCreatedPullRequest()
  await driver.deliverSignedWebhook(pr.number)
  await driver.runSweep()
  await driver.verifyWhyStuck(pr.number)
  await driver.waitForMergedObservation(pr.number)
  await driver.closeAgedPullRequest()
  await driver.restartControlPlaneAndWorker()
  await driver.verifyPersistence()
  await driver.logoutAndVerifyAuthDenial()
  await driver.saveEvidence({ mode, pr: pr.number })
}

export type ReleaseSmokeDriverFactory = (input: {
  readonly mode: SmokeMode
  readonly env: Readonly<Record<string, string>>
}) => Promise<ReleaseSmokeDriver>

export function parseReleaseSmokeTarget(
  mode: string | undefined,
  env: Readonly<Record<string, string | undefined>>
): { readonly mode: SmokeMode; readonly target: ReleaseSmokeTarget } {
  if (mode !== "api_key" && mode !== "subscription") {
    throw new Error("release smoke mode must be api_key or subscription")
  }
  const required = [
    "SMOKE_GITHUB_REPOSITORY",
    "SMOKE_GITHUB_REPOSITORY_ID",
    "SMOKE_DISPOSABLE_MARKER",
    "SMOKE_OWNER_EMAIL",
    "SMOKE_OWNER_PASSWORD",
    "SMOKE_GITHUB_APP_ID",
    "SMOKE_GITHUB_INSTALLATION_ID",
    "SMOKE_GITHUB_PRIVATE_KEY",
    "SMOKE_GITHUB_WEBHOOK_SECRET",
    "SMOKE_GITHUB_BASE_BRANCH",
  ]
  required.push(
    mode === "api_key"
      ? "SMOKE_OPENAI_API_KEY"
      : "SMOKE_SUBSCRIPTION_TIMEOUT_SECONDS"
  )
  for (const name of required) {
    if (!env[name]) throw new Error(`${name} is required`)
  }
  const repositoryId = Number(env.SMOKE_GITHUB_REPOSITORY_ID)
  if (!Number.isSafeInteger(repositoryId) || repositoryId <= 0) {
    throw new Error("SMOKE_GITHUB_REPOSITORY_ID must be a positive integer")
  }
  return {
    mode,
    target: {
      repository: env.SMOKE_GITHUB_REPOSITORY ?? "",
      repositoryId,
      marker: env.SMOKE_DISPOSABLE_MARKER ?? "",
    },
  }
}

export async function createProductionReleaseSmokeDriver(): Promise<ReleaseSmokeDriver> {
  void chromium
  throw new Error(
    "production Playwright release smoke driver is not implemented"
  )
}

export async function main(
  argv = process.argv.slice(2),
  env = process.env,
  factory: ReleaseSmokeDriverFactory = createProductionReleaseSmokeDriver
): Promise<void> {
  const parsed = parseReleaseSmokeTarget(argv[0], env)
  const definedEnv = Object.fromEntries(
    Object.entries(env).filter(
      (entry): entry is [string, string] => entry[1] !== undefined
    )
  )
  const driver = await factory({ mode: parsed.mode, env: definedEnv })
  await runReleaseSmoke(parsed.mode, driver, parsed.target)
}

if (import.meta.url === `file://${process.argv[1]}`) {
  await main()
}
