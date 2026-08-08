import { readFileSync } from "node:fs"
import { join } from "node:path"
import { expect, test } from "vitest"
import { parse } from "yaml"

const ROOT = join(import.meta.dirname, "..", "..", "..")

function text(path: string): string {
  return readFileSync(join(ROOT, path), "utf8")
}

test("официальный Compose stack содержит web, convex, worker и persistent volumes", () => {
  const compose = text("compose.yaml")
  const value = parse(compose) as {
    services: Record<
      string,
      { healthcheck?: unknown; volumes?: readonly string[]; restart?: string }
    >
  }
  for (const service of ["web", "convex", "worker"]) {
    expect(value.services[service]?.healthcheck).toBeTruthy()
    expect(value.services[service]?.restart).toBeTruthy()
  }
  expect(value.services.convex?.volumes?.join(" ")).toContain("convex-data")
  expect(value.services.worker?.volumes?.join(" ")).toContain("codex-home")
})

test("env example перечисляет обязательный runtime и release-smoke contract", () => {
  const env = text(".env.example")
  for (const name of [
    "PUBLIC_WEB_URL",
    "PUBLIC_CONVEX_URL",
    "INTERNAL_CONVEX_URL",
    "BETTER_AUTH_SECRET",
    "SECRET_MASTER_KEY",
    "WORKER_TOKEN",
    "BETTER_AUTH_ALLOWED_ORIGIN",
    "CONVEX_DATA_PATH",
    "CODEX_HOME_ROOT",
    "WORKSPACE_ROOT",
    "SMOKE_SUBSCRIPTION_TIMEOUT_SECONDS",
    "SMOKE_OWNER_EMAIL",
    "SMOKE_OWNER_PASSWORD",
    "SMOKE_GITHUB_APP_ID",
    "SMOKE_GITHUB_INSTALLATION_ID",
    "SMOKE_GITHUB_PRIVATE_KEY",
    "SMOKE_GITHUB_WEBHOOK_SECRET",
    "SMOKE_GITHUB_REPOSITORY",
    "SMOKE_GITHUB_REPOSITORY_ID",
    "SMOKE_DISPOSABLE_MARKER",
    "SMOKE_GITHUB_BASE_BRANCH",
    "SMOKE_OPENAI_API_KEY",
  ]) {
    expect(env).toContain(`${name}=`)
  }
  for (const line of env.split("\n")) {
    const [name, value = ""] = line.split("=", 2)
    if (/SECRET|TOKEN|PASSWORD|PRIVATE_KEY|API_KEY/.test(name ?? "")) {
      expect(value).toBe("")
    }
  }
})

test("images имеют health checks, worker запускается не root и содержит git", () => {
  const worker = text("apps/worker/Dockerfile")
  const web = text("apps/web/Dockerfile")
  const convex = text("packages/convex/Dockerfile")
  expect(worker).toMatch(/\bgit\b/)
  expect(worker).toMatch(/\bUSER\s+(?!root\b)/)
  expect(worker).toMatch(/HEALTHCHECK/)
  expect(web).toMatch(/HEALTHCHECK/)
  expect(convex).toMatch(/HEALTHCHECK/)
})

test("root commands управляют stack и запускают оба destructive release smoke", () => {
  const justfile = text("justfile")
  expect(justfile).toMatch(/\nup:/)
  expect(justfile).toMatch(/docker compose[^\n]*up/)
  expect(justfile).toMatch(/\nstatus:/)
  expect(justfile).toContain("status-check.ts")
  expect(justfile).toMatch(/\ndown:/)
  expect(justfile).toMatch(/\nsmoke-release mode:/)
  expect(justfile).toContain("api_key")
  expect(justfile).toContain("subscription")
})

test("deployable packages не маскируют готовность no-op scripts", () => {
  const worker = JSON.parse(text("apps/worker/package.json"))
  const convex = JSON.parse(text("packages/convex/package.json"))
  expect(JSON.stringify(worker.scripts)).not.toContain("node --eval")
  expect(JSON.stringify(convex.scripts)).not.toContain("node --eval")
  expect(worker.scripts.start).toBeTruthy()
  expect(worker.scripts.dev).toBeTruthy()
  expect(convex.scripts.build).toBeTruthy()
})
