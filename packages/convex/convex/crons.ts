import {
  cronJobs,
  type FunctionReference,
  makeFunctionReference,
} from "convex/server"

const crons = cronJobs()

crons.interval(
  "production",
  { hours: 1 },
  makeFunctionReference<"mutation">(
    "wakeUp:production"
  ) as unknown as FunctionReference<"mutation", "internal">
)
crons.interval(
  "sweep",
  { hours: 6 },
  makeFunctionReference<"mutation">(
    "wakeUp:sweep"
  ) as unknown as FunctionReference<"mutation", "internal">
)

export default crons
