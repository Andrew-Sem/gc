export type DeviceLoginBootstrap = {
  readonly deviceUrl: string
  readonly userCode: string
}

export type SharedGitHubApp = {
  readonly appId: number
  readonly clientId: string
  readonly clientSecret: string
  readonly privateKey: string
  readonly slug: string
  readonly webhookSecret: string
}

export type GitHubOAuthAuthorization = {
  readonly accessToken: string
  readonly githubUserId: number
}

export type GitHubInstallationInspection = {
  readonly appId: number
  readonly clientId: string
  readonly permissions: Readonly<Record<string, string>>
  readonly suspended: boolean
}

export type GitHubInstallationToken = {
  readonly token: string
  readonly expiresAt: number
}

export type ConvexExternalPorts = {
  readonly now: () => number
  readonly exchangeGitHubOAuthCode?: (input: {
    readonly code: string
    readonly codeVerifier: string
    readonly clientId: string
    readonly clientSecret: string
  }) => Promise<GitHubOAuthAuthorization>
  readonly listGitHubUserInstallations?: (input: {
    readonly accessToken: string
    readonly page: number
  }) => Promise<{
    readonly installationIds: readonly number[]
    readonly hasNext: boolean
  }>
  readonly inspectGitHubAppInstallation?: (input: {
    readonly appId: number
    readonly clientId: string
    readonly installationId: number
    readonly privateKey: string
  }) => Promise<GitHubInstallationInspection>
  readonly listGitHubInstallationRepositories?: (input: {
    readonly appId: number
    readonly installationId: number
    readonly privateKey: string
    readonly page: number
  }) => Promise<{
    readonly repositories: readonly string[]
    readonly hasNext: boolean
  }>
  readonly revokeGitHubUserAuthorization?: (input: {
    readonly clientId: string
    readonly clientSecret: string
    readonly accessToken: string
    readonly githubUserId: number
  }) => Promise<void>
  readonly createGitHubInstallationToken?: (input: {
    readonly appId: number
    readonly installationId: number
    readonly privateKey: string
    readonly repo?: string
  }) => Promise<GitHubInstallationToken>
  readonly bootstrapCodexDeviceLogin?: (input: {
    readonly tenant: string
    readonly codexHome: string
  }) => Promise<DeviceLoginBootstrap>
  readonly recordControlPlaneOperation?: (input: {
    readonly operation: string
    readonly taskId?: string
    readonly leaseToken?: string
    readonly command?: unknown
    readonly deliveryId?: string
    readonly tenant?: string
    readonly repo?: string
    readonly appId?: number
    readonly installationId?: number
    readonly githubUserId?: number
    readonly oauthCode?: string
    readonly codeVerifier?: string
    readonly repositoryPage?: number
    readonly githubCredentialKind?: "installation_token"
    readonly githubCredentialExpiresAt?: number
    readonly sharedPrivateKeyPresent?: boolean
  }) => void
  /** Legacy leaf port retained only for older probes; shared-App setup does not call it. */
  readonly discoverInstallationRepositories?: (input: {
    readonly tenant: string
    readonly appId: number
    readonly installationId: number
    readonly privateKey: string
  }) => Promise<readonly string[]>
}

type RuntimeConfiguration = {
  readonly masterKey: string
  readonly workerToken: string
  readonly codexHomeRoot: string
  readonly publicWebUrl?: string
  readonly sharedGitHubApp?: SharedGitHubApp
  readonly ports: ConvexExternalPorts
}

let configured: RuntimeConfiguration | null = null

function requiredEnvironment(name: string): string {
  const value = process.env[name]
  if (!value) throw new Error(`${name} is required`)
  return value
}

export function configureConvexRuntime(
  configuration: RuntimeConfiguration | null
): void {
  configured = configuration
}

export function runtimeConfiguration(): RuntimeConfiguration {
  if (configured) return configured
  return {
    masterKey: requiredEnvironment("SECRET_MASTER_KEY"),
    workerToken: requiredEnvironment("WORKER_TOKEN"),
    codexHomeRoot: requiredEnvironment("CODEX_HOME_ROOT"),
    ports: { now: Date.now },
  }
}

