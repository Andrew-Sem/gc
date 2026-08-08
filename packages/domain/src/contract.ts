export type Timestamp = number
export type ReportId = string
export type SweepId = string

export type PrRef = {
  readonly tenant: string
  readonly repo: string
  readonly number: number
}

export type RequiredCheck = {
  readonly context: string
  readonly appId: number
}

export type CheckStatus =
  | "queued"
  | "in_progress"
  | "completed"
  | "waiting"
  | "requested"
  | "pending"

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
  readonly id: number
  readonly check: RequiredCheck
  readonly status: CheckStatus
  readonly conclusion: CheckConclusion | null
  readonly createdAt: Timestamp
  readonly updatedAt: Timestamp
  readonly startedAt: Timestamp | null
  readonly completedAt: Timestamp | null
}

export type RequiredChecksSnapshot = {
  readonly required: readonly RequiredCheck[]
  readonly runs: readonly CheckRun[]
}

export type PrSnapshot = {
  readonly headSha: string
  readonly baseRef: string
  readonly draft: boolean
  readonly open: boolean
  readonly mergeable: boolean | null
  readonly createdAt: Timestamp
  readonly updatedAt: Timestamp
}

export type RepositorySettings = {
  readonly productionEveryHours: number
  readonly openPrLimit: number
  readonly closeAfterHours: number
}

export type TenantSettings = {
  readonly model: string
  readonly reasoning: "low" | "medium" | "high" | "xhigh"
  readonly prompt: string
}

export type LookupScope =
  | {
      readonly kind: "checks"
      readonly sweepId: SweepId
      readonly pr: PrRef
      readonly headSha: string
    }
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
export type CodexConnectionMethod = "api_key" | "subscription"

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
  | { readonly kind: "required_check_failed"; readonly check: RequiredCheck }
  | { readonly kind: "required_checks_pending"; readonly pending: number }
  | { readonly kind: "required_checks_unknown" }
  | { readonly kind: "lookup_incomplete"; readonly failure: LookupFailure }
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
      readonly type: "RequiredChecksObserved"
      readonly reportId: ReportId
      readonly sweepId: SweepId
      readonly pr: PrRef
      readonly headSha: string
      readonly snapshot: RequiredChecksSnapshot
      readonly at: Timestamp
    }
  | {
      readonly type: "LookupIncomplete"
      readonly reportId: ReportId
      readonly scope: LookupScope
      readonly failure: LookupFailure
      readonly at: Timestamp
    }
  | {
      readonly type: "PrCreated"
      readonly reportId: ReportId
      readonly pr: PrRef
      readonly headSha: string
      readonly branch: string
      readonly at: Timestamp
    }
  | {
      readonly type: "AgentRunFinished"
      readonly reportId: ReportId
      readonly tenant: string
      readonly repo: string
      readonly pr: PrRef | null
      readonly outcome: AgentOutcome
      readonly usage: AgentUsage
      readonly at: Timestamp
    }
  | {
      readonly type: "MergeApproved"
      readonly sweepId: SweepId
      readonly pr: PrRef
      readonly headSha: string
      readonly at: Timestamp
    }
  | {
      readonly type: "MergeFailed"
      readonly reportId: ReportId
      readonly sweepId: SweepId
      readonly pr: PrRef
      readonly headSha: string
      readonly failure: MergeFailure
      readonly at: Timestamp
    }
  | {
      readonly type: "CloseApproved"
      readonly sweepId: SweepId
      readonly pr: PrRef
      readonly ageHours: number
      readonly at: Timestamp
    }
  | {
      readonly type: "AdmissionApproved"
      readonly tenant: string
      readonly repo: string
      readonly tenantSettings: TenantSettings
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
      readonly sweepId: SweepId
      readonly pr: PrRef
      readonly refusal: MergeRefusal
      readonly at: Timestamp
    }
  | {
      readonly type: "GitHubAppConnected"
      readonly reportId: ReportId
      readonly tenant: string
      readonly appId: number
      readonly installationId: number
      readonly at: Timestamp
    }
  | {
      readonly type: "RepositoryConnected"
      readonly reportId: ReportId
      readonly tenant: string
      readonly repo: string
      readonly at: Timestamp
    }
  | {
      readonly type: "CodexConnected"
      readonly reportId: ReportId
      readonly tenant: string
      readonly method: CodexConnectionMethod
      readonly at: Timestamp
    }
  | {
      readonly type: "TenantSettingsChanged"
      readonly reportId: ReportId
      readonly tenant: string
      readonly settings: TenantSettings
      readonly at: Timestamp
    }
  | {
      readonly type: "RepositorySettingsChanged"
      readonly reportId: ReportId
      readonly tenant: string
      readonly repo: string
      readonly settings: RepositorySettings
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
      readonly type: "ReportRequiredChecks"
      readonly reportId: ReportId
      readonly sweepId: SweepId
      readonly pr: PrRef
      readonly headSha: string
      readonly snapshot: RequiredChecksSnapshot
      readonly at: Timestamp
    }
  | {
      readonly type: "ReportLookupFailure"
      readonly reportId: ReportId
      readonly scope: LookupScope
      readonly failure: LookupFailure
      readonly at: Timestamp
    }
  | {
      readonly type: "ReportPrCreated"
      readonly reportId: ReportId
      readonly pr: PrRef
      readonly headSha: string
      readonly branch: string
      readonly at: Timestamp
    }
  | {
      readonly type: "ReportAgentRun"
      readonly reportId: ReportId
      readonly tenant: string
      readonly repo: string
      readonly pr: PrRef | null
      readonly outcome: AgentOutcome
      readonly usage: AgentUsage
      readonly at: Timestamp
    }
  | {
      readonly type: "ReportMergeFailure"
      readonly reportId: ReportId
      readonly sweepId: SweepId
      readonly pr: PrRef
      readonly headSha: string
      readonly failure: MergeFailure
      readonly at: Timestamp
    }
  | {
      readonly type: "ReportGitHubAppConnected"
      readonly reportId: ReportId
      readonly tenant: string
      readonly appId: number
      readonly installationId: number
      readonly at: Timestamp
    }
  | {
      readonly type: "ReportRepositoryConnected"
      readonly reportId: ReportId
      readonly tenant: string
      readonly repo: string
      readonly at: Timestamp
    }
  | {
      readonly type: "ReportCodexConnected"
      readonly reportId: ReportId
      readonly tenant: string
      readonly method: CodexConnectionMethod
      readonly at: Timestamp
    }
  | {
      readonly type: "ReportTenantSettingsChanged"
      readonly reportId: ReportId
      readonly tenant: string
      readonly settings: TenantSettings
      readonly at: Timestamp
    }
  | {
      readonly type: "ReportRepositorySettingsChanged"
      readonly reportId: ReportId
      readonly tenant: string
      readonly repo: string
      readonly settings: RepositorySettings
      readonly at: Timestamp
    }
  | {
      readonly type: "TickProduction"
      readonly tenant: string
      readonly repo: string
      readonly settings: RepositorySettings
      readonly tenantSettings: TenantSettings
      readonly now: Timestamp
    }
  | {
      readonly type: "TickSweep"
      readonly sweepId: SweepId
      readonly tenant: string
      readonly repo: string
      readonly settings: RepositorySettings
      readonly now: Timestamp
    }

