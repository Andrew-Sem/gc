import type { DomainCommand } from "@workspace/domain"
import { expect, test } from "vitest"
import type { WebhookDelivery } from "./contract"
import {
  APP_ID,
  checkRunPayload,
  HEAD,
  NOW,
  pullRequestPayload,
  REPO,
  TENANT,
} from "./fixtures"
import { commandsOf } from "./webhook"

const context = { tenant: TENANT, at: NOW }

function delivery(event: string, payload: unknown): WebhookDelivery {
  return { id: "d-1", event, payload }
}

const CREATED_AT = Date.parse("2026-08-05T00:00:00Z")

const observePr: DomainCommand = {
  type: "ObservePr",
  pr: { tenant: TENANT, repo: REPO, number: 1 },
  snapshot: {
    headSha: HEAD,
    baseRef: "main",
    draft: false,
    open: true,
    mergeable: true,
    createdAt: CREATED_AT,
    updatedAt: Date.parse("2026-08-05T00:05:00Z"),
  },
  at: NOW,
}

function observeCheck(number = 1, overrides: Record<string, unknown> = {}) {
  return {
    type: "ObserveCheckRun",
    pr: { tenant: TENANT, repo: REPO, number },
    headSha: HEAD,
    run: {
      id: 777,
      check: { context: "build", appId: APP_ID },
      status: "completed",
      conclusion: "success",
      createdAt: CREATED_AT,
      updatedAt: Date.parse("2026-08-05T00:05:00Z"),
      startedAt: CREATED_AT,
      completedAt: Date.parse("2026-08-05T00:05:00Z"),
      ...overrides,
    },
    at: NOW,
  } as const
}

const cases: readonly (readonly [
  string,
  WebhookDelivery,
  readonly DomainCommand[],
])[] = [
  [
    "pull_request становится наблюдением PR",
    delivery("pull_request", pullRequestPayload()),
    [observePr],
  ],
  [
    "закрытый PR приезжает с open false",
    delivery("pull_request", pullRequestPayload({ state: "closed" })),
    [
      {
        ...observePr,
        snapshot: { ...observePr.snapshot, open: false },
      },
    ],
  ],
  [
    "неизвестная mergeability сохраняется как null",
    delivery("pull_request", pullRequestPayload({ mergeable: null })),
    [
      {
        ...observePr,
        snapshot: { ...observePr.snapshot, mergeable: null },
      },
    ],
  ],
  [
    "check_run несёт identity context + appId",
    delivery("check_run", checkRunPayload()),
    [observeCheck()],
  ],
  [
    "check_run с двумя PR создаёт две команды в порядке payload",
    delivery(
      "check_run",
      checkRunPayload({ pull_requests: [{ number: 2 }, { number: 1 }] })
    ),
    [observeCheck(2), observeCheck(1)],
  ],
  [
    "повторяющиеся ассоциации одного PR схлопываются",
    delivery(
      "check_run",
      checkRunPayload({ pull_requests: [{ number: 1 }, { number: 1 }] })
    ),
    [observeCheck(1)],
  ],
  [
    "waiting сохраняется без преобразования",
    delivery(
      "check_run",
      checkRunPayload({
        status: "waiting",
        conclusion: null,
        completed_at: null,
      })
    ),
    [
      observeCheck(1, {
        status: "waiting",
        conclusion: null,
        completedAt: null,
      }),
    ],
  ],
  [
    "проверка вне PR отбрасывается",
    delivery("check_run", checkRunPayload({ pull_requests: [] })),
    [],
  ],
  [
    "check_run без app.id отбрасывается",
    delivery("check_run", checkRunPayload({ app: {} })),
    [],
  ],
  ["неизвестное событие отбрасывается", delivery("ping", {}), []],
  ["чужой формат отбрасывается", delivery("pull_request", { foo: 1 }), []],
]

test.each(cases)("%s", (_name, given, expected) => {
  expect(commandsOf(given, context)).toEqual(expected)
})

test.each([
  "opened",
  "reopened",
  "synchronize",
  "converted_to_draft",
  "ready_for_review",
  "edited",
  "closed",
])("pull_request action %s наблюдается", (action) => {
  expect(
    commandsOf(
      delivery("pull_request", { ...pullRequestPayload(), action }),
      context
    )
  ).toEqual([observePr])
})

test.each(["assigned", "labeled"])(
  "pull_request action %s игнорируется",
  (action) => {
    expect(
      commandsOf(
        delivery("pull_request", { ...pullRequestPayload(), action }),
        context
      )
    ).toEqual([])
  }
)

test.each([
  {
    action: "created",
    payload: {
      status: "queued",
      conclusion: null,
      started_at: null,
      completed_at: null,
    },
    expected: {
      status: "queued",
      conclusion: null,
      startedAt: null,
      completedAt: null,
    },
  },
  {
    action: "rerequested",
    payload: {
      status: "queued",
      conclusion: null,
      started_at: null,
      completed_at: null,
    },
    expected: {
      status: "queued",
      conclusion: null,
      startedAt: null,
      completedAt: null,
    },
  },
  { action: "completed", payload: {}, expected: {} },
])("check_run action $action наблюдается", ({ action, payload, expected }) => {
  expect(
    commandsOf(
      delivery("check_run", {
        ...checkRunPayload(payload),
        action,
      }),
      context
    )
  ).toEqual([observeCheck(1, expected)])
})

test("check_run requested_action игнорируется", () => {
  expect(
    commandsOf(
      delivery("check_run", {
        ...checkRunPayload(),
        action: "requested_action",
      }),
      context
    )
  ).toEqual([])
})
