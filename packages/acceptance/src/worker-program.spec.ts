import type { WorkerRuntime } from "@workspace/worker"
import { runProductionWorker } from "@workspace/worker/program"
import { expect, test } from "vitest"

test("production worker устанавливает signals и делегирует tested loop", async () => {
  let installed = false
  let runs = 0
  let stopping = false
  await runProductionWorker({} as WorkerRuntime, {
    runOnce: async () => {
      runs += 1
      return "idle"
    },
    sleep: async () => {
      stopping = true
    },
    random: () => 0.5,
    isStopping: () => stopping,
    installStopHandlers: () => {
      installed = true
    },
  })
  expect(installed).toBe(true)
  expect(runs).toBe(1)
})
