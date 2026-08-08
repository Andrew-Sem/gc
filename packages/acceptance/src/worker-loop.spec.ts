import type { WorkerRuntime } from "@workspace/worker"
import { runWorkerLoop } from "@workspace/worker/loop"
import { expect, test } from "vitest"

function runtime(): WorkerRuntime {
  return {} as WorkerRuntime
}

function runSequence(results: readonly ("worked" | "idle" | Error)[]) {
  let index = 0
  return async () => {
    const result = results[index++] ?? "idle"
    if (result instanceof Error) throw result
    return result
  }
}

test("idle worker ждёт, а не крутит busy loop", async () => {
  const sleeps: number[] = []
  let runs = 0
  const sequence = runSequence(["idle"])
  let stopping = false
  await runWorkerLoop(runtime(), {
    runOnce: async () => {
      runs += 1
      return sequence()
    },
    sleep: async (ms) => {
      sleeps.push(ms)
      stopping = true
    },
    random: () => 0.5,
    isStopping: () => stopping,
  })
  expect(sleeps).toEqual([expect.any(Number)])
  expect(sleeps[0]).toBeGreaterThanOrEqual(100)
  expect(runs).toBe(1)
})

test("transient failures получают bounded backoff и loop продолжает работу", async () => {
  const sleeps: number[] = []
  let runs = 0
  const sequence = runSequence([new Error("temporary"), "worked", "idle"])
  let stopping = false
  await runWorkerLoop(runtime(), {
    runOnce: async () => {
      runs += 1
      return sequence()
    },
    sleep: async (ms) => {
      sleeps.push(ms)
      if (sleeps.length === 2) stopping = true
    },
    random: () => 0.5,
    isStopping: () => stopping,
  })
  expect(sleeps[0]).toBeGreaterThanOrEqual(100)
  expect(sleeps[0]).toBeLessThanOrEqual(30_000)
  expect(runs).toBe(3)
})

test("SIGTERM stop state не разрешает claim новой task", async () => {
  let slept = false
  let runs = 0
  await runWorkerLoop(runtime(), {
    runOnce: async () => {
      runs += 1
      return runSequence([])()
    },
    sleep: async () => {
      slept = true
    },
    random: () => 0.5,
    isStopping: () => true,
  })
  expect(slept).toBe(false)
  expect(runs).toBe(0)
})
