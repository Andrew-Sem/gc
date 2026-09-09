import {
  type DefaultFunctionArgs,
  type FunctionReference,
  type FunctionType,
  httpRouter,
  makeFunctionReference,
} from "convex/server"
import {
  createGitHubInstallationToken,
  publicWebUrl,
  runtimeConfiguration,
  sharedGitHubApp,
} from "../src/runtime-ports"
import { httpAction } from "./_generated/server"
import { authComponent, createAuth } from "./auth"
import {
  decryptGitHubFlowValue,
  encryptGitHubFlowValue,
  githubPkceChallenge,
  githubPorts,
  hashGitHubFlowValue,
  MAX_GITHUB_PAGES,
  randomGitHubFlowValue,
  requiredGitHubPermissions,
} from "./github_app_setup"

const http = httpRouter()

authComponent.registerRoutesLazy(http, createAuth, {
  basePath: "/api/auth",
  trustedOrigins: () => [process.env.BETTER_AUTH_ALLOWED_ORIGIN],
  cors: true,
})

function internalReference<
  Type extends FunctionType,
  Args extends DefaultFunctionArgs,
  ReturnValue,
>(name: string): FunctionReference<Type, "internal", Args, ReturnValue> {
  return makeFunctionReference<Type, Args, ReturnValue>(
    name
  ) as unknown as FunctionReference<Type, "internal", Args, ReturnValue>
}

const installationEnvelope = internalReference<
  "query",
  { installationId: number },
  { tenant: string } | null
>("webhook:installationEnvelope")
const acceptDelivery = internalReference<
  "mutation",
  {
    deliveryId: string
    event: string
    payload: unknown
    tenant: string
    at: number
  },
  { duplicate: boolean; commands: number }
>("webhook:acceptDelivery")
const githubSetupReferences = {
  createInstallState: internalReference<
    "mutation",
    { subject: string; sessionId: string; stateHash: string; now: number },
    { tenant: string }
  >("github_app_setup:createInstallState"),
  consumeInstallState: internalReference<
    "mutation",
    {
      subject: string
      sessionId: string
      stateHash: string
      oauthStateHash: string
      encryptedCodeVerifier: string
      installationId: number
      now: number
    },
    { tenant: string }
  >("github_app_setup:consumeInstallState"),
  readOAuthFlow: internalReference<
    "query",
    { subject: string; sessionId: string; stateHash: string; now: number },
    {
      flowId: string
      tenant: string
      installationId: number
      status: string
      encryptedCodeVerifier: string
      codeHash?: string
      encryptedUserToken?: string
      githubUserId?: number
    } | null
  >("github_app_setup:readOAuthFlow"),
  recordOAuthExchange: internalReference<
    "mutation",
    {
      flowId: string
      subject: string
      sessionId: string
      codeHash: string
      encryptedUserToken: string
      githubUserId: number
      now: number
    },
    { encryptedUserToken: string; githubUserId: number }
  >("github_app_setup:recordOAuthExchange"),
  finalizeOAuthFlow: internalReference<
    "mutation",
    {
      flowId: string
      subject: string
      sessionId: string
      codeHash: string
      repositories: string[]
      now: number
    },
    { completed: boolean }
  >("github_app_setup:finalizeOAuthFlow"),
  completeOAuthFlow: internalReference<
    "mutation",
    {
      flowId: string
      subject: string
      sessionId: string
      codeHash: string
      now: number
    },
    null
  >("github_app_setup:completeOAuthFlow"),
} as const
const workerReferences = {
  claim: internalReference<
    "mutation",
    { workerId: string; leaseMs: number; readiness?: boolean },
    unknown
  >("worker:claimTask"),
  renew: internalReference<
    "mutation",
    { taskId: string; leaseToken: string; leaseMs: number },
    unknown
  >("worker:renewTask"),
  credentials: internalReference<
    "query",
    { taskId: string; leaseToken: string },
    unknown
  >("worker:credentials"),
  complete: internalReference<
    "mutation",
    { taskId: string; leaseToken: string },
    unknown
  >("worker:completeTask"),
  release: internalReference<
    "mutation",
    { taskId: string; leaseToken: string },
    unknown
  >("worker:releaseTask"),
  report: internalReference<
    "mutation",
    { taskId: string; leaseToken: string; command: unknown },
    unknown
  >("worker:submitReport"),
  deviceLogin: internalReference<
    "mutation",
    {
      taskId: string
      leaseToken: string
      update: unknown
    },
    unknown
  >("worker:updateDeviceLogin"),
} as const

