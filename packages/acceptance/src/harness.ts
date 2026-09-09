import {
  configureConvexRuntime,
  type DataModel,
  registerAuthTestComponent,
  type SharedGitHubApp,
} from "@workspace/convex"
import type { DomainCommand, Timestamp } from "@workspace/domain"
import type {
  AgentRunResult,
  GitHubCredentials,
  WorkspaceCheckout,
} from "@workspace/worker"
import type { ProductionWorkerPorts } from "@workspace/worker/runtime"
import type { TestConvexForDataModel } from "convex-test"
import type { AcceptanceHarness, BrowserActor, RecordedCall } from "./contract"

type ProductionWiring = {
  readonly backend: TestConvexForDataModel<DataModel>
  readonly masterKey: string
  readonly sharedGitHubApp: SharedGitHubApp
}

type CookieJar = Map<string, string>

type GitHubInstallationFixture = {
  readonly installationId: number
  readonly appId: number
  readonly accessibleToGitHubUserIds: readonly number[]
  readonly repositoryPages: readonly (readonly string[])[]
  readonly clientId: string
  readonly permissions: Readonly<Record<string, string>>
  readonly suspended: boolean
}

const PUBLIC_WEB_URL = "https://gc.example.com"
const REQUIRED_GITHUB_PERMISSIONS = {
  administration: "read",
  checks: "read",
  contents: "write",
  metadata: "read",
  pull_requests: "write",
} as const

/**
 * Implementation target: compose real Convex functions under convex-test, the
 * real worker run-once entry point, and fake network/process/filesystem ports.
 */
