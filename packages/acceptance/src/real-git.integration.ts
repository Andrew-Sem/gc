import { execFile } from "node:child_process"
import { mkdtemp, writeFile } from "node:fs/promises"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { promisify } from "node:util"
import { runRealGitHandoff } from "@workspace/worker/git-handoff"
import { expect, test } from "vitest"

const exec = promisify(execFile)

test("production git adapter создаёт branch, commit и push в bare repository", async () => {
  const root = await mkdtemp(join(tmpdir(), "gc-real-git-"))
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

  const result = await runRealGitHandoff({
    remoteUrl: remote,
    workspaceRoot: join(root, "workspaces"),
    baseBranch: "main",
    branch: "gc/smoke",
    change: {
      path: "obsolete.txt",
      content: "removed by garbage collector\n",
    },
    commitMessage: "chore: remove obsolete code",
  })

  const branch = await exec("git", [
    "--git-dir",
    remote,
    "rev-parse",
    "refs/heads/gc/smoke",
  ])
  expect(branch.stdout.trim()).toBe(result.headSha)
  const message = await exec("git", [
    "--git-dir",
    remote,
    "log",
    "-1",
    "--format=%s",
    "refs/heads/gc/smoke",
  ])
  expect(message.stdout.trim()).toBe("chore: remove obsolete code")
  const content = await exec("git", [
    "--git-dir",
    remote,
    "show",
    "refs/heads/gc/smoke:obsolete.txt",
  ])
  expect(content.stdout).toBe("removed by garbage collector\n")
})
