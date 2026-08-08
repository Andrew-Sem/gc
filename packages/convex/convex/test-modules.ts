declare global {
  interface ImportMeta {
    glob(pattern: string): Record<string, () => Promise<unknown>>
  }
}

export const convexModules = import.meta.glob("./**/*.ts")
