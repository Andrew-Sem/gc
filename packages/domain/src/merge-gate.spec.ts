import type { DomainCommand, MergeRefusal } from "./contract"
import { decider } from "./decider"
import {
  check,
  checkSeen,
  created,
  HEAD,
  HOUR,
  mergeablePr,
  NOW,
  pr,
  prSeen,
  REPO,
  TENANT,
} from "./fixtures"
import { createExecutableSpecSuite } from "./framework/runner"

const { examples, spec } = createExecutableSpecSuite({ decider })

const sweep: DomainCommand = {
  type: "TickSweep",
  tenant: TENANT,
  repo: REPO,
  now: NOW,
}

function approved(number: number) {
  return {
    type: "MergeApproved",
    pr: pr(number),
    headSha: HEAD,
    at: NOW,
  } as const
}

function declined(number: number, refusal: MergeRefusal) {
  return { type: "MergeDeclined", pr: pr(number), refusal, at: NOW } as const
}

function closed(number: number, ageHours: number) {
  return { type: "CloseApproved", pr: pr(number), ageHours, at: NOW } as const
}

spec("свип без открытых PR ничего не решает", {
  given: [],
  when: sweep,
  then: [],
})

spec("все четыре условия сошлись — мержим", {
  given: [...mergeablePr(1)],
  when: sweep,
  then: [approved(1)],
})

examples("одно непройденное условие — отказ с причиной", [
  {
    given: [...mergeablePr(1), prSeen(1, { draft: true })],
    when: sweep,
    then: [declined(1, { kind: "draft" })],
  },
  {
    given: [...mergeablePr(1), prSeen(1, { mergeable: false })],
    when: sweep,
    then: [declined(1, { kind: "not_mergeable" })],
  },
  {
    given: [...mergeablePr(1), checkSeen(1, check({ conclusion: "failure" }))],
    when: sweep,
    then: [declined(1, { kind: "required_check_failed", check: "build" })],
  },
  {
    given: [
      ...mergeablePr(1),
      checkSeen(1, check({ status: "in_progress", conclusion: null })),
    ],
    when: sweep,
    then: [declined(1, { kind: "required_checks_pending", pending: 1 })],
  },
])

examples("обязательных проверок нет вообще — не мержим", [
  {
    given: [created(1), prSeen(1, { createdAt: NOW - 2 * HOUR })],
    when: sweep,
    then: [declined(1, { kind: "no_required_checks" })],
  },
  {
    given: [
      created(1),
      prSeen(1, { createdAt: NOW - 2 * HOUR }),
      checkSeen(1, check({ required: false, name: "preview" })),
    ],
    when: sweep,
    then: [declined(1, { kind: "no_required_checks" })],
  },
])

spec("зелёное относится к другому коммиту — не мержим", {
  given: [
    ...mergeablePr(1),
    prSeen(1, { headSha: "def", createdAt: NOW - 2 * HOUR }),
  ],
  when: sweep,
  then: [declined(1, { kind: "evidence_for_other_head", headSha: "def" })],
})

spec("упавшая необязательная проверка мержу не мешает", {
  given: [
    ...mergeablePr(1),
    checkSeen(
      1,
      check({ name: "preview", required: false, conclusion: "failure" })
    ),
  ],
  when: sweep,
  then: [approved(1)],
})

spec("PR старше суток и не мержится — закрываем", {
  given: [
    ...mergeablePr(1, 25),
    prSeen(1, { draft: true, createdAt: NOW - 25 * HOUR }),
  ],
  when: sweep,
  then: [closed(1, 25)],
})

spec("PR старше суток, но мержится — мерж важнее закрытия", {
  given: [...mergeablePr(1, 25)],
  when: sweep,
  then: [approved(1)],
})

spec("несколько PR решаются за один свип, по возрастанию номера", {
  given: [
    ...mergeablePr(1),
    ...mergeablePr(2),
    prSeen(2, { mergeable: false, createdAt: NOW - 2 * HOUR }),
  ],
  when: sweep,
  then: [approved(1), declined(2, { kind: "not_mergeable" })],
})

spec("закрытый PR в свипе не участвует", {
  given: [...mergeablePr(1), prSeen(1, { open: false })],
  when: sweep,
  then: [],
})
