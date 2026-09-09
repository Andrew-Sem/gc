import { execFile } from "node:child_process"
import { lstat, mkdir, mkdtemp, writeFile } from "node:fs/promises"
import {
  basename,
  dirname,
  isAbsolute,
  relative,
  resolve,
  sep,
} from "node:path"
import { promisify } from "node:util"

const exec = promisify(execFile)
const handoffOutputMarker = "\0gc-git-handoff\0"
const handoffScript = `
set -eu
branch=$1
change_path=$2
commit_message=$3

git switch --create "$branch"
git add -- "$change_path"
git \
  -c user.name="Garbage Collector" \
  -c user.email=gc@localhost \
  -c commit.gpgSign=false \
  -c core.hooksPath=/dev/null \
  commit --message "$commit_message"
head_sha=$(git rev-parse HEAD)
git push --set-upstream origin "$branch"
printf '\\0gc-git-handoff\\0%s\\0' "$head_sha"
git ls-remote --heads origin
`

export type RealGitHandoffInput = {
  readonly remoteUrl: string
  readonly workspaceRoot: string
  readonly baseBranch: string
  readonly branch: string
  readonly change: { readonly path: string; readonly content: string }
  readonly commitMessage: string
}

export type RealGitHandoffResult = {
  readonly workingDirectory: string
  readonly branch: string
  readonly headSha: string
  readonly remoteBranches: readonly string[]
  readonly commitMessage: string
}

export async function runRealGitHandoff(
  input: RealGitHandoffInput
): Promise<RealGitHandoffResult> {
  requireValue("remoteUrl", input.remoteUrl)
  requireValue("baseBranch", input.baseBranch)
  requireValue("branch", input.branch)
  requireValue("commitMessage", input.commitMessage)
  if (!isAbsolute(input.workspaceRoot)) {
    throw new Error("workspaceRoot must be an absolute path")
  }

  await mkdir(input.workspaceRoot, { recursive: true })
  const workingDirectory = await mkdtemp(
    resolve(input.workspaceRoot, "gc-workspace-")
  )
  validateBranch("baseBranch", input.baseBranch)
  validateBranch("branch", input.branch)
  await git(undefined, [
    "clone",
    "--single-branch",
    "--branch",
    input.baseBranch,
    "--",
    input.remoteUrl,
    workingDirectory,
  ])

  const changePath = await safeChangePath(workingDirectory, input.change.path)
  await mkdir(dirname(changePath), { recursive: true })
  await writeFile(changePath, input.change.content, "utf8")
  const handoffOutput = await runHandoffCommands(workingDirectory, [
    input.branch,
    input.change.path,
    input.commitMessage,
  ])
  const resultStart = handoffOutput.lastIndexOf(handoffOutputMarker)
  const headEnd = handoffOutput.indexOf(
    "\0",
    resultStart + handoffOutputMarker.length
  )
  if (resultStart < 0 || headEnd < 0) {
    throw new Error("git handoff returned an invalid result")
  }
  const headSha = handoffOutput.slice(
    resultStart + handoffOutputMarker.length,
    headEnd
  )
  if (!/^[0-9a-f]{40,64}$/.test(headSha)) {
    throw new Error("git handoff returned an invalid head SHA")
  }
  const remoteRefs = handoffOutput.slice(headEnd + 1)
  const remoteBranches = remoteRefs
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => line.split("\t")[1])
    .filter((ref): ref is string => ref?.startsWith("refs/heads/") === true)
    .map((ref) => ref.slice("refs/heads/".length))
    .sort()

  return {
    workingDirectory,
    branch: input.branch,
    headSha,
    remoteBranches,
    commitMessage: input.commitMessage,
  }
}

function requireValue(name: string, value: string): void {
  if (!value.trim()) throw new Error(`${name} is required`)
  if (value.includes("\0")) throw new Error(`${name} is invalid`)
}

