import { runWorkerOnce } from "./index"
import { runProductionWorker } from "./program"
import { createProductionWorkerRuntime } from "./runtime"

const runtime = createProductionWorkerRuntime(
  {
    now: Date.now,
    github: {
      createPullRequest: async () => {
        throw new Error("production GitHub port is not implemented")
      },
      getPullRequest: async () => {
        throw new Error("production GitHub port is not implemented")
      },
      getRequiredChecks: async () => {
        throw new Error("production GitHub port is not implemented")
      },
      listCheckRuns: async () => {
        throw new Error("production GitHub port is not implemented")
      },
      mergePullRequest: async () => {
        throw new Error("production GitHub port is not implemented")
      },
      closePullRequest: async () => {
        throw new Error("production GitHub port is not implemented")
      },
    },
    workspace: {
      prepare: async () => {
        throw new Error("production workspace port is not implemented")
      },
      push: async () => {
        throw new Error("production workspace port is not implemented")
      },
    },
    codex: {
      run: async () => {
        throw new Error("production Codex port is not implemented")
      },
    },
  },
  {
    request: async () => {
      throw new Error("production Convex transport is not implemented")
    },
  },
  "production-worker",
  process.env.WORKER_TOKEN
)

let stopping = false
await runProductionWorker(runtime, {
  runOnce: runWorkerOnce,
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
