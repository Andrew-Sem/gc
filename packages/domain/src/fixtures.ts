import type { DomainEvent, PrRef, PrSnapshot, Timestamp } from "./contract"

export const TENANT = "acme"
export const REPO = "acme/app"

export function pr(number: number): PrRef {
  return { tenant: TENANT, repo: REPO, number }
}

export function snapshot(overrides: Partial<PrSnapshot> = {}): PrSnapshot {
  return {
    headSha: "abc",
    baseRef: "main",
    draft: false,
    open: true,
    mergeable: true,
    createdAt: 0,
    ...overrides,
  }
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
