import { type FunctionReference, makeFunctionReference } from "convex/server"
import { runtimeConfiguration } from "../src/runtime-ports"
import { internalMutation } from "./_generated/server"
import { eligibleConnectedRepositories, journalEvents } from "./control_plane"

const productionTick = makeFunctionReference<"mutation">(
  "tick:productionRepository"
) as unknown as FunctionReference<"mutation", "internal">
const sweepTick = makeFunctionReference<"mutation">(
  "tick:sweepRepository"
) as unknown as FunctionReference<"mutation", "internal">

export const production = internalMutation({
  args: {},
  handler: async (ctx) => {
    const now = runtimeConfiguration().ports.now()
    const connected = await eligibleConnectedRepositories(
      ctx.db,
      await journalEvents(ctx.db)
    )
    for (const { tenant, repo } of connected) {
      const schedule = await ctx.db
        .query("repositorySchedule")
        .filter((query) =>
          query.and(
            query.eq(query.field("tenant"), tenant),
            query.eq(query.field("repo"), repo)
          )
        )
        .first()
      if (!schedule || now < schedule.nextProductionAt) continue
      await ctx.runMutation(productionTick, { tenant, repo, now })
      const settings = (await journalEvents(ctx.db))
        .filter(
          (event) =>
            event.type === "RepositorySettingsChanged" &&
            event.tenant === tenant &&
            event.repo === repo
        )
        .at(-1)
      if (settings?.type === "RepositorySettingsChanged") {
        await ctx.db.patch(schedule._id, {
          nextProductionAt:
            schedule.nextProductionAt +
            settings.settings.productionEveryHours * 3_600_000,
        })
      }
    }
  },
})

export const sweep = internalMutation({
  args: {},
  handler: async (ctx) => {
    const now = runtimeConfiguration().ports.now()
    for (const { tenant, repo } of await eligibleConnectedRepositories(
      ctx.db,
      await journalEvents(ctx.db)
    )) {
      await ctx.runMutation(sweepTick, { tenant, repo, now })
    }
  },
})
