import type {
  CheckRun,
  DomainEvent,
  PrRef,
  PrSnapshot,
  Timestamp,
} from "./contract"

export const TENANT = "acme"
export const REPO = "acme/app"

export const HOUR = 3_600_000
export const NOW: Timestamp = 1000 * HOUR
export const HEAD = "abc"

export function pr(number: number): PrRef {
  return { tenant: TENANT, repo: REPO, number }
}

export function snapshot(overrides: Partial<PrSnapshot> = {}): PrSnapshot {
  return {
    headSha: HEAD,
    baseRef: "main",
    draft: false,
    open: true,
    mergeable: true,
    createdAt: 0,
    ...overrides,
  }
}

export function check(overrides: Partial<CheckRun> = {}): CheckRun {
  return {
    id: overrides.name ?? "build",
    name: "build",
    required: true,
    status: "completed",
    conclusion: "success",
    startedAt: 0,
    completedAt: 0,
    ...overrides,
  }
}

export function prSeen(
  number: number,
  overrides: Partial<PrSnapshot> = {},
  at: Timestamp = 0
): DomainEvent {
  return {
    type: "PrStateChanged",
    pr: pr(number),
    snapshot: snapshot(overrides),
    at,
  }
}

export function checkSeen(
  number: number,
  run: CheckRun,
  headSha: string = HEAD,
  at: Timestamp = 0
): DomainEvent {
  return {
    type: "CheckRunStateChanged",
    pr: pr(number),
    headSha,
    run,
    at,
  }
}

/**
 * PR, готовый к мержу: заведён, наблюдался открытым, обязательная проверка
 * прошла на текущем head. Возраст задаётся через ageHours.
 */
export function mergeablePr(
  number: number,
  ageHours = 2,
  snapshotOverrides: Partial<PrSnapshot> = {}
): readonly DomainEvent[] {
  return [
    created(number),
    prSeen(number, { createdAt: NOW - ageHours * HOUR, ...snapshotOverrides }),
    checkSeen(number, check()),
  ]
}

export function created(number: number, at: Timestamp = 0): DomainEvent {
  return {
    type: "PrCreated",
    pr: pr(number),
    headSha: "abc",
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
    snapshot: snapshot({ open: false }),
    at,
  }
}

export function openPrs(count: number): readonly DomainEvent[] {
  return Array.from({ length: count }, (_, index) => created(index + 1))
}
