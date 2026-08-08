import { httpRouter } from "convex/server"
import { httpAction } from "./_generated/server"

const http = httpRouter()

http.route({
  path: "/github/webhook",
  method: "POST",
  handler: httpAction(async () => {
    throw new Error("production GitHub webhook route is not implemented")
  }),
})
http.route({
  path: "/worker/protocol",
  method: "POST",
  handler: httpAction(async () => {
    throw new Error("production worker protocol is not implemented")
  }),
})
http.route({
  path: "/health/live",
  method: "GET",
  handler: httpAction(async () => {
    throw new Error("production liveness is not implemented")
  }),
})
http.route({
  path: "/health/ready",
  method: "GET",
  handler: httpAction(async () => {
    throw new Error("production readiness is not implemented")
  }),
})

export default http
