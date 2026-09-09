// @vitest-environment jsdom

import assert from "node:assert/strict"
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createElement } from "react"
import { afterEach, test } from "vitest"
import { App, createProductionBrowserClient } from "web/app"
import type { BrowserTransport } from "web/client"

afterEach(cleanup)

test("browser saves the optional OpenAI Base URL beside tenant settings", async () => {
  const mutations: Record<string, unknown>[] = []
  const setup = {
    tenant: "acme",
    github: { connected: false },
    connectedRepositories: [],
    codex: { connected: false, method: null },
    tenantSettings: null,
    openaiBaseUrl: "https://initial-gateway.example/v1",
    repositorySettings: {},
  }
  const transport: BrowserTransport = {
    session: async () => ({ authenticated: true, tenant: "acme" }),
    registerEmail: async () => undefined,
    signInEmail: async () => undefined,
    signOut: async () => undefined,
    query: async () => setup,
    mutation: async (_reference, args) => {
      mutations.push(args)
    },
    action: async () => undefined,
    subscribe: (_reference, _args, onValue) => {
      onValue(setup)
      return () => undefined
    },
  }
  const user = userEvent.setup()
  render(
    createElement(App, { client: createProductionBrowserClient(transport) })
  )

  const baseUrl = screen.getByLabelText("OpenAI Base URL") as HTMLInputElement
  await waitFor(() => {
    assert.equal(baseUrl.value, "https://initial-gateway.example/v1")
  })
  fireEvent.change(baseUrl, {
    target: { value: "https://gateway.example/v1" },
  })
  await user.type(
    screen.getByLabelText("Промпт агента"),
    "Remove obsolete code"
  )
  await user.click(
    screen.getByRole("button", { name: "Сохранить настройки тенанта" })
  )

  assert.deepEqual(mutations.at(-1), {
    tenant: "acme",
    settings: {
      model: "gpt-5.6-luna",
      reasoning: "high",
      prompt: "Remove obsolete code",
    },
    openaiBaseUrl: "https://gateway.example/v1",
  })
})
