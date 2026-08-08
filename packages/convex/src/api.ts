import { type FunctionReference, makeFunctionReference } from "convex/server"

export const publicApi = {
  setup: makeFunctionReference<"query">("setup:get"),
  prStatus: makeFunctionReference<"query">("status:listOpenPrs"),
  connectGitHubApp: makeFunctionReference<"action">("setup:connectGitHubApp"),
  selectRepositories: makeFunctionReference<"mutation">(
    "setup:selectRepositories"
  ),
  connectCodexWithApiKey: makeFunctionReference<"mutation">(
    "setup:connectCodexWithApiKey"
  ),
  startCodexDeviceLogin: makeFunctionReference<"action">(
    "setup:startCodexDeviceLogin"
  ),
  saveTenantSettings: makeFunctionReference<"mutation">("settings:saveTenant"),
  saveRepositorySettings: makeFunctionReference<"mutation">(
    "settings:saveRepository"
  ),
  runProductionTick: makeFunctionReference<"action">("operator:runProduction"),
  runSweep: makeFunctionReference<"action">("operator:runSweep"),
}

export const systemApi = {
  productionWakeUp: makeFunctionReference<"mutation">(
    "tick:productionRepository"
  ) as unknown as FunctionReference<"mutation", "internal">,
  sweep: makeFunctionReference<"mutation">(
    "tick:sweepRepository"
  ) as unknown as FunctionReference<"mutation", "internal">,
  productionCron: makeFunctionReference<"mutation">(
    "wakeUp:production"
  ) as unknown as FunctionReference<"mutation", "internal">,
  sweepCron: makeFunctionReference<"mutation">(
    "wakeUp:sweep"
  ) as unknown as FunctionReference<"mutation", "internal">,
  claimTask: makeFunctionReference<"mutation">(
    "worker:claimTask"
  ) as unknown as FunctionReference<"mutation", "internal">,
  completeTask: makeFunctionReference<"mutation">(
    "worker:completeTask"
  ) as unknown as FunctionReference<"mutation", "internal">,
  releaseTask: makeFunctionReference<"mutation">(
    "worker:releaseTask"
  ) as unknown as FunctionReference<"mutation", "internal">,
  submitWorkerReport: makeFunctionReference<"mutation">(
    "worker:submitReport"
  ) as unknown as FunctionReference<"mutation", "internal">,
}
