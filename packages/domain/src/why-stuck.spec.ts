import { expect, test } from "vitest"
import type { DomainEvent, PrStatusLine } from "./contract"
import {
  check,
  checkSeen,
  HEAD,
  HOUR,
  mergeablePr,
  NOW,
  pr,
  prSeen,
} from "./fixtures"
import { projectOpenPrs } from "./projections"

const cases: readonly (readonly [
  string,
  readonly DomainEvent[],
  readonly PrStatusLine[],
])[] = [
  ["пустой журнал — пустой экран", [], []],
  [
    "зелёный PR: смержится на ближайшем свипе",
    [...mergeablePr(1, 2)],
    [
      {
        pr: pr(1),
        headSha: HEAD,
        verdict: "will_merge",
        reason: null,
        ageHours: 2,
        hoursUntilClose: 22,
      },
    ],
  ],
  [
    "упавшая проверка: ждём и показываем причину",
    [...mergeablePr(1, 5), checkSeen(1, check({ conclusion: "failure" }))],
    [
      {
        pr: pr(1),
        headSha: HEAD,
        verdict: "waiting",
        reason: { kind: "required_check_failed", check: "build" },
        ageHours: 5,
        hoursUntilClose: 19,
      },
    ],
  ],
  [
    "старый и не мержится: будет закрыт",
    [
      ...mergeablePr(1, 25),
      prSeen(1, { draft: true, createdAt: NOW - 25 * HOUR }),
    ],
    [
      {
        pr: pr(1),
        headSha: HEAD,
        verdict: "will_close",
        reason: { kind: "draft" },
        ageHours: 25,
        hoursUntilClose: 0,
      },
    ],
  ],
  [
    "закрытый PR на экране не показывается",
    [...mergeablePr(1, 2), prSeen(1, { open: false })],
    [],
  ],
  [
    "несколько PR идут по возрастанию номера",
    [
      ...mergeablePr(1, 2),
      ...mergeablePr(2, 3),
      prSeen(2, { mergeable: false, createdAt: NOW - 3 * HOUR }),
    ],
    [
      {
        pr: pr(1),
        headSha: HEAD,
        verdict: "will_merge",
        reason: null,
        ageHours: 2,
        hoursUntilClose: 22,
      },
      {
        pr: pr(2),
        headSha: HEAD,
        verdict: "waiting",
        reason: { kind: "not_mergeable" },
        ageHours: 3,
        hoursUntilClose: 21,
      },
    ],
  ],
]

test.each(cases)("%s", (_name, events, expected) => {
  expect(projectOpenPrs(events, NOW)).toEqual(expected)
})
