import type { RepositorySettings, TenantSettings } from "@workspace/domain"

export const TENANT = "acme"
export const OTHER_TENANT = "globex"
export const REPO = "acme/app"
export const OTHER_REPO = "acme/other"
export const HEAD = "abc"
export const HOUR = 3_600_000
export const NOW = Date.parse("2026-08-05T06:00:00Z")
export const SHARED_GITHUB_APP_ID = 7
export const SHARED_GITHUB_APP_CLIENT_ID = "Iv1.shared-client-id"
export const SHARED_GITHUB_APP_CLIENT_SECRET = "shared-client-secret"
export const SHARED_GITHUB_APP_PRIVATE_KEY = "shared-github-private-key"
export const SHARED_GITHUB_APP_SLUG = "garbage-collector"
export const SHARED_GITHUB_APP_WEBHOOK_SECRET = "shared-webhook-secret"
export const OWNER_GITHUB_USER_ID = 100
export const OTHER_OWNER_GITHUB_USER_ID = 200

export const tenantSettings: TenantSettings = {
  model: "gpt-5.6-luna",
  reasoning: "xhigh",
  prompt: "Remove obsolete code",
}

export const repositorySettings: RepositorySettings = {
  productionEveryHours: 2,
  openPrLimit: 5,
  closeAfterHours: 18,
}

export function pullRequestPayload(overrides: Record<string, unknown> = {}) {
  return {
    action: "synchronize",
    installation: { id: 42 },
    repository: { full_name: REPO },
    pull_request: {
      number: 1,
      draft: false,
      state: "open",
      mergeable: true,
      created_at: "2026-08-05T00:00:00Z",
      updated_at: "2026-08-05T00:05:00Z",
      head: { sha: HEAD },
      base: { ref: "main" },
      ...overrides,
    },
  }
}

export function checkRunResponse(overrides: Record<string, unknown> = {}) {
  return {
    id: 777,
    name: "build",
    app: { id: 1001 },
    status: "completed",
    conclusion: "failure",
    created_at: "2026-08-05T05:55:00Z",
    updated_at: "2026-08-05T06:00:00Z",
    started_at: "2026-08-05T05:55:00Z",
    completed_at: "2026-08-05T06:00:00Z",
    head_sha: HEAD,
    pull_requests: [{ number: 1 }],
    ...overrides,
  }
}

export function checkRunPayload(overrides: Record<string, unknown> = {}) {
  return {
    action: "completed",
    installation: { id: 42 },
    repository: { full_name: REPO },
    check_run: checkRunResponse(overrides),
  }
}

export function pullRequestResponse(overrides: Record<string, unknown> = {}) {
  return {
    number: 1,
    draft: false,
    state: "open",
    mergeable: true,
    created_at: "2026-08-05T05:55:00Z",
    updated_at: "2026-08-05T06:00:00Z",
    head: { sha: HEAD },
    base: { ref: "main" },
    ...overrides,
  }
}
