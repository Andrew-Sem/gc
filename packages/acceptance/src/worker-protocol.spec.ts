import type { WorkspaceCheckout } from "@workspace/worker"
import { createProductionWorkerRuntime } from "@workspace/worker/runtime"
import { expect, test } from "vitest"

function leafPorts() {
  return {
    now: () => 0,
    github: {
      createPullRequest: async () => ({
        number: 1,
        branch: "gc/worker-a",
        headSha: "a".repeat(40),
      }),
      getPullRequest: async () => ({}),
      getRequiredChecks: async () => ({}),
      listCheckRuns: async () => ({}),
      mergePullRequest: async () => ({}),
      closePullRequest: async () => ({}),
    },
    workspace: {
      prepare: async () => ({
        workingDirectory: "/work/repo",
        baseRef: "main",
        branch: "gc/worker-a",
      }),
      push: async (checkout: WorkspaceCheckout) => ({
        branch: checkout.branch,
        headSha: "a".repeat(40),
      }),
    },
    codex: {
      run: async () => ({
        outcome: "finished" as const,
        usage: { inputTokens: 1, outputTokens: 2, reasoningTokens: 3 },
      }),
    },
  }
}

test("worker protocol отправляет credential, а missing credential запрещён", async () => {
  const requests: unknown[] = []
  const runtime = createProductionWorkerRuntime(
    leafPorts(),
    {
      request: async (request) => {
        requests.push(request)
        if (request.operation === "credentials") {
          return { github: {}, codex: {}, expiresAt: 10_000 }
        }
        return null
      },
    },
    "worker-a",
    "worker-secret"
  )

  await runtime.tasks.claim("worker-a", 60_000)
  await runtime.tasks.renew("task-1", "lease-1", 60_000)
  await runtime.credentials.get("task-1", "lease-1")
  await runtime.tasks.complete("task-1", "lease-1")
  await runtime.tasks.release("task-1", "lease-1")
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({
        operation: "claim",
        credential: "worker-secret",
        workerId: "worker-a",
      }),
      expect.objectContaining({
        operation: "renew",
        taskId: "task-1",
        leaseToken: "lease-1",
      }),
      expect.objectContaining({
        operation: "credentials",
        taskId: "task-1",
        leaseToken: "lease-1",
      }),
      expect.objectContaining({
        operation: "complete",
        taskId: "task-1",
        leaseToken: "lease-1",
      }),
      expect.objectContaining({
        operation: "release",
        taskId: "task-1",
        leaseToken: "lease-1",
      }),
    ])
  )
  expect(() =>
    createProductionWorkerRuntime(
      leafPorts(),
      { request: async () => null },
      "worker-a",
      ""
    )
  ).toThrow(/credential/i)
})
