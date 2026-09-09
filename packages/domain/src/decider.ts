import type {
  CheckRun,
  DomainCommand,
  DomainError,
  DomainEvent,
  LookupFailure,
  PrRef,
  PrSnapshot,
  RequiredChecksSnapshot,
  SweepId,
} from "./contract"
import { HOUR_MS } from "./contract"
import { accept, type Decider, type Decision } from "./framework/runner"
import { mergeRefusal } from "./merge-rules"

type SweepEvidence =
  | {
      readonly sweepId: SweepId
      readonly kind: "complete"
      readonly headSha: string
      readonly snapshot: RequiredChecksSnapshot
    }
  | {
      readonly sweepId: SweepId
      readonly kind: "incomplete"
      readonly headSha: string
      readonly failure: LookupFailure
    }

type TrackedRun = {
  readonly headSha: string
  readonly run: CheckRun
}

type TrackedPr = {
  readonly pr: PrRef
  readonly createdHeadSha: string
  readonly createdAt: number
  readonly snapshot: PrSnapshot | null
  readonly runs: readonly TrackedRun[]
  readonly evidence: readonly SweepEvidence[]
}

export type GcState = {
  readonly prs: readonly TrackedPr[]
  readonly reportKeys: readonly string[]
  readonly productionTicks: readonly string[]
  readonly sweepDecisions: readonly string[]
  readonly mergeApprovals: readonly string[]
}

export const initialState: GcState = {
  prs: [],
  reportKeys: [],
  productionTicks: [],
  sweepDecisions: [],
  mergeApprovals: [],
}

function prKey(pr: PrRef): string {
  return `${pr.tenant}\u0000${pr.repo}\u0000${pr.number}`
}

function samePr(left: PrRef, right: PrRef): boolean {
  return prKey(left) === prKey(right)
}

function reportKey(event: DomainEvent): string | null {
  switch (event.type) {
    case "RequiredChecksObserved":
    case "LookupIncomplete":
    case "PrCreated":
    case "AgentRunFinished":
    case "MergeFailed":
    case "GitHubAppConnected":
    case "RepositoryConnected":
    case "CodexConnected":
    case "TenantSettingsChanged":
    case "RepositorySettingsChanged":
      return `${event.type}\u0000${event.reportId}`
    default:
      return null
  }
}

function commandReportKey(command: DomainCommand): string | null {
  switch (command.type) {
    case "ReportRequiredChecks":
      return `RequiredChecksObserved\u0000${command.reportId}`
    case "ReportLookupFailure":
      return `LookupIncomplete\u0000${command.reportId}`
    case "ReportPrCreated":
      return `PrCreated\u0000${command.reportId}`
    case "ReportAgentRun":
      return `AgentRunFinished\u0000${command.reportId}`
    case "ReportMergeFailure":
      return `MergeFailed\u0000${command.reportId}`
    case "ReportGitHubAppConnected":
      return `GitHubAppConnected\u0000${command.reportId}`
    case "ReportRepositoryConnected":
      return `RepositoryConnected\u0000${command.reportId}`
    case "ReportCodexConnected":
      return `CodexConnected\u0000${command.reportId}`
    case "ReportTenantSettingsChanged":
      return `TenantSettingsChanged\u0000${command.reportId}`
    case "ReportRepositorySettingsChanged":
      return `RepositorySettingsChanged\u0000${command.reportId}`
    default:
      return null
  }
}

function sweepPrKey(sweepId: SweepId, pr: PrRef): string {
  return `${sweepId}\u0000${prKey(pr)}`
}

function mergeApprovalKey(
  sweepId: SweepId,
  pr: PrRef,
  headSha: string
): string {
  return `${sweepPrKey(sweepId, pr)}\u0000${headSha}`
}

function productionTickKey(tenant: string, repo: string, at: number): string {
  return `${tenant}\u0000${repo}\u0000${at}`
}

function updatePr(
  state: GcState,
  pr: PrRef,
  update: (current: TrackedPr) => TrackedPr
): GcState {
  return {
    ...state,
    prs: state.prs.map((current) =>
      samePr(current.pr, pr) ? update(current) : current
    ),
  }
}

