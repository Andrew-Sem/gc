import type { DomainCommand } from "./contract"
import { decider } from "./decider"
import {
  check,
  checkSeen,
  created,
  HEAD,
  NOW,
  pr,
  prSeen,
  REPO,
  snapshot,
  TENANT,
} from "./fixtures"
import { createExecutableSpecSuite } from "./framework/runner"

const { examples, sequence, spec } = createExecutableSpecSuite({ decider })

const observePr = (
  number: number,
  overrides: Partial<Parameters<typeof snapshot>[0]> = {}
): DomainCommand => ({
  type: "ObservePr",
  pr: pr(number),
  snapshot: snapshot(overrides),
  at: NOW,
})

const observeCheck = (
  number: number,
  overrides: Parameters<typeof check>[0] = {}
): DomainCommand => ({
  type: "ObserveCheckRun",
  pr: pr(number),
  headSha: HEAD,
  run: check(overrides),
  at: NOW,
})

// --- Заведение PR -----------------------------------------------------------

spec("отчёт о созданном PR становится фактом", {
  given: [],
  when: {
    type: "ReportPrCreated",
    pr: pr(1),
    headSha: HEAD,
    branch: "gc/1",
    at: NOW,
  },
  then: [
    { type: "PrCreated", pr: pr(1), headSha: HEAD, branch: "gc/1", at: NOW },
  ],
})

spec("повторный отчёт о том же PR факта не порождает", {
  given: [created(1)],
  when: {
    type: "ReportPrCreated",
    pr: pr(1),
    headSha: HEAD,
    branch: "gc/1",
    at: NOW,
  },
  then: [],
})

// --- Наблюдение PR ----------------------------------------------------------

spec("первое наблюдение известного PR становится фактом", {
  given: [created(1)],
  when: observePr(1),
  then: [{ type: "PrStateChanged", pr: pr(1), snapshot: snapshot(), at: NOW }],
})

spec("тот же снимок повторно факта не порождает", {
  given: [created(1), prSeen(1)],
  when: observePr(1),
  then: [],
})

examples("изменившийся снимок становится фактом", [
  {
    given: [created(1), prSeen(1)],
    when: observePr(1, { headSha: "def" }),
    then: [
      {
        type: "PrStateChanged",
        pr: pr(1),
        snapshot: snapshot({ headSha: "def" }),
        at: NOW,
      },
    ],
  },
  {
    given: [created(1), prSeen(1)],
    when: observePr(1, { open: false }),
    then: [
      {
        type: "PrStateChanged",
        pr: pr(1),
        snapshot: snapshot({ open: false }),
        at: NOW,
      },
    ],
  },
  {
    given: [created(1), prSeen(1)],
    when: observePr(1, { draft: true }),
    then: [
      {
        type: "PrStateChanged",
        pr: pr(1),
        snapshot: snapshot({ draft: true }),
        at: NOW,
      },
    ],
  },
])

spec("чужой PR не наблюдаем — мы следим только за своими", {
  given: [],
  when: observePr(99),
  then: [],
})

// --- Наблюдение проверок ----------------------------------------------------

spec("первое наблюдение проверки становится фактом", {
  given: [created(1), prSeen(1)],
  when: observeCheck(1),
  then: [
    {
      type: "CheckRunStateChanged",
      pr: pr(1),
      headSha: HEAD,
      run: check(),
      at: NOW,
    },
  ],
})

spec("то же состояние проверки повторно факта не порождает", {
  given: [created(1), prSeen(1), checkSeen(1, check())],
  when: observeCheck(1),
  then: [],
})

spec("сменившийся результат проверки становится фактом", {
  given: [created(1), prSeen(1), checkSeen(1, check())],
  when: observeCheck(1, { conclusion: "failure" }),
  then: [
    {
      type: "CheckRunStateChanged",
      pr: pr(1),
      headSha: HEAD,
      run: check({ conclusion: "failure" }),
      at: NOW,
    },
  ],
})

spec("проверка чужого PR не наблюдается", {
  given: [],
  when: observeCheck(99),
  then: [],
})

// --- Отчёты воркера и эффектов ----------------------------------------------

spec("отчёт о заходе агента становится фактом", {
  given: [created(1)],
  when: {
    type: "ReportAgentRun",
    pr: pr(1),
    outcome: "finished",
    usage: { inputTokens: 10, outputTokens: 20, reasoningTokens: 30 },
    at: NOW,
  },
  then: [
    {
      type: "AgentRunFinished",
      pr: pr(1),
      outcome: "finished",
      usage: { inputTokens: 10, outputTokens: 20, reasoningTokens: 30 },
      at: NOW,
    },
  ],
})

spec("отчёт о провале мержа становится фактом", {
  given: [created(1)],
  when: {
    type: "ReportMergeFailure",
    pr: pr(1),
    headSha: HEAD,
    failure: "head_changed",
    at: NOW,
  },
  then: [
    {
      type: "MergeFailed",
      pr: pr(1),
      headSha: HEAD,
      failure: "head_changed",
      at: NOW,
    },
  ],
})

spec("оборванный запрос становится фактом", {
  given: [],
  when: {
    type: "ReportLookupFailure",
    scope: { kind: "inventory", tenant: TENANT, repo: REPO },
    failure: "rate_limited",
    at: NOW,
  },
  then: [
    {
      type: "LookupIncomplete",
      scope: { kind: "inventory", tenant: TENANT, repo: REPO },
      failure: "rate_limited",
      at: NOW,
    },
  ],
})

// --- Приём меняет то, что видит admission -----------------------------------

sequence(
  "закрытие, пришедшее наблюдением, освобождает место в бэклоге",
  [created(1), prSeen(1)],
  [
    {
      when: observePr(1, { open: false }),
      then: [
        {
          type: "PrStateChanged",
          pr: pr(1),
          snapshot: snapshot({ open: false }),
          at: NOW,
        },
      ],
    },
    {
      when: {
        type: "TickProduction",
        tenant: TENANT,
        repo: REPO,
        now: NOW,
      },
      then: [
        { type: "AdmissionApproved", tenant: TENANT, repo: REPO, at: NOW },
      ],
    },
  ]
)
