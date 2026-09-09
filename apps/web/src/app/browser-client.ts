import { publicApi } from "@workspace/convex/api"
import type {
  PrStatusLine,
  RepositorySettings,
  TenantSettings,
} from "@workspace/domain"
import type { SetupPresenterInput } from "../pages/setup"

export type BrowserTransport = {
  session(): Promise<{
    readonly authenticated: boolean
    readonly tenant?: string
  }>
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
  connectCodexWithApiKey(apiKey: string): Promise<void>
  startCodexDeviceLogin(): Promise<void>
  saveTenantSettings(
    settings: TenantSettings,
    openaiBaseUrl?: string
  ): Promise<void>
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
  transport: BrowserTransport
): ProductionBrowserClient {
  let tenant: string | undefined

  async function session() {
    const value = await transport.session()
    tenant = value.tenant ?? tenant
    return value
  }

  function rememberSetup(value: unknown): SetupPresenterInput {
    const setup = value as SetupPresenterInput
    tenant = setup.tenant
    return setup
  }

  async function currentTenant(): Promise<string> {
    if (tenant) return tenant
    return (await client.setup()).tenant
  }

  const client: ProductionBrowserClient = {
    session,
    register: async (email, password) => {
      await transport.registerEmail(email, password)
      await session()
    },
    signIn: async (email, password) => {
      await transport.signInEmail(email, password)
      await session()
    },
    signOut: async () => {
      await transport.signOut()
      tenant = undefined
    },
    setup: async () =>
      rememberSetup(
        await transport.query(publicApi.setup, tenant ? { tenant } : {})
      ),
    connectCodexWithApiKey: async (apiKey) => {
      await transport.mutation(publicApi.connectCodexWithApiKey, {
        tenant: await currentTenant(),
        apiKey,
      })
    },
    startCodexDeviceLogin: async () => {
      await transport.action(publicApi.startCodexDeviceLogin, {
        tenant: await currentTenant(),
      })
    },
    saveTenantSettings: async (settings, openaiBaseUrl) => {
      await transport.mutation(publicApi.saveTenantSettings, {
        tenant: await currentTenant(),
        settings,
        ...(openaiBaseUrl === undefined ? {} : { openaiBaseUrl }),
      })
    },
    saveRepositorySettings: async (repo, settings) => {
      await transport.mutation(publicApi.saveRepositorySettings, {
        tenant: await currentTenant(),
        repo,
        settings,
      })
    },
    runProductionTick: async (repo) => {
      await transport.action(publicApi.runProductionTick, {
        tenant: await currentTenant(),
        repo,
      })
    },
    runSweep: async (repo) => {
      await transport.action(publicApi.runSweep, {
        tenant: await currentTenant(),
        repo,
      })
    },
    subscribeSetup: (onValue) =>
      transport.subscribe(
        publicApi.setup,
        tenant ? { tenant } : {},
        (value) => {
          onValue(rememberSetup(value))
        }
      ),
    subscribePrStatus: (repo, onValue) =>
      transport.subscribe(
        publicApi.prStatus,
        {
          ...(tenant ? { tenant } : {}),
          repo,
          now: Date.now(),
        },
        (value) => onValue(value as readonly PrStatusLine[])
      ),
  }

  return client
}