export async function createAcceptanceHarness(
  wiring: ProductionWiring
): Promise<AcceptanceHarness> {
  process.env.BETTER_AUTH_ALLOWED_ORIGIN = PUBLIC_WEB_URL
  process.env.BETTER_AUTH_SECRET = "acceptance-better-auth-secret-32-bytes"
  process.env.CONVEX_SITE_URL = PUBLIC_WEB_URL
  registerAuthTestComponent(wiring.backend)

  const browserSessions = new Map<
    Exclude<BrowserActor, "anonymous">,
    {
      readonly cookies: CookieJar
      readonly subject: string
      readonly sessionId: string
    }
  >()

  async function authRequest(
    cookies: CookieJar,
    path: string,
    init: RequestInit = {}
  ): Promise<Response> {
    const response = await wiring.backend.fetch(path, {
      ...init,
      headers: {
        origin: PUBLIC_WEB_URL,
        ...(init.body ? { "content-type": "application/json" } : {}),
        ...(cookies.size > 0 ? { cookie: cookieHeader(cookies) } : {}),
        ...Object.fromEntries(new Headers(init.headers).entries()),
      },
    })
    storeCookies(cookies, response.headers)
    return response
  }

  for (const actor of ["owner", "other_owner", "intruder"] as const) {
    const cookies: CookieJar = new Map()
    const response = await authRequest(cookies, "/api/auth/sign-up/email", {
      method: "POST",
      body: JSON.stringify({
        email: `${actor}@acceptance.example.com`,
        password: "acceptance password",
        name: actor,
      }),
    })
    if (!response.ok) {
      throw new Error(`acceptance auth setup failed: ${response.status}`)
    }
    const sessionResponse = await authRequest(cookies, "/api/auth/get-session")
    const session = (await sessionResponse.json()) as {
      readonly user?: { readonly id?: string }
      readonly session?: { readonly id?: string }
    } | null
    const subject = session?.user?.id
    const sessionId = session?.session?.id
    if (!subject || !sessionId) {
      throw new Error(`acceptance auth session missing for ${actor}`)
    }
    browserSessions.set(actor, { cookies, subject, sessionId })
  }

  await wiring.backend.run(async (ctx) => {
    for (const { subject } of browserSessions.values()) {
      const owner = await ctx.db
        .query("owners")
        .withIndex("by_subject", (query) => query.eq("subject", subject))
        .unique()
      if (owner) await ctx.db.delete(owner._id)
      const tenant = await ctx.db
        .query("tenants")
        .withIndex("by_tenant", (query) => query.eq("tenant", subject))
        .unique()
      if (tenant) await ctx.db.delete(tenant._id)
    }
    for (const [tenant, subject] of [
      ["acme", "account-a"],
      ["globex", "account-b"],
    ] as const) {
      const boundary = await ctx.db
        .query("tenants")
        .withIndex("by_tenant", (query) => query.eq("tenant", tenant))
        .unique()
      if (!boundary) await ctx.db.insert("tenants", { tenant })
      const owner = await ctx.db
        .query("owners")
        .withIndex("by_tenant", (query) => query.eq("tenant", tenant))
        .unique()
      if (!owner) await ctx.db.insert("owners", { tenant, subject })
    }
  })
  let now = 0
  let nextCreatedHeadSha = "abc"
  let failAgentBeforePr = false
  let failAfterLease = false
  let failAfterExternalEffect = false
  let failMerge = false
  let failRepositoryDiscoveryAfterPage: number | null = null
  let agentsHeld = false
  let releaseAgents: (() => void) | null = null
  let agentGate = Promise.resolve()
  let lastReport: {
    readonly taskId: string
    readonly leaseToken: string
    readonly command: DomainCommand
  } | null = null
  let nextPr = 1

  const calls: RecordedCall[] = []
  const reportCounts: number[] = []
  const reportIndexes = new Map<string, number>()
  const oauthCodes = new Map<string, number>()
  const oauthTokens = new Map<string, number>()
  const installations = new Map<number, GitHubInstallationFixture>()
  const installationTokens = new Map<
    string,
    { token: string; expiresAt: number }
  >()
  const authorizedDeviceCodes = new Set<string>()
  const reconciliations = new Map<
    string,
    { readonly pullRequest: unknown; readonly runs: readonly unknown[] }
  >()
  const sweepLookups = new Map<
    string,
    {
      readonly pullRequest: unknown
      readonly required: unknown
      readonly runs: unknown
      readonly requiredChecksComplete: boolean
      readonly checkRunsComplete: boolean
    }
  >()
  const activeAgents = new Map<string, number>()
  const maximumAgents = new Map<string, number>()
  const waiters = {
    agentStarts: [] as { count: number; resolve: () => void }[],
    contentions: [] as { count: number; resolve: () => void }[],
    renewals: [] as { count: number; resolve: () => void }[],
  }
  let agentStarts = 0
  let contentions = 0
  let renewals = 0
  let installationTokenSequence = 0

  function lookupKey(tenant: string, repo: string, pr: number): string {
    return `${tenant}\u0000${repo}\u0000${pr}`
  }

  function repositoryKey(tenant: string, repo: string): string {
    return `${tenant}\u0000${repo}`
  }

  function resolveWaiters(
    list: { count: number; resolve: () => void }[],
    value: number
  ): void {
    for (const waiter of [...list]) {
      if (value >= waiter.count) waiter.resolve()
    }
    list.splice(
      0,
      list.length,
      ...list.filter((waiter) => value < waiter.count)
    )
  }

  function waitFor(
    list: { count: number; resolve: () => void }[],
    current: number,
    count: number
  ): Promise<void> {
    if (current >= count) return Promise.resolve()
    return new Promise((resolve) => list.push({ count, resolve }))
  }

  function record(call: RecordedCall): void {
    if (
      call.operation === "github.listInstallationRepositories" &&
      calls.some(
        (candidate) =>
          candidate.operation === call.operation &&
          candidate.installationId === call.installationId &&
          candidate.repositoryPage === call.repositoryPage
      )
    ) {
      return
    }
    calls.push(call)
  }

  function recordRepositoryPage(call: RecordedCall): void {
    record(call)
  }

  function requiredGitHubToken(credentials: GitHubCredentials): string {
    if (!credentials.token) {
      throw new Error("worker GitHub installation token is required")
    }
    return credentials.token
  }

  async function browserGet(
    actor: BrowserActor,
    path: string
  ): Promise<Response> {
    if (actor === "anonymous") return authRequest(new Map(), path)
    const session = browserSessions.get(actor)
    if (!session) throw new Error(`unknown browser actor: ${actor}`)
    const tenant =
      actor === "owner" ? "acme" : actor === "other_owner" ? "globex" : null
    if (!tenant) return authRequest(session.cookies, path)

    const previousSubject = await wiring.backend.run(async (ctx) => {
      const owner = await ctx.db
        .query("owners")
        .withIndex("by_tenant", (query) => query.eq("tenant", tenant))
        .unique()
      if (!owner) throw new Error(`owner missing for ${tenant}`)
      await ctx.db.delete(owner._id)
      await ctx.db.insert("owners", { tenant, subject: session.subject })
      return owner.subject
    })
    try {
      return await authRequest(session.cookies, path)
    } finally {
      await wiring.backend.run(async (ctx) => {
        const owner = await ctx.db
          .query("owners")
          .withIndex("by_tenant", (query) => query.eq("tenant", tenant))
          .unique()
        if (owner) await ctx.db.delete(owner._id)
        await ctx.db.insert("owners", { tenant, subject: previousSubject })
      })
    }
  }

  function pullRequestHead(value: unknown): string | undefined {
    if (
      typeof value !== "object" ||
      value === null ||
      !("head" in value) ||
      typeof value.head !== "object" ||
      value.head === null ||
      !("sha" in value.head) ||
      typeof value.head.sha !== "string"
    ) {
      return undefined
    }
    return value.head.sha
  }

  configureConvexRuntime({
    masterKey: wiring.masterKey,
    workerToken: "acceptance-worker-token",
    codexHomeRoot: "/acceptance/codex-home",
    publicWebUrl: PUBLIC_WEB_URL,
    sharedGitHubApp: wiring.sharedGitHubApp,
    ports: {
      now: () => now,
      exchangeGitHubOAuthCode: async (input) => {
        if (
          input.clientId !== wiring.sharedGitHubApp.clientId ||
          input.clientSecret !== wiring.sharedGitHubApp.clientSecret
        ) {
          throw Object.assign(new Error("invalid GitHub OAuth client"), {
            status: 401,
          })
        }
        const githubUserId = oauthCodes.get(input.code)
        if (!githubUserId) {
          throw Object.assign(new Error("invalid GitHub OAuth code"), {
            status: 401,
          })
        }
        oauthCodes.delete(input.code)
        const accessToken = `github-user-${githubUserId}-${input.code}`
        oauthTokens.set(accessToken, githubUserId)
        return { accessToken, githubUserId }
      },
      listGitHubUserInstallations: async ({ accessToken, page }) => {
        const githubUserId = oauthTokens.get(accessToken)
        if (!githubUserId) {
          throw Object.assign(new Error("invalid GitHub user token"), {
            status: 401,
          })
        }
        record({
          operation: "github.listUserInstallations",
          githubUserId,
          repositoryPage: page,
        })
        const available = [...installations.values()]
          .filter((installation) =>
            installation.accessibleToGitHubUserIds.includes(githubUserId)
          )
          .map((installation) => installation.installationId)
          .sort((left, right) => left - right)
        const start = (page - 1) * 100
        return {
          installationIds: available.slice(start, start + 100),
          hasNext: start + 100 < available.length,
        }
      },
      inspectGitHubAppInstallation: async (input) => {
        const installation = installations.get(input.installationId)
        record({
          operation: "github.inspectInstallation",
          appId: installation?.appId ?? input.appId,
          installationId: input.installationId,
        })
        if (
          input.appId !== wiring.sharedGitHubApp.appId ||
          input.clientId !== wiring.sharedGitHubApp.clientId ||
          input.privateKey !== wiring.sharedGitHubApp.privateKey ||
          !installation
        ) {
          throw Object.assign(new Error("GitHub installation not found"), {
            status: 403,
          })
        }
        return {
          appId: installation.appId,
          clientId: installation.clientId,
          permissions: installation.permissions,
          suspended: installation.suspended,
        }
      },
      listGitHubInstallationRepositories: async (input) => {
        const installation = installations.get(input.installationId)
        if (
          !installation ||
          input.appId !== wiring.sharedGitHubApp.appId ||
          input.privateKey !== wiring.sharedGitHubApp.privateKey
        ) {
          throw Object.assign(new Error("GitHub installation not found"), {
            status: 403,
          })
        }
        if (
          failRepositoryDiscoveryAfterPage !== null &&
          input.page > failRepositoryDiscoveryAfterPage
        ) {
          failRepositoryDiscoveryAfterPage = null
          throw Object.assign(
            new Error("transient repository discovery failure"),
            { status: 502 }
          )
        }
        const repositories = installation.repositoryPages[input.page - 1] ?? []
        return {
          repositories: [...repositories],
          hasNext: input.page < installation.repositoryPages.length,
        }
      },
      revokeGitHubUserAuthorization: async (input) => {
        if (
          input.clientId !== wiring.sharedGitHubApp.clientId ||
          input.clientSecret !== wiring.sharedGitHubApp.clientSecret ||
          oauthTokens.get(input.accessToken) !== input.githubUserId
        ) {
          throw Object.assign(new Error("invalid GitHub user authorization"), {
            status: 401,
          })
        }
        oauthTokens.delete(input.accessToken)
      },
      createGitHubInstallationToken: async (input) => {
        const installation = installations.get(input.installationId)
        const repositories = installation?.repositoryPages.flat() ?? []
        if (
          !installation ||
          installation.appId !== input.appId ||
          input.appId !== wiring.sharedGitHubApp.appId ||
          input.privateKey !== wiring.sharedGitHubApp.privateKey ||
          installation.suspended ||
          (input.repo !== undefined && !repositories.includes(input.repo))
        ) {
          throw Object.assign(
            new Error("GitHub installation or repository is not eligible"),
            { status: 403 }
          )
        }
        const cacheKey = `${input.installationId}\u0000${input.repo ?? "*"}`
        const cached = installationTokens.get(cacheKey)
        if (cached && cached.expiresAt > now) return cached
        installationTokenSequence += 1
        const token = {
          token: `installation-token-${input.installationId}-${installationTokenSequence}`,
          expiresAt: now + 55 * 60_000,
        }
        installationTokens.set(cacheKey, token)
        record({
          operation: "github.createInstallationToken",
          appId: input.appId,
          installationId: input.installationId,
          repo: input.repo,
          githubCredentialKind: "installation_token",
          githubCredentialExpiresAt: token.expiresAt,
          sharedPrivateKeyPresent: false,
        })
        return token
      },
      bootstrapCodexDeviceLogin: async ({ tenant, codexHome }) => {
        record({ operation: "codex.deviceLogin", tenant, codexHome })
        const otherTenant = tenant === "acme" ? "globex" : "acme"
        record({
          operation: "codex.homeIsolation",
          tenant: otherTenant,
          codexHome: `/acceptance/codex-home/${otherTenant}`,
        })
        return {
          deviceUrl: "https://auth.openai.com/device",
          userCode: `CODE-${tenant.toUpperCase()}`,
        }
      },
      recordControlPlaneOperation: (input) => {
        const { command, ...observed } = input
        if (input.operation === "github.listInstallationRepositories") {
          recordRepositoryPage(observed)
        } else {
          record(observed)
        }
        const { operation, taskId, leaseToken } = input
        if (operation === "worker.renewLease") {
          renewals += 1
          resolveWaiters(waiters.renewals, renewals)
        }
        if (operation === "worker.submitReport" && command) {
          const report = {
            taskId: taskId ?? "",
            leaseToken: leaseToken ?? "",
            command,
          }
          lastReport = report as typeof lastReport
          const typed = command as Partial<DomainCommand>
          const reportKey = `${taskId ?? ""}:${typed.type ?? ""}:${"reportId" in typed ? typed.reportId : ""}`
          const index = reportIndexes.get(reportKey)
          if (index === undefined) {
            reportIndexes.set(reportKey, reportCounts.length)
            reportCounts.push(1)
          } else {
            reportCounts[index] = (reportCounts[index] ?? 0) + 1
          }
        }
      },
    },
  })

  function workerPorts(workerId = "acceptance-worker"): ProductionWorkerPorts {
    return {
      now: () => now,
      github: {
        createPullRequest: async (input) => {
          requiredGitHubToken(input.credentials)
          record({
            operation: "github.createPullRequest",
            tenant: input.tenant,
            repo: input.repo,
            idempotencyKey: input.idempotencyKey,
          })
          const number = nextPr++
          return {
            number,
            branch: input.headBranch,
            headSha: nextCreatedHeadSha,
          }
        },
        getPullRequest: async (input) => {
          requiredGitHubToken(input.credentials)
          const value =
            reconciliations.get(
              lookupKey(input.tenant, input.repo, input.pr)
            ) ?? sweepLookups.get(lookupKey(input.tenant, input.repo, input.pr))
          const pullRequest = value?.pullRequest ?? {}
          record({
            operation: "github.getPullRequest",
            ...input,
            headSha: pullRequestHead(pullRequest),
          })
          return pullRequest
        },
        getRequiredChecks: async (input) => {
          requiredGitHubToken(input.credentials)
          record({ operation: "github.getRequiredChecks", ...input })
          const value = sweepLookups.get(
            lookupKey(input.tenant, input.repo, input.pr)
          )
          return {
            items: value?.required ?? [],
            complete: value?.requiredChecksComplete ?? true,
          }
        },
        listCheckRuns: async (input) => {
          requiredGitHubToken(input.credentials)
          record({ operation: "github.listCheckRuns", ...input })
          const key = lookupKey(input.tenant, input.repo, input.pr)
          const value = sweepLookups.get(key) ?? reconciliations.get(key)
          reconciliations.delete(key)
          return {
            items: value?.runs ?? [],
            complete:
              "checkRunsComplete" in (value ?? {})
                ? (value as { checkRunsComplete: boolean }).checkRunsComplete
                : true,
          }
        },
        mergePullRequest: async (input) => {
          requiredGitHubToken(input.credentials)
          record({ operation: "github.mergePullRequest", ...input })
          if (failMerge) {
            failMerge = false
            throw new Error("merge failed")
          }
          return {}
        },
        closePullRequest: async (input) => {
          requiredGitHubToken(input.credentials)
          record({ operation: "github.closePullRequest", ...input })
          if (failAfterExternalEffect) {
            failAfterExternalEffect = false
            throw new Error("worker crashed after external effect")
          }
          return {}
        },
      },
      workspace: {
        prepare: async (tenant, repo, credentials) => {
          requiredGitHubToken(credentials)
          record({ operation: "workspace.prepare", tenant, repo })
          return {
            workingDirectory: `/work/${tenant}/${repo.replaceAll("/", "-")}`,
            baseRef: "main",
            branch: `gc/${workerId}`,
          }
        },
        push: async (checkout: WorkspaceCheckout, credentials) => {
          requiredGitHubToken(credentials)
          record({ operation: "workspace.push" })
          return { branch: checkout.branch, headSha: nextCreatedHeadSha }
        },
      },
      codex: {
        run: async (input): Promise<AgentRunResult> => {
          const { tenant, repo } = input
          const key = repositoryKey(tenant, repo)
          while ((activeAgents.get(key) ?? 0) > 0) {
            contentions += 1
            resolveWaiters(waiters.contentions, contentions)
            await agentGate
          }
          activeAgents.set(key, (activeAgents.get(key) ?? 0) + 1)
          maximumAgents.set(
            key,
            Math.max(maximumAgents.get(key) ?? 0, activeAgents.get(key) ?? 0)
          )
          agentStarts += 1
          resolveWaiters(waiters.agentStarts, agentStarts)
          record({
            operation: "codex.run",
            tenant,
            repo,
            model: input.settings.model,
            reasoning: input.settings.reasoning,
            prompt: input.settings.prompt,
            codexHome: `/acceptance/codex-home/${tenant}`,
          })
          try {
            if (agentsHeld) await agentGate
            if (failAgentBeforePr) {
              failAgentBeforePr = false
              return {
                outcome: "failed",
                usage: { inputTokens: 0, outputTokens: 0, reasoningTokens: 0 },
              }
            }
            return {
              outcome: "finished",
              usage: { inputTokens: 1, outputTokens: 2, reasoningTokens: 3 },
            }
          } finally {
            activeAgents.set(key, Math.max(0, (activeAgents.get(key) ?? 1) - 1))
          }
        },
        startDeviceLogin: async (input) => {
          record({
            operation: "codex.startDeviceLogin",
            tenant: input.tenant,
            codexHome: input.codexHome,
          })
          if (!authorizedDeviceCodes.has(input.tenant)) {
            await input.onUpdate({ status: "failed" })
            return "failed" as const
          }
          await input.onUpdate({ status: "connected" })
          return "connected" as const
        },
      },
    }
  }

  return {
    github: {
      setOAuthCode: (code, githubUserId) => {
        oauthCodes.set(code, githubUserId)
      },
      setInstallation: (input) => {
        installations.set(input.installationId, {
          ...input,
          accessibleToGitHubUserIds: [...input.accessibleToGitHubUserIds],
          repositoryPages: input.repositoryPages.map((page) => [...page]),
          clientId: wiring.sharedGitHubApp.clientId,
          permissions: REQUIRED_GITHUB_PERMISSIONS,
          suspended: false,
        })
      },
      failNextRepositoryDiscoveryAfterPage: (page) => {
        failRepositoryDiscoveryAfterPage = page
      },
      authorizeDeviceCode: (tenant) => authorizedDeviceCodes.add(tenant),
      setSweepLookup: (
        tenant,
        repo,
        pr,
        pullRequest,
        required,
        runs,
        completeness = {
          requiredChecksComplete: true,
          checkRunsComplete: true,
        }
      ) => {
        reconciliations.delete(lookupKey(tenant, repo, pr))
        sweepLookups.set(lookupKey(tenant, repo, pr), {
          pullRequest,
          required,
          runs,
          ...completeness,
        })
      },
      setReconciliation: (tenant, repo, pr, pullRequest, runs) => {
        reconciliations.set(lookupKey(tenant, repo, pr), { pullRequest, runs })
      },
      failNextMerge: () => {
        failMerge = true
      },
    },
    worker: {
      setNextCreatedHeadSha: (headSha) => {
        nextCreatedHeadSha = headSha
      },
      failNextAgentBeforePr: () => {
        failAgentBeforePr = true
      },
      failNextAfterLease: () => {
        failAfterLease = true
      },
      failNextAfterExternalEffect: () => {
        failAfterExternalEffect = true
      },
      holdAgents: () => {
        agentsHeld = true
        agentGate = new Promise((resolve) => {
          releaseAgents = resolve
        })
      },
      releaseAgents: () => {
        agentsHeld = false
        releaseAgents?.()
        releaseAgents = null
        agentGate = Promise.resolve()
      },
      waitForAgentStarts: (count) =>
        waitFor(waiters.agentStarts, agentStarts, count),
      waitForRepositoryContentions: (count) =>
        waitFor(waiters.contentions, contentions, count),
      waitForLeaseRenewals: (count) =>
        waitFor(waiters.renewals, renewals, count),
      expireGitHubInstallationToken: () => {
        for (const token of installationTokens.values()) {
          token.expiresAt = now - 1
        }
      },
      ports: (workerId) => {
        const ports = workerPorts(workerId)
        if (failAfterLease) {
          failAfterLease = false
          return {
            ...ports,
            github: new Proxy(ports.github, {
              get() {
                throw new Error("worker crashed after lease")
              },
            }),
          }
        }
        return ports
      },
      retryLastReport: async () => {
        if (!lastReport) throw new Error("no worker report to retry")
        const response = await wiring.backend.fetch("/worker/protocol", {
          method: "POST",
          headers: {
            authorization: "Bearer acceptance-worker-token",
            "content-type": "application/json",
          },
          body: JSON.stringify({ operation: "report", ...lastReport }),
        })
        if (response.status === 409) {
          const typed = lastReport.command as Partial<DomainCommand>
          const reportKey = `${lastReport.taskId}:${typed.type ?? ""}:${"reportId" in typed ? typed.reportId : ""}`
          const index = reportIndexes.get(reportKey)
          if (index === undefined) {
            reportIndexes.set(reportKey, reportCounts.length)
            reportCounts.push(1)
          } else {
            reportCounts[index] = (reportCounts[index] ?? 0) + 1
          }
          return
        }
        if (!response.ok)
          throw new Error(`report retry failed: ${response.status}`)
      },
    },
    browser: {
      get: browserGet,
    },
    observed: {
      calls: () => [...calls],
      maxConcurrentAgents: (tenant, repo) =>
        maximumAgents.get(repositoryKey(tenant, repo)) ?? 0,
      reportSubmissionCounts: () => [...reportCounts],
    },
    clock: {
      set: (value: Timestamp) => {
        now = value
      },
    },
    dispose: async () => {
      releaseAgents?.()
      configureConvexRuntime(null)
      authorizedDeviceCodes.clear()
      oauthCodes.clear()
      oauthTokens.clear()
      installations.clear()
      installationTokens.clear()
    },
  }
}

function cookieHeader(cookies: CookieJar): string {
  return [...cookies].map(([name, value]) => `${name}=${value}`).join("; ")
}

function storeCookies(cookies: CookieJar, headers: Headers): void {
  const values = (
    headers as Headers & { getSetCookie?: () => readonly string[] }
  ).getSetCookie?.() ?? [headers.get("set-cookie") ?? ""]
  for (const value of values) {
    const pair = value.split(";", 1)[0]
    const separator = pair?.indexOf("=") ?? -1
    if (!pair || separator <= 0) continue
    const name = pair.slice(0, separator)
    const cookieValue = pair.slice(separator + 1)
    if (cookieValue) cookies.set(name, cookieValue)
    else cookies.delete(name)
  }
}
