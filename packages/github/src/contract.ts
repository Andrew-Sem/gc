import type {
  LookupFailure,
  PrRef,
  ReportId,
  SweepId,
  Timestamp,
} from "@workspace/domain"

/** Одна доставка GitHub. `id` — значение X-GitHub-Delivery. */
export type WebhookDelivery = {
  readonly id: string
  readonly event: string
  readonly payload: unknown
}

/** Время приёма приходит извне; payload не считается часами домена. */
export type ParseContext = {
  readonly tenant: string
  readonly at: Timestamp
}

export type RequiredChecksLookup = {
  readonly reportId: ReportId
  readonly sweepId: SweepId
  readonly pr: PrRef
  readonly headSha: string
  readonly requiredChecks: unknown
  readonly checkRuns: unknown
  readonly requiredChecksComplete: boolean
  readonly checkRunsComplete: boolean
  readonly failure?: LookupFailure
  readonly at: Timestamp
}

export type ReconciliationSnapshot = {
  readonly pr: PrRef
  readonly headSha: string
  readonly pullRequest: unknown
  readonly checkRuns: unknown
  readonly at: Timestamp
}
