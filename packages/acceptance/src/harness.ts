import type { AcceptanceHarness } from "./contract"

type ProductionWiring = {
  readonly backend: unknown
  readonly masterKey: string
}

/**
 * Implementation target: compose real Convex functions under convex-test, the
 * real worker run-once entry point, and fake network/process/filesystem ports.
 */
export async function createAcceptanceHarness(
  _wiring: ProductionWiring
): Promise<AcceptanceHarness> {
  throw new Error("production acceptance harness is not implemented")
}
