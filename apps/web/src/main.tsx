import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

import "@workspace/ui/globals.css"
import { App } from "@/app"

const rootElement = document.getElementById("root")
if (!rootElement) {
  throw new Error("Не найден монтажный узел #root в index.html")
}

createRoot(rootElement).render(
  <StrictMode>
    <App />
  </StrictMode>
)