function validateBranch(name: string, branch: string): void {
  if (
    branch === "@" ||
    branch.startsWith("-") ||
    branch.startsWith("/") ||
    branch.endsWith("/") ||
    branch.endsWith(".") ||
    branch.includes("..") ||
    branch.includes("@{") ||
    branch.includes("//") ||
    hasInvalidRefCharacter(branch) ||
    branch
      .split("/")
      .some((part) => part.startsWith(".") || part.endsWith(".lock"))
  ) {
    throw new Error(`${name} is not a valid git branch`)
  }
}

function hasInvalidRefCharacter(value: string): boolean {
  const forbidden = "~^:?*[\\"
  return Array.from(value).some(
    (character) =>
      character.charCodeAt(0) <= 32 ||
      character.charCodeAt(0) === 127 ||
      forbidden.includes(character)
  )
}

async function safeChangePath(
  root: string,
  requestedPath: string
): Promise<string> {
  requireValue("change.path", requestedPath)
  if (isAbsolute(requestedPath)) {
    throw new Error("change.path must be relative to the workspace")
  }
  const parts = requestedPath.split(/[\\/]+/)
  if (parts.includes(".git")) {
    throw new Error("change.path cannot modify git metadata")
  }

  const target = resolve(root, requestedPath)
  const fromRoot = relative(root, target)
  if (
    fromRoot === ".." ||
    fromRoot.startsWith(`..${sep}`) ||
    isAbsolute(fromRoot)
  ) {
    throw new Error("change.path must stay within the workspace")
  }

  let current = root
  for (const part of parts.slice(0, -1)) {
    if (!part || part === ".") continue
    current = resolve(current, part)
    try {
      if ((await lstat(current)).isSymbolicLink()) {
        throw new Error("change.path cannot traverse symbolic links")
      }
    } catch (error) {
      if (isMissingFile(error)) break
      throw error
    }
  }
  try {
    if ((await lstat(target)).isSymbolicLink()) {
      throw new Error("change.path cannot be a symbolic link")
    }
  } catch (error) {
    if (!isMissingFile(error)) throw error
  }
  if (!basename(target)) throw new Error("change.path must name a file")
  return target
}

function isMissingFile(error: unknown): boolean {
  return (
    typeof error === "object" &&
    error !== null &&
    "code" in error &&
    error.code === "ENOENT"
  )
}

async function git(
  workingDirectory: string | undefined,
  args: string[]
): Promise<string> {
  try {
    const { stdout } = await exec("git", args, {
      cwd: workingDirectory,
      encoding: "utf8",
      env: {
        ...process.env,
        GCM_INTERACTIVE: "Never",
        GIT_TERMINAL_PROMPT: "0",
      },
      maxBuffer: 10 * 1024 * 1024,
    })
    return stdout
  } catch (error) {
    const message =
      typeof error === "object" &&
      error !== null &&
      "stderr" in error &&
      typeof error.stderr === "string"
        ? error.stderr.trim().split("\n").at(-1)
        : undefined
    throw new Error(
      `git ${args[0] ?? "command"} failed${message ? `: ${message}` : ""}`
    )
  }
}

async function runHandoffCommands(
  workingDirectory: string,
  args: readonly [branch: string, changePath: string, commitMessage: string]
): Promise<string> {
  try {
    const { stdout } = await exec(
      "/bin/sh",
      ["-c", handoffScript, "git-handoff", ...args],
      {
        cwd: workingDirectory,
        encoding: "utf8",
        env: {
          ...process.env,
          GCM_INTERACTIVE: "Never",
          GIT_TERMINAL_PROMPT: "0",
        },
        maxBuffer: 10 * 1024 * 1024,
      }
    )
    return stdout
  } catch (error) {
    const message =
      typeof error === "object" &&
      error !== null &&
      "stderr" in error &&
      typeof error.stderr === "string"
        ? error.stderr.trim().split("\n").at(-1)
        : undefined
    throw new Error(`git handoff failed${message ? `: ${message}` : ""}`)
  }
}
