import type {
  CheckConclusion,
  CheckStatus,
  DomainCommand,
  DomainEvent,
  MergeRefusal,
  SweepId,
} from "./contract"
import { decider } from "./decider"
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
  SWEEP,
  TENANT,
} from "./fixtures"
import { createExecutableSpecSuite } from "./framework/runner"

const { examples, sequence, spec } = createExecutableSpecSuite({ decider })

function sweep(
  sweepId: SweepId = SWEEP,
  closeAfterHours = 24,
  now = NOW
): DomainCommand {
  return {
    type: "TickSweep",
    sweepId,
    tenant: TENANT,
    repo: REPO,
    settings: repositorySettings({ closeAfterHours }),
    now,
  }
}

function approved(number: number, sweepId: SweepId = SWEEP, at = NOW) {
  return {
    type: "MergeApproved",
    sweepId,
    pr: pr(number),
    headSha: HEAD,
    at,
  } as const
}

function declined(
  number: number,
  refusal: MergeRefusal,
  sweepId: SweepId = SWEEP,
  at = NOW
) {
  return {
    type: "MergeDeclined",
    sweepId,
    pr: pr(number),
    refusal,
    at,
  } as const
}

function closed(
  number: number,
  ageHours: number,
  sweepId: SweepId = SWEEP,
  at = NOW
) {
  return {
    type: "CloseApproved",
    sweepId,
    pr: pr(number),
    ageHours,
    at,
  } as const
}

function knownPr(number: number, ageHours: number): readonly DomainEvent[] {
  const createdAt = NOW - ageHours * HOUR
  return [created(number, createdAt), prSeen(number, { createdAt }, createdAt)]
}

spec("свип без открытых PR ничего не решает", {
  given: [],
  when: sweep(),
  then: [],
})

spec("все четыре условия сошлись — мержим", {
  given: [...mergeablePr(1)],
  when: sweep(),
  then: [approved(1)],
})

examples("одно непройденное условие — отказ с причиной", [
  {
    given: [...mergeablePr(1), prSeen(1, { draft: true, updatedAt: NOW }, NOW)],
    when: sweep(),
    then: [declined(1, { kind: "draft" })],
  },
  {
    given: [
      ...mergeablePr(1),
      prSeen(1, { mergeable: false, updatedAt: NOW }, NOW),
    ],
    when: sweep(),
    then: [declined(1, { kind: "not_mergeable" })],
  },
  {
    given: [
      ...mergeablePr(1),
      prSeen(1, { mergeable: null, updatedAt: NOW }, NOW),
    ],
    when: sweep(),
    then: [declined(1, { kind: "not_mergeable" })],
  },
  {
    given: [
      ...knownPr(1, 2),
      requiredChecksSeen(
        1,
        checksSnapshot({ runs: [check({ conclusion: "failure" })] })
      ),
    ],
    when: sweep(),
    then: [
      declined(1, {
        kind: "required_check_failed",
        check: requiredCheck(),
      }),
    ],
  },
  {
    given: [
      ...knownPr(1, 2),
      requiredChecksSeen(
        1,
        checksSnapshot({
          runs: [check({ status: "in_progress", conclusion: null })],
        })
      ),
    ],
    when: sweep(),
    then: [declined(1, { kind: "required_checks_pending", pending: 1 })],
  },
])

examples(
  "success, neutral и skipped считаются успешными",
  (["success", "neutral", "skipped"] as const).map(
    (conclusion: CheckConclusion) => ({
      given: [
        ...knownPr(1, 2),
        requiredChecksSeen(
          1,
          checksSnapshot({ runs: [check({ conclusion })] })
        ),
      ],
      when: sweep(),
      then: [approved(1)],
    })
  )
)

examples(
  "остальные completed conclusions отказывают",
  (
    ["failure", "cancelled", "timed_out", "action_required", "stale"] as const
  ).map((conclusion: CheckConclusion) => ({
    given: [
      ...knownPr(1, 2),
      requiredChecksSeen(1, checksSnapshot({ runs: [check({ conclusion })] })),
    ],
    when: sweep(),
    then: [
      declined(1, {
        kind: "required_check_failed",
        check: requiredCheck(),
      }),
    ],
  }))
)

examples(
  "незавершённый статус ждёт",
  (["queued", "in_progress", "waiting", "requested", "pending"] as const).map(
    (status: CheckStatus) => ({
      given: [
        ...knownPr(1, 2),
        requiredChecksSeen(
          1,
          checksSnapshot({ runs: [check({ status, conclusion: null })] })
        ),
      ],
      when: sweep(),
      then: [declined(1, { kind: "required_checks_pending", pending: 1 })],
    })
  )
)

