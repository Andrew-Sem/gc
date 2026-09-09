import { v } from "convex/values"
import {
  createGitHubInstallationToken,
  exchangeGitHubOAuthCode,
  inspectGitHubAppInstallation,
  listGitHubInstallationRepositories,
  listGitHubUserInstallations,
  revokeGitHubUserAuthorization,
  runtimeConfiguration,
  sharedGitHubApp,
} from "../src/runtime-ports"
import { decryptSecretValue, encryptSecretValue } from "../src/secret-crypto"
import { internalMutation, internalQuery } from "./_generated/server"
import { dispatchCommand } from "./control_plane"

export const FLOW_TTL_MS = 15 * 60 * 1000
export const MAX_GITHUB_PAGES = 100

export function randomGitHubFlowValue(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(32))
  let binary = ""
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary)
    .replaceAll("+", "-")
    .replaceAll("/", "_")
    .replace(/=+$/, "")
}

export async function hashGitHubFlowValue(value: string): Promise<string> {
  const digest = new Uint8Array(
    await crypto.subtle.digest("SHA-256", new TextEncoder().encode(value))
  )
  return [...digest].map((byte) => byte.toString(16).padStart(2, "0")).join("")
}

export async function encryptGitHubFlowValue(value: string): Promise<string> {
  return encryptSecretValue(runtimeConfiguration().masterKey, value)
}

export async function decryptGitHubFlowValue(value: string): Promise<string> {
  return decryptSecretValue(runtimeConfiguration().masterKey, value)
}

export async function githubPkceChallenge(verifier: string): Promise<string> {
  const digest = new Uint8Array(
    await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier))
  )
  let binary = ""
  for (const byte of digest) binary += String.fromCharCode(byte)
  return btoa(binary)
    .replaceAll("+", "-")
    .replaceAll("/", "_")
    .replace(/=+$/, "")
}

export const createInstallState = internalMutation({
  args: {
    subject: v.string(),
    sessionId: v.string(),
    stateHash: v.string(),
    now: v.number(),
  },
  handler: async (ctx, input) => {
    const owner = await ctx.db
      .query("owners")
      .withIndex("by_subject", (query) => query.eq("subject", input.subject))
      .unique()
    if (!owner) throw new Error("tenant ownership required")
    await ctx.db.insert("githubAppInstallStates", {
      subject: input.subject,
      sessionId: input.sessionId,
      stateHash: input.stateHash,
      tenant: owner.tenant,
      createdAt: input.now,
      expiresAt: input.now + FLOW_TTL_MS,
    })
    return { tenant: owner.tenant }
  },
})

export const consumeInstallState = internalMutation({
  args: {
    subject: v.string(),
    sessionId: v.string(),
    stateHash: v.string(),
    oauthStateHash: v.string(),
    encryptedCodeVerifier: v.string(),
    installationId: v.number(),
    now: v.number(),
  },
  handler: async (ctx, input) => {
    const installState = await ctx.db
      .query("githubAppInstallStates")
      .withIndex("by_state_hash", (query) =>
        query.eq("stateHash", input.stateHash)
      )
      .unique()
    if (
      !installState ||
      installState.consumedAt !== undefined ||
      installState.expiresAt < input.now
    ) {
      throw new Error("invalid or expired install state")
    }
    if (
      installState.subject !== input.subject ||
      installState.sessionId !== input.sessionId
    ) {
      throw new Error("tenant ownership required")
    }
    await ctx.db.patch(installState._id, { consumedAt: input.now })
    await ctx.db.insert("githubAppOAuthFlows", {
      stateHash: input.oauthStateHash,
      subject: input.subject,
      sessionId: input.sessionId,
      tenant: installState.tenant,
      installationId: input.installationId,
      status: "awaiting_code",
      encryptedCodeVerifier: input.encryptedCodeVerifier,
      reportId: `github-app:${crypto.randomUUID()}`,
      createdAt: input.now,
      expiresAt: input.now + FLOW_TTL_MS,
    })
    return { tenant: installState.tenant }
  },
})