type WorkerProtocolInput = {
  readonly operation?: string
  readonly workerId?: string
  readonly leaseMs?: number
  readonly taskId?: string
  readonly leaseToken?: string
  readonly command?: unknown
  readonly update?: unknown
  readonly readiness?: boolean
}
type GitHubOAuthFlow = {
  flowId: string
  tenant: string
  installationId: number
  status: string
  encryptedCodeVerifier: string
  codeHash?: string
  encryptedUserToken?: string
  githubUserId?: number
} | null

type ClaimInput = {
  readonly workerId: string
  readonly leaseMs: number
  readonly readiness?: boolean
}
type LeaseInput = {
  readonly taskId: string
  readonly leaseToken: string
  readonly leaseMs: number
}
type TaskInput = { readonly taskId: string; readonly leaseToken: string }
type ReportInput = TaskInput & { readonly command: unknown }

function json(status: number, value: unknown): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "content-type": "application/json" },
  })
}

function redirect(location: string): Response {
  return new Response(null, { status: 303, headers: { location } })
}

async function authenticatedSession(
  ctx: Parameters<typeof createAuth>[0] | unknown,
  request: Request
): Promise<{ subject: string; sessionId: string } | null> {
  const session = await createAuth(
    ctx as Parameters<typeof createAuth>[0]
  ).api.getSession({
    headers: request.headers,
  })
  if (!session?.user.id) return null
  const value = session as typeof session & { session?: { id?: string } }
  if (!value.session?.id) return null
  return {
    subject: session.user.id,
    sessionId: value.session.id,
  }
}

function callbackParameter(url: URL, name: string): string | null {
  const value = url.searchParams.get(name)?.trim()
  return value ? value : null
}

function positiveInteger(value: string | null): number | null {
  if (!value || !/^\d+$/.test(value)) return null
  const number = Number(value)
  return Number.isSafeInteger(number) && number > 0 ? number : null
}

function installationIdOf(payload: unknown): number | null {
  if (
    typeof payload !== "object" ||
    payload === null ||
    !("installation" in payload) ||
    typeof payload.installation !== "object" ||
    payload.installation === null ||
    !("id" in payload.installation) ||
    typeof payload.installation.id !== "number"
  ) {
    return null
  }
  return payload.installation.id
}

async function signatureFor(secret: string, body: string): Promise<string> {
  const key = await crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"]
  )
  const signature = new Uint8Array(
    await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(body))
  )
  return `sha256=${[...signature]
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("")}`
}

function safeEqual(left: string, right: string): boolean {
  if (left.length !== right.length) return false
  let difference = 0
  for (let index = 0; index < left.length; index += 1) {
    difference |= left.charCodeAt(index) ^ right.charCodeAt(index)
  }
  return difference === 0
}

