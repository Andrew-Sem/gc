import type { CodexOptions, CodexSettings } from "./contract"

/**
 * Опции запуска агента. Вынесены в чистую функцию именно затем, чтобы таблица
 * ловила неверный sandboxMode — иначе агент тихо выйдет из песочницы.
 *
 * Реализуется под таблицы wiring.spec.ts.
 */
export function codexOptions(settings: CodexSettings): CodexOptions {
  return {
    model: settings.model,
    modelReasoningEffort: settings.reasoning,
    workingDirectory: settings.checkoutPath,
    sandboxMode: "workspace-write",
    networkAccessEnabled: false,
    skipGitRepoCheck: false,
  }
}
