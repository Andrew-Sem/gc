import type { WorkerRuntime } from "./index"

const IDLE_DELAY_MS = 1_000
const MIN_BACKOFF_MS = 100
const INITIAL_BACKOFF_MS = 500
const MAX_BACKOFF_MS = 30_000

export type WorkerLoopControl = {
  readonly runOnce: (runtime: WorkerRuntime) => Promise<"worked" | "idle">
  readonly sleep: (ms: number) => Promise<void>
  readonly random: () => number
  readonly isStopping: () => boolean
}

export async function runWorkerLoop(
  runtime: WorkerRuntime,
  control: WorkerLoopControl
): Promise<void> {
  let consecutiveFailures = 0

  while (!control.isStopping()) {
    try {
      const result = await control.runOnce(runtime)
      consecutiveFailures = 0
      if (result === "idle" && !control.isStopping()) {
        await control.sleep(jitter(IDLE_DELAY_MS, control.random))
      }
    } catch {
      consecutiveFailures += 1
      if (control.isStopping()) return
      const exponential = Math.min(
        MAX_BACKOFF_MS,
        INITIAL_BACKOFF_MS * 2 ** (consecutiveFailures - 1)
      )
      await control.sleep(jitter(exponential, control.random))
    }
  }
}

function jitter(baseMs: number, random: () => number): number {
  const sample = Math.min(1, Math.max(0, random()))
  return Math.min(
    MAX_BACKOFF_MS,
    Math.max(MIN_BACKOFF_MS, Math.round(baseMs * (0.5 + sample)))
  )
}
