import type { PrStatusLine } from "@workspace/domain"
import { renderSetup, type SetupPresenterInput } from "../pages/setup"
import { renderWhyStuck } from "../pages/why-stuck"

const emptySetup: SetupPresenterInput = {
  tenant: "",
  github: { connected: false },
  connectedRepositories: [],
  codex: { connected: false },
}

function render(
  setup: SetupPresenterInput,
  lines: readonly PrStatusLine[]
): string {
  return `${renderSetup(setup)}${renderWhyStuck(lines)}`
}

export type ProductionScreenSource = {
  setup(): Promise<SetupPresenterInput>
  prStatus(): Promise<readonly PrStatusLine[]>
}

async function renderFrom(source: ProductionScreenSource): Promise<string> {
  const [setup, lines] = await Promise.all([source.setup(), source.prStatus()])
  return render(setup, lines)
}

export const productionScreens = { emptySetup, render, renderFrom }
