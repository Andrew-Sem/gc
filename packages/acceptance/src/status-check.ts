import { existsSync, readFileSync } from "node:fs"
import { pathToFileURL } from "node:url"

export type StackReadinessProbe = {
  web(): Promise<boolean>
  convex(): Promise<boolean>
  worker(): Promise<boolean>
}

export async function assertStackReady(
  probe: StackReadinessProbe = defaultReadinessProbe()
): Promise<void> {
  for (const service of ["web", "convex", "worker"] as const) {
    let ready = false
    try {
      ready = await probe[service]()
    } catch {
      ready = false
    }
    if (!ready) throw new Error(`${service} is not ready`)
  }
}

function defaultReadinessProbe(): StackReadinessProbe {
  const webUrl = healthUrl(
    runtimeValue("WEB_HEALTH_URL") ?? runtimeValue("PUBLIC_WEB_URL"),
    `http://localhost:${runtimeValue("WEB_PORT") ?? "4173"}`
  )
  const convexUrl =
    runtimeValue("PUBLIC_CONVEX_URL") ??
    `http://localhost:${runtimeValue("CONVEX_API_PORT") ?? "3211"}`
  const workerUrl = healthUrl(
    runtimeValue("WORKER_HEALTH_URL"),
    `http://localhost:${runtimeValue("WORKER_HEALTH_PORT") ?? "8081"}`
  )
  return {
    web: () => fetchReady(webUrl),
    convex: () => fetchConvexReady(convexUrl),
    worker: () => fetchReady(workerUrl),
  }
}

let fileEnvironment: ReadonlyMap<string, string> | undefined

function runtimeValue(name: string): string | undefined {
  const environmentValue = process.env[name]?.trim()
  if (environmentValue) return environmentValue
  fileEnvironment ??= readEnvironmentFile()
  return fileEnvironment.get(name)
}

function readEnvironmentFile(): ReadonlyMap<string, string> {
  const values = new Map<string, string>()
  if (!existsSync(".env")) return values
  for (const rawLine of readFileSync(".env", "utf8").split("\n")) {
    const line = rawLine.trim()
    if (!line || line.startsWith("#")) continue
    const separator = line.indexOf("=")
    if (separator < 1) continue
    const name = line.slice(0, separator).trim()
    let value = line.slice(separator + 1).trim()
    if (
      value.length >= 2 &&
      ((value.startsWith('"') && value.endsWith('"')) ||
        (value.startsWith("'") && value.endsWith("'")))
    ) {
      value = value.slice(1, -1)
    }
    if (value) values.set(name, value)
  }
  return values
}

function healthUrl(configured: string | undefined, fallback: string): string {
  return new URL("/health/ready", configured?.trim() || fallback).toString()
}

async function fetchReady(url: string): Promise<boolean> {
  const response = await fetch(url, {
    method: "GET",
    redirect: "error",
    signal: AbortSignal.timeout(5_000),
  })
  return response.ok
}

async function fetchConvexReady(url: string): Promise<boolean> {
  const response = await fetch(new URL("/api/query", url), {
    method: "POST",
    headers: {
      "content-type": "application/json",
      "Convex-Client": "garbage-collector-status",
    },
    body: JSON.stringify({
      path: "status:readiness",
      format: "convex_encoded_json",
      args: [{}],
    }),
    redirect: "error",
    signal: AbortSignal.timeout(5_000),
  })
  if (!response.ok) return false
  const body = (await response.json()) as {
    readonly status?: string
    readonly value?: { readonly status?: string }
  }
  return body.status === "success" && body.value?.status === "ready"
}

const entryPath = process.argv[1]
if (entryPath && import.meta.url === pathToFileURL(entryPath).href) {
  try {
    await assertStackReady()
    process.stdout.write("web, convex, and worker are ready\n")
  } catch (error) {
    process.stderr.write(
      `${error instanceof Error ? error.message : "stack is not ready"}\n`
    )
    process.exitCode = 1
  }
}
