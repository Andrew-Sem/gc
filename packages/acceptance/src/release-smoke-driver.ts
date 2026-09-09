import { execFile } from "node:child_process"
import { createHmac, createSign, randomUUID } from "node:crypto"
import { existsSync } from "node:fs"
import { mkdir, writeFile } from "node:fs/promises"
import { fileURLToPath } from "node:url"
import { promisify } from "node:util"
import {
  type Browser,
  type BrowserContext,
  chromium,
  type Locator,
  type Page,
} from "@playwright/test"
import type {
  ReleaseSmokeDriver,
  ReleaseSmokeTarget,
  SmokeMode,
} from "./release-smoke"

const execFileAsync = promisify(execFile)
const workspaceRoot = fileURLToPath(new URL("../../../", import.meta.url))
const githubApi = "https://api.github.com"
const passingConclusions = new Set(["success", "neutral", "skipped"])
const hourMs = 3_600_000

type JsonObject = Record<string, unknown>

type DriverInput = {
  readonly mode: SmokeMode
  readonly env: Readonly<Record<string, string>>
}

type PullRequest = JsonObject & {
  readonly number: number
  readonly node_id: string
  readonly title: string
  readonly body: string | null
  readonly state: string
  readonly draft: boolean
  readonly merged?: boolean
  readonly mergeable: boolean | null
  readonly created_at: string
  readonly updated_at: string
  readonly head: JsonObject & { readonly ref: string; readonly sha: string }
  readonly base: JsonObject & { readonly ref: string }
}

type Repository = JsonObject & {
  readonly id: number
  readonly full_name: string
  readonly name: string
  readonly owner: JsonObject & { readonly login: string }
  readonly topics: readonly string[]
}

type Setup = {
  readonly tenant: string
  readonly connectedRepositories: readonly string[]
  readonly codex: {
    readonly connected: boolean
    readonly method: SmokeMode | null
  }
  readonly tenantSettings: unknown
  readonly repositorySettings: Readonly<Record<string, unknown>>
}

type Evidence = {
  startedAt: string
  completedAt?: string
  mode: SmokeMode
  repository?: string
  repositoryId?: number
  markerVerified?: boolean
  healthReady?: boolean
  sessionReloaded?: boolean
  githubConnected?: boolean
  codexConnected?: boolean
  minimalCodexCall?: boolean
  primaryPr?: number
  primaryPrUrl?: string
  signedWebhook?: boolean
  invalidWebhookDenied?: boolean
  duplicateWebhookIgnored?: boolean
  whyStuck?: string
  merged?: boolean
  closePr?: number
  closePrUrl?: string
  ageClosed?: boolean
  restartVerified?: boolean
  subscriptionPersisted?: boolean
  logoutVerified?: boolean
  unauthenticatedSetupDenied?: boolean
  unauthenticatedOperatorDenied?: boolean
  invalidWorkerCredentialDenied?: boolean
  intruderTenantDenied?: boolean
}

export async function createPlaywrightReleaseSmokeDriver(
  input: DriverInput
): Promise<ReleaseSmokeDriver> {
  return new ProductionReleaseSmokeDriver(input)
}

class ProductionReleaseSmokeDriver implements ReleaseSmokeDriver {
  readonly #mode: SmokeMode
  readonly #env: Readonly<Record<string, string>>
  readonly #webUrl: string
  readonly #convexApiUrl: string
  readonly #convexSiteUrl: string
  readonly #workerHealthUrl: string
  readonly #model: string
  readonly #runId: string
  readonly #pollIntervalMs: number
  readonly #prTimeoutMs: number
  readonly #checkTimeoutMs: number
  readonly #mergeTimeoutMs: number
  readonly #closeTimeoutMs: number
  readonly #subscriptionTimeoutMs: number
  readonly #evidence: Evidence
  #browser: Browser | null = null
  #page: Page | null = null
  #target: ReleaseSmokeTarget | null = null
  #tenant: string | null = null
  #installationId: number | null = null
  #installationToken: { token: string; expiresAt: number } | null = null
  #initialPullRequests = new Set<number>()
  #primaryPr: PullRequest | null = null
  #closePr: PullRequest | null = null
  #expectedPrimaryPath: string | null = null
  #expectedClosePath: string | null = null

