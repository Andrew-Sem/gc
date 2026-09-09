import assert from "node:assert/strict"
import {
  configureConvexRuntime,
  convexModules,
  publicApi,
  schema,
  systemApi,
} from "@workspace/convex"
import { convexTest } from "convex-test"
import { test } from "vitest"

const tenant = "acme"
const repo = "acme/app"
const owner = "owner-acme"
const settings = {
  model: "gpt-5.6-luna",
  reasoning: "high" as const,
  prompt: "Remove obsolete code",
}

test("OpenAI base URL is normalized at the tenant boundary and snapshotted for RunAgent", async () => {
  configureConvexRuntime({
    masterKey: "openai-base-url-probe-master-key",
    workerToken: "openai-base-url-probe-worker-token",
    codexHomeRoot: "/probe/codex-home",
    ports: {
      now: () => 1_000,
      discoverInstallationRepositories: async () => [],
    },
  })
  try {
    const backend = convexTest(schema, convexModules)
    await backend.run(async (ctx) => {
      await ctx.db.insert("tenants", { tenant })
      await ctx.db.insert("owners", { tenant, subject: owner })
      await ctx.db.insert("journal", {
        event: { type: "RepositoryConnected", tenant, repo, at: 1 },
      })
    })
    const asOwner = backend.withIdentity({
      subject: owner,
      tokenIdentifier: `better-auth|${owner}`,
    })

    await asOwner.mutation(publicApi.saveTenantSettings, {
      tenant,
      settings,
      openaiBaseUrl: "  HTTPS://Gateway.Example:443/v1/../v1  ",
    })
    await asOwner.mutation(publicApi.saveRepositorySettings, {
      tenant,
      repo,
      settings: {
        productionEveryHours: 1,
        openPrLimit: 5,
        closeAfterHours: 24,
      },
    })
    assert.equal(
      (await asOwner.query(publicApi.setup, { tenant })).openaiBaseUrl,
      "https://gateway.example/v1"
    )

    await asOwner.mutation(publicApi.saveTenantSettings, {
      tenant,
      settings: { ...settings, prompt: "Old caller keeps the boundary value" },
    })
    assert.equal(
      (await asOwner.query(publicApi.setup, { tenant })).openaiBaseUrl,
      "https://gateway.example/v1"
    )

    for (const openaiBaseUrl of [
      "relative/path",
      "ftp://gateway.example/v1",
      "https://user:pass@gateway.example/v1",
      "https://gateway.example/v1?tenant=acme",
      "https://gateway.example/v1#fragment",
    ]) {
      await assert.rejects(
        asOwner.mutation(publicApi.saveTenantSettings, {
          tenant,
          settings: { ...settings, prompt: `invalid ${openaiBaseUrl}` },
          openaiBaseUrl,
        })
      )
    }
    assert.equal(
      (await asOwner.query(publicApi.setup, { tenant })).tenantSettings?.prompt,
      "Old caller keeps the boundary value"
    )

    await backend.mutation(systemApi.productionWakeUp, {
      tenant,
      repo,
      now: 2_000,
    })
    const pending = await backend.run(async (ctx) =>
      (await ctx.db.query("tasks").collect()).find(
        (task) => task.effect?.type === "RunAgent"
      )
    )
    assert.ok(pending)
    assert.deepEqual(pending.runAgentSnapshot, {
      openaiBaseUrl: "https://gateway.example/v1",
    })
    assert.equal(
      JSON.stringify(pending.effect).includes("gateway.example"),
      false
    )

    await asOwner.mutation(publicApi.saveTenantSettings, {
      tenant,
      settings: { ...settings, prompt: "Changed after due tick" },
      openaiBaseUrl: "https://new-gateway.example/v2",
    })
    const claimed = await backend.mutation(systemApi.claimTask, {
      workerId: "probe-worker",
      leaseMs: 60_000,
    })
    assert.ok(claimed)
    const credentials = await backend.query(systemApi.workerCredentials, {
      taskId: claimed.id,
      leaseToken: claimed.leaseToken,
    })
    assert.equal(credentials.codex.baseUrl, "https://gateway.example/v1")
    assert.equal(
      (await asOwner.query(publicApi.setup, { tenant })).openaiBaseUrl,
      "https://new-gateway.example/v2"
    )

    const journal = await backend.run(async (ctx) =>
      (await ctx.db.query("journal").collect()).map((row) => row.event)
    )
    assert.equal(JSON.stringify(journal).includes("gateway.example"), false)
    assert.equal(JSON.stringify(journal).includes("openaiBaseUrl"), false)
  } finally {
    configureConvexRuntime(null)
  }
})
