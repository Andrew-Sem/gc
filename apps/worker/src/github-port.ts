import type { GitHubCredentials, WorkerRuntime } from "./index"

const API = "https://api.github.com"
const MAX_PAGES = 20

export function createGitHubPort(): WorkerRuntime["github"] {
  return {
    async createPullRequest(input) {
      const { owner, name } = splitRepo(input.repo)
      const token = await installationToken(input.credentials, input.signal)
      const existing = await requestJson<unknown[]>(
        token,
        `/repos/${owner}/${name}/pulls?state=all&head=${encodeURIComponent(`${owner}:${input.headBranch}`)}`,
        { signal: input.signal }
      )
      const pull = existing[0]
      if (pull) return pullRequestResult(pull, input.headBranch, input.headSha)

      const created = await requestJson<unknown>(
        token,
        `/repos/${owner}/${name}/pulls`,
        {
          method: "POST",
          body: JSON.stringify({
            title: input.title,
            body: input.body,
            base: input.baseRef,
            head: input.headBranch,
          }),
          signal: input.signal,
        }
      )
      return pullRequestResult(created, input.headBranch, input.headSha)
    },
    async getPullRequest(input) {
      const { owner, name } = splitRepo(input.repo)
      return requestJson(
        await installationToken(input.credentials, input.signal),
        `/repos/${owner}/${name}/pulls/${input.pr}`,
        { signal: input.signal }
      )
    },
    async getRequiredChecks(input) {
      const { owner, name } = splitRepo(input.repo)
      let value: unknown
      try {
        value = await requestJson<unknown>(
          await installationToken(input.credentials, input.signal),
          `/repos/${owner}/${name}/branches/${encodeURIComponent(input.baseRef)}/protection/required_status_checks`,
          { signal: input.signal }
        )
      } catch (error) {
        if (isStatus(error, 404)) return { items: [], complete: true }
        throw error
      }
      return {
        items:
          isRecord(value) && Array.isArray(value.checks) ? value.checks : [],
        complete: true,
      }
    },
    async listCheckRuns(input) {
      const { owner, name } = splitRepo(input.repo)
      const token = await installationToken(input.credentials, input.signal)
      const items: unknown[] = []
      for (let page = 1; page <= MAX_PAGES; page += 1) {
        const value = await requestJson<unknown>(
          token,
          `/repos/${owner}/${name}/commits/${input.headSha}/check-runs?per_page=100&page=${page}`,
          { signal: input.signal }
        )
        const runs =
          isRecord(value) && Array.isArray(value.check_runs)
            ? value.check_runs
            : []
        items.push(...runs)
        if (runs.length < 100) return { items, complete: true }
      }
      return { items, complete: false }
    },
    async mergePullRequest(input) {
      const { owner, name } = splitRepo(input.repo)
      return requestJson(
        await installationToken(input.credentials, input.signal),
        `/repos/${owner}/${name}/pulls/${input.pr}/merge`,
        {
          method: "PUT",
          body: JSON.stringify({ sha: input.headSha }),
          signal: input.signal,
        }
      )
    },
    async closePullRequest(input) {
      const { owner, name } = splitRepo(input.repo)
      return requestJson(
        await installationToken(input.credentials, input.signal),
        `/repos/${owner}/${name}/pulls/${input.pr}`,
        {
          method: "PATCH",
          body: JSON.stringify({ state: "closed" }),
          signal: input.signal,
        }
      )
    },
  }
}

export async function repositoryCloneInfo(
  repo: string,
  credentials: GitHubCredentials,
  signal?: AbortSignal
): Promise<{
  readonly cloneUrl: string
  readonly defaultBranch: string
  readonly token: string
}> {
  const { owner, name } = splitRepo(repo)
  const token = await installationToken(credentials, signal)
  const value = await requestJson<unknown>(token, `/repos/${owner}/${name}`, {
    signal,
  })
  if (!isRecord(value) || typeof value.clone_url !== "string") {
    throw new Error("GitHub repository clone URL is missing")
  }
  return {
    cloneUrl: value.clone_url,
    defaultBranch:
      typeof value.default_branch === "string" ? value.default_branch : "main",
    token,
  }
}

async function installationToken(
  credentials: GitHubCredentials,
  signal?: AbortSignal
): Promise<string> {
  signal?.throwIfAborted()
  return requiredString("GitHub installation token", credentials.token)
}

async function requestJson<T>(
  token: string,
  path: string,
  init: RequestInit = {}
): Promise<T> {
  const response = await fetch(`${API}${path}`, {
    ...init,
    headers: {
      ...githubHeaders(token),
      ...(init.headers ?? {}),
    },
  })
  if (!response.ok) throw await githubError(response)
  if (response.status === 204) return {} as T
  return (await response.json()) as T
}

async function githubError(
  response: Response
): Promise<Error & { status: number }> {
  let message = `GitHub request failed with ${response.status}`
  try {
    const value = (await response.json()) as unknown
    if (isRecord(value) && typeof value.message === "string") {
      message = value.message
    }
  } catch {
    // Preserve the HTTP status when GitHub returns a non-JSON proxy response.
  }
  return Object.assign(new Error(message), { status: response.status })
}

function githubHeaders(token: string): Record<string, string> {
  return {
    accept: "application/vnd.github+json",
    authorization: `Bearer ${token}`,
    "content-type": "application/json",
    "user-agent": "garbage-collector-worker",
    "x-github-api-version": "2022-11-28",
  }
}

function pullRequestResult(
  value: unknown,
  fallbackBranch: string,
  fallbackHead: string
) {
  if (!isRecord(value) || typeof value.number !== "number") {
    throw new Error("GitHub pull request response is invalid")
  }
  return {
    number: value.number,
    branch:
      isRecord(value.head) && typeof value.head.ref === "string"
        ? value.head.ref
        : fallbackBranch,
    headSha:
      isRecord(value.head) && typeof value.head.sha === "string"
        ? value.head.sha
        : fallbackHead,
  }
}

function splitRepo(repo: string): { owner: string; name: string } {
  const [owner, name, ...rest] = repo.split("/")
  if (!owner || !name || rest.length > 0) {
    throw new Error(`repository must be owner/name: ${repo}`)
  }
  return { owner, name }
}

function requiredString(
  name: string,
  value: string | null | undefined
): string {
  if (!value) throw new Error(`${name} is required`)
  return value
}

function isStatus(error: unknown, status: number): boolean {
  return (
    isRecord(error) &&
    typeof error.status === "number" &&
    error.status === status
  )
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null
}
