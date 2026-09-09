import { strict as assert } from "node:assert"
import { execFile } from "node:child_process"
import { mkdtemp, writeFile } from "node:fs/promises"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { pathToFileURL } from "node:url"
import { promisify } from "node:util"
import type {
  GitHubCredentials,
  WorkerRuntime,
  WorkerTask,
} from "@workspace/worker"
import { runWorkerOnce } from "@workspace/worker"
import { codexSubprocessEnvironment } from "../../../apps/worker/src/codex-port"
import { createWorkspacePort } from "../../../apps/worker/src/workspace-port"

const exec = promisify(execFile)

await probeCodexEnvironment()
await probeLeaseFence()
await probeImmediateCompletion()
await probeStableBranchRetry()
process.stdout.write("worker hardening probe passed\n")

async function probeCodexEnvironment(): Promise<void> {
  const env = codexSubprocessEnvironment("/tenants/acme/codex", {
    PATH: "/usr/local/bin:/usr/bin",
    LANG: "C.UTF-8",
    HTTPS_PROXY: "http://proxy.invalid",
    WORKER_TOKEN: "worker-control-plane-secret",
    SECRET_MASTER_KEY: "encryption-control-plane-secret",
    BETTER_AUTH_SECRET: "auth-control-plane-secret",
    INTERNAL_CONVEX_URL: "http://convex.internal",
    OPENAI_API_KEY: "unscoped-model-secret",
  })

  assert.deepEqual(env, {
    PATH: "/usr/local/bin:/usr/bin",
    LANG: "C.UTF-8",
    HTTPS_PROXY: "http://proxy.invalid",
    CODEX_HOME: "/tenants/acme/codex",
  })
}

async function probeLeaseFence(): Promise<void> {
  const task = closeTask("lease-fence")
  let renewals = 0
  let aborted = false
  let completed = false
  const runtime = taskRuntime([task], {
    renew: async (_taskId, _leaseToken, _leaseMs, signal) => {
      renewals += 1
      if (renewals === 1) return
      signal?.throwIfAborted()
      throw new Error("lease renewal rejected")
    },
    getPullRequest: ({ signal }) =>
      new Promise((_resolve, reject) => {
        signal?.addEventListener(
          "abort",
          () => {
            aborted = true
            reject(signal.reason)
          },
          { once: true }
        )
      }),
    complete: async () => {
      completed = true
    },
  })

  const execution = runWorkerOnce(runtime, () => false, 10)
  const rejected = assert.rejects(execution, /lease renewal rejected/)
  await new Promise((resolve) => setTimeout(resolve, 20))
  await rejected
  assert.equal(aborted, true)
  assert.equal(completed, false)
}

async function probeImmediateCompletion(): Promise<void> {
  const operations: string[] = []
  const tasks = [closeTask("first"), closeTask("second")]
  const runtime = taskRuntime(tasks, {
    claim: async () => {
      operations.push("claim")
      return tasks.shift() ?? null
    },
    complete: async (taskId) => {
      operations.push(`complete:${taskId}`)
    },
  })

  assert.equal(await runWorkerOnce(runtime), "worked")
  assert.deepEqual(operations, ["claim", "complete:first"])
  assert.equal(await runWorkerOnce(runtime), "worked")
  assert.deepEqual(operations, [
    "claim",
    "complete:first",
    "claim",
    "complete:second",
  ])
  assert.equal(await runWorkerOnce(runtime), "idle")
  assert.equal(operations.at(-1), "claim")
}

