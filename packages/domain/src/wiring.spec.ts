import { expect, test } from "vitest"
import type { CodexOptions, CodexSettings } from "./contract"
import { EXPECTED_CRONS, EXPECTED_ROUTES, HOUR_MS } from "./contract"
import { codexOptions } from "./wiring"

test("расписание кронов: час на производство, шесть часов на свип", () => {
  expect(EXPECTED_CRONS).toEqual([
    { name: "production", intervalMs: HOUR_MS, target: "tick:production" },
    { name: "sweep", intervalMs: 6 * HOUR_MS, target: "tick:sweep" },
  ])
})

test("роут приёма вебхуков объявлен", () => {
  expect(EXPECTED_ROUTES).toContainEqual({
    path: "/github/webhook",
    method: "POST",
  })
})

const settings: CodexSettings = {
  model: "gpt-5.6-luna",
  reasoning: "xhigh",
  checkoutPath: "/work/acme-app",
}

const cases: readonly (readonly [
  string,
  CodexSettings,
  Partial<CodexOptions>,
])[] = [
  [
    "агент пишет только в чекаут",
    settings,
    { sandboxMode: "workspace-write", workingDirectory: "/work/acme-app" },
  ],
  ["сеть агенту закрыта", settings, { networkAccessEnabled: false }],
  [
    "модель и ризонинг берутся из настроек",
    settings,
    { model: "gpt-5.6-luna", modelReasoningEffort: "xhigh" },
  ],
  [
    "проверка на git-репозиторий не пропускается",
    settings,
    { skipGitRepoCheck: false },
  ],
]

test.each(cases)("%s", (_name, given, expected) => {
  expect(codexOptions(given)).toMatchObject(expected)
})
