import {
  convexModules,
  publicApi,
  registerAuthTestComponent,
  schema,
} from "@workspace/convex"
import { convexTest } from "convex-test"
import type { BrowserTransport } from "web/client"

export type AcceptanceAuthBrowser = {
  readonly transport: BrowserTransport
  register(email: string, password: string): Promise<void>
  registerSecondAccount(email: string, password: string): Promise<void>
  ownerCount(): Promise<number>
  currentTenant(): Promise<string>
  protectedSetup(): Promise<unknown>
  setupAsSecondAccount(tenant: string): Promise<unknown>
  mutateAsSecondAccount(tenant: string): Promise<unknown>
}

type CookieJar = Map<string, string>

export async function createAcceptanceAuthBrowser(): Promise<AcceptanceAuthBrowser> {
  process.env.BETTER_AUTH_ALLOWED_ORIGIN = "http://localhost:4173"
  process.env.BETTER_AUTH_SECRET = "acceptance-better-auth-secret-32-bytes"
  process.env.CONVEX_SITE_URL = "http://localhost:3210"

  const backend = convexTest(schema, convexModules)
  registerAuthTestComponent(backend)
  const primaryCookies: CookieJar = new Map()
  const secondCookies: CookieJar = new Map()

  async function authRequest(
    cookies: CookieJar,
    path: string,
    init: RequestInit = {}
  ): Promise<Response> {
    const response = await backend.fetch(path, {
      ...init,
      headers: {
        origin: "http://localhost:4173",
        ...(init.body ? { "content-type": "application/json" } : {}),
        ...(cookies.size > 0 ? { cookie: cookieHeader(cookies) } : {}),
        ...Object.fromEntries(new Headers(init.headers).entries()),
      },
    })
    storeCookies(cookies, response.headers)
    return response
  }

  async function session(cookies: CookieJar) {
    const response = await authRequest(cookies, "/api/auth/get-session")
    if (!response.ok) throw new Error(`session failed: ${response.status}`)
    return (await response.json()) as {
      readonly user?: { readonly id?: string }
    } | null
  }

  async function register(
    cookies: CookieJar,
    email: string,
    password: string
  ): Promise<void> {
    const response = await authRequest(cookies, "/api/auth/sign-up/email", {
      method: "POST",
      body: JSON.stringify({ email, password, name: email }),
    })
    if (!response.ok) throw new Error(await responseError(response))
    if (cookies.size === 0) {
      throw new Error(
        `Registration cookie missing: ${JSON.stringify(Object.fromEntries(response.headers.entries()))}`
      )
    }
  }

  async function signIn(
    cookies: CookieJar,
    email: string,
    password: string
  ): Promise<void> {
    const response = await authRequest(cookies, "/api/auth/sign-in/email", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    })
    if (!response.ok) throw new Error(await responseError(response))
  }

  async function identity(cookies: CookieJar) {
    const value = await session(cookies)
    const subject = value?.user?.id
    if (!subject) throw new Error("Authentication required")
    return { subject, tokenIdentifier: `better-auth|${subject}` }
  }

  async function queryAs(
    cookies: CookieJar,
    reference: unknown,
    args: Record<string, unknown>
  ): Promise<unknown> {
    return backend
      .withIdentity(await identity(cookies))
      .query(reference as typeof publicApi.setup, args)
  }

  async function mutationAs(
    cookies: CookieJar,
    reference: unknown,
    args: Record<string, unknown>
  ): Promise<unknown> {
    return backend
      .withIdentity(await identity(cookies))
      .mutation(reference as typeof publicApi.saveTenantSettings, args)
  }

  const transport: BrowserTransport = {
    session: async () => {
      const value = await session(primaryCookies)
      return {
        authenticated: Boolean(value?.user?.id),
        ...(value?.user?.id ? { tenant: value.user.id } : {}),
      }
    },
    registerEmail: (email, password) =>
      register(primaryCookies, email, password),
    signInEmail: (email, password) => signIn(primaryCookies, email, password),
    signOut: async () => {
      const response = await authRequest(primaryCookies, "/api/auth/sign-out", {
        method: "POST",
      })
      if (!response.ok) throw new Error(await responseError(response))
    },
    query: (reference, args) => queryAs(primaryCookies, reference, args),
    mutation: (reference, args) => mutationAs(primaryCookies, reference, args),
    action: async () => undefined,
    subscribe: (reference, args, onValue) => {
      let active = true
      void queryAs(primaryCookies, reference, args).then(
        (value) => {
          if (active) onValue(value)
        },
        () => undefined
      )
      return () => {
        active = false
      }
    },
  }

  return {
    transport,
    register: (email, password) => register(primaryCookies, email, password),
    registerSecondAccount: (email, password) =>
      register(secondCookies, email, password),
    ownerCount: async () => {
      for (let attempt = 0; attempt < 100; attempt += 1) {
        const tenant = (await session(primaryCookies))?.user?.id
        const count = tenant
          ? await backend.run(
              async (ctx) =>
                (
                  await ctx.db
                    .query("owners")
                    .withIndex("by_tenant", (query) =>
                      query.eq("tenant", tenant)
                    )
                    .collect()
                ).length
            )
          : 0
        if (count > 0 && primaryCookies.size > 0) return count
        await new Promise((resolve) => setTimeout(resolve, 10))
      }
      return 0
    },
    currentTenant: async () => {
      const value = await session(primaryCookies)
      if (!value?.user?.id) throw new Error("Authentication required")
      return value.user.id
    },
    protectedSetup: () => queryAs(primaryCookies, publicApi.setup, {}),
    setupAsSecondAccount: (tenant) =>
      queryAs(secondCookies, publicApi.setup, { tenant }),
    mutateAsSecondAccount: (tenant) =>
      mutationAs(secondCookies, publicApi.saveTenantSettings, {
        tenant,
        settings: { model: "gpt-5.4", reasoning: "low", prompt: "intrude" },
      }),
  }
}

function cookieHeader(cookies: CookieJar): string {
  return [...cookies].map(([name, value]) => `${name}=${value}`).join("; ")
}

function storeCookies(cookies: CookieJar, headers: Headers): void {
  const values = (
    headers as Headers & { getSetCookie?: () => readonly string[] }
  ).getSetCookie?.() ?? [headers.get("set-cookie") ?? ""]
  for (const value of values) {
    const pair = value.split(";", 1)[0]
    const separator = pair?.indexOf("=") ?? -1
    if (!pair || separator <= 0) continue
    const name = pair.slice(0, separator)
    const cookieValue = pair.slice(separator + 1)
    if (cookieValue) cookies.set(name, cookieValue)
    else cookies.delete(name)
  }
}

async function responseError(response: Response): Promise<string> {
  const value = (await response.json().catch(() => null)) as {
    readonly message?: string
  } | null
  return value?.message ?? `Authentication failed: ${response.status}`
}
