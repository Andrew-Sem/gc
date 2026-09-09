import { isAbsolute } from "node:path"

export type RuntimeEnv = {
  readonly publicWebUrl: string
  readonly publicConvexUrl: string
  readonly internalConvexUrl: string
  readonly betterAuthSecret: string
  readonly secretMasterKey: string
  readonly workerToken: string
  readonly betterAuthAllowedOrigin: string
  readonly convexDataPath: string
  readonly codexHomeRoot: string
  readonly workspaceRoot: string
  readonly githubAppId: number
  readonly githubAppClientId: string
  readonly githubAppClientSecret: string
  readonly githubAppPrivateKey: string
  readonly githubAppSlug: string
  readonly githubAppWebhookSecret: string
  readonly smokeSubscriptionTimeoutSeconds: number
}

const REQUIRED_SECRET_LENGTH = 24
const MAX_SMOKE_TIMEOUT_SECONDS = 24 * 60 * 60

function required(
  values: Readonly<Record<string, string | undefined>>,
  name: string,
  errors: string[]
): string {
  const value = values[name]?.trim()
  if (!value) errors.push(`${name}: required`)
  return value ?? ""
}

function absoluteHttpUrl(
  name: string,
  value: string,
  errors: string[],
  originOnly = false
): string {
  if (!value) return value
  try {
    const url = new URL(value)
    if (url.protocol !== "http:" && url.protocol !== "https:") {
      errors.push(`${name}: must use http or https`)
    }
    if (!url.hostname) errors.push(`${name}: must be an absolute URL`)
    if (url.username || url.password) {
      errors.push(`${name}: must not contain credentials`)
    }
    if (
      originOnly &&
      (url.pathname !== "/" || url.search !== "" || url.hash !== "")
    ) {
      errors.push(
        `${name}: must be an origin without a path, query, or fragment`
      )
    }
  } catch {
    errors.push(`${name}: must be an absolute URL`)
  }
  return value
}

function secret(name: string, value: string, errors: string[]): string {
  if (value && value.length < REQUIRED_SECRET_LENGTH) {
    errors.push(
      `${name}: must be at least ${REQUIRED_SECRET_LENGTH} characters`
    )
  }
  return value
}

function absolutePath(name: string, value: string, errors: string[]): string {
  if (value && (!isAbsolute(value) || value.includes("\0"))) {
    errors.push(`${name}: must be an absolute path`)
  }
  return value
}

function positiveInteger(
  name: string,
  value: string,
  errors: string[]
): number {
  const parsed = Number(value)
  if (
    value &&
    (!/^\d+$/.test(value) || !Number.isSafeInteger(parsed) || parsed <= 0)
  ) {
    errors.push(`${name}: must be a positive integer`)
  }
  return parsed
}

function githubSlug(value: string, errors: string[]): string {
  if (value && !/^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/.test(value)) {
    errors.push("GITHUB_APP_SLUG: must be a lowercase GitHub App slug")
  }
  return value
}

function githubPrivateKey(value: string, errors: string[]): string {
  const normalized = value.replaceAll("\\n", "\n").trim()
  if (
    value &&
    !/^-----BEGIN (?:RSA )?PRIVATE KEY-----[\s\S]+-----END (?:RSA )?PRIVATE KEY-----$/.test(
      normalized
    )
  ) {
    errors.push("GITHUB_APP_PRIVATE_KEY: must be a PEM private key")
  }
  return value
}

export function parseRuntimeEnv(
  values: Readonly<Record<string, string | undefined>>
): RuntimeEnv {
  const errors: string[] = []
  const get = (name: string) => required(values, name, errors)

  const publicWebUrl = absoluteHttpUrl(
    "PUBLIC_WEB_URL",
    get("PUBLIC_WEB_URL"),
    errors
  )
  const publicConvexUrl = absoluteHttpUrl(
    "PUBLIC_CONVEX_URL",
    get("PUBLIC_CONVEX_URL"),
    errors
  )
  const internalConvexUrl = absoluteHttpUrl(
    "INTERNAL_CONVEX_URL",
    get("INTERNAL_CONVEX_URL"),
    errors
  )
  const betterAuthSecret = secret(
    "BETTER_AUTH_SECRET",
    get("BETTER_AUTH_SECRET"),
    errors
  )
  const secretMasterKey = secret(
    "SECRET_MASTER_KEY",
    get("SECRET_MASTER_KEY"),
    errors
  )
  const workerToken = secret("WORKER_TOKEN", get("WORKER_TOKEN"), errors)
  const betterAuthAllowedOrigin = absoluteHttpUrl(
    "BETTER_AUTH_ALLOWED_ORIGIN",
    get("BETTER_AUTH_ALLOWED_ORIGIN"),
    errors,
    true
  )
  const convexDataPath = absolutePath(
    "CONVEX_DATA_PATH",
    get("CONVEX_DATA_PATH"),
    errors
  )
  const codexHomeRoot = absolutePath(
    "CODEX_HOME_ROOT",
    get("CODEX_HOME_ROOT"),
    errors
  )
  const workspaceRoot = absolutePath(
    "WORKSPACE_ROOT",
    get("WORKSPACE_ROOT"),
    errors
  )
  const githubAppId = positiveInteger(
    "GITHUB_APP_ID",
    get("GITHUB_APP_ID"),
    errors
  )
  const githubAppClientId = get("GITHUB_APP_CLIENT_ID")
  if (githubAppClientId && !/^(?:Iv\d\.|Ov\d\.)\S+$/.test(githubAppClientId)) {
    errors.push("GITHUB_APP_CLIENT_ID: must be a GitHub App client ID")
  }
  const githubAppClientSecret = secret(
    "GITHUB_APP_CLIENT_SECRET",
    get("GITHUB_APP_CLIENT_SECRET"),
    errors
  )
  const githubAppPrivateKey = githubPrivateKey(
    get("GITHUB_APP_PRIVATE_KEY"),
    errors
  )
  const githubAppSlug = githubSlug(get("GITHUB_APP_SLUG"), errors)
  const githubAppWebhookSecret = secret(
    "GITHUB_APP_WEBHOOK_SECRET",
    get("GITHUB_APP_WEBHOOK_SECRET"),
    errors
  )
  const timeoutValue = get("SMOKE_SUBSCRIPTION_TIMEOUT_SECONDS")
  const smokeSubscriptionTimeoutSeconds = Number(timeoutValue)
  if (
    timeoutValue &&
    (!/^\d+$/.test(timeoutValue) ||
      !Number.isInteger(smokeSubscriptionTimeoutSeconds) ||
      smokeSubscriptionTimeoutSeconds <= 0 ||
      smokeSubscriptionTimeoutSeconds > MAX_SMOKE_TIMEOUT_SECONDS)
  ) {
    errors.push(
      `SMOKE_SUBSCRIPTION_TIMEOUT_SECONDS: must be an integer between 1 and ${MAX_SMOKE_TIMEOUT_SECONDS}`
    )
  }

  if (errors.length > 0) {
    throw new Error(`Invalid runtime environment:\n${errors.join("\n")}`)
  }

  return {
    publicWebUrl,
    publicConvexUrl,
    internalConvexUrl,
    betterAuthSecret,
    secretMasterKey,
    workerToken,
    betterAuthAllowedOrigin,
    convexDataPath,
    codexHomeRoot,
    workspaceRoot,
    githubAppId,
    githubAppClientId,
    githubAppClientSecret,
    githubAppPrivateKey,
    githubAppSlug,
    githubAppWebhookSecret,
    smokeSubscriptionTimeoutSeconds,
  }
}
