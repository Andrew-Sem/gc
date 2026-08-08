import { expect, test } from "vitest"
import type { ReconciliationSnapshot } from "./contract"
import {
  APP_ID,
  checkRun,
  HEAD,
  NOW,
  pullRequestResponse,
  REPO,
  TENANT,
} from "./fixtures"
import { commandsOfReconciliation } from "./reconciliation"

function reconciliation(
  overrides: Partial<ReconciliationSnapshot> = {}
): ReconciliationSnapshot {
  return {
    pr: { tenant: TENANT, repo: REPO, number: 1 },
    headSha: HEAD,
    pullRequest: pullRequestResponse(),
    checkRuns: [checkRun({ pull_requests: [] })],
    at: NOW,
    ...overrides,
  }
}

test("reconciliation сначала наблюдает текущий PR, затем все runs head", () => {
  expect(commandsOfReconciliation(reconciliation())).toEqual([
    {
      type: "ObservePr",
      pr: { tenant: TENANT, repo: REPO, number: 1 },
      snapshot: {
        headSha: HEAD,
        baseRef: "main",
        draft: false,
        open: true,
        mergeable: true,
        createdAt: Date.parse("2026-08-05T00:00:00Z"),
        updatedAt: Date.parse("2026-08-05T00:05:00Z"),
      },
      at: NOW,
    },
    {
      type: "ObserveCheckRun",
      pr: { tenant: TENANT, repo: REPO, number: 1 },
      headSha: HEAD,
      run: {
        id: 777,
        check: { context: "build", appId: APP_ID },
        status: "completed",
        conclusion: "success",
        createdAt: Date.parse("2026-08-05T00:00:00Z"),
        updatedAt: Date.parse("2026-08-05T00:05:00Z"),
        startedAt: Date.parse("2026-08-05T00:00:00Z"),
        completedAt: Date.parse("2026-08-05T00:05:00Z"),
      },
      at: NOW,
    },
  ])
})

test("reconciliation не зависит от pull_requests association раннего webhook", () => {
  expect(commandsOfReconciliation(reconciliation())).toHaveLength(2)
})

test("run другого head reconciliation не принимает", () => {
  expect(
    commandsOfReconciliation(
      reconciliation({ checkRuns: [checkRun({ head_sha: "other" })] })
    )
  ).toHaveLength(1)
})

test("reconciliation использует head из свежего PR, а не из PrCreated", () => {
  expect(
    commandsOfReconciliation(
      reconciliation({
        headSha: "old",
        pullRequest: pullRequestResponse({ head: { sha: "new" } }),
        checkRuns: [checkRun({ head_sha: "new" })],
      })
    )
  ).toMatchObject([
    { type: "ObservePr", snapshot: { headSha: "new" } },
    { type: "ObserveCheckRun", headSha: "new" },
  ])
})
