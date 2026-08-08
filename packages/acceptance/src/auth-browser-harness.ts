import type { BrowserTransport } from "web/client"

export type AcceptanceAuthBrowser = {
  readonly transport: BrowserTransport
  register(email: string, password: string): Promise<void>
  registerSecondAccount(email: string, password: string): Promise<void>
  ownerCount(): Promise<number>
  currentTenant(): Promise<string>
  protectedSetup(): Promise<unknown>
  setupAsSecondAccount(tenant: string): Promise<unknown>
  mutateAsSecondAccount(tenant: string): Promise<unknown>
}

export async function createAcceptanceAuthBrowser(): Promise<AcceptanceAuthBrowser> {
  throw new Error("production Better Auth browser harness is not implemented")
}
