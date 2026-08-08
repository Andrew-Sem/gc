export const INSTALLATION = 42
export const TENANT = "acme"
export const REPO = "acme/app"
export const HEAD = "abc"
export const NOW = 3_600_000_000
export const APP_ID = 1001

export function pullRequestPayload(overrides: Record<string, unknown> = {}) {
  return {
    action: "synchronize",
    installation: { id: INSTALLATION },
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

export function checkRun(overrides: Record<string, unknown> = {}) {
  return {
    id: 777,
    name: "build",
    app: { id: APP_ID },
    status: "completed",
    conclusion: "success",
    created_at: "2026-08-05T00:00:00Z",
    updated_at: "2026-08-05T00:05:00Z",
    started_at: "2026-08-05T00:00:00Z",
    completed_at: "2026-08-05T00:05:00Z",
    head_sha: HEAD,
    pull_requests: [{ number: 1 }],
    ...overrides,
  }
}

export function checkRunPayload(overrides: Record<string, unknown> = {}) {
  return {
    action: "completed",
    installation: { id: INSTALLATION },
    repository: { full_name: REPO },
    check_run: checkRun(overrides),
  }
}

export function pullRequestResponse(overrides: Record<string, unknown> = {}) {
  return {
    number: 1,
    draft: false,
    state: "open",
    mergeable: true,
    created_at: "2026-08-05T00:00:00Z",
    updated_at: "2026-08-05T00:05:00Z",
    head: { sha: HEAD },
    base: { ref: "main" },
    ...overrides,
  }
}
