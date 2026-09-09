import { projectOpenPrs } from "@workspace/domain"
import { v } from "convex/values"
import { query } from "./_generated/server"
import { assertOwner } from "./auth"
import { journalEvents } from "./control_plane"

export const listOpenPrs = query({
  args: { tenant: v.string(), repo: v.string(), now: v.number() },
  handler: async (ctx, { tenant, repo, now }) => {
    await assertOwner(ctx, tenant)
    return projectOpenPrs(await journalEvents(ctx.db), now).filter(
      (line) => line.pr.tenant === tenant && line.pr.repo === repo
    )
  },
})

export const readiness = query({
  args: {},
  handler: async (ctx) => {
    await ctx.db.query("journal").first()
    return { status: "ready" as const }
  },
})
