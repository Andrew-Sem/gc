import type { Timestamp } from "@workspace/domain"

/**
 * Одна доставка вебхука. `id` — значение заголовка X-GitHub-Delivery, по нему
 * отбрасываются дубли на приёме.
 */
export type WebhookDelivery = {
  readonly id: string
  readonly event: string
  readonly payload: unknown
}

/**
 * Что известно на момент разбора помимо самой доставки. Обязательность
 * проверок в payload не приезжает — она берётся из защиты ветки отдельным
 * запросом и кладётся сюда.
 */
export type ParseContext = {
  readonly requiredChecks: readonly string[]
  readonly at: Timestamp
}