async function probeStableBranchRetry(): Promise<void> {
  const root = await mkdtemp(join(tmpdir(), "gc-branch-retry-"))
  const remote = join(root, "remote.git")
  const seed = join(root, "seed")
  await exec("git", ["init", "--bare", remote])
  await exec("git", ["init", "-b", "main", seed])
  await exec("git", ["-C", seed, "config", "user.email", "gc@example.com"])
  await exec("git", ["-C", seed, "config", "user.name", "Garbage Collector"])
  await writeFile(join(seed, "README.md"), "seed\n")
  await exec("git", ["-C", seed, "add", "README.md"])
  await exec("git", ["-C", seed, "commit", "-m", "seed"])
  await exec("git", ["-C", seed, "remote", "add", "origin", remote])
  await exec("git", ["-C", seed, "push", "origin", "main"])

  const cloneInfo = async () => ({
    cloneUrl: pathToFileURL(remote).toString(),
    defaultBranch: "main",
    token: "unused-for-file-remote",
  })
  const workspace = createWorkspacePort(join(root, "workspaces"), cloneInfo)
  const credentials: GitHubCredentials = {}
  const key = "worker:stable-task:agent-workspace"
  const first = await workspace.prepare("acme", "acme/app", credentials, key)
  await writeFile(join(first.workingDirectory, "obsolete.txt"), "first push\n")
  const pushed = await workspace.push(first, credentials)
  await workspace.cleanup?.(first)

  const retry = await workspace.prepare("acme", "acme/app", credentials, key)
  assert.equal(retry.branch, pushed.branch)
  assert.equal(retry.remoteBranchHead, pushed.headSha)
  const upstream = await exec("git", [
    "-C",
    retry.workingDirectory,
    "rev-parse",
    "--abbrev-ref",
    "--symbolic-full-name",
    "@{upstream}",
  ])
  assert.equal(upstream.stdout.trim(), `origin/${retry.branch}`)
  assert.deepEqual(await workspace.push(retry, credentials), {
    branch: retry.branch,
    headSha: pushed.headSha,
    changed: true,
  })
  await workspace.cleanup?.(retry)
}

function closeTask(id: string): WorkerTask {
  return {
    id,
    leaseToken: `lease-${id}`,
    executionCount: 1,
    effect: {
      type: "ClosePr",
      sweepId: `sweep-${id}`,
      pr: { tenant: "acme", repo: "acme/app", number: 1 },
    },
  }
}

function taskRuntime(
  tasks: WorkerTask[],
  overrides: {
    readonly claim?: WorkerRuntime["tasks"]["claim"]
    readonly renew?: WorkerRuntime["tasks"]["renew"]
    readonly complete?: WorkerRuntime["tasks"]["complete"]
    readonly getPullRequest?: WorkerRuntime["github"]["getPullRequest"]
  } = {}
): WorkerRuntime {
  return {
    workerId: "integration-worker",
    now: () => 0,
    tasks: {
      claim: overrides.claim ?? (async () => tasks.shift() ?? null),
      renew: overrides.renew ?? (async () => undefined),
      complete: overrides.complete ?? (async () => undefined),
      release: async () => undefined,
    },
    credentials: {
      get: async () => ({ github: {}, codex: {}, expiresAt: 60_000 }),
    },
    reports: {
      submit: async () => undefined,
      updateDeviceLogin: async () => undefined,
    },
    github: {
      createPullRequest: async () => ({
        number: 1,
        branch: "gc/test",
        headSha: "a".repeat(40),
      }),
      getPullRequest:
        overrides.getPullRequest ?? (async () => ({ state: "closed" })),
      getRequiredChecks: async () => ({ items: [], complete: true }),
      listCheckRuns: async () => ({ items: [], complete: true }),
      mergePullRequest: async () => ({}),
      closePullRequest: async () => ({}),
    },
    workspace: {
      prepare: async () => ({
        workingDirectory: "/tmp/unused",
        baseRef: "main",
        branch: "gc/test",
      }),
      push: async (checkout) => ({
        branch: checkout.branch,
        headSha: "a".repeat(40),
      }),
    },
    codex: {
      run: async () => ({
        outcome: "finished",
        usage: { inputTokens: 0, outputTokens: 0, reasoningTokens: 0 },
      }),
    },
  }
}
