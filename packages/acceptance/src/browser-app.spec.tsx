// @vitest-environment jsdom

import { cleanup, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import type { PrStatusLine } from "@workspace/domain"
import { afterEach, expect, test } from "vitest"
import { App, createProductionBrowserClient } from "web/app"
import type { BrowserTransport } from "web/client"

const REPO = "acme/app"

function browserTransport(
  initiallyAuthenticated = false,
  connectedRepositories: readonly string[] = []
) {
  let authenticated = initiallyAuthenticated
  const calls: { readonly kind: string; readonly args: unknown }[] = []
  const state: {
    lines: readonly PrStatusLine[]
  } = { lines: [] }
  const subscribers = new Set<(value: unknown) => void>()
  const transport: BrowserTransport = {
    session: async () => {
      calls.push({ kind: "session", args: {} })
      return { authenticated }
    },
    registerEmail: async (email, password) => {
      authenticated = true
      calls.push({ kind: "register", args: { email, password } })
    },
    signInEmail: async (email, password) => {
      authenticated = true
      calls.push({ kind: "signIn", args: { email, password } })
    },
    signOut: async () => {
      authenticated = false
      calls.push({ kind: "signOut", args: {} })
    },
    query: async (_reference, args) => {
      calls.push({ kind: "query", args })
      return "repo" in args ? state.lines : setupView(connectedRepositories)
    },
    mutation: async (_reference, args) => {
      calls.push({ kind: "mutation", args: redact(args) })
    },
    action: async (_reference, args) => {
      calls.push({ kind: "action", args: redact(args) })
    },
    subscribe: (_reference, args, onValue) => {
      subscribers.add(onValue)
      onValue("repo" in args ? state.lines : setupView(connectedRepositories))
      return () => subscribers.delete(onValue)
    },
  }
  return {
    calls,
    state,
    subscribers,
    transport,
    publish() {
      for (const subscriber of subscribers) subscriber(state.lines)
    },
  }
}

function setupView(connectedRepositories: readonly string[] = []) {
  return {
    tenant: "acme",
    github: { connected: connectedRepositories.length > 0 },
    connectedRepositories,
    codex: {
      connected: false,
      method: null,
      deviceUrl: "https://auth.openai.com/device",
      userCode: "ABCD-EFGH",
    },
    tenantSettings: null,
    repositorySettings: {},
  }
}

function redact(args: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(args).map(([key, value]) => [
      key,
      /key|secret/i.test(key) ? "[redacted]" : value,
    ])
  )
}

afterEach(cleanup)

test("owner регистрируется, возвращается после reload и выходит", async () => {
  const fake = browserTransport()
  const user = userEvent.setup()
  const client = createProductionBrowserClient(fake.transport)
  const view = render(<App client={client} />)

  await user.type(screen.getByLabelText("Email"), "owner@example.com")
  await user.type(screen.getByLabelText("Пароль"), "correct horse battery")
  await user.click(screen.getByRole("button", { name: "Создать аккаунт" }))
  expect(await screen.findByRole("heading", { name: "Настройка" })).toBeTruthy()

  view.unmount()
  const sessionCallsBeforeReload = fake.calls.filter(
    (call) => call.kind === "session"
  ).length
  render(<App client={createProductionBrowserClient(fake.transport)} />)
  expect(await screen.findByRole("heading", { name: "Настройка" })).toBeTruthy()
  expect(fake.calls.filter((call) => call.kind === "session")).toHaveLength(
    sessionCallsBeforeReload + 1
  )

  await user.click(screen.getByRole("button", { name: "Выйти" }))
  expect(await screen.findByRole("button", { name: "Войти" })).toBeTruthy()
  expect(fake.calls).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ kind: "register" }),
      expect.objectContaining({ kind: "signOut" }),
      expect.objectContaining({ kind: "session" }),
    ])
  )
})

test("owner начинает стандартную установку shared GitHub App одной same-origin ссылкой", async () => {
  const fake = browserTransport(true)
  render(<App client={createProductionBrowserClient(fake.transport)} />)

  const connect = await screen.findByRole("link", {
    name: "Подключить GitHub",
  })
  expect(connect.getAttribute("href")).toBe("/github/app/connect")
  expect(screen.queryByLabelText("App ID")).toBeNull()
  expect(screen.queryByLabelText("Installation ID")).toBeNull()
  expect(screen.queryByLabelText("Private key")).toBeNull()
  expect(screen.queryByLabelText("Webhook secret")).toBeNull()
  expect(screen.queryByRole("checkbox")).toBeNull()
  expect(
    screen.queryByRole("button", { name: /Сохранить repositories/i })
  ).toBeNull()
  expect(fake.calls.filter((call) => call.kind === "action")).toEqual([])
  expect(fake.calls.filter((call) => call.kind === "mutation")).toEqual([])
})

