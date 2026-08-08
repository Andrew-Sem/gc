import type { DomainCommand } from "@workspace/domain"
import type { RequiredChecksLookup } from "./contract"

/** Чистая нормализация полного lookup required checks. */
export function commandOfRequiredChecks(
  _lookup: RequiredChecksLookup
): DomainCommand {
  throw new Error("not implemented")
}
