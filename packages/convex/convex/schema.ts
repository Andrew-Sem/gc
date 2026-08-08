import { defineSchema, defineTable } from "convex/server"
import { v } from "convex/values"

export default defineSchema({
  journal: defineTable({ event: v.any() }),
  tasks: defineTable({
    status: v.string(),
    executionCount: v.number(),
    leaseUntil: v.optional(v.number()),
  }),
  secrets: defineTable({ tenant: v.string(), ciphertext: v.string() }),
  deliveries: defineTable({ deliveryId: v.string(), tenant: v.string() }),
  installations: defineTable({
    installationId: v.number(),
    tenant: v.string(),
  }),
  operatorAudit: defineTable({
    tenant: v.string(),
    repo: v.string(),
    operation: v.string(),
    at: v.number(),
  }),
})
