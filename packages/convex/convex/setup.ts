import type { DomainEvent } from "@workspace/domain"
import {
  type FunctionReference,
  type GenericMutationCtx,
  makeFunctionReference,
} from "convex/server"
import { v } from "convex/values"
import type { DataModel } from "../src/data-model"
import { runtimeConfiguration, tenantCodexHome } from "../src/runtime-ports"
import { encryptSecretValue } from "../src/secret-crypto"
import { action, internalMutation, mutation, query } from "./_generated/server"
import { assertOwner, requireOwner } from "./auth"
import {
  dispatchCommand,
  journalEvents,
  latestByType,
  reportId,
} from "./control_plane"

const queueDeviceLoginReference = makeFunctionReference<"mutation">(
  "setup:queueDeviceLogin"
) as unknown as FunctionReference<"mutation", "internal">

async function replaceSecret(
  ctx: GenericMutationCtx<DataModel>,
  tenant: string,
  kind: string,
  plaintext: string
): Promise<void> {
  const rows = await ctx.db
    .query("secrets")
    .withIndex("by_tenant_kind", (query) =>
      query.eq("tenant", tenant).eq("kind", kind)
    )
    .collect()
  for (const row of rows) await ctx.db.delete(row._id)
  await ctx.db.insert("secrets", {
    tenant,
    kind,
    ciphertext: await encryptSecretValue(
      runtimeConfiguration().masterKey,
      plaintext
    ),
  })
}

export const connectCodexWithApiKey = mutation({
  args: { tenant: v.string(), apiKey: v.string() },
  handler: async (ctx, { tenant, apiKey }) => {
    await requireOwner(ctx, tenant)
    await replaceSecret(ctx, tenant, "codex_api_key", apiKey)
    await dispatchCommand(ctx.db, {
      type: "ReportCodexConnected",
      reportId: reportId("codex-api-key"),
      tenant,
      method: "api_key",
      at: runtimeConfiguration().ports.now(),
    })
  },
})

export const queueDeviceLogin = internalMutation({
  args: {
    tenant: v.string(),
    deviceUrl: v.optional(v.string()),
    userCode: v.optional(v.string()),
    at: v.number(),
  },
  handler: async (ctx, input) => {
    const existing = await ctx.db
      .query("codexDeviceLogins")
      .filter((query) => query.eq(query.field("tenant"), input.tenant))
      .first()
    if (existing?.status === "pending") return
    if (existing) await ctx.db.delete(existing._id)
    const taskId = await ctx.db.insert("tasks", {
      status: "pending",
      executionCount: 0,
      effect: {
        type: "StartCodexDeviceLogin",
        tenant: input.tenant,
      },
      tenant: input.tenant,
      reportSubmissions: 0,
    })
    await ctx.db.insert("codexDeviceLogins", {
      tenant: input.tenant,
      taskId,
      deviceUrl: input.deviceUrl,
      userCode: input.userCode,
      status: "pending",
      startedAt: input.at,
    })
  },
})

export const startCodexDeviceLogin = action({
  args: { tenant: v.string() },
  handler: async (ctx, { tenant }) => {
    await ctx.runMutation(
      makeFunctionReference<"mutation">(
        "auth:assertOwnerMutation"
      ) as unknown as FunctionReference<"mutation", "internal">,
      { tenant }
    )
    const codexHome = tenantCodexHome(tenant)
    const login =
      await runtimeConfiguration().ports.bootstrapCodexDeviceLogin?.({
        tenant,
        codexHome,
      })
    await ctx.runMutation(queueDeviceLoginReference, {
      tenant,
      deviceUrl: login?.deviceUrl,
      userCode: login?.userCode,
      at: runtimeConfiguration().ports.now(),
    })
  },
})

export const get = query({
  args: { tenant: v.string() },
  handler: async (ctx, { tenant }) => {
    await assertOwner(ctx, tenant)
    const events = await journalEvents(ctx.db)
    const installation = await ctx.db
      .query("installations")
      .withIndex("by_tenant", (query) => query.eq("tenant", tenant))
      .unique()
    const deviceLogin = await ctx.db
      .query("codexDeviceLogins")
      .filter((query) => query.eq(query.field("tenant"), tenant))
      .first()
    const tenantBoundary = await ctx.db
      .query("tenants")
      .withIndex("by_tenant", (query) => query.eq("tenant", tenant))
      .unique()
    const historicalRepositories = events
      .filter(
        (
          event
        ): event is Extract<DomainEvent, { type: "RepositoryConnected" }> =>
          event.type === "RepositoryConnected" && event.tenant === tenant
      )
      .map((event) => event.repo)
    const eligibleRepositories = new Set(
      installation && !installation.suspended && !installation.deleted
        ? installation.availableRepositories
        : []
    )
    const connectedRepositories = historicalRepositories.filter((repo) =>
      eligibleRepositories.has(repo)
    )
    const codex = latestByType(
      events,
      "CodexConnected",
      (event) => event.tenant === tenant
    )
    const deviceStatus = deviceLogin?.status ?? null
    const tenantSettings = latestByType(
      events,
      "TenantSettingsChanged",
      (event) => event.tenant === tenant
    )
    const repositorySettings: Record<string, unknown> = {}
    for (const event of events) {
      if (
        event.type === "RepositorySettingsChanged" &&
        event.tenant === tenant
      ) {
        repositorySettings[event.repo] = event.settings
      }
    }
    return {
      tenant,
      github: {
        connected:
          installation !== null &&
          !installation.suspended &&
          !installation.deleted,
      },
      connectedRepositories: [...new Set(connectedRepositories)].sort(),
      codex: {
        connected: codex !== null,
        method: codex?.method ?? null,
        status: codex?.method === "subscription" ? "connected" : deviceStatus,
        deviceUrl:
          deviceStatus === "pending" ? (deviceLogin?.deviceUrl ?? null) : null,
        userCode:
          deviceStatus === "pending" ? (deviceLogin?.userCode ?? null) : null,
      },
      tenantSettings: tenantSettings?.settings ?? null,
      openaiBaseUrl: tenantBoundary?.openaiBaseUrl ?? null,
      repositorySettings,
    }
  },
})