export function sharedGitHubApp(): SharedGitHubApp {
  const configuredApp = runtimeConfiguration().sharedGitHubApp
  if (configuredApp) return configuredApp
  const appId = Number(requiredEnvironment("GITHUB_APP_ID"))
  if (!Number.isSafeInteger(appId) || appId <= 0) {
    throw new Error("GITHUB_APP_ID is invalid")
  }
  return {
    appId,
    clientId: requiredEnvironment("GITHUB_APP_CLIENT_ID"),
    clientSecret: requiredEnvironment("GITHUB_APP_CLIENT_SECRET"),
    privateKey: requiredEnvironment("GITHUB_APP_PRIVATE_KEY"),
    slug: requiredEnvironment("GITHUB_APP_SLUG"),
    webhookSecret: requiredEnvironment("GITHUB_APP_WEBHOOK_SECRET"),
  }
}

export function tenantCodexHome(tenant: string): string {
  return `${runtimeConfiguration().codexHomeRoot}/tenant-${encodeURIComponent(tenant)}`
}

export function publicWebUrl(): string {
  return (
    runtimeConfiguration().publicWebUrl ?? requiredEnvironment("PUBLIC_WEB_URL")
  )
}

const GITHUB_API = "https://api.github.com"
const GITHUB_API_VERSION = "2022-11-28"
const encoder = new TextEncoder()

export async function exchangeGitHubOAuthCode(input: {
  readonly code: string
  readonly codeVerifier: string
  readonly clientId: string
  readonly clientSecret: string
}): Promise<GitHubOAuthAuthorization> {
  const response = await fetch("https://github.com/login/oauth/access_token", {
    method: "POST",
    headers: {
      accept: "application/json",
      "content-type": "application/json",
      "user-agent": "garbage-collector-control-plane",
    },
    body: JSON.stringify({
      client_id: input.clientId,
      client_secret: input.clientSecret,
      code: input.code,
      code_verifier: input.codeVerifier,
    }),
  })
  const value = await responseJson(response, "GitHub OAuth exchange")
  if (!isRecord(value) || typeof value.access_token !== "string") {
    throw new Error("GitHub OAuth exchange response is invalid")
  }
  const user = await githubRequest(value.access_token, "/user")
  if (
    !isRecord(user) ||
    !Number.isSafeInteger(user.id) ||
    Number(user.id) <= 0
  ) {
    throw new Error("GitHub OAuth user response is invalid")
  }
  return { accessToken: value.access_token, githubUserId: Number(user.id) }
}

export async function listGitHubUserInstallations(input: {
  readonly accessToken: string
  readonly page: number
}): Promise<{ readonly installationIds: number[]; readonly hasNext: boolean }> {
  const value = await githubRequest(
    input.accessToken,
    `/user/installations?per_page=100&page=${input.page}`
  )
  if (!isRecord(value) || !Array.isArray(value.installations)) {
    throw new Error("GitHub user installations response is invalid")
  }
  const installationIds = value.installations.map((installation) => {
    if (
      !isRecord(installation) ||
      !Number.isSafeInteger(installation.id) ||
      Number(installation.id) <= 0
    ) {
      throw new Error("GitHub user installation is invalid")
    }
    return Number(installation.id)
  })
  return {
    installationIds,
    hasNext:
      installationIds.length === 100 &&
      (typeof value.total_count !== "number" ||
        input.page * 100 < value.total_count),
  }
}

