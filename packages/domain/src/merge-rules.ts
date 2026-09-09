import type {
  CheckRun,
  MergeRefusal,
  PrSnapshot,
  RequiredCheck,
  RequiredChecksSnapshot,
} from "./contract"

function checkKey(check: RequiredCheck): string {
  return `${check.context}\u0000${check.appId}`
}

function isLaterAttempt(candidate: CheckRun, current: CheckRun): boolean {
  if (candidate.id === current.id) {
    return candidate.updatedAt > current.updatedAt
  }
  return (
    candidate.createdAt > current.createdAt ||
    (candidate.createdAt === current.createdAt && candidate.id > current.id)
  )
}

function latestRequiredRuns(
  snapshot: RequiredChecksSnapshot,
  observedRuns: readonly CheckRun[]
): readonly (CheckRun | null)[] {
  const latestByCheck = new Map<string, CheckRun>()

  for (const run of [...snapshot.runs, ...observedRuns]) {
    const key = checkKey(run.check)
    const current = latestByCheck.get(key)
    if (current === undefined || isLaterAttempt(run, current)) {
      latestByCheck.set(key, run)
    }
  }

  return snapshot.required.map(
    (check) => latestByCheck.get(checkKey(check)) ?? null
  )
}

export function mergeRefusal(
  pr: PrSnapshot,
  evidenceHeadSha: string | null,
  checks: RequiredChecksSnapshot | null,
  observedRuns: readonly CheckRun[] = []
): MergeRefusal | null {
  if (pr.draft) return { kind: "draft" }
  if (checks === null || evidenceHeadSha === null) {
    return { kind: "required_checks_unknown" }
  }
  if (evidenceHeadSha !== pr.headSha) {
    return { kind: "evidence_for_other_head", headSha: pr.headSha }
  }
  if (checks.required.length === 0) return { kind: "no_required_checks" }

  const latestRuns = latestRequiredRuns(checks, observedRuns)
  for (const [index, run] of latestRuns.entries()) {
    if (
      run !== null &&
      run.status === "completed" &&
      run.conclusion !== null &&
      !["success", "neutral", "skipped"].includes(run.conclusion)
    ) {
      return { kind: "required_check_failed", check: checks.required[index] }
    }
  }

  const pending = latestRuns.filter(
    (run) =>
      run === null || run.status !== "completed" || run.conclusion === null
  ).length
  if (pending > 0) return { kind: "required_checks_pending", pending }
  if (pr.mergeable !== true) return { kind: "not_mergeable" }
  return null
}