  constructor({ mode, env }: DriverInput) {
    this.#mode = mode
    this.#env = env
    this.#webUrl = requiredUrl(env, "PUBLIC_WEB_URL")
    this.#convexApiUrl = requiredUrl(env, "PUBLIC_CONVEX_URL")
    this.#convexSiteUrl = healthBaseUrl(
      env.CONVEX_HEALTH_URL,
      env.CONVEX_SITE_PORT,
      3210
    )
    this.#workerHealthUrl = new URL(
      "/health/ready",
      healthBaseUrl(env.WORKER_HEALTH_URL, env.WORKER_HEALTH_PORT, 8081)
    ).toString()
    if (new URL(this.#webUrl).protocol !== "https:") {
      throw new Error("PUBLIC_WEB_URL must use public HTTPS for release smoke")
    }
    this.#model = env.SMOKE_CODEX_MODEL?.trim() || "gpt-5.3-codex"
    this.#runId = `${Date.now()}-${randomUUID().slice(0, 8)}`
    this.#pollIntervalMs = seconds(env, "SMOKE_POLL_INTERVAL_SECONDS", 2, 1, 60)
    this.#prTimeoutMs = seconds(env, "SMOKE_PR_TIMEOUT_SECONDS", 1200, 30, 7200)
    this.#checkTimeoutMs = seconds(
      env,
      "SMOKE_CHECK_TIMEOUT_SECONDS",
      1200,
      30,
      7200
    )
    this.#mergeTimeoutMs = seconds(
      env,
      "SMOKE_MERGE_TIMEOUT_SECONDS",
      600,
      30,
      3600
    )
    this.#closeTimeoutMs = seconds(
      env,
      "SMOKE_CLOSE_TIMEOUT_SECONDS",
      600,
      30,
      3600
    )
    this.#subscriptionTimeoutMs = seconds(
      env,
      "SMOKE_SUBSCRIPTION_TIMEOUT_SECONDS",
      600,
      1,
      86_400
    )
    this.#evidence = {
      startedAt: new Date().toISOString(),
      mode,
    }
  }

  async verifyDisposableRepository(input: ReleaseSmokeTarget): Promise<void> {
    this.#target = input
    const { owner, name } = splitRepository(input.repository)
    const repository = await this.#github<Repository>(
      `/repos/${encodeURIComponent(owner)}/${encodeURIComponent(name)}`
    )
    if (
      repository.id !== input.repositoryId ||
      repository.full_name !== input.repository ||
      repository.owner.login !== owner ||
      repository.name !== name
    ) {
      throw new Error(
        "disposable repository identity does not exactly match owner/name and numeric ID"
      )
    }

    const topicMarked = repository.topics.includes(input.marker)
    const fixtureMarked = topicMarked
      ? false
      : await this.#hasExactMarkerFixture(owner, name, input.marker)
    if (!topicMarked && !fixtureMarked) {
      throw new Error(
        "disposable repository marker is missing as an exact topic or fixture path"
      )
    }

    this.#initialPullRequests = new Set(
      (await this.#listPullRequests("all")).map((pull) => pull.number)
    )
    this.#evidence.repository = input.repository
    this.#evidence.repositoryId = input.repositoryId
    this.#evidence.markerVerified = true
  }

  async startCleanStack(): Promise<void> {
    await this.#compose(["down", "--volumes", "--remove-orphans"], 5 * 60_000)
    await this.#compose(["up", "--build", "--detach"], 30 * 60_000)
  }

  async waitForHealth(): Promise<void> {
    const webHealth = new URL("/health/ready", this.#webUrl).toString()
    const convexHealth = new URL(
      "/health/ready",
      this.#convexSiteUrl
    ).toString()
    await poll(
      "production stack health",
      5 * 60_000,
      this.#pollIntervalMs,
      async () => {
        const [web, convex, worker] = await Promise.all([
          ready(webHealth),
          ready(convexHealth),
          ready(this.#workerHealthUrl),
        ])
        return web && convex && worker ? true : null
      }
    )
    this.#evidence.healthReady = true
  }

  async registerOrSignIn(): Promise<void> {
    const page = await this.#browserPage()
    await page.goto(this.#webUrl, { waitUntil: "domcontentloaded" })
    await page.getByRole("heading", { name: "Вход владельца" }).waitFor()
    await page
      .getByLabel("Email")
      .fill(required(this.#env, "SMOKE_OWNER_EMAIL"))
    await page
      .getByLabel("Пароль")
      .fill(required(this.#env, "SMOKE_OWNER_PASSWORD"))
    await page.getByRole("button", { name: "Создать аккаунт" }).click()

    const registered = await visibleWithin(
      page.getByRole("heading", { name: "Настройка" }),
      10_000
    )
    if (!registered) {
      await page.getByRole("button", { name: "Войти" }).click()
      await page
        .getByRole("heading", { name: "Настройка" })
        .waitFor({ timeout: 20_000 })
    }
    this.#tenant = await this.#sessionTenant()
  }

  async reloadAndVerifySession(): Promise<void> {
    const page = this.#requiredPage()
    await page.reload({ waitUntil: "domcontentloaded" })
    await page
      .getByRole("heading", { name: "Настройка" })
      .waitFor({ timeout: 20_000 })
    const tenant = await this.#sessionTenant()
    if (tenant !== this.#requiredTenant()) {
      throw new Error("owner session tenant changed after reload")
    }
    this.#evidence.sessionReloaded = true
  }

  async connectGitHubThroughInstallFlow(): Promise<void> {
    const appPage = this.#requiredPage()
    const target = this.#requiredTarget()
    const connectLink = appPage.getByRole("link", {
      name: "Подключить GitHub",
      exact: true,
    })
    await connectLink.waitFor({ timeout: 30_000 })
    const connectHref = await connectLink.getAttribute("href")
    if (connectHref !== "/github/app/connect") {
      throw new Error("production GitHub connect link has an unexpected route")
    }

    const context = await this.#githubBrowserContext()
    try {
      const page = await context.newPage()
      page.on("request", (request) => {
        this.#captureInstallationId(request.url())
      })
      await page.goto(this.#webUrl, { waitUntil: "domcontentloaded" })
      await page
        .getByRole("heading", { name: "Настройка" })
        .waitFor({ timeout: 20_000 })
      await page
        .getByRole("link", { name: "Подключить GitHub", exact: true })
        .click()
      await this.#completeGitHubInstallFlow(page)
      this.#captureInstallationId(page.url())
      if (!this.#installationId) {
        this.#installationId = await this.#discoverInstallationId()
      }
      if (!this.#installationId) {
        throw new Error(
          "GitHub install flow completed without a verifiable installation for the target repository"
        )
      }
    } finally {
      await context.close()
    }

    await appPage.reload({ waitUntil: "domcontentloaded" })
    await appPage
      .getByRole("heading", { name: "Настройка" })
      .waitFor({ timeout: 20_000 })
    await poll(
      "connected repository discovery",
      60_000,
      this.#pollIntervalMs,
      async () =>
        (await this.#setup()).connectedRepositories.includes(target.repository)
          ? true
          : null
    )
    await appPage
      .getByText(target.repository, { exact: true })
      .waitFor({ timeout: 30_000 })
    this.#evidence.githubConnected = true
  }

  async connectCodex(mode: SmokeMode): Promise<void> {
    if (mode !== this.#mode) throw new Error("release smoke mode changed")
    const page = this.#requiredPage()
    if (mode === "api_key") {
      await page
        .getByLabel("OpenAI API key")
        .fill(required(this.#env, "SMOKE_OPENAI_API_KEY"))
      await page.getByRole("button", { name: "Подключить API key" }).click()
      await page
        .getByText("Codex подключён: API key", { exact: true })
        .waitFor({ timeout: 30_000 })
    } else {
      await page
        .getByRole("button", { name: "Войти по подписке Codex" })
        .click()
      const link = page.getByRole("link", { name: "Открыть device login" })
      await link.waitFor({ timeout: 60_000 })
      const deviceUrl = await link.getAttribute("href")
      const codeText = await page.getByText(/^Код: /).textContent()
      const userCode = codeText?.replace(/^Код:\s*/, "").trim()
      if (!deviceUrl || !userCode) {
        throw new Error("subscription device URL/code was not rendered")
      }
      process.stdout.write(`Codex device URL: ${deviceUrl}\n`)
      process.stdout.write(`Codex device code: ${userCode}\n`)
      process.stdout.write(
        `Waiting up to ${Math.floor(this.#subscriptionTimeoutMs / 1000)} seconds for subscription approval.\n`
      )
      await page
        .getByText("Codex подключён: subscription", { exact: true })
        .waitFor({ timeout: this.#subscriptionTimeoutMs })
    }

    this.#expectedPrimaryPath = `gc-release-smoke-${this.#runId}.txt`
    await this.#savePolicy(this.#agentPrompt(this.#expectedPrimaryPath), "24")
    this.#evidence.codexConnected = true
  }

  async runMinimalCodexCall(): Promise<void> {
    if (this.#mode === "api_key") {
      const response = await fetch("https://api.openai.com/v1/responses", {
        method: "POST",
        headers: {
          authorization: `Bearer ${required(this.#env, "SMOKE_OPENAI_API_KEY")}`,
          "content-type": "application/json",
        },
        body: JSON.stringify({
          model: this.#model,
          input: "Reply with exactly: garbage-collector-release-smoke-ok",
          max_output_tokens: 64,
          store: false,
        }),
        signal: AbortSignal.timeout(120_000),
      })
      if (!response.ok) {
        throw new Error(`minimal Codex API call failed with ${response.status}`)
      }
      const value = (await response.json()) as unknown
      if (!isObject(value) || typeof value.id !== "string") {
        throw new Error("minimal Codex API response is invalid")
      }
    } else {
      await this.#subscriptionCodexCall()
    }
    this.#evidence.minimalCodexCall = true
  }

  async waitForSystemCreatedPullRequest(): Promise<{
    readonly number: number
  }> {
    const pull = await this.#waitForNewSystemPullRequest(
      this.#initialPullRequests,
      this.#prTimeoutMs,
      this.#requiredExpectedPrimaryPath()
    )
    this.#primaryPr = pull
    this.#evidence.primaryPr = pull.number
    this.#evidence.primaryPrUrl = this.#pullRequestUrl(pull.number)
    return { number: pull.number }
  }

  async deliverSignedWebhook(pr: number): Promise<void> {
    this.#assertPrimaryPr(pr)
    await this.#deliverPullRequestWebhook("opened", pr, true)
    this.#evidence.signedWebhook = true
  }

  async runProductionTick(): Promise<void> {
    await this.#ownerAction("operator:runProduction")
  }

  async runSweep(): Promise<void> {
    const pull = this.#requiredPrimaryPr()
    await this.#convertPullRequestToDraft(pull)
    await poll(
      `PR #${pull.number} draft conversion`,
      60_000,
      this.#pollIntervalMs,
      async () =>
        (await this.#getPullRequest(pull.number)).draft ? true : null
    )
    await this.#deliverPullRequestWebhook(
      "converted_to_draft",
      pull.number,
      false
    )
    await this.#ownerAction("operator:runSweep")
  }

  async verifyWhyStuck(pr: number): Promise<void> {
    this.#assertPrimaryPr(pr)
    const article = this.#requiredPage()
      .getByRole("article")
      .filter({
        has: this.#requiredPage().getByRole("heading", { name: `#${pr}` }),
      })
    await article.waitFor({ timeout: 30_000 })
    const text = (await article.innerText()).trim()
    if (!text.includes(`#${pr}`)) {
      throw new Error("live why-stuck UI did not render the system PR")
    }
    this.#evidence.whyStuck = text.replaceAll(/\s+/g, " ").slice(0, 500)
  }

  async waitForMergedObservation(pr: number): Promise<void> {
    this.#assertPrimaryPr(pr)
    await this.#markPullRequestReady(this.#requiredPrimaryPr())
    await poll(
      `PR #${pr} ready-for-review conversion`,
      60_000,
      this.#pollIntervalMs,
      async () => (!(await this.#getPullRequest(pr)).draft ? true : null)
    )
    await this.#deliverPullRequestWebhook("ready_for_review", pr, false)
    await this.#waitForPassingRequiredChecks(pr)
    await this.#ownerAction("operator:runSweep")
    let lastSweepAt = Date.now()
    const merged = await poll(
      `PR #${pr} merge`,
      this.#mergeTimeoutMs,
      this.#pollIntervalMs,
      async () => {
        const pull = await this.#getPullRequest(pr)
        if (pull.merged) return pull
        if (Date.now() - lastSweepAt >= 60_000) {
          await this.#ownerAction("operator:runSweep")
          lastSweepAt = Date.now()
        }
        return null
      }
    )
    await this.#deliverPullRequestWebhook("closed", merged.number, false)
    await this.#requiredPage()
      .getByRole("heading", { name: `#${pr}` })
      .waitFor({ state: "detached", timeout: 30_000 })
    this.#evidence.merged = true
  }

  async closeAgedPullRequest(): Promise<void> {
    const excluded = new Set(this.#initialPullRequests)
    excluded.add(this.#requiredPrimaryPr().number)
    this.#expectedClosePath = `gc-release-smoke-close-${this.#runId}.txt`
    await this.#savePolicy(this.#agentPrompt(this.#expectedClosePath), "1")
    await this.#ownerAction("operator:runProduction")
    const pull = await this.#waitForNewSystemPullRequest(
      excluded,
      this.#prTimeoutMs,
      this.#requiredExpectedClosePath()
    )
    this.#closePr = pull
    this.#evidence.closePr = pull.number
    this.#evidence.closePrUrl = this.#pullRequestUrl(pull.number)

    await this.#convertPullRequestToDraft(pull)
    await poll(
      `PR #${pull.number} draft conversion`,
      60_000,
      this.#pollIntervalMs,
      async () =>
        (await this.#getPullRequest(pull.number)).draft ? true : null
    )
    await this.#deliverPullRequestWebhook(
      "converted_to_draft",
      pull.number,
      false
    )
    await this.#requiredPage()
      .getByRole("heading", { name: `#${pull.number}` })
      .waitFor({ timeout: 30_000 })
    process.stdout.write(
      `Waiting for PR #${pull.number} to reach its configured 1-hour close age.\n`
    )
    await delay(hourMs)
    await this.#ownerAction("operator:runSweep")

    let lastSweepAt = Date.now()
    const closed = await poll(
      `PR #${pull.number} age close`,
      this.#closeTimeoutMs,
      this.#pollIntervalMs,
      async () => {
        const current = await this.#getPullRequest(pull.number)
        if (current.state === "closed") return current
        if (Date.now() - lastSweepAt >= 30_000) {
          await this.#ownerAction("operator:runSweep")
          lastSweepAt = Date.now()
        }
        return null
      }
    )
    if (closed.merged) {
      throw new Error("age-close PR was merged instead of closed")
    }
    await this.#deliverPullRequestWebhook("closed", pull.number, false)
    this.#evidence.ageClosed = true
  }

  async restartControlPlaneAndWorker(): Promise<void> {
    await this.#compose(["restart", "convex", "worker"], 5 * 60_000)
    await this.waitForHealth()
  }

  async verifyPersistence(): Promise<void> {
    const page = this.#requiredPage()
    await page.reload({ waitUntil: "domcontentloaded" })
    await page
      .getByRole("heading", { name: "Настройка" })
      .waitFor({ timeout: 30_000 })
    const setup = await this.#setup()
    const target = this.#requiredTarget()
    if (
      setup.tenant !== this.#requiredTenant() ||
      !setup.connectedRepositories.includes(target.repository) ||
      !setup.codex.connected ||
      setup.codex.method !== this.#mode ||
      !setup.tenantSettings ||
      !setup.repositorySettings[target.repository]
    ) {
      throw new Error("control-plane state did not persist across restart")
    }
    const primary = await this.#getPullRequest(this.#requiredPrimaryPr().number)
    const close = await this.#getPullRequest(this.#requiredClosePr().number)
    if (!primary.merged || close.state !== "closed" || close.merged) {
      throw new Error("GitHub PR outcomes changed across restart")
    }
    if (this.#mode === "subscription") {
      await this.#subscriptionCodexCall()
      this.#evidence.subscriptionPersisted = true
    }
    this.#evidence.restartVerified = true
  }

  async logoutAndVerifyAuthDenial(): Promise<void> {
    const page = this.#requiredPage()
    await page.getByRole("button", { name: "Выйти" }).click()
    await page
      .getByRole("heading", { name: "Вход владельца" })
      .waitFor({ timeout: 20_000 })

    const tokenStatus = await page.evaluate(async () => {
      const response = await fetch("/api/auth/convex/token", {
        credentials: "include",
      })
      return response.status
    })
    if (tokenStatus !== 401) {
      throw new Error("logout did not revoke browser Convex authentication")
    }
    this.#evidence.logoutVerified = true

    const setup = await this.#rawConvexCall(
      "query",
      "setup:get",
      { tenant: this.#requiredTenant() },
      null
    )
    if (setup.ok) throw new Error("unauthenticated setup query was accepted")
    this.#evidence.unauthenticatedSetupDenied = true

    const operator = await this.#rawConvexCall(
      "action",
      "operator:runProduction",
      {
        tenant: this.#requiredTenant(),
        repo: this.#requiredTarget().repository,
      },
      null
    )
    if (operator.ok)
      throw new Error("unauthenticated operator action was accepted")
    this.#evidence.unauthenticatedOperatorDenied = true

    const workerResponse = await fetch(
      new URL("/worker/protocol", this.#convexSiteUrl),
      {
        method: "POST",
        headers: {
          authorization: "Bearer release-smoke-invalid",
          "content-type": "application/json",
        },
        body: JSON.stringify({ operation: "claim", workerId: "smoke" }),
        signal: AbortSignal.timeout(10_000),
      }
    )
    if (workerResponse.status !== 401) {
      throw new Error("invalid worker credential was not denied")
    }
    this.#evidence.invalidWorkerCredentialDenied = true

    const intruderEmail = smokeIntruderEmail(
      required(this.#env, "SMOKE_OWNER_EMAIL"),
      this.#runId
    )
    await page.getByLabel("Email").fill(intruderEmail)
    await page
      .getByLabel("Пароль")
      .fill(required(this.#env, "SMOKE_OWNER_PASSWORD"))
    await page.getByRole("button", { name: "Создать аккаунт" }).click()
    await page
      .getByRole("heading", { name: "Настройка" })
      .waitFor({ timeout: 20_000 })
    const intruderToken = await page.evaluate(async () => {
      const response = await fetch("/api/auth/convex/token", {
        credentials: "include",
      })
      if (!response.ok)
        throw new Error(`Convex token failed: ${response.status}`)
      const value = await response.json()
      return value.token as string
    })
    const intruder = await this.#rawConvexCall(
      "query",
      "setup:get",
      { tenant: this.#requiredTenant() },
      intruderToken
    )
    if (intruder.ok) throw new Error("another tenant read the owner setup")
    this.#evidence.intruderTenantDenied = true
    await page.getByRole("button", { name: "Выйти" }).click()
    await page
      .getByRole("heading", { name: "Вход владельца" })
      .waitFor({ timeout: 20_000 })
  }

  async saveEvidence(input: {
    readonly mode: SmokeMode
    readonly pr: number
  }): Promise<void> {
    if (
      input.mode !== this.#mode ||
      input.pr !== this.#requiredPrimaryPr().number
    ) {
      throw new Error("release smoke evidence does not match the completed run")
    }
    const evidenceRoot = this.#env.SMOKE_EVIDENCE_DIR
      ? fileURLToPath(
          new URL(
            this.#env.SMOKE_EVIDENCE_DIR,
            new URL(`file://${workspaceRoot}/`)
          )
        )
      : `${workspaceRoot}/artifacts/release-smoke`
    await mkdir(evidenceRoot, { recursive: true })
    const prefix = `${new Date().toISOString().replaceAll(/[:.]/g, "-")}-${this.#mode}`
    const screenshot = `${evidenceRoot}/${prefix}.png`
    await this.#requiredPage().screenshot({ path: screenshot, fullPage: true })
    this.#evidence.completedAt = new Date().toISOString()
    await writeFile(
      `${evidenceRoot}/${prefix}.json`,
      `${JSON.stringify(this.#evidence, null, 2)}\n`,
      { encoding: "utf8", mode: 0o600 }
    )
    await this.#browser?.close()
    this.#browser = null
    this.#page = null
  }

  async dispose(): Promise<void> {
    await this.#browser?.close()
    this.#browser = null
    this.#page = null
  }

  async #browserPage(): Promise<Page> {
    if (this.#page) return this.#page
    this.#browser = await chromium.launch({
      headless: this.#env.SMOKE_HEADLESS !== "false",
    })
    const context = await this.#browser.newContext({
      baseURL: this.#webUrl,
      ignoreHTTPSErrors: this.#env.SMOKE_IGNORE_HTTPS_ERRORS === "true",
      extraHTTPHeaders: new URL(this.#webUrl).hostname.endsWith(
        ".ngrok-free.app"
      )
        ? { "ngrok-skip-browser-warning": "1" }
        : undefined,
    })
    this.#page = await context.newPage()
    return this.#page
  }

  async #githubBrowserContext(): Promise<BrowserContext> {
    const storageState = required(this.#env, "SMOKE_GITHUB_STORAGE_STATE")
    if (!existsSync(storageState)) {
      throw new Error(
        `SMOKE_GITHUB_STORAGE_STATE does not exist: ${storageState}`
      )
    }
    if (!this.#browser) throw new Error("Playwright browser is not started")
    const context = await this.#browser.newContext({
      baseURL: this.#webUrl,
      storageState,
      ignoreHTTPSErrors: this.#env.SMOKE_IGNORE_HTTPS_ERRORS === "true",
      extraHTTPHeaders: new URL(this.#webUrl).hostname.endsWith(
        ".ngrok-free.app"
      )
        ? { "ngrok-skip-browser-warning": "1" }
        : undefined,
    })
    await context.addCookies(await this.#requiredPage().context().cookies())
    return context
  }

  async #completeGitHubInstallFlow(page: Page): Promise<void> {
    const deadline = Date.now() + 3 * 60_000
    while (Date.now() < deadline) {
      await page
        .waitForLoadState("domcontentloaded", { timeout: 30_000 })
        .catch(() => undefined)
      await this.#assertGitHubSessionIsAuthenticated(page)
      this.#captureInstallationId(page.url())

      const url = new URL(page.url())
      if (url.origin === new URL(this.#webUrl).origin) {
        if (url.searchParams.get("github") === "connected") return
        const body = (await page.locator("body").innerText()).trim()
        throw new Error(
          `GitHub install flow returned to the app without completing: ${body.slice(0, 500)}`
        )
      }
      if (url.hostname !== "github.com") {
        throw new Error(
          `GitHub install flow navigated to an unexpected origin: ${url.origin}`
        )
      }

      if (await this.#selectGitHubRepository(page)) continue
      if (await this.#selectGitHubInstallationOwner(page)) continue
      if (await this.#authorizeGitHubOAuth(page)) continue

      const body = (await page.locator("body").innerText())
        .replaceAll(/\s+/g, " ")
        .trim()
        .slice(0, 500)
      throw new Error(
        `GitHub install flow requires unsupported interaction at ${page.url()}: ${body}`
      )
    }
    throw new Error("GitHub install and OAuth flow timed out")
  }

  async #assertGitHubSessionIsAuthenticated(page: Page): Promise<void> {
    const url = new URL(page.url())
    if (url.hostname !== "github.com") return
    const interactiveReauthentication =
      url.pathname === "/login" ||
      url.pathname.startsWith("/session") ||
      url.pathname.includes("two-factor") ||
      url.pathname.includes("account_verification") ||
      url.pathname.includes("sudo") ||
      (await firstVisible([
        page.locator('input[type="password"]'),
        page.locator('[autocomplete="one-time-code"]'),
        page.getByText(/Two-factor authentication/i),
        page.getByText(/Verify your identity/i),
        page.getByText(/Re-authenticate/i),
        page.getByRole("heading", { name: /Sign in to GitHub/i }),
      ])) !== null
    if (interactiveReauthentication) {
      throw new Error(
        "SMOKE_GITHUB_STORAGE_STATE is unauthenticated or GitHub requires interactive reauthentication; refresh the saved Playwright storage state without adding password or 2FA automation"
      )
    }
  }

  async #selectGitHubInstallationOwner(page: Page): Promise<boolean> {
    const owner = splitRepository(this.#requiredTarget().repository).owner
    const direct = await firstVisible([
      page.getByRole("button", {
        name: new RegExp(
          `(?:Install|Configure|Select).*${escapeRegExp(owner)}`,
          "i"
        ),
      }),
      page.getByRole("link", {
        name: new RegExp(
          `(?:Install|Configure|Select).*${escapeRegExp(owner)}`,
          "i"
        ),
      }),
    ])
    if (direct) {
      await direct.click()
      return true
    }

    const ownerRow = await firstVisible([
      page.locator("li").filter({ hasText: owner }),
      page.locator(".Box-row").filter({ hasText: owner }),
      page.locator('[data-testid*="account"]').filter({ hasText: owner }),
    ])
    if (!ownerRow) return false
    const action = await firstVisible([
      ownerRow.getByRole("button", { name: /Install|Configure|Select/i }),
      ownerRow.getByRole("link", { name: /Install|Configure|Select/i }),
    ])
    if (!action) return false
    await action.click()
    return true
  }

  async #selectGitHubRepository(page: Page): Promise<boolean> {
    const selectedOnly = await firstVisible([
      page.getByLabel(/Only select repositories/i),
      page.getByRole("radio", { name: /Only select repositories/i }),
    ])
    if (!selectedOnly) return false
    await selectedOnly.check()

    const target = this.#requiredTarget()
    const repositoryName = splitRepository(target.repository).name
    let repository = await firstVisible([
      page.getByLabel(target.repository, { exact: true }),
      page.getByLabel(repositoryName, { exact: true }),
      page.getByRole("checkbox", { name: target.repository, exact: true }),
      page.getByRole("checkbox", { name: repositoryName, exact: true }),
    ])
    if (!repository) {
      const selector = await firstVisible([
        page.getByRole("button", { name: /Select repositories/i }),
        page.getByText(/Select repositories/i, { exact: true }),
      ])
      if (selector) {
        await selector.click()
        const search = await firstVisible([
          page.getByPlaceholder(/Search repositories/i),
          page.getByRole("textbox", { name: /Search repositories/i }),
        ])
        if (search) await search.fill(repositoryName)
        repository = await firstVisible([
          page.getByRole("option", { name: repositoryName, exact: true }),
          page.getByRole("menuitemcheckbox", {
            name: repositoryName,
            exact: true,
          }),
          page.getByText(repositoryName, { exact: true }),
        ])
      }
    }
    if (!repository) {
      throw new Error(
        `GitHub repository selection did not offer ${target.repository}`
      )
    }
    if ((await repository.getAttribute("type")) === "checkbox") {
      if (!(await repository.isChecked())) await repository.check()
    } else {
      await repository.click()
    }

    const form = selectedOnly.locator("xpath=ancestor::form[1]")
    const submit = await firstVisible([
      form.getByRole("button", {
        name: /^(Install(?: & Authorize)?|Save|Update access)$/i,
      }),
      page.getByRole("button", {
        name: /^(Install(?: & Authorize)?|Save|Update access)$/i,
      }),
    ])
    if (!submit) {
      throw new Error(
        "GitHub repository selection did not expose a submit action"
      )
    }
    await submit.click()
    return true
  }

  async #authorizeGitHubOAuth(page: Page): Promise<boolean> {
    const url = new URL(page.url())
    if (url.pathname !== "/login/oauth/authorize") return false
    const authorize = await firstVisible([
      page.getByRole("button", { name: /^Authorize\b/i }),
      page.getByRole("button", { name: /^Continue\b/i }),
      page.getByRole("link", { name: /^Authorize\b/i }),
    ])
    if (!authorize) {
      throw new Error(
        "GitHub OAuth authorization page did not expose an authorize action"
      )
    }
    await authorize.click()
    return true
  }

  #captureInstallationId(value: string): void {
    const installationId = new URL(value).searchParams.get("installation_id")
    if (!installationId) return
    const parsed = Number(installationId)
    if (Number.isSafeInteger(parsed) && parsed > 0)
      this.#installationId = parsed
  }

  async #sessionTenant(): Promise<string> {
    const session = await this.#requiredPage().evaluate(async () => {
      const response = await fetch("/api/auth/get-session", {
        credentials: "include",
      })
      if (!response.ok) throw new Error(`session failed: ${response.status}`)
      return response.json()
    })
    if (!isObject(session) || !isObject(session.user)) {
      throw new Error("authenticated Better Auth session is missing user")
    }
    const tenant = session.user.id
    if (typeof tenant !== "string" || !tenant) {
      throw new Error("authenticated Better Auth session is missing tenant ID")
    }
    return tenant
  }

  async #savePolicy(prompt: string, closeAfterHours: string): Promise<void> {
    const page = this.#requiredPage()
    const model = page.getByLabel("Модель")
    const optionExists = await model
      .locator(`option[value="${cssEscape(this.#model)}"]`)
      .count()
    if (optionExists === 0) {
      throw new Error(
        `SMOKE_CODEX_MODEL is not available in the production App: ${this.#model}`
      )
    }
    await model.selectOption(this.#model)
    await page.getByLabel("Reasoning").selectOption("low")
    await page.getByLabel("Промпт агента").fill(prompt)
    await page
      .getByRole("button", { name: "Сохранить настройки тенанта" })
      .click()
    await poll(
      "tenant settings save",
      30_000,
      this.#pollIntervalMs,
      async () => {
        const settings = (await this.#setup()).tenantSettings
        return isObject(settings) &&
          settings.model === this.#model &&
          settings.reasoning === "low" &&
          settings.prompt === prompt
          ? true
          : null
      }
    )
    await page.getByLabel("Темп, часов").fill("1")
    await page.getByLabel("Лимит открытых PR").fill("12")
    const closeAge = page.getByLabel("Закрывать через, часов")
    await closeAge.fill(closeAfterHours)
    await page
      .getByRole("button", { name: "Сохранить настройки repository" })
      .click()
    await poll(
      "repository settings save",
      30_000,
      this.#pollIntervalMs,
      async () => {
        const settings = (await this.#setup()).repositorySettings[
          this.#requiredTarget().repository
        ]
        return isObject(settings) &&
          settings.productionEveryHours === 1 &&
          settings.openPrLimit === 12 &&
          settings.closeAfterHours === Number(closeAfterHours)
          ? true
          : null
      }
    )
  }

  #agentPrompt(path: string): string {
    return [
      `Create a new file named ${path}.`,
      `Its complete contents must be exactly: garbage-collector release smoke ${this.#runId}`,
      "Do not modify any other file. Do not run tests. Finish after creating the file.",
    ].join("\n")
  }

  async #waitForNewSystemPullRequest(
    excluded: ReadonlySet<number>,
    timeoutMs: number,
    expectedPath: string
  ): Promise<PullRequest> {
    const baseBranch = required(this.#env, "SMOKE_GITHUB_BASE_BRANCH")
    return poll(
      "system-created pull request",
      timeoutMs,
      this.#pollIntervalMs,
      async () => {
        const pulls = await this.#listPullRequests("open")
        const candidate = pulls.find(
          (pull) =>
            !excluded.has(pull.number) &&
            pull.title === "Garbage collector maintenance" &&
            pull.body ===
              "Automated maintenance produced by garbage-collector." &&
            pull.base.ref === baseBranch &&
            pull.head.ref.startsWith("gc/") &&
            Date.parse(pull.created_at) >=
              Date.parse(this.#evidence.startedAt) - 60_000
        )
        if (!candidate) return null
        return (await this.#branchContainsExpectedChange(
          candidate,
          expectedPath
        ))
          ? candidate
          : null
      }
    )
  }

  async #branchContainsExpectedChange(
    pull: PullRequest,
    expectedPath: string
  ): Promise<boolean> {
    const path = expectedPath
      .split("/")
      .map((part) => encodeURIComponent(part))
      .join("/")
    const response = await this.#githubResponse(
      `/repos/${this.#encodedRepository()}/contents/${path}?ref=${encodeURIComponent(pull.head.sha)}`
    )
    if (response.status === 404) return false
    if (!response.ok) {
      throw new Error(`GitHub PR content lookup failed with ${response.status}`)
    }
    const value = (await response.json()) as unknown
    if (
      !isObject(value) ||
      value.type !== "file" ||
      typeof value.content !== "string" ||
      value.encoding !== "base64"
    ) {
      return false
    }
    const content = Buffer.from(
      value.content.replaceAll("\n", ""),
      "base64"
    ).toString("utf8")
    return content === `garbage-collector release smoke ${this.#runId}`
  }

  async #waitForPassingRequiredChecks(pr: number): Promise<void> {
    const target = this.#requiredTarget()
    const { owner, name } = splitRepository(target.repository)
    const baseBranch = required(this.#env, "SMOKE_GITHUB_BASE_BRANCH")
    let requiredChecks: readonly JsonObject[] | null = null
    await poll(
      `PR #${pr} required checks`,
      this.#checkTimeoutMs,
      this.#pollIntervalMs,
      async () => {
        const pull = await this.#getPullRequest(pr)
        if (pull.draft)
          throw new Error(`merge-path PR #${pr} unexpectedly became draft`)
        if (pull.mergeable === false) {
          throw new Error(`merge-path PR #${pr} is not mergeable on GitHub`)
        }
        if (!requiredChecks) {
          const response = await this.#githubResponse(
            `/repos/${encodeURIComponent(owner)}/${encodeURIComponent(name)}/branches/${encodeURIComponent(baseBranch)}/protection/required_status_checks`
          )
          if (response.status === 404) {
            throw new Error(
              "disposable repository must have branch protection with app-bound required checks"
            )
          }
          if (!response.ok) {
            throw new Error(
              `GitHub required-check inventory failed with ${response.status}`
            )
          }
          const value = (await response.json()) as unknown
          requiredChecks =
            isObject(value) && Array.isArray(value.checks)
              ? value.checks.filter(isObject)
              : []
          if (requiredChecks.length === 0) {
            throw new Error(
              "disposable repository must have at least one app-bound required check"
            )
          }
        }

        const runs = await this.#listCheckRuns(pull.head.sha)
        for (const check of requiredChecks) {
          const context = check.context
          const appId = check.app_id
          if (typeof context !== "string" || typeof appId !== "number") {
            throw new Error("required check is missing exact context + app ID")
          }
          const attempts = runs
            .filter(
              (run) =>
                run.name === context &&
                isObject(run.app) &&
                run.app.id === appId
            )
            .sort(compareCheckRuns)
          const latest = attempts.at(-1)
          if (latest?.status !== "completed") return null
          if (
            typeof latest.conclusion !== "string" ||
            !passingConclusions.has(latest.conclusion)
          ) {
            throw new Error(
              `required check ${context} (app ${appId}) did not pass`
            )
          }
        }
        return true
      }
    )
  }

  async #listCheckRuns(headSha: string): Promise<JsonObject[]> {
    const { owner, name } = splitRepository(this.#requiredTarget().repository)
    const items: JsonObject[] = []
    for (let page = 1; page <= 20; page += 1) {
      const value = await this.#github<unknown>(
        `/repos/${encodeURIComponent(owner)}/${encodeURIComponent(name)}/commits/${encodeURIComponent(headSha)}/check-runs?per_page=100&page=${page}`
      )
      const runs =
        isObject(value) && Array.isArray(value.check_runs)
          ? value.check_runs.filter(isObject)
          : []
      items.push(...runs)
      if (runs.length < 100) return items
    }
    throw new Error(
      "GitHub check-run pagination exceeded the bounded smoke limit"
    )
  }

  async #deliverPullRequestWebhook(
    action: string,
    pr: number,
    checkNegativeBoundary: boolean
  ): Promise<void> {
    const pull = await this.#getPullRequest(pr)
    const repository = await this.#github<Repository>(
      `/repos/${this.#encodedRepository()}`
    )
    const body = JSON.stringify({
      action,
      installation: {
        id: this.#requiredInstallationId(),
      },
      repository,
      pull_request: pull,
    })
    const endpoint = new URL("/github/webhook", this.#webUrl)
    const deliveryId = `release-smoke-${this.#runId}-${action}-${pr}`
    if (checkNegativeBoundary) {
      const denied = await fetch(endpoint, {
        method: "POST",
        headers: {
          "content-type": "application/json",
          "x-github-delivery": `${deliveryId}-invalid`,
          "x-github-event": "pull_request",
          "x-hub-signature-256": signature(
            `${required(this.#env, "GITHUB_APP_WEBHOOK_SECRET")}-invalid`,
            body
          ),
        },
        body,
        signal: AbortSignal.timeout(30_000),
      })
      if (denied.status !== 401) {
        throw new Error("invalid webhook HMAC was not denied")
      }
      this.#evidence.invalidWebhookDenied = true
    }

    const headers = {
      "content-type": "application/json",
      "x-github-delivery": deliveryId,
      "x-github-event": "pull_request",
      "x-hub-signature-256": signature(
        required(this.#env, "GITHUB_APP_WEBHOOK_SECRET"),
        body
      ),
    }
    const accepted = await fetch(endpoint, {
      method: "POST",
      headers,
      body,
      signal: AbortSignal.timeout(30_000),
    })
    if (accepted.status !== 202) {
      throw new Error(`signed webhook failed with ${accepted.status}`)
    }
    if (checkNegativeBoundary) {
      const duplicate = await fetch(endpoint, {
        method: "POST",
        headers,
        body,
        signal: AbortSignal.timeout(30_000),
      })
      const value = (await duplicate.json()) as unknown
      if (
        duplicate.status !== 202 ||
        !isObject(value) ||
        value.duplicate !== true
      ) {
        throw new Error("duplicate webhook delivery was not ignored")
      }
      this.#evidence.duplicateWebhookIgnored = true
    }
  }

  async #ownerAction(path: string): Promise<void> {
    const result = await this.#authenticatedConvexCall("action", path, {
      tenant: this.#requiredTenant(),
      repo: this.#requiredTarget().repository,
    })
    if (!result.ok) throw new Error(`${path} was denied for the owner session`)
  }

  async #setup(): Promise<Setup> {
    const result = await this.#authenticatedConvexCall("query", "setup:get", {
      tenant: this.#requiredTenant(),
    })
    if (!result.ok || !isObject(result.value)) {
      throw new Error("owner setup query failed")
    }
    return result.value as Setup
  }

  async #authenticatedConvexCall(
    kind: "query" | "mutation" | "action",
    path: string,
    args: JsonObject
  ): Promise<{ readonly ok: boolean; readonly value?: unknown }> {
    const token = await this.#requiredPage().evaluate(async () => {
      const response = await fetch("/api/auth/convex/token", {
        credentials: "include",
      })
      if (!response.ok)
        throw new Error(`Convex token failed: ${response.status}`)
      const value = await response.json()
      return value.token as string
    })
    if (!token) throw new Error("owner Convex token is missing")
    return this.#rawConvexCall(kind, path, args, token)
  }

  async #rawConvexCall(
    kind: "query" | "mutation" | "action",
    path: string,
    args: JsonObject,
    token: string | null
  ): Promise<{ readonly ok: boolean; readonly value?: unknown }> {
    const response = await fetch(new URL(`/api/${kind}`, this.#convexApiUrl), {
      method: "POST",
      headers: {
        ...(token ? { authorization: `Bearer ${token}` } : {}),
        "content-type": "application/json",
        "convex-client": "release-smoke",
      },
      body: JSON.stringify({
        path,
        format: "convex_encoded_json",
        args: [args],
      }),
      signal: AbortSignal.timeout(60_000),
    })
    if (!response.ok && response.status !== 560) return { ok: false }
    const value = (await response.json()) as unknown
    if (!isObject(value) || value.status !== "success") return { ok: false }
    return { ok: true, value: value.value }
  }

  async #subscriptionCodexCall(): Promise<void> {
    const codexHomeRoot = required(this.#env, "CODEX_HOME_ROOT")
    const codexHome = `${codexHomeRoot}/tenant-${encodeURIComponent(this.#requiredTenant())}`
    await this.#compose(
      [
        "exec",
        "-T",
        "-e",
        `CODEX_HOME=${codexHome}`,
        "worker",
        "codex",
        "exec",
        "--skip-git-repo-check",
        "--sandbox",
        "read-only",
        "--model",
        this.#model,
        "Reply with exactly: garbage-collector-release-smoke-ok",
      ],
      3 * 60_000
    )
  }

  async #convertPullRequestToDraft(pull: PullRequest): Promise<void> {
    const response = await this.#githubResponse("/graphql", {
      method: "POST",
      body: JSON.stringify({
        query:
          "mutation($id: ID!) { convertPullRequestToDraft(input: { pullRequestId: $id }) { pullRequest { id isDraft } } }",
        variables: { id: pull.node_id },
      }),
    })
    if (!response.ok) {
      throw new Error(`GitHub draft conversion failed with ${response.status}`)
    }
    const value = (await response.json()) as unknown
    if (
      !isObject(value) ||
      (Array.isArray(value.errors) && value.errors.length > 0)
    ) {
      throw new Error("GitHub draft conversion returned an error")
    }
  }

  async #markPullRequestReady(pull: PullRequest): Promise<void> {
    const response = await this.#githubResponse("/graphql", {
      method: "POST",
      body: JSON.stringify({
        query:
          "mutation($id: ID!) { markPullRequestReadyForReview(input: { pullRequestId: $id }) { pullRequest { id isDraft } } }",
        variables: { id: pull.node_id },
      }),
    })
    if (!response.ok) {
      throw new Error(
        `GitHub ready-for-review conversion failed with ${response.status}`
      )
    }
    const value = (await response.json()) as unknown
    if (
      !isObject(value) ||
      (Array.isArray(value.errors) && value.errors.length > 0)
    ) {
      throw new Error("GitHub ready-for-review conversion returned an error")
    }
  }

  async #getPullRequest(pr: number): Promise<PullRequest> {
    const value = await this.#github<unknown>(
      `/repos/${this.#encodedRepository()}/pulls/${pr}`
    )
    if (!isPullRequest(value)) {
      throw new Error(`GitHub returned an invalid PR #${pr}`)
    }
    return value
  }

  async #listPullRequests(state: "open" | "all"): Promise<PullRequest[]> {
    const pulls: PullRequest[] = []
    for (let page = 1; page <= 10; page += 1) {
      const value = await this.#github<unknown>(
        `/repos/${this.#encodedRepository()}/pulls?state=${state}&sort=created&direction=desc&per_page=100&page=${page}`
      )
      if (!Array.isArray(value)) throw new Error("GitHub pull list is invalid")
      const pagePulls = value.filter(isPullRequest)
      pulls.push(...pagePulls)
      if (value.length < 100) return pulls
    }
    throw new Error("GitHub PR pagination exceeded the bounded smoke limit")
  }

  async #hasExactMarkerFixture(
    owner: string,
    name: string,
    marker: string
  ): Promise<boolean> {
    if (
      !marker ||
      marker.startsWith("/") ||
      marker.split("/").some((part) => !part || part === "." || part === "..")
    ) {
      throw new Error(
        "SMOKE_DISPOSABLE_MARKER is not a safe exact fixture path"
      )
    }
    const path = marker
      .split("/")
      .map((part) => encodeURIComponent(part))
      .join("/")
    const response = await this.#githubResponse(
      `/repos/${encodeURIComponent(owner)}/${encodeURIComponent(name)}/contents/${path}?ref=${encodeURIComponent(required(this.#env, "SMOKE_GITHUB_BASE_BRANCH"))}`
    )
    if (response.status === 404) return false
    if (!response.ok) {
      throw new Error(
        `GitHub marker fixture lookup failed with ${response.status}`
      )
    }
    const value = (await response.json()) as unknown
    return isObject(value) && value.type === "file" && value.path === marker
  }

  async #github<T>(path: string, init: RequestInit = {}): Promise<T> {
    const response = await this.#githubResponse(path, init)
    if (!response.ok) {
      throw new Error(`GitHub request failed with ${response.status}`)
    }
    return (await response.json()) as T
  }

  async #githubResponse(
    path: string,
    init: RequestInit = {}
  ): Promise<Response> {
    const token = await this.#installationAccessToken()
    return fetch(`${githubApi}${path}`, {
      ...init,
      headers: {
        accept: "application/vnd.github+json",
        authorization: `Bearer ${token}`,
        "content-type": "application/json",
        "user-agent": "garbage-collector-release-smoke",
        "x-github-api-version": "2022-11-28",
        ...(init.headers ?? {}),
      },
      signal: init.signal ?? AbortSignal.timeout(60_000),
    })
  }

  async #installationAccessToken(): Promise<string> {
    if (
      this.#installationToken &&
      this.#installationToken.expiresAt - Date.now() > 5 * 60_000
    ) {
      return this.#installationToken.token
    }
    this.#installationId ??= await this.#discoverInstallationId()
    const installationId = this.#requiredInstallationId()
    const response = await fetch(
      `${githubApi}/app/installations/${installationId}/access_tokens`,
      {
        method: "POST",
        headers: {
          accept: "application/vnd.github+json",
          authorization: `Bearer ${this.#appJwt()}`,
          "content-type": "application/json",
          "user-agent": "garbage-collector-release-smoke",
          "x-github-api-version": "2022-11-28",
        },
        signal: AbortSignal.timeout(60_000),
      }
    )
    if (!response.ok) {
      throw new Error(
        `GitHub installation-token exchange failed with ${response.status}`
      )
    }
    const value = (await response.json()) as unknown
    if (!isObject(value) || typeof value.token !== "string") {
      throw new Error("GitHub installation-token response is invalid")
    }
    const expiresAt =
      typeof value.expires_at === "string"
        ? Date.parse(value.expires_at)
        : Date.now() + 55 * 60_000
    this.#installationToken = { token: value.token, expiresAt }
    return value.token
  }

  async #discoverInstallationId(): Promise<number | null> {
    const response = await fetch(
      `${githubApi}/repos/${this.#encodedRepository()}/installation`,
      {
        headers: {
          accept: "application/vnd.github+json",
          authorization: `Bearer ${this.#appJwt()}`,
          "user-agent": "garbage-collector-release-smoke",
          "x-github-api-version": "2022-11-28",
        },
        signal: AbortSignal.timeout(60_000),
      }
    )
    if (response.status === 404) return null
    if (!response.ok) {
      throw new Error(
        `GitHub installation discovery failed with ${response.status}`
      )
    }
    const value = (await response.json()) as unknown
    if (!isObject(value) || typeof value.id !== "number") {
      throw new Error("GitHub installation discovery response is invalid")
    }
    return value.id
  }

  #appJwt(): string {
    const appId = positiveInteger(this.#env, "GITHUB_APP_ID")
    const now = Math.floor(Date.now() / 1000)
    const header = base64url(JSON.stringify({ alg: "RS256", typ: "JWT" }))
    const payload = base64url(
      JSON.stringify({ iat: now - 60, exp: now + 9 * 60, iss: appId })
    )
    const unsigned = `${header}.${payload}`
    const signer = createSign("RSA-SHA256")
    signer.update(unsigned)
    signer.end()
    return `${unsigned}.${signer.sign(
      privateKey(this.#env.GITHUB_APP_PRIVATE_KEY, "GITHUB_APP_PRIVATE_KEY"),
      "base64url"
    )}`
  }

  async #compose(args: readonly string[], timeout: number): Promise<void> {
    try {
      await execFileAsync("docker", ["compose", ...args], {
        cwd: workspaceRoot,
        env: { ...process.env, ...this.#env },
        timeout,
        maxBuffer: 20 * 1024 * 1024,
      })
    } catch {
      throw new Error(`docker compose ${args[0] ?? "command"} failed`)
    }
  }

  #encodedRepository(): string {
    const { owner, name } = splitRepository(this.#requiredTarget().repository)
    return `${encodeURIComponent(owner)}/${encodeURIComponent(name)}`
  }

  #pullRequestUrl(pr: number): string {
    return `https://github.com/${this.#requiredTarget().repository}/pull/${pr}`
  }

  #requiredPage(): Page {
    if (!this.#page) throw new Error("Playwright browser is not started")
    return this.#page
  }

  #requiredTarget(): ReleaseSmokeTarget {
    if (!this.#target) throw new Error("disposable repository is not verified")
    return this.#target
  }

  #requiredTenant(): string {
    if (!this.#tenant) throw new Error("owner tenant is not authenticated")
    return this.#tenant
  }

  #requiredInstallationId(): number {
    if (!this.#installationId) {
      throw new Error("GitHub installation ID was not discovered")
    }
    return this.#installationId
  }

  #requiredPrimaryPr(): PullRequest {
    if (!this.#primaryPr) throw new Error("system PR is not created")
    return this.#primaryPr
  }

  #requiredClosePr(): PullRequest {
    if (!this.#closePr) throw new Error("age-close PR is not created")
    return this.#closePr
  }

  #requiredExpectedPrimaryPath(): string {
    if (!this.#expectedPrimaryPath)
      throw new Error("primary Codex change is not configured")
    return this.#expectedPrimaryPath
  }

  #requiredExpectedClosePath(): string {
    if (!this.#expectedClosePath)
      throw new Error("close Codex change is not configured")
    return this.#expectedClosePath
  }

  #assertPrimaryPr(pr: number): void {
    if (pr !== this.#requiredPrimaryPr().number) {
      throw new Error("release smoke PR changed between steps")
    }
  }
}