examples("обязательных проверок нет вообще — не мержим", [
  {
    given: [
      ...knownPr(1, 2),
      requiredChecksSeen(1, checksSnapshot({ required: [], runs: [] })),
    ],
    when: sweep(),
    then: [declined(1, { kind: "no_required_checks" })],
  },
  {
    given: [
      ...knownPr(1, 2),
      requiredChecksSeen(
        1,
        checksSnapshot({
          required: [],
          runs: [check({ check: requiredCheck("preview") })],
        })
      ),
    ],
    when: sweep(),
    then: [declined(1, { kind: "no_required_checks" })],
  },
])

spec("обязательная проверка без попытки считается pending", {
  given: [
    ...knownPr(1, 2),
    requiredChecksSeen(1, checksSnapshot({ runs: [] })),
  ],
  when: sweep(),
  then: [declined(1, { kind: "required_checks_pending", pending: 1 })],
})

spec("одинаковый context другого app не удовлетворяет требование", {
  given: [
    ...knownPr(1, 2),
    requiredChecksSeen(
      1,
      checksSnapshot({
        runs: [check({ check: requiredCheck("build", 2002) })],
      })
    ),
  ],
  when: sweep(),
  then: [declined(1, { kind: "required_checks_pending", pending: 1 })],
})

spec("последняя попытка успешна — старая ошибка не мешает", {
  given: [
    ...knownPr(1, 2),
    requiredChecksSeen(
      1,
      checksSnapshot({
        runs: [
          check({ id: 1, createdAt: 1, conclusion: "failure" }),
          check({ id: 2, createdAt: 2, conclusion: "success" }),
        ],
      })
    ),
  ],
  when: sweep(),
  then: [approved(1)],
})

examples("последняя попытка определяет отказ", [
  {
    given: [
      ...knownPr(1, 2),
      requiredChecksSeen(
        1,
        checksSnapshot({
          runs: [
            check({ id: 1, createdAt: 1, conclusion: "success" }),
            check({
              id: 2,
              createdAt: 2,
              status: "pending",
              conclusion: null,
            }),
          ],
        })
      ),
    ],
    when: sweep(),
    then: [declined(1, { kind: "required_checks_pending", pending: 1 })],
  },
  {
    given: [
      ...knownPr(1, 2),
      requiredChecksSeen(
        1,
        checksSnapshot({
          runs: [
            check({ id: 1, createdAt: 1, conclusion: "success" }),
            check({ id: 2, createdAt: 2, conclusion: "failure" }),
          ],
        })
      ),
    ],
    when: sweep(),
    then: [
      declined(1, {
        kind: "required_check_failed",
        check: requiredCheck(),
      }),
    ],
  },
])

spec("при одинаковом createdAt больший id означает более новую попытку", {
  given: [
    ...knownPr(1, 2),
    requiredChecksSeen(
      1,
      checksSnapshot({
        runs: [
          check({ id: 2, createdAt: 1, conclusion: "success" }),
          check({ id: 1, createdAt: 1, conclusion: "failure" }),
        ],
      })
    ),
  ],
  when: sweep(),
  then: [approved(1)],
})

spec("зелёное относится к другому коммиту — не мержим", {
  given: [
    created(1, NOW - 2 * HOUR),
    prSeen(1, { headSha: "def", createdAt: NOW - 2 * HOUR }, NOW - 2 * HOUR),
    requiredChecksSeen(1),
  ],
  when: sweep(),
  then: [declined(1, { kind: "evidence_for_other_head", headSha: "def" })],
})

spec("необязательные runs в полном снимке игнорируются", {
  given: [
    ...knownPr(1, 2),
    requiredChecksSeen(
      1,
      checksSnapshot({
        runs: [
          check(),
          check({
            id: 778,
            check: requiredCheck("preview", 2002),
            conclusion: "failure",
          }),
        ],
      })
    ),
  ],
  when: sweep(),
  then: [approved(1)],
})

spec("вебхук без полного снимка текущего свипа не разрешает мерж", {
  given: [
    ...knownPr(1, 2),
    checkSeen(1, check({ createdAt: NOW - 1, updatedAt: NOW - 1 }), HEAD, NOW),
  ],
  when: sweep(),
  then: [declined(1, { kind: "required_checks_unknown" })],
})

sequence(
  "оборванный lookup блокирует только свой свип",
  [...knownPr(1, 2), lookupIncomplete(1, "rate_limited", "sweep-a")],
  [
    {
      when: sweep("sweep-a"),
      then: [
        declined(
          1,
          { kind: "lookup_incomplete", failure: "rate_limited" },
          "sweep-a"
        ),
      ],
    },
    {
      when: {
        type: "ReportRequiredChecks",
        reportId: "checks-b",
        sweepId: "sweep-b",
        pr: pr(1),
        headSha: HEAD,
        snapshot: checksSnapshot(),
        at: NOW,
      },
      then: [
        {
          type: "RequiredChecksObserved",
          reportId: "checks-b",
          sweepId: "sweep-b",
          pr: pr(1),
          headSha: HEAD,
          snapshot: checksSnapshot(),
          at: NOW,
        },
      ],
    },
    { when: sweep("sweep-b"), then: [approved(1, "sweep-b")] },
  ]
)

