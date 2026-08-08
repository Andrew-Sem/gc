import type { DomainCommand } from "@workspace/domain"
import type { ReconciliationSnapshot } from "./contract"

/** Чистая нормализация обязательной reconciliation после создания PR. */
export function commandsOfReconciliation(
  _snapshot: ReconciliationSnapshot
): readonly DomainCommand[] {
  return []
}
