import { commandsOf } from "@workspace/github"
import type { GenericMutationCtx } from "convex/server"
import { v } from "convex/values"
import type { DataModel } from "../src/data-model"
import { runtimeConfiguration, sharedGitHubApp } from "../src/runtime-ports"
import { internalMutation, internalQuery } from "./_generated/server"
import { dispatchCommand } from "./control_plane"

export const installationEnvelope = internalQuery({
  args: { installationId: v.number() },
  handler: async (ctx, { installationId }) => {
    const installation = await ctx.db
      .query("installations")
      .withIndex("by_installation_id", (query) =>
        query.eq("installationId", installationId)
      )
      .unique()
    return installation ? { tenant: installation.tenant } : null
  },
})

function repositoryNames(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.flatMap((repository) =>
    typeof repository === "object" &&
    repository !== null &&
    "full_name" in repository &&
    typeof repository.full_name === "string"
      ? [repository.full_name]
      : []
  )
}

function actionOf(payload: unknown): string | null {
  return typeof payload === "object" &&
    payload !== null &&
    "action" in payload &&
    typeof payload.action === "string"
    ? payload.action
    : null
}

async function applyInstallationLifecycle(
  ctx: GenericMutationCtx<DataModel>,
  input: {
    readonly event: string
    readonly payload: unknown
    readonly tenant: string
    readonly at: number
  }
): Promise<boolean> {
  const payload = input.payload
  if (
    typeof payload !== "object" ||
    payload === null ||
    !("installation" in payload) ||
    typeof payload.installation !== "object" ||
    payload.installation === null ||
    !("id" in payload.installation) ||
    typeof payload.installation.id !== "number"
  ) {
    return false
  }
  const installationId = payload.installation.id
  const installation = await ctx.db
    .query("installations")
    .withIndex("by_installation_id", (query) =>
      query.eq("installationId", installationId)
    )
    .unique()
  if (!installation || installation.tenant !== input.tenant) return false
  const action = actionOf(payload)
  if (input.event === "installation_repositories") {
    const available = new Set(installation.availableRepositories)
    for (const repo of repositoryNames(
      "repositories_removed" in payload ? payload.repositories_removed : []
    )) {
      available.delete(repo)
    }
    for (const repo of repositoryNames(
      "repositories_added" in payload ? payload.repositories_added : []
    )) {
      available.add(repo)
      await dispatchCommand(ctx.db, {
        type: "ReportRepositoryConnected",
        reportId: `installation-repositories:${installationId}:${repo}`,
        tenant: input.tenant,
        repo,
        at: input.at,
      })
    }
    await ctx.db.patch(installation._id, {
      availableRepositories: [...available].sort(),
      updatedAt: input.at,
    })
    return true
  }
  if (input.event !== "installation") return false
  if (action === "deleted") {
    await ctx.db.patch(installation._id, {
      deleted: true,
      suspended: true,
      availableRepositories: [],
      updatedAt: input.at,
    })
    return true
  }
  if (action === "suspend") {
    await ctx.db.patch(installation._id, {
      suspended: true,
      updatedAt: input.at,
    })
    return true
  }
  if (action === "unsuspend" || action === "created") {
    await ctx.db.patch(installation._id, {
      appId: sharedGitHubApp().appId,
      deleted: false,
      suspended: false,
      updatedAt: input.at,
    })
    return true
  }
  return false
}

function isInstallationLifecycle(event: string): boolean {
  return event === "installation" || event === "installation_repositories"
}

export const acceptDelivery = internalMutation({
  args: {
    deliveryId: v.string(),
    event: v.string(),
    payload: v.any(),
    tenant: v.string(),
    at: v.number(),
  },
  handler: async (ctx, input) => {
    runtimeConfiguration().ports.recordControlPlaneOperation?.({
      operation: "webhook.dedupe",
      deliveryId: input.deliveryId,
      tenant: input.tenant,
    })
    const duplicate = await ctx.db
      .query("deliveries")
      .withIndex("by_delivery_id", (query) =>
        query.eq("deliveryId", input.deliveryId)
      )
      .unique()
    if (duplicate) return { duplicate: true, commands: 0 }
    await ctx.db.insert("deliveries", {
      deliveryId: input.deliveryId,
      tenant: input.tenant,
    })
    if (isInstallationLifecycle(input.event)) {
      runtimeConfiguration().ports.recordControlPlaneOperation?.({
        operation: "webhook.parse",
        deliveryId: input.deliveryId,
        tenant: input.tenant,
      })
    }
    if (await applyInstallationLifecycle(ctx, input)) {
      return { duplicate: false, commands: 0 }
    }
    runtimeConfiguration().ports.recordControlPlaneOperation?.({
      operation: "webhook.parse",
      deliveryId: input.deliveryId,
      tenant: input.tenant,
    })
    const commands = commandsOf(
      { id: input.deliveryId, event: input.event, payload: input.payload },
      { tenant: input.tenant, at: input.at }
    )
    for (const command of commands) await dispatchCommand(ctx.db, command)
    return { duplicate: false, commands: commands.length }
  },
})
