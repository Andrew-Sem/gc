export type StackReadinessProbe = {
  web(): Promise<boolean>
  convex(): Promise<boolean>
  worker(): Promise<boolean>
}

export async function assertStackReady(
  _probe?: StackReadinessProbe
): Promise<void> {
  throw new Error("production stack readiness checker is not implemented")
}

if (import.meta.url === `file://${process.argv[1]}`) {
  await assertStackReady()
}
