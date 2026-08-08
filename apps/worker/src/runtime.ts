import type { DomainCommand } from "@workspace/domain"
import type { WorkerRuntime, WorkerTask } from "./index"

export type ProductionWorkerPorts = Pick<
  WorkerRuntime,
  "now" | "github" | "workspace" | "codex"
>

export type WorkerControlPlaneTransport = {
  request(request: {
    readonly operation:
      | "claim"
      | "renew"
      | "credentials"
      | "complete"
      | "release"
      | "report"
    readonly credential: string
    readonly workerId?: string
    readonly leaseMs?: number
    readonly taskId?: string
    readonly leaseToken?: string
    readonly command?: DomainCommand
  }): Promise<unknown>
}

/** Compose the real Convex, GitHub, Codex, workspace, and clock ports here. */
export function createProductionWorkerRuntime(
  _ports?: ProductionWorkerPorts,
  _controlPlane?: WorkerControlPlaneTransport,
  _workerId = "production-worker",
  _credential = ""
): WorkerRuntime {
  if (!_credential) throw new Error("worker credential is required")
  if (_ports && _controlPlane) {
    return {
      ..._ports,
      workerId: _workerId,
      tasks: {
        claim: async (workerId, leaseMs) =>
          (await _controlPlane.request({
            operation: "claim",
            credential: _credential,
            workerId,
            leaseMs,
          })) as WorkerTask | null,
        complete: async (taskId, leaseToken) => {
          await _controlPlane.request({
            operation: "complete",
            credential: _credential,
            taskId,
            leaseToken,
          })
        },
        renew: async (taskId, leaseToken, leaseMs) => {
          await _controlPlane.request({
            operation: "renew",
            credential: _credential,
            taskId,
            leaseToken,
            leaseMs,
          })
        },
        release: async (taskId, leaseToken) => {
          await _controlPlane.request({
            operation: "release",
            credential: _credential,
            taskId,
            leaseToken,
          })
        },
      },
      credentials: {
        get: async (taskId, leaseToken) =>
          (await _controlPlane.request({
            operation: "credentials",
            credential: _credential,
            taskId,
            leaseToken,
          })) as {
            readonly github: unknown
            readonly codex: unknown
            readonly expiresAt: number
          },
      },
      reports: {
        submit: async (taskId, leaseToken, command: DomainCommand) => {
          await _controlPlane.request({
            operation: "report",
            credential: _credential,
            taskId,
            leaseToken,
            command,
          })
        },
      },
    }
  }
  throw new Error("production worker ports are not implemented")
}