http.route({
  path: "/github/webhook",
  method: "POST",
  handler: httpAction(async (ctx, request) => {
    const body = await request.text()
    const deliveryId = request.headers.get("x-github-delivery") ?? ""
    runtimeConfiguration().ports.recordControlPlaneOperation?.({
      operation: "webhook.verifySignature",
      deliveryId,
    })
    const supplied = request.headers.get("x-hub-signature-256") ?? ""
    const expected = await signatureFor(sharedGitHubApp().webhookSecret, body)
    if (!safeEqual(supplied, expected)) {
      return json(401, { error: "unauthorized" })
    }
    let payload: unknown
    try {
      payload = JSON.parse(body)
    } catch {
      return json(400, { error: "invalid json" })
    }
    runtimeConfiguration().ports.recordControlPlaneOperation?.({
      operation: "webhook.resolveTenant",
      deliveryId,
    })
    const installationId = installationIdOf(payload)
    if (installationId === null) return json(401, { error: "unauthorized" })
    const envelope = await ctx.runQuery(installationEnvelope, {
      installationId,
    })
    if (!envelope) return json(401, { error: "unauthorized" })
    const event = request.headers.get("x-github-event")
    if (!deliveryId || !event) return json(400, { error: "missing headers" })
    const result = await ctx.runMutation(acceptDelivery, {
      deliveryId,
      event,
      payload,
      tenant: envelope.tenant,
      at: runtimeConfiguration().ports.now(),
    })
    return json(202, result)
  }),
})
http.route({
  path: "/github/app/connect",
  method: "GET",
  handler: httpAction(async (ctx, request) => {
    const session = await authenticatedSession(ctx, request)
    if (!session) return json(401, { error: "authentication required" })
    const now = runtimeConfiguration().ports.now()
    const state = randomGitHubFlowValue()
    try {
      await ctx.runMutation(githubSetupReferences.createInstallState, {
        ...session,
        stateHash: await hashGitHubFlowValue(state),
        now,
      })
    } catch {
      return json(403, { error: "tenant ownership required" })
    }
    const app = sharedGitHubApp()
    const installationUrl = new URL(
      `/apps/${encodeURIComponent(app.slug)}/installations/new`,
      "https://github.com"
    )
    installationUrl.searchParams.set("state", state)
    return redirect(installationUrl.toString())
  }),
})
http.route({
  path: "/github/app/setup/callback",
  method: "GET",
  handler: httpAction(async (ctx, request) => {
    const session = await authenticatedSession(ctx, request)
    if (!session) return json(401, { error: "authentication required" })
    const url = new URL(request.url)
    const state = callbackParameter(url, "state")
    const installationId = positiveInteger(
      callbackParameter(url, "installation_id")
    )
    if (!state || installationId === null) {
      return json(400, { error: "invalid callback" })
    }
    const oauthState = randomGitHubFlowValue()
    const verifier = randomGitHubFlowValue()
    const now = runtimeConfiguration().ports.now()
    try {
      await ctx.runMutation(githubSetupReferences.consumeInstallState, {
        ...session,
        stateHash: await hashGitHubFlowValue(state),
        oauthStateHash: await hashGitHubFlowValue(oauthState),
        encryptedCodeVerifier: await encryptGitHubFlowValue(verifier),
        installationId,
        now,
      })
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "invalid callback"
      return json(message.includes("ownership") ? 403 : 400, { error: message })
    }
    const authorize = new URL("https://github.com/login/oauth/authorize")
    authorize.searchParams.set("client_id", sharedGitHubApp().clientId)
    authorize.searchParams.set(
      "redirect_uri",
      new URL("/github/app/oauth/callback", publicWebUrl()).toString()
    )
    authorize.searchParams.set("state", oauthState)
    authorize.searchParams.set(
      "code_challenge",
      await githubPkceChallenge(verifier)
    )
    authorize.searchParams.set("code_challenge_method", "S256")
    return redirect(authorize.toString())
  }),
})
http.route({
  path: "/github/app/oauth/callback",
  method: "GET",
  handler: httpAction(async (ctx, request) => {
    const session = await authenticatedSession(ctx, request)
    if (!session) return json(401, { error: "authentication required" })
    const url = new URL(request.url)
    const state = callbackParameter(url, "state")
    const code = callbackParameter(url, "code")
    if (!state || !code) return json(400, { error: "invalid callback" })
    const now = runtimeConfiguration().ports.now()
    const stateHash = await hashGitHubFlowValue(state)
    const codeHash = await hashGitHubFlowValue(code)
    let flow: GitHubOAuthFlow
    try {
      flow = await ctx.runQuery(githubSetupReferences.readOAuthFlow, {
        ...session,
        stateHash,
        now,
      })
    } catch {
      return json(403, { error: "tenant ownership required" })
    }
    if (!flow) return json(400, { error: "invalid or expired flow" })
    if (flow.status === "completed") {
      return flow.codeHash === codeHash
        ? redirect("/?github=connected")
        : json(400, { error: "OAuth code does not match completed flow" })
    }
    if (flow.codeHash && flow.codeHash !== codeHash) {
      return json(400, { error: "OAuth code does not match flow" })
    }

    const app = sharedGitHubApp()
    let encryptedUserToken = flow.encryptedUserToken
    let githubUserId = flow.githubUserId
    try {
      if (!encryptedUserToken || !githubUserId) {
        const codeVerifier = await decryptGitHubFlowValue(
          flow.encryptedCodeVerifier
        )
        runtimeConfiguration().ports.recordControlPlaneOperation?.({
          operation: "github.exchangeOAuthCode",
          oauthCode: code,
          codeVerifier,
        })
        const exchanged = await githubPorts.exchangeOAuthCode({
          code,
          codeVerifier,
          clientId: app.clientId,
          clientSecret: app.clientSecret,
        })
        const stored = await ctx.runMutation(
          githubSetupReferences.recordOAuthExchange,
          {
            flowId: flow.flowId,
            ...session,
            codeHash,
            encryptedUserToken: await encryptGitHubFlowValue(
              exchanged.accessToken
            ),
            githubUserId: exchanged.githubUserId,
            now,
          }
        )
        encryptedUserToken = stored.encryptedUserToken
        githubUserId = stored.githubUserId
      }
      const userToken = await decryptGitHubFlowValue(encryptedUserToken)
      if (flow.status !== "connected") {
        let accessible = false
        for (let page = 1; page <= MAX_GITHUB_PAGES; page += 1) {
          const result = await githubPorts.listUserInstallations({
            accessToken: userToken,
            page,
          })
          if (result.installationIds.includes(flow.installationId))
            accessible = true
          if (!result.hasNext) break
          if (page === MAX_GITHUB_PAGES)
            throw new Error("GitHub pagination limit exceeded")
        }
        if (!accessible) {
          return json(403, {
            error: "installation is not accessible to GitHub user",
          })
        }
        const inspection = await githubPorts.inspectInstallation({
          appId: app.appId,
          clientId: app.clientId,
          installationId: flow.installationId,
          privateKey: app.privateKey,
        })
        if (
          inspection.appId !== app.appId ||
          inspection.clientId !== app.clientId ||
          inspection.suspended ||
          !requiredGitHubPermissions(inspection.permissions)
        ) {
          return json(403, { error: "installation is not eligible" })
        }
        const repositories: string[] = []
        for (let page = 1; page <= MAX_GITHUB_PAGES; page += 1) {
          runtimeConfiguration().ports.recordControlPlaneOperation?.({
            operation: "github.listInstallationRepositories",
            installationId: flow.installationId,
            repositoryPage: page,
          })
          const result = await githubPorts.listRepositories({
            appId: app.appId,
            installationId: flow.installationId,
            privateKey: app.privateKey,
            page,
          })
          repositories.push(...result.repositories)
          if (!result.hasNext) break
          if (page === MAX_GITHUB_PAGES)
            throw new Error("GitHub pagination limit exceeded")
        }
        try {
          await ctx.runMutation(githubSetupReferences.finalizeOAuthFlow, {
            flowId: flow.flowId,
            ...session,
            codeHash,
            repositories,
            now,
          })
        } catch (error) {
          const status =
            typeof error === "object" && error !== null && "status" in error
              ? Number(error.status)
              : 400
          return json(status === 409 ? 409 : 400, {
            error: error instanceof Error ? error.message : "connection failed",
          })
        }
      }
      runtimeConfiguration().ports.recordControlPlaneOperation?.({
        operation: "github.revokeUserAuthorization",
        githubUserId,
      })
      await githubPorts.revokeAuthorization({
        clientId: app.clientId,
        clientSecret: app.clientSecret,
        accessToken: userToken,
        githubUserId,
      })
      await ctx.runMutation(githubSetupReferences.completeOAuthFlow, {
        flowId: flow.flowId,
        ...session,
        codeHash,
        now,
      })
      return redirect("/?github=connected")
    } catch (error) {
      const status =
        typeof error === "object" && error !== null && "status" in error
          ? Number(error.status)
          : 502
      return json(status === 401 || status === 403 ? 403 : 502, {
        error: error instanceof Error ? error.message : "GitHub setup failed",
      })
    }
  }),
})
http.route({
  path: "/worker/protocol",
  method: "POST",
  handler: httpAction(async (ctx, request) => {
    const authorization = request.headers.get("authorization")
    if (authorization !== `Bearer ${runtimeConfiguration().workerToken}`) {
      return json(401, { error: "unauthorized" })
    }
    let input: WorkerProtocolInput
    try {
      input = (await request.json()) as WorkerProtocolInput
    } catch {
      return json(400, { error: "invalid json" })
    }
    try {
      switch (input.operation) {
        case "claim": {
          const claim = input as ClaimInput
          return json(
            200,
            await ctx.runMutation(workerReferences.claim, {
              workerId: claim.workerId,
              leaseMs: claim.leaseMs,
              readiness: claim.readiness,
            })
          )
        }
        case "renew": {
          const lease = input as LeaseInput
          return json(
            200,
            await ctx.runMutation(workerReferences.renew, {
              taskId: lease.taskId,
              leaseToken: lease.leaseToken,
              leaseMs: lease.leaseMs,
            })
          )
        }
        case "credentials": {
          const task = input as TaskInput
          const credentials = (await ctx.runQuery(
            workerReferences.credentials,
            {
              taskId: task.taskId,
              leaseToken: task.leaseToken,
            }
          )) as {
            github: {
              appId?: number
              installationId?: number
              token?: string
            }
            githubRequired?: boolean
            githubRepo?: string
            tenant?: string
            expiresAt: number
            [key: string]: unknown
          }
          if (credentials.githubRequired && credentials.github.installationId) {
            const app = sharedGitHubApp()
            if (
              credentials.github.appId !== app.appId ||
              !credentials.github.installationId
            ) {
              throw new Error(
                "GitHub installation or repository is not eligible"
              )
            }
            const createToken =
              runtimeConfiguration().ports.createGitHubInstallationToken ??
              createGitHubInstallationToken
            const token = await createToken({
              appId: app.appId,
              installationId: credentials.github.installationId,
              privateKey: app.privateKey,
              ...(credentials.githubRepo
                ? { repo: credentials.githubRepo }
                : {}),
            })
            runtimeConfiguration().ports.recordControlPlaneOperation?.({
              operation: "worker.githubCredentials",
              taskId: task.taskId,
              leaseToken: task.leaseToken,
              tenant: credentials.tenant,
              repo: credentials.githubRepo,
              appId: app.appId,
              installationId: credentials.github.installationId,
              githubCredentialKind: "installation_token",
              githubCredentialExpiresAt: token.expiresAt,
              sharedPrivateKeyPresent: false,
            })
            credentials.github = {
              appId: app.appId,
              installationId: credentials.github.installationId,
              token: token.token,
            }
            credentials.expiresAt = token.expiresAt
          } else if (
            credentials.githubRequired &&
            (runtimeConfiguration().sharedGitHubApp ||
              process.env.GITHUB_APP_ID)
          ) {
            throw new Error("GitHub installation or repository is not eligible")
          }
          delete credentials.githubRequired
          delete credentials.githubRepo
          delete credentials.tenant
          return json(200, credentials)
        }
        case "complete": {
          const task = input as TaskInput
          return json(
            200,
            await ctx.runMutation(workerReferences.complete, {
              taskId: task.taskId,
              leaseToken: task.leaseToken,
            })
          )
        }
        case "release": {
          const task = input as TaskInput
          return json(
            200,
            await ctx.runMutation(workerReferences.release, {
              taskId: task.taskId,
              leaseToken: task.leaseToken,
            })
          )
        }
        case "report": {
          const report = input as ReportInput
          return json(
            200,
            await ctx.runMutation(workerReferences.report, {
              taskId: report.taskId,
              leaseToken: report.leaseToken,
              command: report.command,
            })
          )
        }
        case "deviceLogin": {
          const task = input as TaskInput & { readonly update: unknown }
          return json(
            200,
            await ctx.runMutation(workerReferences.deviceLogin, {
              taskId: task.taskId,
              leaseToken: task.leaseToken,
              update: task.update,
            })
          )
        }
        default:
          return json(400, { error: "unknown operation" })
      }
    } catch (error) {
      return json(409, {
        error: error instanceof Error ? error.message : "protocol failure",
      })
    }
  }),
})
http.route({
  path: "/health/live",
  method: "GET",
  handler: httpAction(async () => json(200, { status: "live" })),
})
http.route({
  path: "/health/ready",
  method: "GET",
  handler: httpAction(async (ctx) => {
    runtimeConfiguration()
    await ctx.runQuery(
      internalReference<"query", Record<string, never>, unknown>(
        "status:readiness"
      ),
      {}
    )
    return json(200, { status: "ready" })
  }),
})

export default http
