import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

import "@workspace/ui/globals.css"
import { App, createProductionBrowserClient } from "@/app"
import type { BrowserTransport } from "@/app/browser-client"

const transport: BrowserTransport = {
  session: async () => {
    throw new Error("production Better Auth transport is not implemented")
  },
  registerEmail: async () => {
    throw new Error("production Better Auth transport is not implemented")
  },
  signInEmail: async () => {
    throw new Error("production Better Auth transport is not implemented")
  },
  signOut: async () => {
    throw new Error("production Better Auth transport is not implemented")
  },
  query: async () => {
    throw new Error("production Convex transport is not implemented")
  },
  mutation: async () => {
    throw new Error("production Convex transport is not implemented")
  },
  action: async () => {
    throw new Error("production Convex transport is not implemented")
  },
  subscribe: () => {
    throw new Error("production Convex subscription is not implemented")
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
