import { expect, test } from "vitest"
import type { DomainEvent, Effect } from "./contract"
import { effectsOf } from "./effects"
import { HEAD, NOW, pr, REPO, TENANT } from "./fixtures"

const admissionApproved: DomainEvent = {
  type: "AdmissionApproved",
  tenant: TENANT,
  repo: REPO,
  at: NOW,
}

const mergeApproved: DomainEvent = {
  type: "MergeApproved",
  pr: pr(1),
  headSha: HEAD,
  at: NOW,
}

const closeApproved: DomainEvent = {
  type: "CloseApproved",
  pr: pr(1),
  ageHours: 25,
  at: NOW,
}

const mergeDeclined: DomainEvent = {
  type: "MergeDeclined",
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
    [{ type: "RunAgent", tenant: TENANT, repo: REPO }],
  ],
  [
    "одобренный мерж вызывает мерж",
    [mergeApproved],
    [{ type: "MergePr", pr: pr(1), headSha: HEAD }],
  ],
  [
    "одобренное закрытие закрывает PR",
    [closeApproved],
    [{ type: "ClosePr", pr: pr(1) }],
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
        },
        at: NOW,
      },
    ],
    [],
  ],
  [
    "порядок эффектов повторяет порядок фактов",
    [mergeApproved, admissionApproved],
    [
      { type: "MergePr", pr: pr(1), headSha: HEAD },
      { type: "RunAgent", tenant: TENANT, repo: REPO },
    ],
  ],
]

test.each(cases)("%s", (_name, events, expected) => {
  expect(effectsOf(events)).toEqual(expected)
})
