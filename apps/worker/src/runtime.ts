import type { DomainCommand } from "@workspace/domain"
import type {
  DeviceLoginUpdate,
  WorkerCredentials,
  WorkerReportResult,
  WorkerRuntime,
  WorkerTask,
} from "./index"

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
      | "deviceLogin"
    readonly credential: string
    readonly workerId?: string
    readonly leaseMs?: number
    readonly taskId?: string
    readonly leaseToken?: string
    readonly command?: DomainCommand
    readonly update?: DeviceLoginUpdate
    readonly signal?: AbortSignal
  }): Promise<unknown>
}

export function createHttpControlPlaneTransport(
  protocolUrl: string
): WorkerControlPlaneTransport {
  const endpoint = new URL(protocolUrl).toString()
  return {
    async request({ credential, signal, ...request }) {
      const response = await fetch(endpoint, {
        method: "POST",
        headers: {
          authorization: `Bearer ${credential}`,
          "content-type": "application/json",
        },
        body: JSON.stringify(request),
        signal,
      })
      if (!response.ok) {
        throw new Error(`worker protocol failed: ${response.status}`)
      }
      return response.json()
    },
  }
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
        complete: async (taskId, leaseToken, signal) => {
          await _controlPlane.request({
            operation: "complete",
            credential: _credential,
            taskId,
            leaseToken,
            signal,
          })
        },
        renew: async (taskId, leaseToken, leaseMs, signal) => {
          await _controlPlane.request({
            operation: "renew",
            credential: _credential,
            taskId,
            leaseToken,
            leaseMs,
            signal,
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
        get: async (taskId, leaseToken, signal) =>
          (await _controlPlane.request({
            operation: "credentials",
            credential: _credential,
            taskId,
            leaseToken,
            signal,
          })) as WorkerCredentials,
      },
      reports: {
        submit: async (taskId, leaseToken, command: DomainCommand, signal) => {
          return (await _controlPlane.request({
            operation: "report",
            credential: _credential,
            taskId,
            leaseToken,
            command,
            signal,
          })) as WorkerReportResult
        },
        updateDeviceLogin: async (taskId, leaseToken, update, signal) => {
          await _controlPlane.request({
            operation: "deviceLogin",
            credential: _credential,
            taskId,
            leaseToken,
            update,
            signal,
          })
        },
      },
    }
  }
  throw new Error("production worker ports are not implemented")
}