async function poll<T>(
  description: string,
  timeoutMs: number,
  intervalMs: number,
  operation: () => Promise<T | null>
): Promise<T> {
  const deadline = Date.now() + timeoutMs
  let lastError: unknown = null
  while (Date.now() < deadline) {
    try {
      const result = await operation()
      if (result !== null) return result
      lastError = null
    } catch (error) {
      lastError = error
    }
    await delay(Math.min(intervalMs, Math.max(1, deadline - Date.now())))
  }
  if (lastError instanceof Error) {
    throw new Error(`${description} timed out: ${lastError.message}`)
  }
  throw new Error(
    `${description} timed out after ${Math.ceil(timeoutMs / 1000)}s`
  )
}

async function ready(url: string): Promise<boolean> {
  try {
    const response = await fetch(url, {
      redirect: "error",
      signal: AbortSignal.timeout(5_000),
    })
    return response.ok
  } catch {
    return false
  }
}

async function visibleWithin(
  locator: ReturnType<Page["getByRole"]>,
  timeout: number
): Promise<boolean> {
  try {
    await locator.waitFor({ timeout })
    return true
  } catch {
    return false
  }
}

function required(env: Readonly<Record<string, string>>, name: string): string {
  const value = env[name]?.trim()
  if (!value) throw new Error(`${name} is required`)
  return value
}

