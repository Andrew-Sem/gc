import { execFile } from "node:child_process"
import { mkdir, mkdtemp, rm } from "node:fs/promises"
import { resolve } from "node:path"
import { promisify } from "node:util"
import { repositoryCloneInfo } from "./github-port"
import type { WorkerRuntime, WorkspaceCheckout } from "./index"

const exec = promisify(execFile)

export function createWorkspacePort(
  workspaceRoot: string,
  cloneInfo: typeof repositoryCloneInfo = repositoryCloneInfo
): WorkerRuntime["workspace"] {
  return {
    async prepare(tenant, repo, credentials, idempotencyKey, signal) {
      await mkdir(workspaceRoot, { recursive: true })
      const directory = await mkdtemp(resolve(workspaceRoot, "gc-workspace-"))
      const info = await cloneInfo(repo, credentials, signal)
      const branch = stableBranch(tenant, idempotencyKey)
      const authenticated = authenticatedCloneUrl(info.cloneUrl, info.token)
      try {
        await git(
          undefined,
          [
            "clone",
            "--single-branch",
            "--branch",
            info.defaultBranch,
            "--",
            authenticated,
            directory,
          ],
          false,
          signal
        )
        const remoteBranch = await git(
          directory,
          ["ls-remote", "--exit-code", "--heads", "origin", branch],
          true,
          signal
        )
        const remoteBranchHead = remoteBranchSha(remoteBranch)
        if (remoteBranchHead) {
          await git(
            directory,
            [
              "fetch",
              "origin",
              `+refs/heads/${branch}:refs/remotes/origin/${branch}`,
            ],
            false,
            signal
          )
          await git(
            directory,
            ["switch", "--create", branch, `refs/remotes/origin/${branch}`],
            false,
            signal
          )
          await git(
            directory,
            [
              "config",
              "--local",
              "--add",
              "remote.origin.fetch",
              `+refs/heads/${branch}:refs/remotes/origin/${branch}`,
            ],
            false,
            signal
          )
          await git(
            directory,
            ["config", "--local", `branch.${branch}.remote`, "origin"],
            false,
            signal
          )
          await git(
            directory,
            [
              "config",
              "--local",
              `branch.${branch}.merge`,
              `refs/heads/${branch}`,
            ],
            false,
            signal
          )
        } else {
          await git(directory, ["switch", "--create", branch], false, signal)
        }
        await git(
          directory,
          ["remote", "set-url", "origin", info.cloneUrl],
          false,
          signal
        )
        return {
          workingDirectory: directory,
          baseRef: info.defaultBranch,
          branch,
          repo,
          remoteBranchExists: remoteBranchHead !== null,
          remoteBranchHead: remoteBranchHead ?? undefined,
        }
      } catch (error) {
        await rm(directory, { recursive: true, force: true })
        throw error
      }
    },
    async push(checkout, credentials, signal) {
      const status = (
        await requiredGit(
          checkout.workingDirectory,
          ["status", "--porcelain"],
          signal
        )
      ).trim()
      if (status) {
        await git(checkout.workingDirectory, ["add", "--all"], false, signal)
        await git(
          checkout.workingDirectory,
          [
            "-c",
            "user.name=Garbage Collector",
            "-c",
            "user.email=gc@localhost",
            "-c",
            "commit.gpgSign=false",
            "-c",
            "core.hooksPath=/dev/null",
            "commit",
            "--message",
            "chore: automated garbage collection",
          ],
          false,
          signal
        )
      }
      const headSha = (
        await requiredGit(
          checkout.workingDirectory,
          ["rev-parse", "HEAD"],
          signal
        )
      ).trim()
      if (!status && !checkout.remoteBranchExists) {
        return { branch: checkout.branch, headSha, changed: false }
      }
      const repo = checkout.repo ?? (await repositoryFromCheckout(checkout))
      const info = await cloneInfo(repo, credentials, signal)
      const expectedHead = checkout.remoteBranchHead ?? ""
      await git(
        checkout.workingDirectory,
        [
          "push",
          `--force-with-lease=refs/heads/${checkout.branch}:${expectedHead}`,
          authenticatedCloneUrl(info.cloneUrl, info.token),
          `HEAD:refs/heads/${checkout.branch}`,
        ],
        false,
        signal
      )
      return { branch: checkout.branch, headSha, changed: true }
    },
    cleanup: async (checkout) => {
      await rm(checkout.workingDirectory, { recursive: true, force: true })
    },
  }
}

async function repositoryFromCheckout(
  checkout: WorkspaceCheckout
): Promise<string> {
  const origin = (
    await requiredGit(checkout.workingDirectory, [
      "remote",
      "get-url",
      "origin",
    ])
  ).trim()
  const match = origin.match(/github\.com[/:]([^/]+\/.+?)(?:\.git)?$/)
  if (!match?.[1])
    throw new Error("workspace origin is not a GitHub repository")
  return match[1]
}

function stableBranch(tenant: string, key: string): string {
  const safeTenant = tenant.replaceAll(/[^a-zA-Z0-9._-]/g, "-")
  const safeKey = key.replaceAll(/[^a-zA-Z0-9._-]/g, "-").slice(-48)
  return `gc/${safeTenant}/${safeKey}`
}

function authenticatedCloneUrl(cloneUrl: string, token: string): string {
  const url = new URL(cloneUrl)
  if (url.protocol !== "http:" && url.protocol !== "https:") return cloneUrl
  url.username = "x-access-token"
  url.password = token
  return url.toString()
}

async function git(
  cwd: string | undefined,
  args: string[],
  allowFailure = false,
  signal?: AbortSignal
): Promise<string | null> {
  try {
    const { stdout } = await exec("git", args, {
      cwd,
      encoding: "utf8",
      env: {
        ...process.env,
        GCM_INTERACTIVE: "Never",
        GIT_TERMINAL_PROMPT: "0",
      },
      signal,
      maxBuffer: 10 * 1024 * 1024,
    })
    return stdout
  } catch (error) {
    if (signal?.aborted) throw abortReason(signal, error)
    if (allowFailure) return null
    const detail =
      typeof error === "object" &&
      error !== null &&
      "stderr" in error &&
      typeof error.stderr === "string"
        ? error.stderr.trim().split("\n").at(-1)
        : null
    throw new Error(
      `git ${args[0] ?? "command"} failed${detail ? `: ${detail}` : ""}`
    )
  }
}

async function requiredGit(
  cwd: string | undefined,
  args: string[],
  signal?: AbortSignal
): Promise<string> {
  const result = await git(cwd, args, false, signal)
  if (result === null) throw new Error(`git ${args[0] ?? "command"} failed`)
  return result
}

function remoteBranchSha(output: string | null): string | null {
  if (output === null) return null
  const sha = output.trim().split(/\s+/, 1)[0]
  if (!sha || !/^[0-9a-f]{40,64}$/.test(sha)) {
    throw new Error("remote branch returned an invalid head SHA")
  }
  return sha
}

function abortReason(signal: AbortSignal, fallback: unknown): Error {
  if (signal.reason instanceof Error) return signal.reason
  return fallback instanceof Error ? fallback : new Error("git command aborted")
}
