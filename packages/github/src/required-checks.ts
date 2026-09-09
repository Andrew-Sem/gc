import type { DomainCommand } from "@workspace/domain"
import type { RequiredChecksLookup } from "./contract"
import {
  checkRunHead,
  parseCheckRun,
  parseRequiredCheck,
} from "./github-values"

function failureCommand(
  lookup: RequiredChecksLookup,
  failure: "rate_limited" | "forbidden" | "upstream_error" | "page_limit"
): DomainCommand {
  return {
    type: "ReportLookupFailure",
    reportId: lookup.reportId,
    scope: {
      kind: "checks",
      sweepId: lookup.sweepId,
      pr: lookup.pr,
      headSha: lookup.headSha,
    },
    failure,
    at: lookup.at,
  }
}

/** Чистая нормализация полного lookup required checks. */
export function commandOfRequiredChecks(
  lookup: RequiredChecksLookup
): DomainCommand {
  if (lookup.failure !== undefined) {
    return failureCommand(lookup, lookup.failure)
  }

  if (!lookup.requiredChecksComplete || !lookup.checkRunsComplete) {
    return failureCommand(lookup, "page_limit")
  }

  if (
    !Array.isArray(lookup.requiredChecks) ||
    !Array.isArray(lookup.checkRuns)
  ) {
    return failureCommand(lookup, "upstream_error")
  }

  const required = []
  for (const value of lookup.requiredChecks) {
    const check = parseRequiredCheck(value)
    if (check === null) {
      return failureCommand(lookup, "upstream_error")
    }
    required.push(check)
  }

  const runs = []
  for (const value of lookup.checkRuns) {
    const headSha = checkRunHead(value)
    if (headSha === null) {
      return failureCommand(lookup, "upstream_error")
    }
    if (headSha !== lookup.headSha) {
      continue
    }

    const run = parseCheckRun(value)
    if (run === null) {
      return failureCommand(lookup, "upstream_error")
    }
    runs.push(run.run)
  }

  return {
    type: "ReportRequiredChecks",
    reportId: lookup.reportId,
    sweepId: lookup.sweepId,
    pr: lookup.pr,
    headSha: lookup.headSha,
    snapshot: { required, runs },
    at: lookup.at,
  }
}