function requiredUrl(
  env: Readonly<Record<string, string>>,
  name: string
): string {
  const value = required(env, name)
  try {
    return new URL(value).toString()
  } catch {
    throw new Error(`${name} must be an absolute URL`)
  }
}

function healthBaseUrl(
  configured: string | undefined,
  port: string | undefined,
  fallbackPort: number
): string {
  if (configured?.trim()) {
    try {
      return new URL(configured).toString()
    } catch {
      throw new Error("health URL must be an absolute URL")
    }
  }
  const numericPort = port ? Number(port) : fallbackPort
  if (
    !Number.isInteger(numericPort) ||
    numericPort < 1 ||
    numericPort > 65_535
  ) {
    throw new Error("health port must be an integer between 1 and 65535")
  }
  return `http://127.0.0.1:${numericPort}`
}

function seconds(
  env: Readonly<Record<string, string>>,
  name: string,
  fallback: number,
  minimum: number,
  maximum: number
): number {
  const raw = env[name]
  const value = raw === undefined ? fallback : Number(raw)
  if (!Number.isInteger(value) || value < minimum || value > maximum) {
    throw new Error(
      `${name} must be an integer between ${minimum} and ${maximum}`
    )
  }
  return value * 1000
}

function positiveInteger(
  env: Readonly<Record<string, string>>,
  name: string
): number {
  const value = Number(required(env, name))
  if (!Number.isSafeInteger(value) || value <= 0) {
    throw new Error(`${name} must be a positive integer`)
  }
  return value
}

