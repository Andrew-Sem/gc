import type {
  PrStatusLine,
  RepositorySettings,
  TenantSettings,
} from "@workspace/domain"
import type { SetupPresenterInput } from "../pages/setup"

export type BrowserTransport = {
  session(): Promise<{ readonly authenticated: boolean }>
  registerEmail(email: string, password: string): Promise<void>
  signInEmail(email: string, password: string): Promise<void>
  signOut(): Promise<void>
  query(reference: unknown, args: Record<string, unknown>): Promise<unknown>
  mutation(reference: unknown, args: Record<string, unknown>): Promise<unknown>
  action(reference: unknown, args: Record<string, unknown>): Promise<unknown>
  subscribe(
    reference: unknown,
    args: Record<string, unknown>,
    onValue: (value: unknown) => void
  ): () => void
}

export type ProductionBrowserClient = {
  session(): Promise<{ readonly authenticated: boolean }>
  register(email: string, password: string): Promise<void>
  signIn(email: string, password: string): Promise<void>
  signOut(): Promise<void>
  setup(): Promise<SetupPresenterInput>
  connectGitHubApp(input: {
    readonly appId: number
    readonly installationId: number
    readonly privateKey: string
    readonly webhookSecret: string
  }): Promise<void>
  selectRepositories(repos: readonly string[]): Promise<void>
  connectCodexWithApiKey(apiKey: string): Promise<void>
  startCodexDeviceLogin(): Promise<void>
  saveTenantSettings(settings: TenantSettings): Promise<void>
  saveRepositorySettings(
    repo: string,
    settings: RepositorySettings
  ): Promise<void>
  runProductionTick(repo: string): Promise<void>
  runSweep(repo: string): Promise<void>
  subscribeSetup(onValue: (setup: SetupPresenterInput) => void): () => void
  subscribePrStatus(
    repo: string,
    onValue: (lines: readonly PrStatusLine[]) => void
  ): () => void
}

export type DeviceLoginState =
  | { readonly status: "idle" }
  | {
      readonly status: "pending"
      readonly deviceUrl: string
      readonly userCode: string
    }
  | { readonly status: "connected" }
  | { readonly status: "expired" }
  | { readonly status: "failed"; readonly message: string }

export function createProductionBrowserClient(
  _transport: BrowserTransport
): ProductionBrowserClient {
  throw new Error("production browser client is not implemented")
}
