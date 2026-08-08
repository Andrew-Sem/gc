import { expect, test } from "vitest"
import type {
  CheckConclusion,
  CheckStatus,
  DomainEvent,
  PrStatusLine,
} from "./contract"
import {
  check,
  checkSeen,
  checksSnapshot,
  created,
  HEAD,
  HOUR,
  lookupIncomplete,
  mergeablePr,
  NOW,
  pr,
  prSeen,
  REPO,
  repositorySettings,
  requiredCheck,
  requiredChecksSeen,
  TENANT,
} from "./fixtures"
import { projectOpenPrs } from "./projections"

type ProjectionCase = readonly [
  string,
  readonly DomainEvent[],
  readonly PrStatusLine[],
]

const passingCases: readonly ProjectionCase[] = (
  ["neutral", "skipped"] as const
).map((conclusion: CheckConclusion) => [
  `${conclusion} остаётся готовым к мержу`,
  [
    ...mergeablePr(1, 2),
    requiredChecksSeen(
      1,
      checksSnapshot({
        runs: [check(), check({ id: 778, createdAt: NOW, conclusion })],
      }),
      "latest"
    ),
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
  ],
])

const pendingCases: readonly ProjectionCase[] = (
  ["waiting", "requested", "pending"] as const
).map((status: CheckStatus) => [
  `${status} показывает ожидание`,
  [
    ...mergeablePr(1, 2),
    requiredChecksSeen(
      1,
      checksSnapshot({
        runs: [
          check(),
          check({
            id: 778,
            createdAt: NOW,
            status,
            conclusion: null,
          }),
        ],
      }),
      "latest"
    ),
  ],
  [
    {
      pr: pr(1),
      headSha: HEAD,
      verdict: "waiting",
      reason: { kind: "required_checks_pending", pending: 1 },
      ageHours: 2,
      hoursUntilClose: 22,
    },
  ],
])

const cases: readonly ProjectionCase[] = [
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
    "поздний вебхук пересчитывает прогноз по последней попытке",
    [
      ...mergeablePr(1, 5),
      checkSeen(
        1,
        check({ id: 778, createdAt: NOW, conclusion: "failure" }),
        HEAD,
        NOW
      ),
    ],
    [
      {
        pr: pr(1),
        headSha: HEAD,
        verdict: "waiting",
        reason: {
          kind: "required_check_failed",
          check: requiredCheck(),
        },
        ageHours: 5,
        hoursUntilClose: 19,
      },
    ],
  ],
  [
    "новый успешный webhook исправляет старый красный прогноз",
    [
      created(1, NOW - 5 * HOUR),
      prSeen(1, { createdAt: NOW - 5 * HOUR }, NOW - 5 * HOUR),
      requiredChecksSeen(
        1,
        checksSnapshot({
          runs: [check({ conclusion: "failure", updatedAt: 1 })],
        }),
        "latest"
      ),
      checkSeen(
        1,
        check({ id: 778, createdAt: NOW, conclusion: "success" }),
        HEAD,
        NOW
      ),
    ],
    [
      {
        pr: pr(1),
        headSha: HEAD,
        verdict: "will_merge",
        reason: null,
        ageHours: 5,
        hoursUntilClose: 19,
      },
    ],
  ],
  [
    "более новое состояние того же run заменяет старое",
    [
      created(1, NOW - 2 * HOUR),
      prSeen(1, { createdAt: NOW - 2 * HOUR }, NOW - 2 * HOUR),
      requiredChecksSeen(
        1,
        checksSnapshot({
          runs: [
            check({ status: "in_progress", conclusion: null, updatedAt: 1 }),
          ],
        }),
        "latest"
      ),
      checkSeen(1, check({ conclusion: "success", updatedAt: 2 }), HEAD, NOW),
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
    ],
  ],
  [
    "проверка того же context от другого app не меняет прогноз",
    [
      ...mergeablePr(1, 2),
      checkSeen(
        1,
        check({
          id: 778,
          check: requiredCheck("build", 2002),
          createdAt: NOW,
          conclusion: "failure",
        }),
        HEAD,
        NOW
      ),
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
    ],
  ],
  ...passingCases,
  ...pendingCases,
  [
    "последний полный inventory заменяет старый, а не объединяется с ним",
    [
      ...mergeablePr(1, 2, {}, "old"),
      requiredChecksSeen(
        1,
        checksSnapshot({
          required: [requiredCheck(), requiredCheck("preview", 2002)],
          runs: [
            check(),
            check({
              id: 778,
              check: requiredCheck("preview", 2002),
              conclusion: "success",
            }),
          ],
        }),
        "old-inventory"
      ),
      requiredChecksSeen(
        1,
        checksSnapshot({
          runs: [
            check(),
            check({
              id: 778,
              check: requiredCheck("preview", 2002),
              conclusion: "success",
            }),
          ],
        }),
        "latest-inventory"
      ),
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
    ],
  ],
  [
    "inventory старого head не разрешает мерж нового head",
    [...mergeablePr(1, 2), prSeen(1, { headSha: "new", updatedAt: NOW }, NOW)],
    [
      {
        pr: pr(1),
        headSha: "new",
        verdict: "waiting",
        reason: { kind: "evidence_for_other_head", headSha: "new" },
        ageHours: 2,
        hoursUntilClose: 22,
      },
    ],
  ],
  [
    "оборванный новый свип не стирает последний полный снимок экрана",
    [...mergeablePr(1, 2), lookupIncomplete(1, "rate_limited", "later")],
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
    "старый и не мержится: будет закрыт",
    [
      ...mergeablePr(1, 25),
      prSeen(
        1,
        {
          draft: true,
          createdAt: NOW - 25 * HOUR,
          updatedAt: NOW,
        },
        NOW
      ),
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
    "экран использует последний настраиваемый closeAfterHours",
    [
      {
        type: "RepositorySettingsChanged",
        reportId: "settings-1",
        tenant: TENANT,
        repo: REPO,
        settings: repositorySettings({ closeAfterHours: 18 }),
        at: 0,
      },
      ...mergeablePr(1, 2),
    ],
    [
      {
        pr: pr(1),
        headSha: HEAD,
        verdict: "will_merge",
        reason: null,
        ageHours: 2,
        hoursUntilClose: 16,
      },
    ],
  ],
  [
    "новый closeAfterHours применяется к уже открытому PR на точной границе",
    [
      ...mergeablePr(1, 18),
      prSeen(
        1,
        {
          draft: true,
          createdAt: NOW - 18 * HOUR,
          updatedAt: NOW,
        },
        NOW
      ),
      {
        type: "RepositorySettingsChanged",
        reportId: "settings-later",
        tenant: TENANT,
        repo: REPO,
        settings: repositorySettings({ closeAfterHours: 18 }),
        at: NOW,
      },
    ],
    [
      {
        pr: pr(1),
        headSha: HEAD,
        verdict: "will_close",
        reason: { kind: "draft" },
        ageHours: 18,
        hoursUntilClose: 0,
      },
    ],
  ],
  [
    "закрытый PR на экране не показывается",
    [...mergeablePr(1, 2), prSeen(1, { open: false, updatedAt: NOW }, NOW)],
    [],
  ],
  [
    "несколько PR идут по возрастанию номера",
    [
      ...mergeablePr(1, 2),
      ...mergeablePr(2, 3),
      prSeen(
        2,
        {
          mergeable: false,
          createdAt: NOW - 3 * HOUR,
          updatedAt: NOW,
        },
        NOW
      ),
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