export const readOAuthFlow = internalQuery({
  args: {
    subject: v.string(),
    sessionId: v.string(),
    stateHash: v.string(),
    now: v.number(),
  },
  handler: async (ctx, input) => {
    const flow = await ctx.db
      .query("githubAppOAuthFlows")
      .withIndex("by_state_hash", (query) =>
        query.eq("stateHash", input.stateHash)
      )
      .unique()
    if (!flow || (flow.status !== "completed" && flow.expiresAt < input.now)) {
      return null
    }
    if (flow.subject !== input.subject || flow.sessionId !== input.sessionId) {
      throw new Error("tenant ownership required")
    }
    return {
      flowId: flow._id,
      tenant: flow.tenant,
      installationId: flow.installationId,
      status: flow.status,
      encryptedCodeVerifier: flow.encryptedCodeVerifier,
      codeHash: flow.codeHash,
      encryptedUserToken: flow.encryptedUserToken,
      githubUserId: flow.githubUserId,
    }
  },
})

export const recordOAuthExchange = internalMutation({
  args: {
    flowId: v.id("githubAppOAuthFlows"),
    subject: v.string(),
    sessionId: v.string(),
    codeHash: v.string(),
    encryptedUserToken: v.string(),
    githubUserId: v.number(),
    now: v.number(),
  },
  handler: async (ctx, input) => {
    const flow = await ctx.db.get(input.flowId)
    if (
      !flow ||
      flow.subject !== input.subject ||
      flow.sessionId !== input.sessionId ||
      flow.expiresAt < input.now
    ) {
      throw new Error("invalid or expired OAuth flow")
    }
    if (flow.status !== "awaiting_code") {
      if (
        flow.codeHash === input.codeHash &&
        flow.encryptedUserToken &&
        flow.githubUserId
      ) {
        return {
          encryptedUserToken: flow.encryptedUserToken,
          githubUserId: flow.githubUserId,
        }
      }
      throw new Error("OAuth code has already been used")
    }
    await ctx.db.patch(flow._id, {
      status: "exchanged",
      codeHash: input.codeHash,
      encryptedUserToken: input.encryptedUserToken,
      githubUserId: input.githubUserId,
    })
    return {
      encryptedUserToken: input.encryptedUserToken,
      githubUserId: input.githubUserId,
    }
  },
})

export const finalizeOAuthFlow = internalMutation({
  args: {
    flowId: v.id("githubAppOAuthFlows"),
    subject: v.string(),
    sessionId: v.string(),
    codeHash: v.string(),
    repositories: v.array(v.string()),
    now: v.number(),
  },
  handler: async (ctx, input) => {
    const flow = await ctx.db.get(input.flowId)
    if (
      !flow ||
      flow.subject !== input.subject ||
      flow.sessionId !== input.sessionId ||
      flow.codeHash !== input.codeHash ||
      !flow.encryptedUserToken ||
      !flow.githubUserId ||
      flow.expiresAt < input.now
    ) {
      throw new Error("invalid or expired OAuth flow")
    }
    if (flow.status === "completed" || flow.status === "connected") {
      return { completed: flow.status === "completed" }
    }
    if (flow.status !== "exchanged") {
      throw new Error("OAuth flow is not ready for finalization")
    }

    const existing = await ctx.db
      .query("installations")
      .withIndex("by_installation_id", (query) =>
        query.eq("installationId", flow.installationId)
      )
      .unique()
    if (existing && existing.tenant !== flow.tenant) {
      throw Object.assign(
        new Error("GitHub installation ID is already connected"),
        { status: 409 }
      )
    }
    const app = sharedGitHubApp()
    const repositories = [...new Set(input.repositories)].sort()
    if (existing) {
      await ctx.db.patch(existing._id, {
        appId: app.appId,
        availableRepositories: repositories,
        suspended: false,
        deleted: false,
        updatedAt: input.now,
      })
    } else {
      await ctx.db.insert("installations", {
        tenant: flow.tenant,
        appId: app.appId,
        installationId: flow.installationId,
        availableRepositories: repositories,
        suspended: false,
        deleted: false,
        updatedAt: input.now,
      })
    }
    await dispatchCommand(ctx.db, {
      type: "ReportGitHubAppConnected",
      reportId: flow.reportId,
      tenant: flow.tenant,
      appId: app.appId,
      installationId: flow.installationId,
      at: input.now,
    })
    for (const repo of repositories) {
      await dispatchCommand(ctx.db, {
        type: "ReportRepositoryConnected",
        reportId: `${flow.reportId}:repository:${repo}`,
        tenant: flow.tenant,
        repo,
        at: input.now,
      })
    }
    await ctx.db.patch(flow._id, {
      status: "connected",
      connectedAt: input.now,
    })
    return { completed: false }
  },
})

