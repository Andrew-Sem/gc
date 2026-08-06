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

/**
 * `decide` не отклоняет команды. Неприменимая, дублирующая или относящаяся к
 * чужому PR команда — это `{ ok: true, events: [] }`, а не ошибка. Вариантов
 * здесь нет намеренно: если появится первый, это изменение семантики, а не
 * добавление кода.
 */
export type DomainError = never

export type Effect =
  | {
      readonly type: "RunAgent"
      readonly tenant: string
      readonly repo: string
    }
  | {
      readonly type: "MergePr"
      readonly pr: PrRef
      readonly headSha: string
    }
  | {
      readonly type: "ClosePr"
      readonly pr: PrRef
    }

export type Verdict = "will_merge" | "waiting" | "will_close"

export type PrStatusLine = {
  readonly pr: PrRef
  readonly headSha: string
  readonly verdict: Verdict
  readonly reason: MergeRefusal | null
  readonly ageHours: number
  readonly hoursUntilClose: number
}

export const HOUR_MS = 3_600_000

export type CronSpec = {
  readonly name: string
  readonly intervalMs: number
  readonly target: string
}

export const EXPECTED_CRONS: readonly CronSpec[] = [
  { name: "production", intervalMs: HOUR_MS, target: "tick:production" },
  { name: "sweep", intervalMs: 6 * HOUR_MS, target: "tick:sweep" },
]

export type RouteSpec = {
  readonly path: string
  readonly method: "GET" | "POST"
}

export const EXPECTED_ROUTES: readonly RouteSpec[] = [
  { path: "/github/webhook", method: "POST" },
]

export type SandboxMode = "read-only" | "workspace-write" | "danger-full-access"

export type CodexSettings = {
  readonly model: string
  readonly reasoning: "low" | "medium" | "high" | "xhigh"
  readonly checkoutPath: string
}

export type CodexOptions = {
  readonly model: string
  readonly modelReasoningEffort: CodexSettings["reasoning"]
  readonly workingDirectory: string
  readonly sandboxMode: SandboxMode
  readonly networkAccessEnabled: boolean
  readonly skipGitRepoCheck: boolean
}
