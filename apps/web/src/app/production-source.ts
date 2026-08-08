import { publicApi } from "@workspace/convex/api"
import type { PrStatusLine, Timestamp } from "@workspace/domain"
import type { SetupPresenterInput } from "../pages/setup"
import type { ProductionScreenSource } from "./production-screens"

export type ProductionQueryTransport = {
  query(
    reference: typeof publicApi.setup | typeof publicApi.prStatus,
    args: Record<string, unknown>
  ): Promise<unknown>
}

export type ProductionScreenContext = {
  readonly tenant: string
  readonly repo: string
  readonly now: Timestamp
}

export function createProductionScreenSource(
  transport?: ProductionQueryTransport,
  context?: ProductionScreenContext
): ProductionScreenSource {
  if (transport && context) {
    return {
      setup: async () =>
        (await transport.query(publicApi.setup, {
          tenant: context.tenant,
        })) as SetupPresenterInput,
      prStatus: async () =>
        (await transport.query(
          publicApi.prStatus,
          context
        )) as readonly PrStatusLine[],
    }
  }
  return {
    setup: async () => {
      throw new Error("production setup query source is not implemented")
    },
    prStatus: async () => {
      throw new Error("production status query source is not implemented")
    },
  }
}
