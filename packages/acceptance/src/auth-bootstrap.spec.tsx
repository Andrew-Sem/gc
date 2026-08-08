// @vitest-environment jsdom

import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { expect, test } from "vitest"
import { App, createProductionBrowserClient } from "web/app"
import { createAcceptanceAuthBrowser } from "./auth-browser-harness"

test("Better Auth registration атомарно создаёт owner tenant и cookie переживает reload", async () => {
  const harness = await createAcceptanceAuthBrowser()
  const user = userEvent.setup()
  const firstClient = createProductionBrowserClient(harness.transport)
  const first = render(<App client={firstClient} />)

  await user.type(screen.getByLabelText("Email"), "owner@example.com")
  await user.type(screen.getByLabelText("Пароль"), "correct horse battery")
  await user.click(screen.getByRole("button", { name: "Создать аккаунт" }))
  expect(await harness.ownerCount()).toBe(1)
  const tenant = await harness.currentTenant()

  first.unmount()
  render(<App client={createProductionBrowserClient(harness.transport)} />)
  expect(await screen.findByRole("heading", { name: "Настройка" })).toBeTruthy()
  expect(await harness.currentTenant()).toBe(tenant)

  await user.click(screen.getByRole("button", { name: "Выйти" }))
  await expect(harness.protectedSetup()).rejects.toThrow()
})

test("второй account не читает и не меняет tenant первого owner", async () => {
  const harness = await createAcceptanceAuthBrowser()
  await harness.register("owner@example.com", "correct horse battery")
  const tenant = await harness.currentTenant()
  await harness.registerSecondAccount(
    "intruder@example.com",
    "different password"
  )
  await expect(harness.setupAsSecondAccount(tenant)).rejects.toThrow()
  await expect(harness.mutateAsSecondAccount(tenant)).rejects.toThrow()
  expect(await harness.ownerCount()).toBe(1)
})