export async function inspectGitHubAppInstallation(input: {
  readonly appId: number
  readonly clientId: string
  readonly installationId: number
  readonly privateKey: string
}): Promise<GitHubInstallationInspection> {
  const jwt = await githubAppJwt(input.appId, input.privateKey)
  const [app, installation] = await Promise.all([
    githubRequest(jwt, "/app"),
    githubRequest(jwt, `/app/installations/${input.installationId}`),
  ])
  if (
    !isRecord(app) ||
    app.id !== input.appId ||
    app.client_id !== input.clientId
  ) {
    throw Object.assign(
      new Error("GitHub installation belongs to another App"),
      {
        status: 403,
      }
    )
  }
  if (!isRecord(installation) || installation.app_id !== input.appId) {
    throw Object.assign(
      new Error("GitHub installation belongs to another App"),
      {
        status: 403,
      }
    )
  }
  const permissions = isRecord(installation.permissions)
    ? Object.fromEntries(
        Object.entries(installation.permissions).filter(
          (entry): entry is [string, string] => typeof entry[1] === "string"
        )
      )
    : {}
  return {
    appId: input.appId,
    clientId: input.clientId,
    permissions,
    suspended:
      installation.suspended_at !== null &&
      installation.suspended_at !== undefined,
  }
}

export async function listGitHubInstallationRepositories(input: {
  readonly appId: number
  readonly installationId: number
  readonly privateKey: string
  readonly page: number
}): Promise<{ readonly repositories: string[]; readonly hasNext: boolean }> {
  const token = await createGitHubInstallationToken(input)
  const value = await githubRequest(
    token.token,
    `/installation/repositories?per_page=100&page=${input.page}`
  )
  if (!isRecord(value) || !Array.isArray(value.repositories)) {
    throw new Error("GitHub installation repositories response is invalid")
  }
  const repositories = value.repositories.map((repository) => {
    if (!isRecord(repository) || typeof repository.full_name !== "string") {
      throw new Error("GitHub installation repository is invalid")
    }
    return repository.full_name
  })
  return {
    repositories,
    hasNext:
      repositories.length === 100 &&
      (typeof value.total_count !== "number" ||
        input.page * 100 < value.total_count),
  }
}

export async function createGitHubInstallationToken(input: {
  readonly appId: number
  readonly installationId: number
  readonly privateKey: string
  readonly repo?: string
}): Promise<GitHubInstallationToken> {
  const body = input.repo
    ? JSON.stringify({
        repositories: [input.repo.split("/").at(-1)],
        permissions: {
          administration: "read",
          checks: "read",
          contents: "write",
          pull_requests: "write",
          metadata: "read",
        },
      })
    : undefined
  const value = await githubRequest(
    await githubAppJwt(input.appId, input.privateKey),
    `/app/installations/${input.installationId}/access_tokens`,
    { method: "POST", ...(body ? { body } : {}) }
  )
  if (!isRecord(value) || typeof value.token !== "string") {
    throw new Error("GitHub installation token response is invalid")
  }
  return {
    token: value.token,
    expiresAt:
      typeof value.expires_at === "string"
        ? Date.parse(value.expires_at)
        : Date.now() + 55 * 60_000,
  }
}

export async function revokeGitHubUserAuthorization(input: {
  readonly clientId: string
  readonly clientSecret: string
  readonly accessToken: string
}): Promise<void> {
  const response = await fetch(
    `${GITHUB_API}/applications/${input.clientId}/grant`,
    {
      method: "DELETE",
      headers: {
        accept: "application/vnd.github+json",
        authorization: `Basic ${btoa(`${input.clientId}:${input.clientSecret}`)}`,
        "content-type": "application/json",
        "user-agent": "garbage-collector-control-plane",
        "x-github-api-version": GITHUB_API_VERSION,
      },
      body: JSON.stringify({ access_token: input.accessToken }),
    }
  )
  if (!response.ok && response.status !== 404) {
    throw Object.assign(
      new Error(`GitHub authorization revoke failed with ${response.status}`),
      {
        status: response.status,
      }
    )
  }
}

async function githubAppJwt(
  appId: number,
  privateKey: string
): Promise<string> {
  const now = Math.floor(Date.now() / 1000)
  const header = encodeBase64Url(
    encoder.encode(JSON.stringify({ alg: "RS256", typ: "JWT" }))
  )
  const payload = encodeBase64Url(
    encoder.encode(
      JSON.stringify({ iat: now - 60, exp: now + 9 * 60, iss: appId })
    )
  )
  const unsigned = `${header}.${payload}`
  const key = await crypto.subtle.importKey(
    "pkcs8",
    privateKeyDer(privateKey),
    { name: "RSASSA-PKCS1-v1_5", hash: "SHA-256" },
    false,
    ["sign"]
  )
  const signature = new Uint8Array(
    await crypto.subtle.sign("RSASSA-PKCS1-v1_5", key, encoder.encode(unsigned))
  )
  return `${unsigned}.${encodeBase64Url(signature)}`
}

