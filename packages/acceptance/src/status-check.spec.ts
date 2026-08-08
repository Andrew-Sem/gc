import { expect, test } from "vitest"
import { assertStackReady } from "./status-check"

test("status проверяет readiness всех трёх services", async () => {
  const calls: string[] = []
  await assertStackReady({
    web: async () => {
      calls.push("web")
      return true
    },
    convex: async () => {
      calls.push("convex")
      return true
    },
    worker: async () => {
      calls.push("worker")
      return true
    },
  })
  expect(calls).toEqual(["web", "convex", "worker"])
})

test.each(["web", "convex", "worker"] as const)(
  "status падает, когда %s не ready",
  async (failed) => {
    const probe = {
      web: async () => failed !== "web",
      convex: async () => failed !== "convex",
      worker: async () => failed !== "worker",
    }
    await expect(assertStackReady(probe)).rejects.toThrow(failed)
  }
)
