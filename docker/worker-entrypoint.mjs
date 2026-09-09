import { execFileSync, spawn } from "node:child_process"
import { constants } from "node:fs"
import { access, mkdir } from "node:fs/promises"
import { createServer } from "node:http"
import { isAbsolute } from "node:path"

const REQUIRED_SECRET_LENGTH = 24
const WORKER_PROCESS_ENV_ALLOWLIST = [
  "HOME",
  "LANG",
  "LC_ALL",
  "NODE_EXTRA_CA_CERTS",
  "NODE_ENV",
  "PATH",
  "SSL_CERT_DIR",
  "SSL_CERT_FILE",
  "TEMP",
  "TMP",
  "TMPDIR",
  "WORKER_ID",
]

function fail(error) {
  process.stderr.write(
    `${error instanceof Error ? error.message : "Worker startup failed"}\n`
  )
  process.exit(1)
}

let workerToken
let protocolUrl
let port
let workspaceRoot
try {
  workerToken = requiredSecret("WORKER_TOKEN")
  protocolUrl = absoluteHttpUrl("WORKER_PROTOCOL_URL")
  const codexHomeRoot = absolutePath("CODEX_HOME_ROOT")
  workspaceRoot = absolutePath("WORKSPACE_ROOT")
  port = healthPort()
  execFileSync("git", ["--version"], { stdio: "ignore" })
  execFileSync("codex", ["--version"], { stdio: "ignore" })
  await mkdir(codexHomeRoot, { recursive: true })
  await mkdir(workspaceRoot, { recursive: true })
  await access(codexHomeRoot, constants.R_OK | constants.W_OK)
  await access(workspaceRoot, constants.R_OK | constants.W_OK)
} catch (error) {
  fail(error)
}

function requiredEnvironment(name) {
  const value = process.env[name]?.trim()
  if (!value) throw new Error(`${name}: required`)
  return value
}

function requiredSecret(name) {
  const value = requiredEnvironment(name)
  if (value.length < REQUIRED_SECRET_LENGTH) {
    throw new Error(
      `${name}: must be at least ${REQUIRED_SECRET_LENGTH} characters`
    )
  }
  return value
}

function absoluteHttpUrl(name) {
  const value = requiredEnvironment(name)
  let url
  try {
    url = new URL(value)
  } catch {
    throw new Error(`${name}: must be an absolute HTTP URL`)
  }
  if (
    (url.protocol !== "http:" && url.protocol !== "https:") ||
    !url.hostname ||
    url.username ||
    url.password
  ) {
    throw new Error(`${name}: must be an absolute HTTP URL without credentials`)
  }
  return url.toString()
}

function absolutePath(name) {
  const value = requiredEnvironment(name)
  if (!isAbsolute(value) || value.includes("\0")) {
    throw new Error(`${name}: must be an absolute path`)
  }
  return value
}

function healthPort() {
  const value = environmentValue("WORKER_HEALTH_PORT")?.trim() || "8081"
  const parsed = Number(value)
  if (
    !/^\d+$/.test(value) ||
    !Number.isInteger(parsed) ||
    parsed < 1 ||
    parsed > 65535
  ) {
    throw new Error(
      "WORKER_HEALTH_PORT: must be an integer between 1 and 65535"
    )
  }
  return parsed
}

function environmentValue(name) {
  return process.env[name]
}

function workerEnvironment() {
  const environment = {}
  for (const name of WORKER_PROCESS_ENV_ALLOWLIST) {
    const value = process.env[name]
    if (value !== undefined) environment[name] = value
  }
  environment.WORKER_PROTOCOL_URL = protocolUrl
  environment.WORKER_TOKEN = workerToken
  environment.WORKSPACE_ROOT = workspaceRoot
  return environment
}

const worker = spawn(process.execPath, ["/app/main.js"], {
  env: workerEnvironment(),
  stdio: "inherit",
})

async function claimProtocolIsReady() {
  try {
    const response = await fetch(protocolUrl, {
      method: "POST",
      headers: {
        authorization: `Bearer ${workerToken}`,
        "content-type": "application/json",
      },
      body: JSON.stringify({
        operation: "claim",
        workerId: "readiness-probe",
        leaseMs: 1,
        readiness: true,
      }),
      signal: AbortSignal.timeout(3_000),
    })
    return response.ok && (await response.json()) === null
  } catch {
    return false
  }
}

const server = createServer(async (request, response) => {
  response.setHeader("content-type", "application/json")
  if (request.url === "/health/live") {
    response.statusCode = worker.exitCode === null ? 200 : 503
    response.end(
      JSON.stringify({ status: worker.exitCode === null ? "live" : "stopped" })
    )
    return
  }
  if (request.url === "/health/ready") {
    const ready = worker.exitCode === null && (await claimProtocolIsReady())
    response.statusCode = ready ? 200 : 503
    response.end(JSON.stringify({ status: ready ? "ready" : "not-ready" }))
    return
  }
  response.statusCode = 404
  response.end(JSON.stringify({ error: "not found" }))
})

server.listen(port, "0.0.0.0")

function stop(signal) {
  server.close()
  if (worker.exitCode === null) worker.kill(signal)
}

process.once("SIGINT", () => stop("SIGINT"))
process.once("SIGTERM", () => stop("SIGTERM"))
worker.once("error", fail)
worker.once("exit", (code, signal) => {
  server.close(() => {
    process.exitCode = code ?? (signal ? 1 : 0)
  })
})
