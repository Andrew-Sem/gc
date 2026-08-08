import { parseRuntimeEnv } from "@workspace/convex/runtime-env"
import { expect, test } from "vitest"

const valid = {
  PUBLIC_WEB_URL: "https://gc.example.com",
  PUBLIC_CONVEX_URL: "https://convex.example.com",
  INTERNAL_CONVEX_URL: "http://convex:3210",
  BETTER_AUTH_SECRET: "better-auth-secret-with-enough-entropy",
  SECRET_MASTER_KEY: "master-key-with-enough-entropy",
  WORKER_TOKEN: "worker-token-with-enough-entropy",
  BETTER_AUTH_ALLOWED_ORIGIN: "https://gc.example.com",
  CONVEX_DATA_PATH: "/data/convex",
  CODEX_HOME_ROOT: "/data/codex-home",
  WORKSPACE_ROOT: "/workspaces",
  SMOKE_SUBSCRIPTION_TIMEOUT_SECONDS: "600",
}

test("runtime env нормализуется и принимает только абсолютные URLs", () => {
  expect(parseRuntimeEnv(valid)).toEqual({
    publicWebUrl: valid.PUBLIC_WEB_URL,
    publicConvexUrl: valid.PUBLIC_CONVEX_URL,
    internalConvexUrl: valid.INTERNAL_CONVEX_URL,
    betterAuthSecret: valid.BETTER_AUTH_SECRET,
    secretMasterKey: valid.SECRET_MASTER_KEY,
    workerToken: valid.WORKER_TOKEN,
    betterAuthAllowedOrigin: valid.BETTER_AUTH_ALLOWED_ORIGIN,
    convexDataPath: valid.CONVEX_DATA_PATH,
    codexHomeRoot: valid.CODEX_HOME_ROOT,
    workspaceRoot: valid.WORKSPACE_ROOT,
    smokeSubscriptionTimeoutSeconds: 600,
  })
})

test.each(Object.keys(valid))(
  "отсутствующий %s останавливает startup",
  (name) => {
    const values = { ...valid, [name]: undefined }
    expect(() => parseRuntimeEnv(values)).toThrow(name)
  }
)

test("ошибка env не печатает значения соседних secrets", () => {
  expect(() =>
    parseRuntimeEnv({ ...valid, PUBLIC_WEB_URL: "not-a-url" })
  ).toThrowError(expect.not.stringContaining(valid.SECRET_MASTER_KEY))
})
