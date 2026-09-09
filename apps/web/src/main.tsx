import { createAuthClient } from "better-auth/client"
import { ConvexReactClient } from "convex/react"
import type { FunctionReference } from "convex/server"
import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

import "@workspace/ui/globals.css"
import { App, createProductionBrowserClient } from "@/app"
import type { BrowserTransport } from "@/app/browser-client"

const convexUrl = import.meta.env.VITE_CONVEX_URL
if (!convexUrl) {
  throw new Error("VITE_CONVEX_URL is required")
}

const authUrl = import.meta.env.VITE_AUTH_URL ?? window.location.origin
const authClient = createAuthClient({
  baseURL: new URL("/api/auth", authUrl).toString(),
})
const convex = new ConvexReactClient(convexUrl)

function tenantOf(value: unknown): string | undefined {
  if (!value || typeof value !== "object") return undefined
  const user = (value as { readonly user?: unknown }).user
  if (!user || typeof user !== "object") return undefined
  const id = (user as { readonly id?: unknown }).id
  return typeof id === "string" ? id : undefined
}

async function convexToken(): Promise<string | null> {
  const response = await fetch(new URL("/api/auth/convex/token", authUrl), {
    credentials: "include",
  })
  if (response.status === 401) return null
  if (!response.ok) throw new Error(`Convex auth failed: ${response.status}`)
  const body = (await response.json()) as { readonly token?: string }
  return body.token ?? null
}

function enableConvexAuth() {
  convex.setAuth(convexToken)
}

function assertAuthResponse(response: {
  readonly error?: { readonly message?: string } | null
}) {
  if (response.error) {
    throw new Error(response.error.message ?? "Authentication failed")
  }
}

const transport: BrowserTransport = {
  session: async () => {
    const response = await authClient.getSession()
    assertAuthResponse(response)
    const authenticated = response.data !== null
    if (authenticated) enableConvexAuth()
    return { authenticated, tenant: tenantOf(response.data) }
  },
  registerEmail: async (email, password) => {
    const response = await authClient.signUp.email({
      email,
      password,
      name: email,
    })
    assertAuthResponse(response)
    enableConvexAuth()
  },
  signInEmail: async (email, password) => {
    const response = await authClient.signIn.email({ email, password })
    assertAuthResponse(response)
    enableConvexAuth()
  },
  signOut: async () => {
    const response = await authClient.signOut()
    assertAuthResponse(response)
    convex.clearAuth()
  },
  query: (reference, args) =>
    convex.query(reference as FunctionReference<"query">, args),
  mutation: (reference, args) =>
    convex.mutation(reference as FunctionReference<"mutation">, args),
  action: (reference, args) =>
    convex.action(reference as FunctionReference<"action">, args),
  subscribe: (reference, args, onValue) => {
    const watch = convex.watchQuery(
      reference as FunctionReference<"query">,
      args
    )
    return watch.onUpdate(() => onValue(watch.localQueryResult()))
  },
}

const rootElement = document.getElementById("root")
if (!rootElement) {
  throw new Error("Не найден монтажный узел #root в index.html")
}

createRoot(rootElement).render(
  <StrictMode>
    <App client={createProductionBrowserClient(transport)} />
  </StrictMode>
)
