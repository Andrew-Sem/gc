import { expect, test } from "vitest"
import type { DomainEvent, Effect } from "./contract"
import { effectsOf } from "./effects"
import { HEAD, NOW, pr, REPO, SWEEP, TENANT, tenantSettings } from "./fixtures"

const admissionApproved: DomainEvent = {
  type: "AdmissionApproved",
  tenant: TENANT,
  repo: REPO,
  tenantSettings,
  at: NOW,
}

const mergeApproved: DomainEvent = {
  type: "MergeApproved",
  sweepId: SWEEP,
  pr: pr(1),
  headSha: HEAD,
  at: NOW,
}

const closeApproved: DomainEvent = {
  type: "CloseApproved",
  sweepId: SWEEP,
  pr: pr(1),
  ageHours: 25,
  at: NOW,
}

const prCreated: DomainEvent = {
  type: "PrCreated",
  reportId: "pr-1",
  pr: pr(1),
  headSha: HEAD,
  branch: "gc/1",
  at: NOW,
}

const mergeDeclined: DomainEvent = {
  type: "MergeDeclined",
  sweepId: SWEEP,
  pr: pr(1),
  refusal: { kind: "draft" },
  at: NOW,
}

const cases: readonly (readonly [
  string,
  readonly DomainEvent[],
  readonly Effect[],
])[] = [
  ["пустой список фактов не даёт эффектов", [], []],
  [
    "одобренная работа запускает агента",
    [admissionApproved],
    [{ type: "RunAgent", tenant: TENANT, repo: REPO, tenantSettings }],
  ],
  [
    "созданный PR запускает обязательную reconciliation",
    [prCreated],
    [{ type: "ReconcilePr", pr: pr(1), headSha: HEAD }],
  ],
  [
    "одобренный мерж вызывает мерж",
    [mergeApproved],
    [{ type: "MergePr", sweepId: SWEEP, pr: pr(1), headSha: HEAD }],
  ],
  [
    "одобренное закрытие закрывает PR",
    [closeApproved],
    [{ type: "ClosePr", sweepId: SWEEP, pr: pr(1) }],
  ],
  ["отказ эффекта не порождает", [mergeDeclined], []],
  [
    "наблюдения эффектов не порождают",
    [
      {
        type: "PrStateChanged",
        pr: pr(1),
        snapshot: {
          headSha: HEAD,
          baseRef: "main",
          draft: false,
          open: true,
          mergeable: true,
          createdAt: 0,
          updatedAt: 0,
        },
        at: NOW,
      },
    ],
    [],
  ],
  [
    "порядок эффектов повторяет порядок фактов",
    [mergeApproved, prCreated, admissionApproved],
    [
      { type: "MergePr", sweepId: SWEEP, pr: pr(1), headSha: HEAD },
      { type: "ReconcilePr", pr: pr(1), headSha: HEAD },
      { type: "RunAgent", tenant: TENANT, repo: REPO, tenantSettings },
    ],
  ],
]

test.each(cases)("%s", (_name, events, expected) => {
  expect(effectsOf(events)).toEqual(expected)
})
