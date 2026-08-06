import type { DomainEvent, Effect } from "./contract"

/**
 * Чистое отображение фактов в действия. Реализуется под таблицы
 * effects.spec.ts.
 */
export function effectsOf(_events: readonly DomainEvent[]): readonly Effect[] {
  return []
}
