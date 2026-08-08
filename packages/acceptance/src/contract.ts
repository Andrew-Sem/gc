import type {
  DomainEvent,
  PrStatusLine,
  RepositorySettings,
  TenantSettings,
  Timestamp,
} from "@workspace/domain"
import type { ProductionWorkerPorts } from "@workspace/worker/runtime"

export type GitHubAppInput = {
  readonly tenant: string
  readonly appId: number
  readonly installationId: number
  readonly privateKey: string
  readonly webhookSecret: string
}

export type SetupView = {
  readonly tenant: string
  readonly availableRepositories: readonly string[]
  readonly connectedRepositories: readonly string[]
  readonly codex: {
    readonly connected: boolean
    readonly method: "api_key" | "subscription" | null
    readonly deviceUrl: string | null
    readonly userCode: string | null
  }
  readonly tenantSettings: TenantSettings | null
  readonly repositorySettings: Readonly<Record<string, RepositorySettings>>
}

export type RecordedCall = {
  readonly operation: string
  readonly tenant?: string
  readonly repo?: string
  readonly pr?: number
  readonly headSha?: string
  readonly codexHome?: string
  readonly model?: string
  readonly reasoning?: TenantSettings["reasoning"]
  readonly prompt?: string
  readonly idempotencyKey?: string
  readonly taskId?: string
  readonly leaseToken?: string
}

export type AcceptanceHarness = {
  readonly github: {
    setInstallationRepositories(
      tenant: string,
      repositories: readonly string[]
    ): void
    authorizeDeviceCode(tenant: string): void
    setSweepLookup(
      tenant: string,
      repo: string,
      pr: number,
      pullRequest: unknown,
      required: unknown,
      runs: unknown,
      completeness?: {
        readonly requiredChecksComplete: boolean
        readonly checkRunsComplete: boolean
      }
    ): void
    setReconciliation(
      tenant: string,
      repo: string,
      pr: number,
      pullRequest: unknown,
      runs: readonly unknown[]
    ): void
    failNextMerge(): void
  }
  readonly worker: {
    setNextCreatedHeadSha(headSha: string): void
    failNextAgentBeforePr(): void
    failNextAfterLease(): void
    failNextAfterExternalEffect(): void
    holdAgents(): void
    releaseAgents(): void
    waitForAgentStarts(count: number): Promise<void>
    waitForRepositoryContentions(count: number): Promise<void>
    waitForLeaseRenewals(count: number): Promise<void>
    ports(workerId?: string): ProductionWorkerPorts
    retryLastReport(): Promise<void>
  }
  readonly observed: {
    calls(): readonly RecordedCall[]
    maxConcurrentAgents(tenant: string, repo: string): number
    reportSubmissionCounts(): readonly number[]
  }
  readonly clock: {
    set(now: Timestamp): void
  }
  dispose(): Promise<void>
}

export type AcceptanceCase = {
  readonly name: string
  readonly usesWorker: boolean
  run(
    harness: AcceptanceHarness,
    app: ProductionApp,
    worker: ProductionWorker
  ): Promise<void>
}

export type ProductionWorker = {
  runOnce(workerId?: string): Promise<"worked" | "idle">
  drain(): Promise<void>
}

export type ProductionApp = {
  deliver(
    event: string,
    deliveryId: string,
    payload: unknown
  ): Promise<Response>
  deliverWithInvalidSignature(
    event: string,
    deliveryId: string,
    payload: unknown
  ): Promise<Response>
  connectGitHubApp(input: GitHubAppInput): Promise<void>
  selectRepositories(tenant: string, repos: readonly string[]): Promise<void>
  connectCodexWithApiKey(tenant: string, apiKey: string): Promise<void>
  startCodexDeviceLogin(tenant: string): Promise<void>
  saveTenantSettings(tenant: string, settings: TenantSettings): Promise<void>
  saveRepositorySettings(
    tenant: string,
    repo: string,
    settings: RepositorySettings
  ): Promise<void>
  productionWakeUp(now: Timestamp): Promise<void>
  sweep(now: Timestamp): Promise<void>
  productionRepositoryTick(now: Timestamp): Promise<void>
  sweepRepositoryTick(now: Timestamp): Promise<void>
  ownerRunProductionTick(tenant: string, repo: string): Promise<void>
  ownerRunSweep(tenant: string, repo: string): Promise<void>
  intruderRunProductionTick(tenant: string, repo: string): Promise<unknown>
  intruderRunSweep(tenant: string, repo: string): Promise<unknown>
  intruderSaveTenantSettings(
    tenant: string,
    settings: TenantSettings
  ): Promise<unknown>
  intruderConnectGitHubApp(input: GitHubAppInput): Promise<unknown>
  intruderSelectRepositories(
    tenant: string,
    repos: readonly string[]
  ): Promise<unknown>
  intruderConnectCodexWithApiKey(
    tenant: string,
    apiKey: string
  ): Promise<unknown>
  intruderStartCodexDeviceLogin(tenant: string): Promise<unknown>
  intruderSaveRepositorySettings(
    tenant: string,
    repo: string,
    settings: RepositorySettings
  ): Promise<unknown>
  intruderPrStatus(
    tenant: string,
    repo: string,
    now: Timestamp
  ): Promise<unknown>
  unauthenticatedPrStatus(
    tenant: string,
    repo: string,
    now: Timestamp
  ): Promise<unknown>
  journal(): Promise<readonly DomainEvent[]>
  operatorAudit(): Promise<readonly unknown[]>
  taskExecutionCounts(): Promise<readonly number[]>
  completedTasks(): Promise<number>
  pendingTasks(): Promise<number>
  secretStorage(tenant: string): Promise<unknown>
  deliveryRecorded(deliveryId: string): Promise<boolean>
  deliveryTenant(deliveryId: string): Promise<string | null>
  installationTenant(installationId: number): Promise<string | null>
  unauthenticatedSetup(tenant: string): Promise<unknown>
  intruderSetup(tenant: string): Promise<unknown>
  setup(tenant: string): Promise<SetupView>
  prStatus(
    tenant: string,
    repo: string,
    now: Timestamp
  ): Promise<readonly PrStatusLine[]>
  productionQuery(
    reference: unknown,
    args: Record<string, unknown>
  ): Promise<unknown>
}