/**
 * `decide` не отклоняет команды. Неприменимая, дублирующая или относящаяся к
 * чужому PR команда — это `{ ok: true, events: [] }`, а не ошибка.
 */
export type DomainError = never

export type Effect =
  | {
      readonly type: "RunAgent"
      readonly tenant: string
      readonly repo: string
      readonly tenantSettings: TenantSettings
    }
  | {
      readonly type: "MergePr"
      readonly sweepId: SweepId
      readonly pr: PrRef
      readonly headSha: string
    }
  | {
      readonly type: "ClosePr"
      readonly sweepId: SweepId
      readonly pr: PrRef
    }
  | {
      readonly type: "ReconcilePr"
      readonly pr: PrRef
      readonly headSha: string
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
  { name: "production", intervalMs: HOUR_MS, target: "wakeUp:production" },
  { name: "sweep", intervalMs: 6 * HOUR_MS, target: "wakeUp:sweep" },
]

export type RouteSpec = {
  readonly path: string
  readonly method: "GET" | "POST"
}

export const EXPECTED_ROUTES: readonly RouteSpec[] = [
  { path: "/github/webhook", method: "POST" },
  { path: "/worker/protocol", method: "POST" },
  { path: "/health/live", method: "GET" },
  { path: "/health/ready", method: "GET" },
]

export type SandboxMode = "read-only" | "workspace-write" | "danger-full-access"

export type CodexSettings = {
  readonly model: string
  readonly reasoning: TenantSettings["reasoning"]
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