test("owner подключает Codex обоими способами и сохраняет настройки", async () => {
  const fake = browserTransport(true, [REPO])
  const user = userEvent.setup()
  render(<App client={createProductionBrowserClient(fake.transport)} />)

  await user.type(screen.getByLabelText("OpenAI API key"), "openai-api-key")
  await user.click(screen.getByRole("button", { name: "Подключить API key" }))
  expect(await screen.findByText("Codex подключён: API key")).toBeTruthy()

  await user.click(
    screen.getByRole("button", { name: "Войти по подписке Codex" })
  )
  expect(
    (
      await screen.findByRole("link", { name: "Открыть device login" })
    ).getAttribute("href")
  ).toMatch(/^https:/)
  expect(screen.getByText(/Код:/)).toBeTruthy()

  await user.selectOptions(screen.getByLabelText("Модель"), "gpt-5.6-luna")
  await user.selectOptions(screen.getByLabelText("Reasoning"), "xhigh")
  await user.type(
    screen.getByLabelText("Промпт агента"),
    "Remove obsolete code"
  )
  await user.click(
    screen.getByRole("button", { name: "Сохранить настройки тенанта" })
  )

  await user.clear(screen.getByLabelText("Темп, часов"))
  await user.type(screen.getByLabelText("Темп, часов"), "2")
  await user.clear(screen.getByLabelText("Лимит открытых PR"))
  await user.type(screen.getByLabelText("Лимит открытых PR"), "5")
  await user.clear(screen.getByLabelText("Закрывать через, часов"))
  await user.type(screen.getByLabelText("Закрывать через, часов"), "18")
  await user.click(
    screen.getByRole("button", { name: "Сохранить настройки repository" })
  )
  expect(await screen.findByText("Настройки сохранены")).toBeTruthy()
  expect(
    fake.calls.filter((call) => call.kind === "mutation").length
  ).toBeGreaterThanOrEqual(3)
  expect(
    fake.calls.filter((call) => call.kind === "action").length
  ).toBeGreaterThanOrEqual(1)
})

test("why-stuck обновляется live после нового наблюдения", async () => {
  const fake = browserTransport(true, [REPO])
  render(<App client={createProductionBrowserClient(fake.transport)} />)

  expect(await screen.findByText("Открытых PR нет")).toBeTruthy()
  fake.state.lines = [
    {
      pr: { tenant: "acme", repo: REPO, number: 1 },
      headSha: "abc",
      verdict: "waiting",
      reason: {
        kind: "required_check_failed",
        check: { context: "build", appId: 1001 },
      },
      ageHours: 3,
      hoursUntilClose: 21,
    },
  ]
  fake.publish()

  await waitFor(() => {
    expect(screen.getByText("#1")).toBeTruthy()
    expect(screen.getByText(/build/)).toBeTruthy()
    expect(screen.getByText(/не прошла|failure/i)).toBeTruthy()
    expect(screen.getByText(/следующий свип|ожида/i)).toBeTruthy()
  })
})

test("без session protected UI и subscriptions недоступны", async () => {
  const fake = browserTransport()
  render(<App client={createProductionBrowserClient(fake.transport)} />)
  expect(await screen.findByRole("button", { name: "Войти" })).toBeTruthy()
  expect(screen.queryByRole("link", { name: "Подключить GitHub" })).toBeNull()
  expect(screen.queryByText("Открытых PR нет")).toBeNull()
  expect(fake.calls.filter((call) => call.kind === "query")).toEqual([])
})

test.each([
  ["expired", /истёк/i],
  ["failed", /не удалось|ошибка/i],
] as const)(
  "device login показывает terminal state %s",
  async (status, text) => {
    const fake = browserTransport(true)
    fake.transport.action = async () => {
      throw new Error(status)
    }
    const user = userEvent.setup()
    render(<App client={createProductionBrowserClient(fake.transport)} />)
    await user.click(
      screen.getByRole("button", { name: "Войти по подписке Codex" })
    )
    expect(await screen.findByText(text)).toBeTruthy()
  }
)

test.each([
  ["connected", /Codex подключён/i],
  ["expired", /истёк/i],
  ["failed", /не удалось|ошибка/i],
] as const)("device login live переходит в %s", async (status, text) => {
  const fake = browserTransport(true)
  const user = userEvent.setup()
  render(<App client={createProductionBrowserClient(fake.transport)} />)
  await user.click(
    screen.getByRole("button", { name: "Войти по подписке Codex" })
  )
  expect(await screen.findByText(/Код:/)).toBeTruthy()
  const setup = setupView()
  for (const subscriber of fake.subscribers) {
    subscriber({
      ...setup,
      codex: {
        ...setup.codex,
        connected: status === "connected",
        method: status === "connected" ? "subscription" : null,
        deviceUrl: status === "connected" ? null : setup.codex.deviceUrl,
        userCode: status,
      },
    })
  }
  expect(await screen.findByText(text)).toBeTruthy()
})
