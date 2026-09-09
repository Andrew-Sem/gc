import { spawn } from "node:child_process"
import { access, mkdir } from "node:fs/promises"
import { resolve } from "node:path"
import { Codex } from "@openai/codex-sdk"
import { codexOptions } from "@workspace/domain"
import { acquireAgentLock } from "./agent-lock"
import type { WorkerRuntime } from "./index"

const CODEX_LOGIN_TIMEOUT_MS = 15 * 60_000
const CODEX_ENV_ALLOWLIST = [
  "ALL_PROXY",
  "HOME",
  "HTTPS_PROXY",
  "HTTP_PROXY",
  "LANG",
  "LC_ALL",
  "LC_CTYPE",
  "NODE_EXTRA_CA_CERTS",
  "NO_PROXY",
  "PATH",
  "SSL_CERT_DIR",
  "SSL_CERT_FILE",
  "TEMP",
  "TMP",
  "TMPDIR",
  "all_proxy",
  "https_proxy",
  "http_proxy",
  "no_proxy",
] as const

export function createCodexPort(): WorkerRuntime["codex"] {
  return {
    async run(input) {
      const codexHome = requiredCodexHome(input.credentials.codexHome)
      await mkdir(codexHome, { recursive: true })
      const release = await acquireAgentLock(
        codexHome,
        input.repo,
        input.signal
      )
      try {
        const env = codexEnvironment(codexHome)
        const codex = new Codex({
          apiKey: input.credentials.apiKey ?? undefined,
          baseUrl: input.credentials.baseUrl,
          env,
        })
        const options = codexOptions({
          model: input.settings.model,
          reasoning: input.settings.reasoning,
          checkoutPath: input.workingDirectory,
        })
        const turn = await codex
          .startThread(options)
          .run(input.settings.prompt, { signal: input.signal })
        return {
          outcome: "finished",
          usage: {
            inputTokens: turn.usage?.input_tokens ?? 0,
            outputTokens: turn.usage?.output_tokens ?? 0,
            reasoningTokens: turn.usage?.reasoning_output_tokens ?? 0,
          },
        }
      } finally {
        await release()
      }
    },
    async startDeviceLogin(input) {
      await mkdir(input.codexHome, { recursive: true })
      const authFile = resolve(input.codexHome, "auth.json")
      if (await exists(authFile)) {
        await input.onUpdate({ status: "connected" })
        return "connected"
      }

      const child = spawn("codex", ["login", "--device-auth"], {
        env: codexEnvironment(input.codexHome),
        signal: input.signal,
        stdio: ["ignore", "pipe", "pipe"],
      })
      const timeout = setTimeout(
        () => child.kill("SIGTERM"),
        CODEX_LOGIN_TIMEOUT_MS
      )
      timeout.unref?.()
      let output = ""
      let pendingReported = false
      let updateChain = Promise.resolve()
      const read = async (chunk: Buffer) => {
        output += chunk.toString("utf8")
        if (pendingReported) return
        const login = parseDeviceLogin(output)
        if (!login) return
        pendingReported = true
        await input.onUpdate({ status: "pending", ...login })
      }
      child.stdout.on("data", (chunk: Buffer) => {
        updateChain = updateChain.then(() => read(chunk))
      })
      child.stderr.on("data", (chunk: Buffer) => {
        updateChain = updateChain.then(() => read(chunk))
      })

      const exit = await new Promise<{
        readonly code: number | null
        readonly signal: NodeJS.Signals | null
      }>((resolveExit, reject) => {
        child.once("error", reject)
        child.once("exit", (code, signal) => resolveExit({ code, signal }))
      }).catch(async (error) => {
        if (input.signal?.aborted) throw abortReason(input.signal, error)
        await input.onUpdate({ status: "failed" })
        return null
      })
      clearTimeout(timeout)
      if (!exit) return "failed"
      await updateChain
      if (exit.code === 0 && (await exists(authFile))) {
        await input.onUpdate({ status: "connected" })
        return "connected"
      }
      const status =
        /expired|timed out|timeout/i.test(stripAnsi(output)) ||
        exit.signal === "SIGTERM"
          ? "expired"
          : "failed"
      await input.onUpdate({ status })
      return status
    },
  }
}

export function codexSubprocessEnvironment(
  codexHome: string,
  environment: NodeJS.ProcessEnv = process.env
): Record<string, string> {
  const env: Record<string, string> = {}
  for (const name of CODEX_ENV_ALLOWLIST) {
    const value = environment[name]
    if (value !== undefined) env[name] = value
  }
  env.CODEX_HOME = codexHome
  return env
}

const codexEnvironment = codexSubprocessEnvironment

function requiredCodexHome(value: string | undefined): string {
  if (!value) throw new Error("tenant CODEX_HOME is required")
  return value
}

async function exists(path: string): Promise<boolean> {
  try {
    await access(path)
    return true
  } catch {
    return false
  }
}

export function parseDeviceLogin(
  output: string
): { readonly deviceUrl: string; readonly userCode: string } | null {
  const plain = stripAnsi(output)
  const deviceUrl = plain.match(/https:\/\/[^\s]+\/codex\/device/i)?.[0]
  const userCode = plain.match(/\b[A-Z0-9]{4,6}-[A-Z0-9]{4,6}\b/)?.[0]
  return deviceUrl && userCode ? { deviceUrl, userCode } : null
}

function stripAnsi(value: string): string {
  const escapeCharacter = String.fromCharCode(27)
  return value
    .split(escapeCharacter)
    .map((part, index) =>
      index === 0 ? part : part.replace(/\[[0-?]*[ -/]*[@-~]/, "")
    )
    .join("")
}

function abortReason(signal: AbortSignal, fallback: unknown): Error {
  if (signal.reason instanceof Error) return signal.reason
  return fallback instanceof Error ? fallback : new Error("Codex run aborted")
}