function privateKey(value: string | undefined, name: string): string {
  if (!value) throw new Error(`${name} is required`)
  return value.includes("\n") ? value : value.replaceAll("\\n", "\n")
}

async function firstVisible(
  locators: readonly Locator[]
): Promise<Locator | null> {
  for (const locator of locators) {
    if ((await locator.count()) > 0 && (await locator.first().isVisible())) {
      return locator.first()
    }
  }
  return null
}

function escapeRegExp(value: string): string {
  return value.replaceAll(/[.*+?^${}()|[\]\\]/g, "\\$&")
}

function splitRepository(repository: string): {
  readonly owner: string
  readonly name: string
} {
  const [owner, name, ...rest] = repository.split("/")
  if (!owner || !name || rest.length > 0) {
    throw new Error("SMOKE_GITHUB_REPOSITORY must be exact owner/name")
  }
  return { owner, name }
}

function signature(secret: string, body: string): string {
  return `sha256=${createHmac("sha256", secret).update(body).digest("hex")}`
}

function base64url(value: string): string {
  return Buffer.from(value).toString("base64url")
}

function smokeIntruderEmail(ownerEmail: string, runId: string): string {
  const separator = ownerEmail.lastIndexOf("@")
  if (separator <= 0 || separator === ownerEmail.length - 1) {
    throw new Error("SMOKE_OWNER_EMAIL must be a valid email address")
  }
  return `${ownerEmail.slice(0, separator)}+gc-intruder-${runId.replaceAll(
    /[^a-zA-Z0-9]/g,
    ""
  )}@${ownerEmail.slice(separator + 1)}`
}

