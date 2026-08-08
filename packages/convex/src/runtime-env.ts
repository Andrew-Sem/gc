export type RuntimeEnv = {
  readonly publicWebUrl: string
  readonly publicConvexUrl: string
  readonly internalConvexUrl: string
  readonly betterAuthSecret: string
  readonly secretMasterKey: string
  readonly workerToken: string
  readonly betterAuthAllowedOrigin: string
  readonly convexDataPath: string
  readonly codexHomeRoot: string
  readonly workspaceRoot: string
  readonly smokeSubscriptionTimeoutSeconds: number
}

export function parseRuntimeEnv(
  _values: Readonly<Record<string, string | undefined>>
): RuntimeEnv {
  throw new Error("production runtime env parser is not implemented")
}
