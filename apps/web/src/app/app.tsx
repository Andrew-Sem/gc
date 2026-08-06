import { HomePage } from "@/pages/home"
import { ThemeProvider } from "@/shared/theme"

export function App() {
  return (
    <ThemeProvider>
      <HomePage />
    </ThemeProvider>
  )
}
