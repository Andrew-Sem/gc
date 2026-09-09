import { hostname } from "node:os"
import { createCodexPort } from "./codex-port"
import { createGitHubPort } from "./github-port"
import { runWorkerOnce } from "./index"
import { runProductionWorker } from "./program"
import {
  createHttpControlPlaneTransport,
  createProductionWorkerRuntime,
} from "./runtime"
import { createWorkspacePort } from "./workspace-port"

const runtime = createProductionWorkerRuntime(
  {
    now: Date.now,
    github: createGitHubPort(),
    workspace: createWorkspacePort(requiredEnvironment("WORKSPACE_ROOT")),
    codex: createCodexPort(),
  },
  createHttpControlPlaneTransport(requiredEnvironment("WORKER_PROTOCOL_URL")),
  process.env.WORKER_ID ?? hostname(),
  requiredEnvironment("WORKER_TOKEN")
)

let stopping = false
await runProductionWorker(runtime, {
  runOnce: (workerRuntime) => runWorkerOnce(workerRuntime, () => stopping),
  sleep: (ms) => new Promise((resolve) => setTimeout(resolve, ms)),
  random: Math.random,
  isStopping: () => stopping,
  installStopHandlers: (stop) => {
    const handler = () => {
      stopping = true
      stop()
    }
    process.once("SIGINT", handler)
    process.once("SIGTERM", handler)
  },
})

function requiredEnvironment(name: string): string {
  const value = process.env[name]
  if (!value) throw new Error(`${name} is required`)
  return value
}
