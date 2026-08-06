export type Timestamp = number

export type PrRef = {
  readonly tenant: string
  readonly repo: string
  readonly number: number
}

export type CheckStatus = "queued" | "in_progress" | "completed"

export type CheckConclusion =
  | "success"
  | "failure"
  | "neutral"
  | "cancelled"
  | "timed_out"
  | "action_required"
  | "stale"
  | "skipped"

export type CheckRun = {
  readonly id: string
  readonly name: string
  readonly required: boolean
  readonly status: CheckStatus
  readonly conclusion: CheckConclusion | null
  readonly startedAt: Timestamp | null
  readonly completedAt: Timestamp | null
}

export type PrSnapshot = {
  readonly headSha: string
  readonly baseRef: string
  readonly draft: boolean
  readonly open: boolean
  readonly mergeable: boolean | null
  readonly createdAt: Timestamp
}

export type LookupScope =
  | { readonly kind: "checks"; readonly pr: PrRef; readonly headSha: string }
  | {
      readonly kind: "inventory"
      readonly tenant: string
      readonly repo: string
    }

export type LookupFailure =
  | "rate_limited"
  | "forbidden"
  | "upstream_error"
  | "page_limit"

export type AgentOutcome = "finished" | "failed" | "timed_out"

export type AgentUsage = {
  readonly inputTokens: number
  readonly outputTokens: number
  readonly reasoningTokens: number
}

export type AdmissionRefusal = {
  readonly kind: "backlog_full"
  readonly open: number
  readonly limit: number
}

export type MergeRefusal =
  | { readonly kind: "required_check_failed"; readonly check: string }
  | { readonly kind: "required_checks_pending"; readonly pending: number }
  | { readonly kind: "no_required_checks" }
  | { readonly kind: "not_mergeable" }
  | { readonly kind: "draft" }
  | { readonly kind: "evidence_for_other_head"; readonly headSha: string }

export type MergeFailure =
  | "head_changed"
  | "not_mergeable"
  | "forbidden"
  | "upstream_error"

export type DomainEvent =
  | {
      readonly type: "PrStateChanged"
      readonly pr: PrRef
      readonly snapshot: PrSnapshot
      readonly at: Timestamp
    }
  | {
      readonly type: "CheckRunStateChanged"
      readonly pr: PrRef
      readonly headSha: string
      readonly run: CheckRun
      readonly at: Timestamp
    }
  | {
      readonly type: "LookupIncomplete"
      readonly scope: LookupScope
      readonly failure: LookupFailure
      readonly at: Timestamp
    }
  | {
      readonly type: "PrCreated"
      readonly pr: PrRef
      readonly headSha: string
      readonly branch: string
      readonly at: Timestamp
    }
  | {
      readonly type: "AgentRunFinished"
      readonly pr: PrRef
      readonly outcome: AgentOutcome
      readonly usage: AgentUsage
      readonly at: Timestamp
    }
  | {
      readonly type: "MergeApproved"
      readonly pr: PrRef
      readonly headSha: string
      readonly at: Timestamp
    }
  | {
      readonly type: "MergeFailed"
      readonly pr: PrRef
      readonly headSha: string
      readonly failure: MergeFailure
      readonly at: Timestamp
    }
  | {
      readonly type: "CloseApproved"
      readonly pr: PrRef
      readonly ageHours: number
      readonly at: Timestamp
    }
  | {
      readonly type: "AdmissionApproved"
      readonly tenant: string
      readonly repo: string
      readonly at: Timestamp
    }
  | {
      readonly type: "AdmissionDeclined"
      readonly tenant: string
      readonly repo: string
      readonly refusal: AdmissionRefusal
      readonly at: Timestamp
    }
  | {
      readonly type: "MergeDeclined"
      readonly pr: PrRef
      readonly refusal: MergeRefusal
      readonly at: Timestamp
    }

export type DomainCommand =
  | {
      readonly type: "ObservePr"
      readonly pr: PrRef
      readonly snapshot: PrSnapshot
      readonly at: Timestamp
    }
  | {
      readonly type: "ObserveCheckRun"
      readonly pr: PrRef
      readonly headSha: string
      readonly run: CheckRun
      readonly at: Timestamp
    }
  | {
      readonly type: "ReportLookupFailure"
      readonly scope: LookupScope
      readonly failure: LookupFailure
      readonly at: Timestamp
    }
  | {
      readonly type: "ReportPrCreated"
      readonly pr: PrRef
      readonly headSha: string
      readonly branch: string
      readonly at: Timestamp
    }
  | {
      readonly type: "ReportAgentRun"
      readonly pr: PrRef
      readonly outcome: AgentOutcome
      readonly usage: AgentUsage
      readonly at: Timestamp
    }
  | {
      readonly type: "ReportMergeFailure"
      readonly pr: PrRef
      readonly headSha: string
      readonly failure: MergeFailure
      readonly at: Timestamp
    }
  | {
      readonly type: "TickProduction"
      readonly tenant: string
      readonly repo: string
      readonly now: Timestamp
    }
  | {
      readonly type: "TickSweep"
      readonly tenant: string
      readonly repo: string
      readonly now: Timestamp
    }

export type DomainError =
  | { readonly type: "UnknownPr"; readonly pr: PrRef }
  | { readonly type: "PrAlreadyClosed"; readonly pr: PrRef }
