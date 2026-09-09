import type { DomainEvent, Effect } from "./contract"

/**
 * Чистое отображение фактов в действия. Реализуется под таблицы
 * effects.spec.ts.
 */
export function effectsOf(_events: readonly DomainEvent[]): readonly Effect[] {
  const effects: Effect[] = []

  for (const event of _events) {
    switch (event.type) {
      case "AdmissionApproved":
        effects.push({
          type: "RunAgent",
          tenant: event.tenant,
          repo: event.repo,
          tenantSettings: event.tenantSettings,
        })
        break
      case "PrCreated":
        effects.push({
          type: "ReconcilePr",
          pr: event.pr,
          headSha: event.headSha,
        })
        break
      case "MergeApproved":
        effects.push({
          type: "MergePr",
          sweepId: event.sweepId,
          pr: event.pr,
          headSha: event.headSha,
        })
        break
      case "CloseApproved":
        effects.push({
          type: "ClosePr",
          sweepId: event.sweepId,
          pr: event.pr,
        })
        break
    }
  }

  return effects
}
