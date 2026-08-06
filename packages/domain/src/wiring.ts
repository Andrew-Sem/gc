import type { CodexOptions, CodexSettings } from "./contract"

/**
 * Опции запуска агента. Вынесены в чистую функцию именно затем, чтобы таблица
 * ловила неверный sandboxMode — иначе агент тихо выйдет из песочницы.
 *
 * Реализуется под таблицы wiring.spec.ts.
 */
export function codexOptions(_settings: CodexSettings): CodexOptions {
  return {
    model: "",
    modelReasoningEffort: "low",
    workingDirectory: "",
    sandboxMode: "danger-full-access",
    networkAccessEnabled: true,
    skipGitRepoCheck: true,
  }
}
