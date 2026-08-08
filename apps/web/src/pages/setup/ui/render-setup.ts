export type SetupPresenterInput = {
  readonly tenant: string
  readonly availableRepositories: readonly string[]
  readonly connectedRepositories: readonly string[]
  readonly codex: { readonly connected: boolean }
}

/** Production setup presenter used by the page and vertical acceptance. */
export function renderSetup(_setup: SetupPresenterInput): string {
  return ""
}
