import { defineSchema, defineTable } from "convex/server"
import { v } from "convex/values"

export default defineSchema({
  journal: defineTable({ event: v.any() }),
  tasks: defineTable({
    status: v.string(),
    executionCount: v.number(),
    effect: v.any(),
    tenant: v.string(),
    repo: v.optional(v.string()),
    workerId: v.optional(v.string()),
    leaseToken: v.optional(v.string()),
    leaseUntil: v.optional(v.number()),
    reportSubmissions: v.number(),
    lastCommand: v.optional(v.any()),
    acceptedReports: v.optional(v.array(v.any())),
    observedHeadSha: v.optional(v.string()),
    completedAt: v.optional(v.number()),
    completionLeaseTokenHash: v.optional(v.string()),
    runAgentSnapshot: v.optional(
      v.object({ openaiBaseUrl: v.optional(v.string()) })
    ),
  }),
  secrets: defineTable({
    tenant: v.string(),
    kind: v.string(),
    ciphertext: v.string(),
  }).index("by_tenant_kind", ["tenant", "kind"]),
  deliveries: defineTable({ deliveryId: v.string(), tenant: v.string() }).index(
    "by_delivery_id",
    ["deliveryId"]
  ),
  installations: defineTable({
    installationId: v.number(),
    tenant: v.string(),
    appId: v.number(),
    availableRepositories: v.array(v.string()),
    suspended: v.boolean(),
    deleted: v.boolean(),
    updatedAt: v.number(),
  })
    .index("by_installation_id", ["installationId"])
    .index("by_tenant", ["tenant"]),
  githubAppInstallStates: defineTable({
    stateHash: v.string(),
    subject: v.string(),
    sessionId: v.string(),
    tenant: v.string(),
    createdAt: v.number(),
    expiresAt: v.number(),
    consumedAt: v.optional(v.number()),
  })
    .index("by_state_hash", ["stateHash"])
    .index("by_tenant_created_at", ["tenant", "createdAt"]),
  githubAppOAuthFlows: defineTable({
    stateHash: v.string(),
    subject: v.string(),
    sessionId: v.string(),
    tenant: v.string(),
    installationId: v.number(),
    status: v.string(),
    encryptedCodeVerifier: v.string(),
    codeHash: v.optional(v.string()),
    encryptedUserToken: v.optional(v.string()),
    githubUserId: v.optional(v.number()),
    reportId: v.string(),
    createdAt: v.number(),
    expiresAt: v.number(),
    connectedAt: v.optional(v.number()),
    completedAt: v.optional(v.number()),
  })
    .index("by_state_hash", ["stateHash"])
    .index("by_tenant_created_at", ["tenant", "createdAt"]),
  tenants: defineTable({
    tenant: v.string(),
    openaiBaseUrl: v.optional(v.string()),
  }).index("by_tenant", ["tenant"]),
  owners: defineTable({ tenant: v.string(), subject: v.string() })
    .index("by_tenant", ["tenant"])
    .index("by_subject", ["subject"]),
  repositorySchedule: defineTable({
    tenant: v.string(),
    repo: v.string(),
    nextProductionAt: v.number(),
  }),
  codexDeviceLogins: defineTable({
    tenant: v.string(),
    taskId: v.string(),
    deviceUrl: v.optional(v.string()),
    userCode: v.optional(v.string()),
    status: v.string(),
    startedAt: v.number(),
    finishedAt: v.optional(v.number()),
  }),
  operatorAudit: defineTable({
    tenant: v.string(),
    repo: v.string(),
    operation: v.string(),
    at: v.number(),
  }),
})
