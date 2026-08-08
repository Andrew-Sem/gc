import type { DomainCommand, DomainEvent } from "./contract"
import { decider } from "./decider"
import {
  check,
  checkSeen,
  checksSnapshot,
  created,
  HEAD,
  NOW,
  pr,
  prSeen,
  REPO,
  repositorySettings,
  SWEEP,
  snapshot,
  TENANT,
  tenantSettings,
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

function reportPrCreated(reportId = "report-pr-1"): DomainCommand {
  return {
    type: "ReportPrCreated",
    reportId,
    pr: pr(1),
    headSha: HEAD,
    branch: "gc/1",
    at: NOW,
  }
}

function prCreated(reportId = "report-pr-1"): DomainEvent {
  return {
    type: "PrCreated",
    reportId,
    pr: pr(1),
    headSha: HEAD,
    branch: "gc/1",
    at: NOW,
  }
}

spec("отчёт о созданном PR становится фактом", {
  given: [],
  when: reportPrCreated(),
  then: [prCreated()],
})

examples("повторное создание известного PR факта не порождает", [
  {
    given: [prCreated()],
    when: reportPrCreated(),
    then: [],
  },
  {
    given: [prCreated()],
    when: reportPrCreated("other-report"),
    then: [],
  },
])

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

spec("поздняя доставка старого снимка PR не откатывает состояние", {
  given: [created(1), prSeen(1, { open: false, updatedAt: 200 })],
  when: observePr(1, { open: true, updatedAt: 100 }),
  then: [],
})

examples("изменившийся снимок становится фактом", [
  {
    given: [created(1), prSeen(1)],
    when: observePr(1, { headSha: "def", updatedAt: 1 }),
    then: [
      {
        type: "PrStateChanged",
        pr: pr(1),
        snapshot: snapshot({ headSha: "def", updatedAt: 1 }),
        at: NOW,
      },
    ],
  },
  {
    given: [created(1), prSeen(1)],
    when: observePr(1, { open: false, updatedAt: 1 }),
    then: [
      {
        type: "PrStateChanged",
        pr: pr(1),
        snapshot: snapshot({ open: false, updatedAt: 1 }),
        at: NOW,
      },
    ],
  },
  {
    given: [created(1), prSeen(1)],
    when: observePr(1, { draft: true, updatedAt: 1 }),
    then: [
      {
        type: "PrStateChanged",
        pr: pr(1),
        snapshot: snapshot({ draft: true, updatedAt: 1 }),
        at: NOW,
      },
    ],
  },
])

spec("конфликтующий снимок PR с тем же updatedAt считается старым", {
  given: [created(1), prSeen(1)],
  when: observePr(1, { open: false }),
  then: [],
})

spec("чужой PR не наблюдаем — мы следим только за своими", {
  given: [],
  when: observePr(99),
  then: [],
})

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

spec("поздняя доставка старого состояния check run не откатывает результат", {
  given: [
    created(1),
    prSeen(1),
    checkSeen(1, check({ conclusion: "failure", updatedAt: 200 })),
  ],
  when: observeCheck(1, {
    status: "in_progress",
    conclusion: null,
    updatedAt: 100,
  }),
  then: [],
})

spec("сменившийся результат проверки становится фактом", {
  given: [created(1), prSeen(1), checkSeen(1, check())],
  when: observeCheck(1, { conclusion: "failure", updatedAt: 1 }),
  then: [
    {
      type: "CheckRunStateChanged",
      pr: pr(1),
      headSha: HEAD,
      run: check({ conclusion: "failure", updatedAt: 1 }),
      at: NOW,
    },
  ],
})

spec("конфликтующее состояние check run с тем же updatedAt считается старым", {
  given: [created(1), prSeen(1), checkSeen(1, check())],
  when: observeCheck(1, { conclusion: "failure" }),
  then: [],
})

spec("проверка чужого PR не наблюдается", {
  given: [],
  when: observeCheck(99),
  then: [],
})

const reportChecks: DomainCommand = {
  type: "ReportRequiredChecks",
  reportId: "checks-1",
  sweepId: SWEEP,
  pr: pr(1),
  headSha: HEAD,
  snapshot: checksSnapshot(),
  at: NOW,
}

const checksObserved: DomainEvent = {
  type: "RequiredChecksObserved",
  reportId: "checks-1",
  sweepId: SWEEP,
  pr: pr(1),
  headSha: HEAD,
  snapshot: checksSnapshot(),
  at: NOW,
}

const checksLookupFailed: DomainEvent = {
  type: "LookupIncomplete",
  reportId: "lookup-checks-1",
  scope: { kind: "checks", sweepId: SWEEP, pr: pr(1), headSha: HEAD },
  failure: "rate_limited",
  at: NOW,
}

spec("полный снимок обязательных проверок становится фактом как есть", {
  given: [created(1), prSeen(1)],
  when: reportChecks,
  then: [checksObserved],
})

examples("полный снимок одной попытки свипа терминален", [
  {
    given: [created(1), prSeen(1), checksObserved],
    when: reportChecks,
    then: [],
  },
  {
    given: [created(1), prSeen(1), checksObserved],
    when: { ...reportChecks, reportId: "checks-2" },
    then: [],
  },
  {
    given: [created(1), prSeen(1), checksObserved],
    when: {
      type: "ReportLookupFailure",
      reportId: "lookup-2",
      scope: { kind: "checks", sweepId: SWEEP, pr: pr(1), headSha: HEAD },
      failure: "rate_limited",
      at: NOW,
    },
    then: [],
  },
])

examples("оборванный lookup одной попытки свипа тоже терминален", [
  {
    given: [created(1), prSeen(1), checksLookupFailed],
    when: {
      type: "ReportRequiredChecks",
      reportId: "checks-after-failure",
      sweepId: SWEEP,
      pr: pr(1),
      headSha: HEAD,
      snapshot: checksSnapshot(),
      at: NOW,
    },
    then: [],
  },
  {
    given: [created(1), prSeen(1), checksLookupFailed],
    when: {
      type: "ReportLookupFailure",
      reportId: "other-lookup",
      scope: { kind: "checks", sweepId: SWEEP, pr: pr(1), headSha: HEAD },
      failure: "forbidden",
      at: NOW,
    },
    then: [],
  },
])

examples("терминальность evidence одного свипа относится к PR, а не к head", [
  {
    given: [created(1), prSeen(1), checksObserved],
    when: {
      ...reportChecks,
      reportId: "checks-new-head",
      headSha: "new",
    },
    then: [],
  },
  {
    given: [created(1), prSeen(1), checksLookupFailed],
    when: {
      ...reportChecks,
      reportId: "checks-new-head",
      headSha: "new",
    },
    then: [],
  },
  {
    given: [created(1), prSeen(1), checksObserved],
    when: {
      type: "ReportLookupFailure",
      reportId: "lookup-new-head",
      scope: { kind: "checks", sweepId: SWEEP, pr: pr(1), headSha: "new" },
      failure: "rate_limited",
      at: NOW,
    },
    then: [],
  },
])

sequence(
  "терминальный evidence одного sweepId принимается отдельно для каждого PR",
  [created(1), prSeen(1), created(2), prSeen(2)],
  [
    { when: reportChecks, then: [checksObserved] },
    {
      when: {
        ...reportChecks,
        reportId: "checks-pr-2",
        pr: pr(2),
      },
      then: [
        {
          ...checksObserved,
          reportId: "checks-pr-2",
          pr: pr(2),
        },
      ],
    },
  ]
)

spec("снимок проверок чужого PR не принимается", {
  given: [],
  when: reportChecks,
  then: [],
})

const usage = { inputTokens: 10, outputTokens: 20, reasoningTokens: 30 }

function reportAgent(
  reportId: string,
  prValue: ReturnType<typeof pr> | null = pr(1),
  outcome: "finished" | "failed" = "finished"
): DomainCommand {
  return {
    type: "ReportAgentRun",
    reportId,
    tenant: TENANT,
    repo: REPO,
    pr: prValue,
    outcome,
    usage,
    at: NOW,
  }
}

function agentFinished(
  reportId: string,
  prValue: ReturnType<typeof pr> | null = pr(1),
  outcome: "finished" | "failed" = "finished"
): DomainEvent {
  return {
    type: "AgentRunFinished",
    reportId,
    tenant: TENANT,
    repo: REPO,
    pr: prValue,
    outcome,
    usage,
    at: NOW,
  }
}

sequence(
  "повтор reportId отбрасывается, разные заходы не схлопываются",
  [created(1)],
  [
    { when: reportAgent("agent-1"), then: [agentFinished("agent-1")] },
    { when: reportAgent("agent-1"), then: [] },
    { when: reportAgent("agent-2"), then: [agentFinished("agent-2")] },
  ]
)

sequence(
  "reportId отбрасывает retry независимо от изменившегося payload",
  [],
  [
    {
      when: reportAgent("agent-retry", null, "failed"),
      then: [agentFinished("agent-retry", null, "failed")],
    },
    {
      when: {
        type: "ReportAgentRun",
        reportId: "agent-retry",
        tenant: TENANT,
        repo: REPO,
        pr: null,
        outcome: "finished",
        usage: { inputTokens: 99, outputTokens: 88, reasoningTokens: 77 },
        at: NOW + 1,
      },
      then: [],
    },
  ]
)

spec("провал агента до создания PR журналируется без выдуманного номера", {
  given: [],
  when: reportAgent("agent-before-pr", null, "failed"),
  then: [agentFinished("agent-before-pr", null, "failed")],
})

const reportMergeFailure: DomainCommand = {
  type: "ReportMergeFailure",
  reportId: "merge-1",
  sweepId: SWEEP,
  pr: pr(1),
  headSha: HEAD,
  failure: "head_changed",
  at: NOW,
}

const mergeFailed: DomainEvent = {
  type: "MergeFailed",
  reportId: "merge-1",
  sweepId: SWEEP,
  pr: pr(1),
  headSha: HEAD,
  failure: "head_changed",
  at: NOW,
}

const mergeApproved: DomainEvent = {
  type: "MergeApproved",
  sweepId: SWEEP,
  pr: pr(1),
  headSha: HEAD,
  at: NOW,
}

sequence(
  "один reportId допустим для отчётов разных типов",
  [created(1), mergeApproved],
  [
    {
      when: reportAgent("shared-report"),
      then: [agentFinished("shared-report")],
    },
    {
      when: { ...reportMergeFailure, reportId: "shared-report" },
      then: [{ ...mergeFailed, reportId: "shared-report" }],
    },
  ]
)

sequence(
  "повтор отчёта о провале мержа отбрасывается по reportId",
  [created(1), mergeApproved],
  [
    { when: reportMergeFailure, then: [mergeFailed] },
    { when: reportMergeFailure, then: [] },
  ]
)

const reportLookupFailure: DomainCommand = {
  type: "ReportLookupFailure",
  reportId: "lookup-1",
  scope: { kind: "inventory", tenant: TENANT, repo: REPO },
  failure: "rate_limited",
  at: NOW,
}

const lookupFailed: DomainEvent = {
  type: "LookupIncomplete",
  reportId: "lookup-1",
  scope: { kind: "inventory", tenant: TENANT, repo: REPO },
  failure: "rate_limited",
  at: NOW,
}

sequence(
  "повтор отчёта об оборванном lookup отбрасывается по reportId",
  [],
  [
    { when: reportLookupFailure, then: [lookupFailed] },
    { when: reportLookupFailure, then: [] },
  ]
)

const setupReports = [
  {
    given: [],
    when: {
      type: "ReportGitHubAppConnected",
      reportId: "github-1",
      tenant: TENANT,
      appId: 7,
      installationId: 42,
      at: NOW,
    },
    then: [
      {
        type: "GitHubAppConnected",
        reportId: "github-1",
        tenant: TENANT,
        appId: 7,
        installationId: 42,
        at: NOW,
      },
    ],
  },
  {
    given: [],
    when: {
      type: "ReportRepositoryConnected",
      reportId: "repo-1",
      tenant: TENANT,
      repo: REPO,
      at: NOW,
    },
    then: [
      {
        type: "RepositoryConnected",
        reportId: "repo-1",
        tenant: TENANT,
        repo: REPO,
        at: NOW,
      },
    ],
  },
  {
    given: [],
    when: {
      type: "ReportCodexConnected",
      reportId: "codex-1",
      tenant: TENANT,
      method: "subscription",
      at: NOW,
    },
    then: [
      {
        type: "CodexConnected",
        reportId: "codex-1",
        tenant: TENANT,
        method: "subscription",
        at: NOW,
      },
    ],
  },
  {
    given: [],
    when: {
      type: "ReportTenantSettingsChanged",
      reportId: "tenant-settings-1",
      tenant: TENANT,
      settings: tenantSettings,
      at: NOW,
    },
    then: [
      {
        type: "TenantSettingsChanged",
        reportId: "tenant-settings-1",
        tenant: TENANT,
        settings: tenantSettings,
        at: NOW,
      },
    ],
  },
  {
    given: [],
    when: {
      type: "ReportRepositorySettingsChanged",
      reportId: "repo-settings-1",
      tenant: TENANT,
      repo: REPO,
      settings: repositorySettings({ openPrLimit: 5 }),
      at: NOW,
    },
    then: [
      {
        type: "RepositorySettingsChanged",
        reportId: "repo-settings-1",
        tenant: TENANT,
        repo: REPO,
        settings: repositorySettings({ openPrLimit: 5 }),
        at: NOW,
      },
    ],
  },
] as const

examples("настройка и подключения журналируются без секретов", setupReports)

for (const [index, report] of setupReports.entries()) {
  sequence(
    `повтор setup reportId отбрасывается [${index + 1}]`,
    [],
    [
      { when: report.when, then: report.then },
      { when: report.when, then: [] },
    ]
  )
}

sequence(
  "закрытие, пришедшее наблюдением, освобождает место в бэклоге",
  [created(1), prSeen(1)],
  [
    {
      when: observePr(1, { open: false, updatedAt: 1 }),
      then: [
        {
          type: "PrStateChanged",
          pr: pr(1),
          snapshot: snapshot({ open: false, updatedAt: 1 }),
          at: NOW,
        },
      ],
    },
    {
      when: {
        type: "TickProduction",
        tenant: TENANT,
        repo: REPO,
        settings: repositorySettings(),
        tenantSettings,
        now: NOW,
      },
      then: [
        {
          type: "AdmissionApproved",
          tenant: TENANT,
          repo: REPO,
          tenantSettings,
          at: NOW,
        },
      ],
    },
  ]
)