function knownPr(state: GcState, pr: PrRef): TrackedPr | undefined {
  return state.prs.find((candidate) => samePr(candidate.pr, pr))
}

function upsertRun(
  runs: readonly TrackedRun[],
  headSha: string,
  run: CheckRun
): readonly TrackedRun[] {
  const index = runs.findIndex(
    (known) => known.headSha === headSha && known.run.id === run.id
  )
  if (index < 0) return [...runs, { headSha, run }]
  if (run.updatedAt <= runs[index].run.updatedAt) return runs
  return runs.map((known, knownIndex) =>
    knownIndex === index ? { headSha, run } : known
  )
}

function addReportKey(state: GcState, event: DomainEvent): GcState {
  const key = reportKey(event)
  return key === null || state.reportKeys.includes(key)
    ? state
    : { ...state, reportKeys: [...state.reportKeys, key] }
}

export function evolve(state: GcState, event: DomainEvent): GcState {
  let next = addReportKey(state, event)

  switch (event.type) {
    case "PrCreated": {
      if (knownPr(next, event.pr) !== undefined) return next
      return {
        ...next,
        prs: [
          ...next.prs,
          {
            pr: event.pr,
            createdHeadSha: event.headSha,
            createdAt: event.at,
            snapshot: null,
            runs: [],
            evidence: [],
          },
        ],
      }
    }
    case "PrStateChanged": {
      const current = knownPr(next, event.pr)
      if (
        current === undefined ||
        (current.snapshot !== null &&
          event.snapshot.updatedAt <= current.snapshot.updatedAt)
      ) {
        return next
      }
      return updatePr(next, event.pr, (pr) => ({
        ...pr,
        snapshot: event.snapshot,
      }))
    }
    case "CheckRunStateChanged": {
      if (knownPr(next, event.pr) === undefined) return next
      return updatePr(next, event.pr, (pr) => ({
        ...pr,
        runs: upsertRun(pr.runs, event.headSha, event.run),
      }))
    }
    case "RequiredChecksObserved": {
      if (knownPr(next, event.pr) === undefined) return next
      return updatePr(next, event.pr, (pr) => ({
        ...pr,
        runs: event.snapshot.runs.reduce(
          (runs, run) => upsertRun(runs, event.headSha, run),
          pr.runs
        ),
        evidence: [
          ...pr.evidence,
          {
            sweepId: event.sweepId,
            kind: "complete",
            headSha: event.headSha,
            snapshot: event.snapshot,
          },
        ],
      }))
    }
    case "LookupIncomplete": {
      const scope = event.scope
      if (scope.kind !== "checks") return next
      if (knownPr(next, scope.pr) === undefined) return next
      return updatePr(next, scope.pr, (pr) => ({
        ...pr,
        evidence: [
          ...pr.evidence,
          {
            sweepId: scope.sweepId,
            kind: "incomplete",
            headSha: scope.headSha,
            failure: event.failure,
          },
        ],
      }))
    }
    case "AdmissionApproved":
    case "AdmissionDeclined": {
      const key = productionTickKey(event.tenant, event.repo, event.at)
      return next.productionTicks.includes(key)
        ? next
        : { ...next, productionTicks: [...next.productionTicks, key] }
    }
    case "MergeApproved": {
      const key = sweepPrKey(event.sweepId, event.pr)
      const approvalKey = mergeApprovalKey(
        event.sweepId,
        event.pr,
        event.headSha
      )
      next = next.sweepDecisions.includes(key)
        ? next
        : { ...next, sweepDecisions: [...next.sweepDecisions, key] }
      return next.mergeApprovals.includes(approvalKey)
        ? next
        : {
            ...next,
            mergeApprovals: [...next.mergeApprovals, approvalKey],
          }
    }
    case "MergeDeclined":
    case "CloseApproved": {
      const key = sweepPrKey(event.sweepId, event.pr)
      return next.sweepDecisions.includes(key)
        ? next
        : { ...next, sweepDecisions: [...next.sweepDecisions, key] }
    }
    default:
      return next
  }
}

function isDuplicateReport(state: GcState, command: DomainCommand): boolean {
  const key = commandReportKey(command)
  return key !== null && state.reportKeys.includes(key)
}

