import type {
  DomainCommand,
  Effect,
  TenantSettings,
  Timestamp,
} from "@workspace/domain"

export const TASK_LEASE_MS = 60_000

export type WorkerTask = {
  readonly id: string
  readonly leaseToken: string
  readonly effect: Effect
  readonly executionCount: number
}

export type WorkspaceCheckout = {
  readonly workingDirectory: string
  readonly baseRef: string
  readonly branch: string
}

export type AgentRunResult = {
  readonly outcome: "finished" | "failed"
  readonly usage: {
    readonly inputTokens: number
    readonly outputTokens: number
    readonly reasoningTokens: number
  }
}

export type PushResult = {
  readonly branch: string
  readonly headSha: string
}

export type PullRequestResult = {
  readonly number: number
  readonly branch: string
  readonly headSha: string
}

export type WorkerRuntime = {
  readonly workerId: string
  readonly now: () => Timestamp
  readonly tasks: {
    claim(workerId: string, leaseMs: number): Promise<WorkerTask | null>
    renew(taskId: string, leaseToken: string, leaseMs: number): Promise<void>
    complete(taskId: string, leaseToken: string): Promise<void>
    release(taskId: string, leaseToken: string): Promise<void>
  }
  readonly credentials: {
    get(
      taskId: string,
      leaseToken: string
    ): Promise<{
      readonly github: unknown
      readonly codex: unknown
      readonly expiresAt: Timestamp
    }>
  }
  readonly reports: {
    submit(
      taskId: string,
      leaseToken: string,
      command: DomainCommand
    ): Promise<void>
  }
  readonly github: {
    createPullRequest(input: {
      readonly tenant: string
      readonly repo: string
      readonly baseRef: string
      readonly headBranch: string
      readonly headSha: string
      readonly title: string
      readonly body: string
      readonly idempotencyKey: string
    }): Promise<PullRequestResult>
    getPullRequest(input: {
      readonly tenant: string
      readonly repo: string
      readonly pr: number
    }): Promise<unknown>
    getRequiredChecks(input: {
      readonly tenant: string
      readonly repo: string
      readonly pr: number
      readonly headSha: string
    }): Promise<unknown>
    listCheckRuns(input: {
      readonly tenant: string
      readonly repo: string
      readonly pr: number
      readonly headSha: string
    }): Promise<unknown>
    mergePullRequest(input: {
      readonly tenant: string
      readonly repo: string
      readonly pr: number
      readonly headSha: string
      readonly idempotencyKey: string
    }): Promise<unknown>
    closePullRequest(input: {
      readonly tenant: string
      readonly repo: string
      readonly pr: number
      readonly idempotencyKey: string
    }): Promise<unknown>
  }
  readonly workspace: {
    prepare(tenant: string, repo: string): Promise<WorkspaceCheckout>
    push(checkout: WorkspaceCheckout): Promise<PushResult>
  }
  readonly codex: {
    run(input: {
      readonly workingDirectory: string
      readonly settings: TenantSettings
    }): Promise<AgentRunResult>
  }
}

export async function runWorkerOnce(
  _runtime: WorkerRuntime
): Promise<"worked" | "idle"> {
  throw new Error("production worker is not implemented")
}