function cssEscape(value: string): string {
  return value.replaceAll(/["\\]/g, "\\$&")
}

function isObject(value: unknown): value is JsonObject {
  return typeof value === "object" && value !== null && !Array.isArray(value)
}

function isPullRequest(value: unknown): value is PullRequest {
  return (
    isObject(value) &&
    typeof value.number === "number" &&
    typeof value.node_id === "string" &&
    typeof value.title === "string" &&
    (typeof value.body === "string" || value.body === null) &&
    typeof value.state === "string" &&
    typeof value.draft === "boolean" &&
    (typeof value.merged === "boolean" || value.merged === undefined) &&
    (typeof value.mergeable === "boolean" || value.mergeable === null) &&
    typeof value.created_at === "string" &&
    typeof value.updated_at === "string" &&
    isObject(value.head) &&
    typeof value.head.ref === "string" &&
    typeof value.head.sha === "string" &&
    isObject(value.base) &&
    typeof value.base.ref === "string"
  )
}

function compareCheckRuns(left: JsonObject, right: JsonObject): number {
  const leftTime =
    typeof left.created_at === "string" ? Date.parse(left.created_at) : 0
  const rightTime =
    typeof right.created_at === "string" ? Date.parse(right.created_at) : 0
  if (leftTime !== rightTime) return leftTime - rightTime
  const leftId = typeof left.id === "number" ? left.id : 0
  const rightId = typeof right.id === "number" ? right.id : 0
  return leftId - rightId
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}
