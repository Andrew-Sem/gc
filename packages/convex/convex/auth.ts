import {
  type AuthFunctions,
  createClient,
  type GenericCtx,
} from "@convex-dev/better-auth"
import type { ComponentApi } from "@convex-dev/better-auth/_generated/component.js"
import { convex } from "@convex-dev/better-auth/plugins"
import { type BetterAuthOptions, betterAuth } from "better-auth/minimal"
import {
  componentsGeneric,
  type FunctionReference,
  type GenericMutationCtx,
  type GenericQueryCtx,
  makeFunctionReference,
} from "convex/server"
import { v } from "convex/values"
import type { DataModel } from "../src/data-model"
import { internalMutation } from "./_generated/server"
import authConfig from "./auth.config"

type OwnerReadContext = Pick<GenericQueryCtx<DataModel>, "auth" | "db">

const components = componentsGeneric()
const authFunctions: AuthFunctions = {
  onCreate: internalAuthReference("onCreate"),
  onUpdate: internalAuthReference("onUpdate"),
  onDelete: internalAuthReference("onDelete"),
}
export const authComponent = createClient<DataModel>(
  components.betterAuth as unknown as ComponentApi,
  {
    authFunctions,
    triggers: {
      user: {
        onCreate: async (ctx, user) => {
          const existing = await ctx.db
            .query("owners")
            .withIndex("by_subject", (query) => query.eq("subject", user._id))
            .unique()
          if (existing) return

          const tenant = user._id
          await ctx.db.insert("tenants", { tenant })
          await ctx.db.insert("owners", { tenant, subject: user._id })
        },
      },
    },
  }
)

function internalAuthReference(
  name: string
): FunctionReference<"mutation", "internal"> {
  return makeFunctionReference<"mutation">(
    `auth:${name}`
  ) as unknown as FunctionReference<"mutation", "internal">
}

export const { onCreate, onUpdate, onDelete } = authComponent.triggersApi()

export const createAuthOptions = (
  ctx: GenericCtx<DataModel>
): BetterAuthOptions => {
  const siteUrl =
    process.env.BETTER_AUTH_ALLOWED_ORIGIN ?? "http://localhost:4173"
  return {
    baseURL: siteUrl,
    secret:
      process.env.BETTER_AUTH_SECRET ??
      "convex-better-auth-build-placeholder-secret",
    trustedOrigins: [siteUrl],
    database: authComponent.adapter(ctx),
    emailAndPassword: {
      enabled: true,
      requireEmailVerification: false,
    },
    rateLimit: { storage: "database" },
    plugins: [
      convex({
        authConfig,
        jwksRotateOnTokenGenerationError: true,
        options: { basePath: "/api/auth" },
      }),
    ],
  }
}

export const createAuth = (ctx: GenericCtx<DataModel>) =>
  betterAuth(createAuthOptions(ctx))

export const assertOwnerMutation = internalMutation({
  args: { tenant: v.string() },
  handler: async (ctx, { tenant }) => assertOwner(ctx, tenant),
})

export async function requireOwner(
  ctx: GenericMutationCtx<DataModel>,
  tenant: string
): Promise<string> {
  return assertOwner(ctx, tenant)
}

export async function assertOwner(
  ctx: OwnerReadContext,
  tenant: string
): Promise<string> {
  const identity = await ctx.auth.getUserIdentity()
  if (!identity) throw new Error("authentication required")
  const owner = await ctx.db
    .query("owners")
    .withIndex("by_tenant", (query) => query.eq("tenant", tenant))
    .unique()
  if (!owner || owner.subject !== identity.subject) {
    throw new Error("tenant ownership required")
  }
  return identity.subject
}
