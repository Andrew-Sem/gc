import betterAuthTest from "@convex-dev/better-auth/test"
import type { TestConvexForDataModel } from "convex-test"
import type { DataModel } from "./data-model"

export function registerAuthTestComponent(
  backend: TestConvexForDataModel<DataModel>
): void {
  betterAuthTest.register(backend as never)
}
