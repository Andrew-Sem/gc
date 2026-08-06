import type { DomainEvent, PrStatusLine, Timestamp } from "./contract"

/**
 * Чистая свёртка журнала в модель экрана «почему PR висит». Вердикт
 * пересчитывается на текущем состоянии, а не берётся последний записанный
 * на свипе. Реализуется под таблицы why-stuck.spec.ts.
 */
export function projectOpenPrs(
  _events: readonly DomainEvent[],
  _now: Timestamp
): readonly PrStatusLine[] {
  return []
}
