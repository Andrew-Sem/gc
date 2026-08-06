export const INSTALLATION = 42
export const TENANT = "42"
export const REPO = "acme/app"
export const HEAD = "abc"
export const NOW = 3_600_000_000

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
      head: { sha: HEAD },
      base: { ref: "main" },
      ...overrides,
    },
  }
}

export function checkRunPayload(overrides: Record<string, unknown> = {}) {
  return {
    action: "completed",
    installation: { id: INSTALLATION },
    repository: { full_name: REPO },
    check_run: {
      id: 777,
      name: "build",
      status: "completed",
      conclusion: "success",
      started_at: "2026-08-05T00:00:00Z",
      completed_at: "2026-08-05T00:05:00Z",
      head_sha: HEAD,
      pull_requests: [{ number: 1 }],
      ...overrides,
    },
  }
}
