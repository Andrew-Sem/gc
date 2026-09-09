import type { DomainCommand } from "@workspace/domain"
import type { ReconciliationSnapshot } from "./contract"
import { checkRunHead, parseCheckRun, parsePullRequest } from "./github-values"

/** Чистая нормализация обязательной reconciliation после создания PR. */
export function commandsOfReconciliation(
  snapshot: ReconciliationSnapshot
): readonly DomainCommand[] {
  const pullRequest = parsePullRequest(snapshot.pullRequest)
  if (pullRequest === null || pullRequest.number !== snapshot.pr.number) {
    return []
  }

  const commands: DomainCommand[] = [
    {
      type: "ObservePr",
      pr: snapshot.pr,
      snapshot: pullRequest.snapshot,
      at: snapshot.at,
    },
  ]

  if (!Array.isArray(snapshot.checkRuns)) {
    return commands
  }

  for (const value of snapshot.checkRuns) {
    if (checkRunHead(value) !== pullRequest.snapshot.headSha) {
      continue
    }

    const run = parseCheckRun(value)
    if (run === null) {
      continue
    }

    commands.push({
      type: "ObserveCheckRun",
      pr: snapshot.pr,
      headSha: pullRequest.snapshot.headSha,
      run: run.run,
      at: snapshot.at,
    })
  }

  return commands
}
