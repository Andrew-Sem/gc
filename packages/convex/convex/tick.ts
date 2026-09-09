import type { RepositorySettings, TenantSettings } from "@workspace/domain"
import type { GenericMutationCtx } from "convex/server"
import { v } from "convex/values"
import type { DataModel } from "../src/data-model"
import { runtimeConfiguration } from "../src/runtime-ports"
import { internalMutation } from "./_generated/server"
import {
  dispatchCommand,
  journalEvents,
  latestByType,
  reportId,
} from "./control_plane"

async function repositoryEligible(
  ctx: GenericMutationCtx<DataModel>,
  tenant: string,
  repo: string
): Promise<boolean> {
  const installation = await ctx.db
    .query("installations")
    .withIndex("by_tenant", (query) => query.eq("tenant", tenant))
    .unique()
  if (!installation) {
    return !(
      runtimeConfiguration().sharedGitHubApp || process.env.GITHUB_APP_ID
    )
  }
  return (
    !installation.suspended &&
    !installation.deleted &&
    installation.availableRepositories.includes(repo)
  )
}

function settingsFor(
  events: Awaited<ReturnType<typeof journalEvents>>,
  tenant: string,
  repo: string
): { repository: RepositorySettings; tenant: TenantSettings } | null {
  const repository = latestByType(
    events,
    "RepositorySettingsChanged",
    (event) => event.tenant === tenant && event.repo === repo
  )
  const tenantSettings = latestByType(
    events,
    "TenantSettingsChanged",
    (event) => event.tenant === tenant
  )
  return repository && tenantSettings
    ? { repository: repository.settings, tenant: tenantSettings.settings }
    : null
}

export const productionRepository = internalMutation({
  args: { tenant: v.string(), repo: v.string(), now: v.number() },
  handler: async (ctx, { tenant, repo, now }) => {
    if (!(await repositoryEligible(ctx, tenant, repo))) return
    const settings = settingsFor(await journalEvents(ctx.db), tenant, repo)
    if (!settings) return
    const boundary = await ctx.db
      .query("tenants")
      .withIndex("by_tenant", (query) => query.eq("tenant", tenant))
      .unique()
    await dispatchCommand(
      ctx.db,
      {
        type: "TickProduction",
        tenant,
        repo,
        settings: settings.repository,
        tenantSettings: settings.tenant,
        now,
      },
      {
        runAgentSnapshot: {
          ...(boundary?.openaiBaseUrl
            ? { openaiBaseUrl: boundary.openaiBaseUrl }
            : {}),
        },
      }
    )
  },
})

export const sweepRepository = internalMutation({
  args: { tenant: v.string(), repo: v.string(), now: v.number() },
  handler: async (ctx, { tenant, repo, now }) => {
    if (!(await repositoryEligible(ctx, tenant, repo))) return
    const events = await journalEvents(ctx.db)
    const settings = settingsFor(events, tenant, repo)
    if (!settings) return
    const sweepId = reportId(`sweep:${tenant}:${repo}:${now}`)
    const openPrs = projectOpenPrs(events, now).filter(
      (line) => line.pr.tenant === tenant && line.pr.repo === repo
    )
    for (const line of openPrs) {
      await ctx.db.insert("tasks", {
        status: "pending",
        executionCount: 0,
        effect: {
          type: "LookupRequiredChecks",
          sweepId,
          pr: line.pr,
          headSha: line.headSha,
          now,
          settings: settings.repository,
        },
        tenant,
        repo,
        reportSubmissions: 0,
      })
    }
  },
})

import { projectOpenPrs } from "@workspace/domain"
