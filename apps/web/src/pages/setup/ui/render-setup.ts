import type { RepositorySettings, TenantSettings } from "@workspace/domain"

export type SetupPresenterInput = {
  readonly tenant: string
  readonly github: {
    readonly connected: boolean
  }
  readonly connectedRepositories: readonly string[]
  readonly codex: {
    readonly connected: boolean
    readonly method?: "api_key" | "subscription" | null
    readonly status?: "pending" | "connected" | "expired" | "failed" | null
    readonly deviceUrl?: string | null
    readonly userCode?: string | null
  }
  readonly tenantSettings?: TenantSettings | null
  readonly openaiBaseUrl?: string | null
  readonly repositorySettings?: Readonly<Record<string, RepositorySettings>>
}

/** Production setup presenter used by the page and vertical acceptance. */
export function renderSetup(setup: SetupPresenterInput): string {
  const connected = setup.connectedRepositories
    .map((repo) => `<li>${escapeHtml(repo)} подключён</li>`)
    .join("")
  const codex = setup.codex.connected
    ? `Codex подключён: ${setup.codex.method === "subscription" ? "subscription" : "API key"}`
    : "Codex не подключён"
  const github = setup.github.connected
    ? "GitHub подключён"
    : "GitHub не подключён"

  return `<section><h1>Настройка</h1><p>Тенант: ${escapeHtml(setup.tenant)}</p><p>${github}</p><h2>Repositories</h2><ul>${connected}</ul><p>${codex}</p></section>`
}

function escapeHtml(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;")
}
