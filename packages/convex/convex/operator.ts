import {
  type FunctionReference,
  type GenericActionCtx,
  makeFunctionReference,
} from "convex/server"
import { v } from "convex/values"
import type { DataModel } from "../src/data-model"
import { runtimeConfiguration } from "../src/runtime-ports"
import { action } from "./_generated/server"

const productionTick = makeFunctionReference<"mutation">(
  "tick:productionRepository"
) as unknown as FunctionReference<"mutation", "internal">
const sweepTick = makeFunctionReference<"mutation">(
  "tick:sweepRepository"
) as unknown as FunctionReference<"mutation", "internal">
const audit = makeFunctionReference<"mutation">(
  "operator:recordAudit"
) as unknown as FunctionReference<"mutation", "internal">

import { internalMutation } from "./_generated/server"

const assertOwner = makeFunctionReference<"mutation">(
  "auth:assertOwnerMutation"
) as unknown as FunctionReference<"mutation", "internal">
const assertEligible = makeFunctionReference<"query">(
  "operator:repositoryEligible"
) as unknown as FunctionReference<"query", "internal">

import { internalQuery } from "./_generated/server"

export const repositoryEligible = internalQuery({
  args: { tenant: v.string(), repo: v.string() },
  handler: async (ctx, { tenant, repo }) => {
    const installation = await ctx.db
      .query("installations")
      .withIndex("by_tenant", (query) => query.eq("tenant", tenant))
      .unique()
    return Boolean(
      installation &&
        !installation.suspended &&
        !installation.deleted &&
        installation.availableRepositories.includes(repo)
    )
  },
})

export const recordAudit = internalMutation({
  args: {
    tenant: v.string(),
    repo: v.string(),
    operation: v.string(),
    at: v.number(),
  },
  handler: async (ctx, input) => {
    await ctx.db.insert("operatorAudit", input)
  },
})

async function run(
  ctx: GenericActionCtx<DataModel>,
  tenant: string,
  repo: string,
  operation: "production" | "sweep"
): Promise<void> {
  await ctx.runMutation(assertOwner, { tenant })
  const now = runtimeConfiguration().ports.now()
  if (await ctx.runQuery(assertEligible, { tenant, repo })) {
    await ctx.runMutation(
      operation === "production" ? productionTick : sweepTick,
      {
        tenant,
        repo,
        now,
      }
    )
  }
  await ctx.runMutation(audit, { tenant, repo, operation, at: now })
}

export const runProduction = action({
  args: { tenant: v.string(), repo: v.string() },
  handler: (ctx, { tenant, repo }) => run(ctx, tenant, repo, "production"),
})

export const runSweep = action({
  args: { tenant: v.string(), repo: v.string() },
  handler: (ctx, { tenant, repo }) => run(ctx, tenant, repo, "sweep"),
})
