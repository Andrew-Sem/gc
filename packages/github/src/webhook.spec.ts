import type { DomainCommand } from "@workspace/domain"
import { expect, test } from "vitest"
import type { WebhookDelivery } from "./contract"
import {
  checkRunPayload,
  HEAD,
  NOW,
  pullRequestPayload,
  REPO,
  TENANT,
} from "./fixtures"
import { commandOf } from "./webhook"

const context = { requiredChecks: ["build"], at: NOW }

function delivery(event: string, payload: unknown): WebhookDelivery {
  return { id: "d-1", event, payload }
}

const CREATED_AT = Date.parse("2026-08-05T00:00:00Z")

const cases: readonly (readonly [
  string,
  WebhookDelivery,
  DomainCommand | null,
])[] = [
  [
    "pull_request становится наблюдением PR",
    delivery("pull_request", pullRequestPayload()),
    {
      type: "ObservePr",
      pr: { tenant: TENANT, repo: REPO, number: 1 },
      snapshot: {
        headSha: HEAD,
        baseRef: "main",
        draft: false,
        open: true,
        mergeable: true,
        createdAt: CREATED_AT,
      },
      at: NOW,
    },
  ],
  [
    "закрытый PR приезжает с open false",
    delivery("pull_request", pullRequestPayload({ state: "closed" })),
    {
      type: "ObservePr",
      pr: { tenant: TENANT, repo: REPO, number: 1 },
      snapshot: {
        headSha: HEAD,
        baseRef: "main",
        draft: false,
        open: false,
        mergeable: true,
        createdAt: CREATED_AT,
      },
      at: NOW,
    },
  ],
  [
    "check_run становится наблюдением проверки, обязательность из контекста",
    delivery("check_run", checkRunPayload()),
    {
      type: "ObserveCheckRun",
      pr: { tenant: TENANT, repo: REPO, number: 1 },
      headSha: HEAD,
      run: {
        id: "777",
        name: "build",
        required: true,
        status: "completed",
        conclusion: "success",
        startedAt: Date.parse("2026-08-05T00:00:00Z"),
        completedAt: Date.parse("2026-08-05T00:05:00Z"),
      },
      at: NOW,
    },
  ],
  [
    "проверка не из списка обязательных помечается необязательной",
    delivery("check_run", checkRunPayload({ name: "preview" })),
    {
      type: "ObserveCheckRun",
      pr: { tenant: TENANT, repo: REPO, number: 1 },
      headSha: HEAD,
      run: {
        id: "777",
        name: "preview",
        required: false,
        status: "completed",
        conclusion: "success",
        startedAt: Date.parse("2026-08-05T00:00:00Z"),
        completedAt: Date.parse("2026-08-05T00:05:00Z"),
      },
      at: NOW,
    },
  ],
  [
    "проверка вне PR отбрасывается",
    delivery("check_run", checkRunPayload({ pull_requests: [] })),
    null,
  ],
  ["неизвестное событие отбрасывается", delivery("ping", {}), null],
  ["чужой формат отбрасывается", delivery("pull_request", { foo: 1 }), null],
]

test.each(cases)("%s", (_name, given, expected) => {
  expect(commandOf(given, context)).toEqual(expected)
})
