import type { DomainCommand } from "@workspace/domain"
import type { ParseContext, WebhookDelivery } from "./contract"

/** Чистый разбор одной доставки в ноль или несколько команд домена. */
export function commandsOf(
  _delivery: WebhookDelivery,
  _context: ParseContext
): readonly DomainCommand[] {
  return []
}
