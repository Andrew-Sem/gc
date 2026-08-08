import type { WorkerRuntime } from "./index"
import type { WorkerLoopControl } from "./loop"
import { runWorkerLoop } from "./loop"

export type WorkerProcessControl = WorkerLoopControl & {
  installStopHandlers(stop: () => void): void
}

export async function runProductionWorker(
  runtime: WorkerRuntime,
  control: WorkerProcessControl
): Promise<void> {
  control.installStopHandlers(() => undefined)
  await runWorkerLoop(runtime, control)
}
