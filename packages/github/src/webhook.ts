import type { DomainCommand } from "@workspace/domain"
import type { ParseContext, WebhookDelivery } from "./contract"

/**
 * Чистый разбор доставки в команду домена. Неизвестное событие, чужой формат
 * или неполный payload дают `null` — приём молча пропускает такую доставку.
 *
 * Реализуется под таблицы webhook.spec.ts.
 */
export function commandOf(
  _delivery: WebhookDelivery,
  _context: ParseContext
): DomainCommand | null {
  return null
}
