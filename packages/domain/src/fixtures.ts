import type {
  CheckRun,
  DomainEvent,
  LookupFailure,
  PrRef,
  PrSnapshot,
  RepositorySettings,
  RequiredCheck,
  RequiredChecksSnapshot,
  SweepId,
  Timestamp,
} from "./contract"

export const TENANT = "acme"
export const REPO = "acme/app"
export const HOUR = 3_600_000
export const NOW: Timestamp = 1000 * HOUR
export const HEAD = "abc"
export const SWEEP: SweepId = "sweep-1"
export const APP_ID = 1001
export const tenantSettings = {
  model: "gpt-5.6-luna",
  reasoning: "xhigh",
  prompt: "Remove obsolete code",
} as const

export function pr(number: number): PrRef {
  return { tenant: TENANT, repo: REPO, number }
}

export function repositorySettings(
  overrides: Partial<RepositorySettings> = {}
): RepositorySettings {
  return {
    productionEveryHours: 1,
    openPrLimit: 12,
    closeAfterHours: 24,
    ...overrides,
  }
}

export function requiredCheck(
  context = "build",
  appId = APP_ID
): RequiredCheck {
  return { context, appId }
}

export function snapshot(overrides: Partial<PrSnapshot> = {}): PrSnapshot {
  const createdAt = overrides.createdAt ?? 0
  return {
    headSha: HEAD,
    baseRef: "main",
    draft: false,
    open: true,
    mergeable: true,
    createdAt,
    updatedAt: overrides.updatedAt ?? createdAt,
    ...overrides,
  }
}

export function check(overrides: Partial<CheckRun> = {}): CheckRun {
  const status = overrides.status ?? "completed"
  const createdAt = overrides.createdAt ?? 0
  const updatedAt = overrides.updatedAt ?? createdAt
  return {
    id: 777,
    check: requiredCheck(),
    status,
    conclusion:
      overrides.conclusion ?? (status === "completed" ? "success" : null),
    createdAt,
    updatedAt,
    startedAt:
      overrides.startedAt ??
      (status === "queued" || status === "requested" ? null : createdAt),
    completedAt:
      overrides.completedAt ?? (status === "completed" ? updatedAt : null),
    ...overrides,
  }
}

export function checksSnapshot(
  overrides: Partial<RequiredChecksSnapshot> = {}
): RequiredChecksSnapshot {
  return {
    required: [requiredCheck()],
    runs: [check()],
    ...overrides,
  }
}

export function prSeen(
  number: number,
  overrides: Partial<PrSnapshot> = {},
  at?: Timestamp
): DomainEvent {
  const snapshotValue = snapshot(overrides)
  return {
    type: "PrStateChanged",
    pr: pr(number),
    snapshot: snapshotValue,
    at: at ?? snapshotValue.updatedAt,
  }
}

export function checkSeen(
  number: number,
  run: CheckRun,
  headSha: string = HEAD,
  at?: Timestamp
): DomainEvent {
  return {
    type: "CheckRunStateChanged",
    pr: pr(number),
    headSha,
    run,
    at: at ?? run.updatedAt,
  }
}

export function requiredChecksSeen(
  number: number,
  snapshotValue: RequiredChecksSnapshot = checksSnapshot(),
  sweepId: SweepId = SWEEP,
  headSha: string = HEAD,
  at: Timestamp = NOW
): DomainEvent {
  return {
    type: "RequiredChecksObserved",
    reportId: `checks-${sweepId}-${number}`,
    sweepId,
    pr: pr(number),
    headSha,
    snapshot: snapshotValue,
    at,
  }
}

export function lookupIncomplete(
  number: number,
  failure: LookupFailure = "rate_limited",
  sweepId: SweepId = SWEEP,
  headSha: string = HEAD,
  at: Timestamp = NOW
): DomainEvent {
  return {
    type: "LookupIncomplete",
    reportId: `lookup-${sweepId}-${number}`,
    scope: { kind: "checks", sweepId, pr: pr(number), headSha },
    failure,
    at,
  }
}

export function mergeablePr(
  number: number,
  ageHours = 2,
  snapshotOverrides: Partial<PrSnapshot> = {},
  sweepId: SweepId = SWEEP
): readonly DomainEvent[] {
  const createdAt = NOW - ageHours * HOUR
  const observedAt = NOW - 1
  return [
    created(number, createdAt),
    prSeen(number, { createdAt, ...snapshotOverrides }, createdAt),
    requiredChecksSeen(
      number,
      checksSnapshot({
        runs: [check({ createdAt: observedAt, updatedAt: observedAt })],
      }),
      sweepId,
      HEAD,
      observedAt
    ),
  ]
}

export function created(number: number, at: Timestamp = 0): DomainEvent {
  return {
    type: "PrCreated",
    reportId: `pr-${number}`,
    pr: pr(number),
    headSha: HEAD,
    branch: `gc/${number}`,
    at,
  }
}

export function closedByObservation(
  number: number,
  at: Timestamp = 0
): DomainEvent {
  return {
    type: "PrStateChanged",
    pr: pr(number),
    snapshot: snapshot({ open: false, updatedAt: at }),
    at,
  }
}

export function openPrs(count: number): readonly DomainEvent[] {
  return Array.from({ length: count }, (_, index) => created(index + 1))
}
