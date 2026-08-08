import type { DomainCommand } from "./contract"
import { decider } from "./decider"
import {
  closedByObservation,
  created,
  HOUR,
  openPrs,
  REPO,
  repositorySettings,
  TENANT,
  tenantSettings,
} from "./fixtures"
import { createExecutableSpecSuite } from "./framework/runner"

const { examples, sequence, spec } = createExecutableSpecSuite({ decider })
const NOW = 1_000

function tick(openPrLimit = 12, now = NOW): DomainCommand {
  return {
    type: "TickProduction",
    tenant: TENANT,
    repo: REPO,
    settings: repositorySettings({ openPrLimit }),
    tenantSettings,
    now,
  }
}

function approved(at = NOW) {
  return {
    type: "AdmissionApproved",
    tenant: TENANT,
    repo: REPO,
    tenantSettings,
    at,
  } as const
}

function declined(open: number, limit = 12, at = NOW) {
  return {
    type: "AdmissionDeclined",
    tenant: TENANT,
    repo: REPO,
    refusal: { kind: "backlog_full", open, limit },
    at,
  } as const
}

spec("пустой бэклог пропускает работу", {
  given: [],
  when: tick(),
  then: [approved()],
})

examples("порог объёма", [
  { given: openPrs(11), when: tick(), then: [approved()] },
  { given: openPrs(12), when: tick(), then: [declined(12)] },
  { given: openPrs(13), when: tick(), then: [declined(13)] },
])

spec("закрытый PR освобождает место", {
  given: [...openPrs(12), closedByObservation(1)],
  when: tick(),
  then: [approved()],
})

spec("повторное создание того же PR не занимает два места", {
  given: [...openPrs(11), created(11)],
  when: tick(),
  then: [approved()],
})

spec("наблюдение открытого PR ничего не меняет", {
  given: [
    ...openPrs(12),
    {
      type: "PrStateChanged",
      pr: { tenant: TENANT, repo: REPO, number: 1 },
      snapshot: {
        headSha: "def",
        baseRef: "main",
        draft: false,
        open: true,
        mergeable: true,
        createdAt: 0,
        updatedAt: 500,
      },
      at: 500,
    },
  ],
  when: tick(),
  then: [declined(12)],
})

spec("закрытие двух PR освобождает два места", {
  given: [...openPrs(12), closedByObservation(1), closedByObservation(2)],
  when: tick(),
  then: [approved()],
})

spec("PR другого репозитория и тенанта не занимает backlog", {
  given: [
    ...openPrs(11),
    {
      type: "PrCreated",
      reportId: "foreign-pr",
      pr: { tenant: "globex", repo: "globex/other", number: 1 },
      headSha: "abc",
      branch: "gc/foreign",
      at: 0,
    },
  ],
  when: tick(),
  then: [approved()],
})

sequence("повторный тик при полном бэклоге отказывает так же", openPrs(12), [
  { when: tick(), then: [declined(12)] },
  {
    when: tick(12, NOW + HOUR),
    then: [declined(12, 12, NOW + HOUR)],
  },
])

sequence(
  "retry того же production tick не дублирует решение",
  [],
  [
    { when: tick(), then: [approved()] },
    { when: tick(), then: [] },
  ]
)

sequence("новый лимит действует со следующего тика", openPrs(9), [
  { when: tick(12), then: [approved()] },
  {
    when: tick(5, NOW + HOUR),
    then: [declined(9, 5, NOW + HOUR)],
  },
])

sequence(
  "одобрение не резервирует место для следующего тика",
  [],
  [
    { when: tick(), then: [approved()] },
    {
      when: tick(12, NOW + HOUR),
      then: [approved(NOW + HOUR)],
    },
  ]
)