spec("старый полный снимок не заменяет evidence нового свипа", {
  given: [...mergeablePr(1, 2, {}, "sweep-old")],
  when: sweep("sweep-new"),
  then: [declined(1, { kind: "required_checks_unknown" }, "sweep-new")],
})

sequence("повтор того же TickSweep решения не дублирует", mergeablePr(1), [
  { when: sweep(), then: [approved(1)] },
  { when: sweep(), then: [] },
])

spec("неполный lookup не закрывает старый PR", {
  given: [...knownPr(1, 25), lookupIncomplete(1)],
  when: sweep(),
  then: [declined(1, { kind: "lookup_incomplete", failure: "rate_limited" })],
})

spec("старый PR без терминального evidence текущего свипа не закрывается", {
  given: [...knownPr(1, 25)],
  when: sweep(),
  then: [declined(1, { kind: "required_checks_unknown" })],
})

spec("PR старше суток и не мержится — закрываем", {
  given: [
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
  when: sweep(),
  then: [closed(1, 25)],
})

spec("PR закрывается на точной границе closeAfterHours", {
  given: [
    ...mergeablePr(1, 24),
    prSeen(
      1,
      {
        draft: true,
        createdAt: NOW - 24 * HOUR,
        updatedAt: NOW,
      },
      NOW
    ),
  ],
  when: sweep(),
  then: [closed(1, 24)],
})

spec("PR старше суток, но мержится — мерж важнее закрытия", {
  given: [...mergeablePr(1, 25)],
  when: sweep(),
  then: [approved(1)],
})

sequence(
  "новый порог закрытия действует со следующего свипа",
  [
    ...mergeablePr(1, 25, {}, "sweep-a"),
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
      when: sweep("sweep-a", 30),
      then: [declined(1, { kind: "draft" }, "sweep-a")],
    },
    {
      when: {
        type: "ReportRequiredChecks",
        reportId: "checks-sweep-b-1",
        sweepId: "sweep-b",
        pr: pr(1),
        headSha: HEAD,
        snapshot: checksSnapshot(),
        at: NOW + 6 * HOUR,
      },
      then: [
        {
          type: "RequiredChecksObserved",
          reportId: "checks-sweep-b-1",
          sweepId: "sweep-b",
          pr: pr(1),
          headSha: HEAD,
          snapshot: checksSnapshot(),
          at: NOW + 6 * HOUR,
        },
      ],
    },
    {
      when: sweep("sweep-b", 24, NOW + 6 * HOUR),
      then: [closed(1, 31, "sweep-b", NOW + 6 * HOUR)],
    },
  ]
)

spec("несколько PR решаются за один свип, по возрастанию номера", {
  given: [
    ...mergeablePr(1),
    ...mergeablePr(2),
    prSeen(
      2,
      {
        mergeable: false,
        createdAt: NOW - 2 * HOUR,
        updatedAt: NOW,
      },
      NOW
    ),
  ],
  when: sweep(),
  then: [approved(1), declined(2, { kind: "not_mergeable" })],
})

spec("закрытый PR в свипе не участвует", {
  given: [...mergeablePr(1), prSeen(1, { open: false, updatedAt: NOW }, NOW)],
  when: sweep(),
  then: [],
})

spec("свип игнорирует PR другого репозитория и тенанта", {
  given: [
    ...mergeablePr(1),
    {
      type: "PrCreated",
      reportId: "foreign-pr",
      pr: { tenant: "globex", repo: "globex/other", number: 2 },
      headSha: HEAD,
      branch: "gc/2",
      at: 0,
    },
    {
      type: "PrStateChanged",
      pr: { tenant: "globex", repo: "globex/other", number: 2 },
      snapshot: {
        headSha: HEAD,
        baseRef: "main",
        draft: false,
        open: true,
        mergeable: true,
        createdAt: NOW - 2 * HOUR,
        updatedAt: 0,
      },
      at: 0,
    },
    {
      type: "RequiredChecksObserved",
      reportId: "foreign-checks",
      sweepId: SWEEP,
      pr: { tenant: "globex", repo: "globex/other", number: 2 },
      headSha: HEAD,
      snapshot: checksSnapshot(),
      at: 0,
    },
  ],
  when: sweep(),
  then: [approved(1)],
})
