import { mkdir, open, rm, stat, utimes } from "node:fs/promises"
import { resolve } from "node:path"

const processLocks = new Map<string, Promise<void>>()
const LOCK_HEARTBEAT_MS = 20_000
const LOCK_STALE_MS = 60_000

export async function acquireAgentLock(
  codexHome: string | undefined,
  repo: string,
  signal?: AbortSignal
): Promise<() => Promise<void>> {
  const key = `${codexHome ?? ""}\u0000${repo}`
  const previous = processLocks.get(key) ?? Promise.resolve()
  let releaseProcess: () => void = () => undefined
  const current = new Promise<void>((resolveCurrent) => {
    releaseProcess = resolveCurrent
  })
  const queued = previous.then(() => current)
  processLocks.set(key, queued)
  try {
    await abortable(previous, signal)
  } catch (error) {
    releaseProcess()
    void queued.then(() => {
      if (processLocks.get(key) === queued) processLocks.delete(key)
    })
    throw error
  }

  const releaseFile = codexHome
    ? await acquireFileLock(codexHome, repo, signal)
    : async () => undefined
  return async () => {
    await releaseFile()
    releaseProcess()
    if (processLocks.get(key) === queued) processLocks.delete(key)
  }
}

async function acquireFileLock(
  codexHome: string,
  repo: string,
  signal?: AbortSignal
): Promise<() => Promise<void>> {
  const root = resolve(codexHome, "agent-locks")
  await mkdir(root, { recursive: true })
  const path = resolve(root, `${Buffer.from(repo).toString("base64url")}.lock`)
  for (;;) {
    signal?.throwIfAborted()
    try {
      const handle = await open(path, "wx")
      await handle.writeFile(`${process.pid}\n`, "utf8")
      await handle.close()
      const heartbeat = setInterval(() => {
        const now = new Date()
        void utimes(path, now, now).catch(() => undefined)
      }, LOCK_HEARTBEAT_MS)
      heartbeat.unref?.()
      return async () => {
        clearInterval(heartbeat)
        await rm(path, { force: true })
      }
    } catch (error) {
      if (!isExists(error)) throw error
      if (await isStale(path)) {
        await rm(path, { force: true })
        continue
      }
      await abortable(
        new Promise((resolveWait) => setTimeout(resolveWait, 100)),
        signal
      )
    }
  }
}

async function isStale(path: string): Promise<boolean> {
  try {
    return Date.now() - (await stat(path)).mtimeMs >= LOCK_STALE_MS
  } catch {
    return false
  }
}

function isExists(error: unknown): boolean {
  return (
    typeof error === "object" &&
    error !== null &&
    "code" in error &&
    error.code === "EEXIST"
  )
}

async function abortable<T>(
  promise: Promise<T>,
  signal?: AbortSignal
): Promise<T> {
  if (!signal) return promise
  signal.throwIfAborted()
  let rejectAbort: (reason?: unknown) => void = () => undefined
  const aborted = new Promise<never>((_resolve, reject) => {
    rejectAbort = reject
  })
  const onAbort = () => rejectAbort(signal.reason)
  signal.addEventListener("abort", onAbort, { once: true })
  try {
    return await Promise.race([promise, aborted])
  } finally {
    signal.removeEventListener("abort", onAbort)
  }
}