function sameSnapshot(left: PrSnapshot, right: PrSnapshot): boolean {
  return (
    left.headSha === right.headSha &&
    left.baseRef === right.baseRef &&
    left.draft === right.draft &&
    left.open === right.open &&
    left.mergeable === right.mergeable &&
    left.createdAt === right.createdAt &&
    left.updatedAt === right.updatedAt
  )
}

function sameRun(left: CheckRun, right: CheckRun): boolean {
  return (
    left.id === right.id &&
    left.check.context === right.check.context &&
    left.check.appId === right.check.appId &&
    left.status === right.status &&
    left.conclusion === right.conclusion &&
    left.createdAt === right.createdAt &&
    left.updatedAt === right.updatedAt &&
    left.startedAt === right.startedAt &&
    left.completedAt === right.completedAt
  )
}

export function decide(
  state: GcState,
  command: DomainCommand
): Decision<DomainEvent, DomainError> {
  if (isDuplicateReport(state, command)) return accept()

  switch (command.type) {
    case "ObservePr": {
      const pr = knownPr(state, command.pr)
      if (
        pr === undefined ||
        (pr.snapshot !== null &&
          (command.snapshot.updatedAt <= pr.snapshot.updatedAt ||
            sameSnapshot(command.snapshot, pr.snapshot)))
      ) {
        return accept()
      }
      return accept({
        type: "PrStateChanged",
        pr: command.pr,
        snapshot: command.snapshot,
        at: command.at,
      })
    }
    case "ObserveCheckRun": {
      const pr = knownPr(state, command.pr)
      const current = pr?.runs.find(
        (known) =>
          known.headSha === command.headSha && known.run.id === command.run.id
      )
      if (
        pr === undefined ||
        (current !== undefined &&
          (command.run.updatedAt <= current.run.updatedAt ||
            sameRun(command.run, current.run)))
      ) {
        return accept()
      }
      return accept({
        type: "CheckRunStateChanged",
        pr: command.pr,
        headSha: command.headSha,
        run: command.run,
        at: command.at,
      })
    }
    case "ReportRequiredChecks": {
      const pr = knownPr(state, command.pr)
      const terminal = pr?.evidence.some(
        (evidence) => evidence.sweepId === command.sweepId
      )
      if (pr === undefined || terminal) return accept()
      return accept({
        type: "RequiredChecksObserved",
        reportId: command.reportId,
        sweepId: command.sweepId,
        pr: command.pr,
        headSha: command.headSha,
        snapshot: command.snapshot,
        at: command.at,
      })
    }
    case "ReportLookupFailure": {
      const scope = command.scope
      if (scope.kind === "checks") {
        const pr = knownPr(state, scope.pr)
        const terminal = pr?.evidence.some(
          (evidence) => evidence.sweepId === scope.sweepId
        )
        if (pr === undefined || terminal) return accept()
      }
      return accept({
        type: "LookupIncomplete",
        reportId: command.reportId,
        scope: command.scope,
        failure: command.failure,
        at: command.at,
      })
    }
    case "ReportPrCreated":
      if (knownPr(state, command.pr) !== undefined) return accept()
      return accept({
        type: "PrCreated",
        reportId: command.reportId,
        pr: command.pr,
        headSha: command.headSha,
        branch: command.branch,
        at: command.at,
      })
    case "ReportAgentRun":
      if (
        command.pr !== null &&
        (!samePr(command.pr, {
          tenant: command.tenant,
          repo: command.repo,
          number: command.pr.number,
        }) ||
          knownPr(state, command.pr) === undefined)
      ) {
        return accept()
      }
      return accept({
        type: "AgentRunFinished",
        reportId: command.reportId,
        tenant: command.tenant,
        repo: command.repo,
        pr: command.pr,
        outcome: command.outcome,
        usage: command.usage,
        at: command.at,
      })
    case "ReportMergeFailure": {
      const key = mergeApprovalKey(command.sweepId, command.pr, command.headSha)
      if (!state.mergeApprovals.includes(key)) return accept()
      return accept({
        type: "MergeFailed",
        reportId: command.reportId,
        sweepId: command.sweepId,
        pr: command.pr,
        headSha: command.headSha,
        failure: command.failure,
        at: command.at,
      })
    }
    case "ReportGitHubAppConnected":
      return accept({
        type: "GitHubAppConnected",
        reportId: command.reportId,
        tenant: command.tenant,
        appId: command.appId,
        installationId: command.installationId,
        at: command.at,
      })
    case "ReportRepositoryConnected":
      return accept({
        type: "RepositoryConnected",
        reportId: command.reportId,
        tenant: command.tenant,
        repo: command.repo,
        at: command.at,
      })
    case "ReportCodexConnected":
      return accept({
        type: "CodexConnected",
        reportId: command.reportId,
        tenant: command.tenant,
        method: command.method,
        at: command.at,
      })
    case "ReportTenantSettingsChanged":
      return accept({
        type: "TenantSettingsChanged",
        reportId: command.reportId,
        tenant: command.tenant,
        settings: command.settings,
        at: command.at,
      })
    case "ReportRepositorySettingsChanged":
      return accept({
        type: "RepositorySettingsChanged",
        reportId: command.reportId,
        tenant: command.tenant,
        repo: command.repo,
        settings: command.settings,
        at: command.at,
      })
    case "TickProduction": {
      const tickKey = productionTickKey(
        command.tenant,
        command.repo,
        command.now
      )
      if (state.productionTicks.includes(tickKey)) return accept()
      const open = state.prs.filter(
        (pr) =>
          pr.pr.tenant === command.tenant &&
          pr.pr.repo === command.repo &&
          (pr.snapshot?.open ?? true)
      ).length
      const limit = command.settings.openPrLimit
      return open >= limit
        ? accept({
            type: "AdmissionDeclined",
            tenant: command.tenant,
            repo: command.repo,
            refusal: { kind: "backlog_full", open, limit },
            at: command.now,
          })
        : accept({
            type: "AdmissionApproved",
            tenant: command.tenant,
            repo: command.repo,
            tenantSettings: command.tenantSettings,
            at: command.now,
          })
    }
    case "TickSweep": {
      const events: DomainEvent[] = []
      const prs = state.prs
        .filter(
          (pr) =>
            pr.pr.tenant === command.tenant &&
            pr.pr.repo === command.repo &&
            pr.snapshot?.open === true
        )
        .sort((left, right) => left.pr.number - right.pr.number)

      for (const pr of prs) {
        const decisionKey = sweepPrKey(command.sweepId, pr.pr)
        if (state.sweepDecisions.includes(decisionKey)) continue

        const evidence = pr.evidence.find(
          (candidate) => candidate.sweepId === command.sweepId
        )
        if (evidence?.kind === "incomplete") {
          events.push({
            type: "MergeDeclined",
            sweepId: command.sweepId,
            pr: pr.pr,
            refusal: {
              kind: "lookup_incomplete",
              failure: evidence.failure,
            },
            at: command.now,
          })
          continue
        }

        const snapshot = pr.snapshot as PrSnapshot
        const refusal = mergeRefusal(
          snapshot,
          evidence?.kind === "complete" ? evidence.headSha : null,
          evidence?.kind === "complete" ? evidence.snapshot : null
        )
        if (refusal === null) {
          events.push({
            type: "MergeApproved",
            sweepId: command.sweepId,
            pr: pr.pr,
            headSha: snapshot.headSha,
            at: command.now,
          })
          continue
        }

        const ageHours = (command.now - pr.createdAt) / HOUR_MS
        if (
          evidence?.kind === "complete" &&
          ageHours >= command.settings.closeAfterHours
        ) {
          events.push({
            type: "CloseApproved",
            sweepId: command.sweepId,
            pr: pr.pr,
            ageHours,
            at: command.now,
          })
          continue
        }

        events.push({
          type: "MergeDeclined",
          sweepId: command.sweepId,
          pr: pr.pr,
          refusal,
          at: command.now,
        })
      }
      return accept(...events)
    }
  }
}

export const decider: Decider<
  GcState,
  DomainCommand,
  DomainEvent,
  DomainError
> = {
  initialState,
  evolve,
  decide,
}
