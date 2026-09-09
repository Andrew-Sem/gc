import { v } from "convex/values"
import { runtimeConfiguration } from "../src/runtime-ports"
import { mutation } from "./_generated/server"
import { requireOwner } from "./auth"
import { dispatchCommand, reportId } from "./control_plane"

const tenantSettings = v.object({
  model: v.string(),
  reasoning: v.union(
    v.literal("low"),
    v.literal("medium"),
    v.literal("high"),
    v.literal("xhigh")
  ),
  prompt: v.string(),
})

const repositorySettings = v.object({
  productionEveryHours: v.number(),
  openPrLimit: v.number(),
  closeAfterHours: v.number(),
})

function requireNonEmpty(value: string, name: string): void {
  if (value.trim().length === 0) throw new Error(`${name} must not be empty`)
}

function requirePositiveInteger(value: number, name: string): void {
  if (!Number.isFinite(value) || !Number.isInteger(value) || value <= 0) {
    throw new Error(`${name} must be a finite positive integer`)
  }
}

function requirePositiveHours(value: number, name: string): void {
  if (!Number.isFinite(value) || value <= 0) {
    throw new Error(`${name} must be a finite positive number of hours`)
  }
}

export const saveTenant = mutation({
  args: {
    tenant: v.string(),
    settings: tenantSettings,
    openaiBaseUrl: v.optional(v.string()),
  },
  handler: async (ctx, { tenant, settings, openaiBaseUrl }) => {
    await requireOwner(ctx, tenant)
    requireNonEmpty(settings.model, "model")
    requireNonEmpty(settings.prompt, "prompt")
    if (openaiBaseUrl !== undefined) {
      const normalized = normalizeOpenAiBaseUrl(openaiBaseUrl)
      const boundary = await ctx.db
        .query("tenants")
        .withIndex("by_tenant", (query) => query.eq("tenant", tenant))
        .unique()
      if (boundary) {
        await ctx.db.patch(boundary._id, { openaiBaseUrl: normalized })
      } else {
        await ctx.db.insert("tenants", { tenant, openaiBaseUrl: normalized })
      }
    }
    await dispatchCommand(ctx.db, {
      type: "ReportTenantSettingsChanged",
      reportId: reportId("tenant-settings"),
      tenant,
      settings,
      at: runtimeConfiguration().ports.now(),
    })
  },
})

function normalizeOpenAiBaseUrl(value: string): string | undefined {
  const trimmed = value.trim()
  if (trimmed.length === 0) return undefined

  let url: URL
  try {
    url = new URL(trimmed)
  } catch {
    throw new Error("OpenAI base URL must be an absolute HTTP(S) URL")
  }
  if (
    (url.protocol !== "http:" && url.protocol !== "https:") ||
    url.hostname.length === 0
  ) {
    throw new Error("OpenAI base URL must be an absolute HTTP(S) URL")
  }
  if (url.username || url.password) {
    throw new Error("OpenAI base URL must not contain credentials")
  }
  if (url.href.includes("?") || url.href.includes("#")) {
    throw new Error("OpenAI base URL must not contain a query or fragment")
  }
  return url.toString()
}

export const saveRepository = mutation({
  args: { tenant: v.string(), repo: v.string(), settings: repositorySettings },
  handler: async (ctx, { tenant, repo, settings }) => {
    await requireOwner(ctx, tenant)
    requirePositiveInteger(
      settings.productionEveryHours,
      "productionEveryHours"
    )
    requirePositiveInteger(settings.openPrLimit, "openPrLimit")
    requirePositiveHours(settings.closeAfterHours, "closeAfterHours")
    const now = runtimeConfiguration().ports.now()
    await dispatchCommand(ctx.db, {
      type: "ReportRepositorySettingsChanged",
      reportId: reportId("repository-settings"),
      tenant,
      repo,
      settings,
      at: now,
    })
    const schedules = await ctx.db
      .query("repositorySchedule")
      .filter((query) =>
        query.and(
          query.eq(query.field("tenant"), tenant),
          query.eq(query.field("repo"), repo)
        )
      )
      .collect()
    for (const schedule of schedules) await ctx.db.delete(schedule._id)
    await ctx.db.insert("repositorySchedule", {
      tenant,
      repo,
      nextProductionAt: now + settings.productionEveryHours * 3_600_000,
    })
  },
})
