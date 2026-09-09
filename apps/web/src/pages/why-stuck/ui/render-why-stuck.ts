import type { PrStatusLine } from "@workspace/domain"

/** Production presenter used by the page and vertical acceptance. */
export function renderWhyStuck(lines: readonly PrStatusLine[]): string {
  if (lines.length === 0) {
    return "<section><h2>Почему PR висит</h2><p>Открытых PR нет</p></section>"
  }

  return `<section><h2>Почему PR висит</h2>${lines
    .map(
      (line) =>
        `<article><h3>#${line.pr.number}</h3><p>${escapeHtml(reasonText(line))}</p><p>${escapeHtml(nextAction(line))}</p></article>`
    )
    .join("")}</section>`
}

export function reasonText(line: PrStatusLine): string {
  const reason = line.reason
  if (!reason) return "Все условия merge-gate выполнены."

  switch (reason.kind) {
    case "required_check_failed":
      return `Обязательная проверка ${reason.check.context} (app ${reason.check.appId}) не прошла: failure.`
    case "required_checks_pending":
      return `Ожидаются обязательные проверки: ${reason.pending}.`
    case "required_checks_unknown":
      return "Полный список обязательных проверок пока неизвестен."
    case "lookup_incomplete":
      return `Lookup проверок не завершён: ${reason.failure}.`
    case "no_required_checks":
      return "В защите ветки нет обязательных проверок."
    case "not_mergeable":
      return "GitHub пока не считает PR mergeable."
    case "draft":
      return "PR всё ещё draft."
    case "evidence_for_other_head":
      return `Evidence относится к другому head: ${reason.headSha}.`
  }
}

export function nextAction(line: PrStatusLine): string {
  switch (line.verdict) {
    case "will_merge":
      return "Следующий свип смержит PR."
    case "will_close":
      return "Следующий свип закроет PR по возрасту."
    case "waiting":
      return "Ожидаем новое наблюдение или следующий свип."
  }
}

function escapeHtml(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;")
}