export const completeOAuthFlow = internalMutation({
  args: {
    flowId: v.id("githubAppOAuthFlows"),
    subject: v.string(),
    sessionId: v.string(),
    codeHash: v.string(),
    now: v.number(),
  },
  handler: async (ctx, input) => {
    const flow = await ctx.db.get(input.flowId)
    if (
      !flow ||
      flow.subject !== input.subject ||
      flow.sessionId !== input.sessionId ||
      flow.codeHash !== input.codeHash
    ) {
      throw new Error("invalid OAuth completion")
    }
    if (flow.status === "completed") return
    if (flow.status !== "connected")
      throw new Error("OAuth flow is not connected")
    await ctx.db.patch(flow._id, {
      status: "completed",
      completedAt: input.now,
      encryptedUserToken: undefined,
      encryptedCodeVerifier: "revoked",
    })
  },
})

export const githubPorts = {
  exchangeOAuthCode: (input: Parameters<typeof exchangeGitHubOAuthCode>[0]) =>
    runtimeConfiguration().ports.exchangeGitHubOAuthCode?.(input) ??
    exchangeGitHubOAuthCode(input),
  listUserInstallations: (
    input: Parameters<typeof listGitHubUserInstallations>[0]
  ) =>
    runtimeConfiguration().ports.listGitHubUserInstallations?.(input) ??
    listGitHubUserInstallations(input),
  inspectInstallation: (
    input: Parameters<typeof inspectGitHubAppInstallation>[0]
  ) =>
    runtimeConfiguration().ports.inspectGitHubAppInstallation?.(input) ??
    inspectGitHubAppInstallation(input),
  listRepositories: (
    input: Parameters<typeof listGitHubInstallationRepositories>[0]
  ) =>
    runtimeConfiguration().ports.listGitHubInstallationRepositories?.(input) ??
    listGitHubInstallationRepositories(input),
  revokeAuthorization: (
    input: Parameters<typeof revokeGitHubUserAuthorization>[0] & {
      readonly githubUserId: number
    }
  ) =>
    runtimeConfiguration().ports.revokeGitHubUserAuthorization?.(input) ??
    revokeGitHubUserAuthorization(input),
  createInstallationToken: (
    input: Parameters<typeof createGitHubInstallationToken>[0]
  ) =>
    runtimeConfiguration().ports.createGitHubInstallationToken?.(input) ??
    createGitHubInstallationToken(input),
}

export function requiredGitHubPermissions(
  permissions: Readonly<Record<string, string>>
): boolean {
  const rank = { none: 0, read: 1, write: 2 } as const
  const required = {
    administration: "read",
    checks: "read",
    contents: "write",
    metadata: "read",
    pull_requests: "write",
  } as const
  return Object.entries(required).every(
    ([name, level]) =>
      (rank[permissions[name] as keyof typeof rank] ?? 0) >= rank[level]
  )
}
