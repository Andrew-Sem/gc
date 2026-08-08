import { ThemeProvider } from "../shared/theme"
import type { ProductionBrowserClient } from "./browser-client"

type AppProps = {
  readonly client: ProductionBrowserClient
}

export function App({ client }: AppProps) {
  void client
  return (
    <ThemeProvider>
      <main />
    </ThemeProvider>
  )
}
