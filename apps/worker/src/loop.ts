import type { WorkerRuntime } from "./index"

export type WorkerLoopControl = {
  readonly runOnce: (runtime: WorkerRuntime) => Promise<"worked" | "idle">
  readonly sleep: (ms: number) => Promise<void>
  readonly random: () => number
  readonly isStopping: () => boolean
}

export async function runWorkerLoop(
  _runtime: WorkerRuntime,
  _control: WorkerLoopControl
): Promise<void> {
  throw new Error("production worker loop is not implemented")
}
