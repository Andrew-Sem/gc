import type { DomainCommand } from "@workspace/domain"
import { expect, test } from "vitest"
import type { RequiredChecksLookup } from "./contract"
import { APP_ID, checkRun, HEAD, NOW, REPO, TENANT } from "./fixtures"
import { commandOfRequiredChecks } from "./required-checks"

function lookup(
  overrides: Partial<RequiredChecksLookup> = {}
): RequiredChecksLookup {
  return {
    reportId: "checks-1",
    sweepId: "sweep-1",
    pr: { tenant: TENANT, repo: REPO, number: 1 },
    headSha: HEAD,
    requiredChecks: [{ context: "build", app_id: APP_ID }],
    checkRuns: [checkRun()],
    requiredChecksComplete: true,
    checkRunsComplete: true,
    at: NOW,
    ...overrides,
  }
}

const observed: DomainCommand = {
  type: "ReportRequiredChecks",
  reportId: "checks-1",
  sweepId: "sweep-1",
  pr: { tenant: TENANT, repo: REPO, number: 1 },
  headSha: HEAD,
  snapshot: {
    required: [{ context: "build", appId: APP_ID }],
    runs: [
      {
        id: 777,
        check: { context: "build", appId: APP_ID },
        status: "completed",
        conclusion: "success",
        createdAt: Date.parse("2026-08-05T00:00:00Z"),
        updatedAt: Date.parse("2026-08-05T00:05:00Z"),
        startedAt: Date.parse("2026-08-05T00:00:00Z"),
        completedAt: Date.parse("2026-08-05T00:05:00Z"),
      },
    ],
  },
  at: NOW,
}

test("branch protection context + app_id становится required identity", () => {
  expect(commandOfRequiredChecks(lookup())).toEqual(observed)
})

test("все попытки сохраняются, adapter не выбирает последнюю", () => {
  const command = commandOfRequiredChecks(
    lookup({
      checkRuns: [
        checkRun({ id: 1, conclusion: "failure" }),
        checkRun({ id: 2, conclusion: "success" }),
      ],
    })
  )

  expect(command).toMatchObject({
    type: "ReportRequiredChecks",
    snapshot: {
      runs: [
        { id: 1, conclusion: "failure" },
        { id: 2, conclusion: "success" },
      ],
    },
  })
})

test("attempt другого head не попадает в снимок текущего head", () => {
  const command = commandOfRequiredChecks(
    lookup({
      checkRuns: [
        checkRun({ id: 1, head_sha: HEAD }),
        checkRun({ id: 2, head_sha: "other", conclusion: "failure" }),
      ],
    })
  )

  expect(command).toMatchObject({
    type: "ReportRequiredChecks",
    snapshot: { runs: [{ id: 1 }] },
  })
})

test("одинаковый context от разных app остаётся разным", () => {
  const command = commandOfRequiredChecks(
    lookup({
      requiredChecks: [
        { context: "build", app_id: APP_ID },
        { context: "build", app_id: 2002 },
      ],
    })
  )

  expect(command).toMatchObject({
    snapshot: {
      required: [
        { context: "build", appId: APP_ID },
        { context: "build", appId: 2002 },
      ],
    },
  })
})

test("полный пустой ответ остаётся полным пустым inventory", () => {
  expect(
    commandOfRequiredChecks(lookup({ requiredChecks: [], checkRuns: [] }))
  ).toMatchObject({
    type: "ReportRequiredChecks",
    snapshot: { required: [], runs: [] },
  })
})

test.each([
  { failure: "rate_limited" as const },
  { failure: "forbidden" as const },
  { failure: "upstream_error" as const },
])("ошибка lookup $failure становится ReportLookupFailure", ({ failure }) => {
  expect(commandOfRequiredChecks(lookup({ failure }))).toEqual({
    type: "ReportLookupFailure",
    reportId: "checks-1",
    scope: {
      kind: "checks",
      sweepId: "sweep-1",
      pr: { tenant: TENANT, repo: REPO, number: 1 },
      headSha: HEAD,
    },
    failure,
    at: NOW,
  })
})

test.each([{ requiredChecksComplete: false }, { checkRunsComplete: false }])(
  "partial pages никогда не становятся снимком",
  (incomplete) => {
    expect(commandOfRequiredChecks(lookup(incomplete))).toMatchObject({
      type: "ReportLookupFailure",
      failure: "page_limit",
    })
  }
)