async function githubRequest(
  token: string,
  path: string,
  init: RequestInit = {}
): Promise<unknown> {
  const response = await fetch(`${GITHUB_API}${path}`, {
    ...init,
    headers: {
      accept: "application/vnd.github+json",
      authorization: `Bearer ${token}`,
      "content-type": "application/json",
      "user-agent": "garbage-collector-control-plane",
      "x-github-api-version": GITHUB_API_VERSION,
      ...(init.headers ?? {}),
    },
  })
  return responseJson(response, "GitHub request")
}

async function responseJson(
  response: Response,
  operation: string
): Promise<unknown> {
  if (!response.ok) {
    let message = `${operation} failed with ${response.status}`
    try {
      const body = (await response.json()) as unknown
      if (isRecord(body) && typeof body.message === "string")
        message = body.message
    } catch {
      // Preserve the HTTP status when an intermediary returns non-JSON.
    }
    throw Object.assign(new Error(message), { status: response.status })
  }
  return response.status === 204 ? {} : response.json()
}

function privateKeyDer(pem: string): ArrayBuffer {
  const normalized = pem.replaceAll("\\n", "\n").trim()
  const pkcs8 = pemBody(normalized, "PRIVATE KEY")
  if (pkcs8) return pkcs8.buffer
  const pkcs1 = pemBody(normalized, "RSA PRIVATE KEY")
  if (!pkcs1)
    throw new Error("GitHub private key must be a PEM RSA private key")
  const rsaAlgorithm = Uint8Array.from([
    0x30, 0x0d, 0x06, 0x09, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0x0d, 0x01, 0x01,
    0x01, 0x05, 0x00,
  ])
  return derSequence(
    concatBytes(
      Uint8Array.from([0x02, 0x01, 0x00]),
      rsaAlgorithm,
      derValue(0x04, pkcs1)
    )
  ).buffer
}

function pemBody(pem: string, label: string): Uint8Array<ArrayBuffer> | null {
  const begin = `-----BEGIN ${label}-----`
  const end = `-----END ${label}-----`
  if (!pem.includes(begin) || !pem.includes(end)) return null
  const encoded = pem
    .slice(pem.indexOf(begin) + begin.length, pem.indexOf(end))
    .replaceAll(/\s/g, "")
  if (!encoded) throw new Error("GitHub private key is empty")
  try {
    return Uint8Array.from(atob(encoded), (character) =>
      character.charCodeAt(0)
    )
  } catch {
    throw new Error("GitHub private key is not valid base64")
  }
}

function derSequence(value: Uint8Array): Uint8Array<ArrayBuffer> {
  return derValue(0x30, value)
}

function derValue(tag: number, value: Uint8Array): Uint8Array<ArrayBuffer> {
  return concatBytes(Uint8Array.from([tag]), derLength(value.length), value)
}

function derLength(length: number): Uint8Array<ArrayBuffer> {
  if (length < 0x80) return Uint8Array.from([length])
  const bytes: number[] = []
  for (let value = length; value > 0; value >>= 8) bytes.unshift(value & 0xff)
  return Uint8Array.from([0x80 | bytes.length, ...bytes])
}

function concatBytes(
  ...values: readonly Uint8Array[]
): Uint8Array<ArrayBuffer> {
  const result = new Uint8Array(
    values.reduce((total, value) => total + value.length, 0)
  )
  let offset = 0
  for (const value of values) {
    result.set(value, offset)
    offset += value.length
  }
  return result
}

function encodeBase64Url(value: Uint8Array): string {
  let binary = ""
  for (const byte of value) binary += String.fromCharCode(byte)
  return btoa(binary)
    .replaceAll("+", "-")
    .replaceAll("/", "_")
    .replace(/=+$/, "")
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null
}
