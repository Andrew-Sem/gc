import type {
  PrStatusLine,
  RepositorySettings,
  TenantSettings,
} from "@workspace/domain"
import { Button } from "@workspace/ui/components/button"
import { type FormEvent, useEffect, useState } from "react"
import type { SetupPresenterInput } from "../pages/setup"
import { nextAction, reasonText } from "../pages/why-stuck"
import { ThemeProvider } from "../shared/theme"
import type {
  DeviceLoginState,
  ProductionBrowserClient,
} from "./browser-client"

type AppProps = {
  readonly client: ProductionBrowserClient
}

export function App({ client }: AppProps) {
  const [authenticated, setAuthenticated] = useState<boolean | null>(null)
  const [sessionError, setSessionError] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    client.session().then(
      (session) => {
        if (active) setAuthenticated(session.authenticated)
      },
      (error: unknown) => {
        if (active) {
          setAuthenticated(false)
          setSessionError(errorMessage(error))
        }
      }
    )
    return () => {
      active = false
    }
  }, [client])

  return (
    <ThemeProvider>
      <main className="min-h-svh bg-background text-foreground">
        {authenticated !== true ? (
          <AuthPanel
            client={client}
            error={sessionError}
            onAuthenticated={() => setAuthenticated(true)}
          />
        ) : null}
        {authenticated !== false ? (
          <SetupWorkspace
            active={authenticated === true}
            client={client}
            onSignOut={() => setAuthenticated(false)}
          />
        ) : null}
      </main>
    </ThemeProvider>
  )
}

type AuthPanelProps = {
  readonly client: ProductionBrowserClient
  readonly error: string | null
  readonly onAuthenticated: () => void
}

function AuthPanel({ client, error, onAuthenticated }: AuthPanelProps) {
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [pending, setPending] = useState<"register" | "signIn" | null>(null)
  const [message, setMessage] = useState<string | null>(error)

  async function submit(kind: "register" | "signIn") {
    if (!email.trim() || !password) {
      setMessage("Email и пароль: обязательное поле.")
      return
    }
    setPending(kind)
    setMessage(null)
    try {
      if (kind === "register") await client.register(email.trim(), password)
      else await client.signIn(email.trim(), password)
      onAuthenticated()
    } catch (submitError) {
      setMessage(errorMessage(submitError))
    } finally {
      setPending(null)
    }
  }

  return (
    <section className="mx-auto flex min-h-svh max-w-lg items-center px-5 py-12">
      <div className="w-full rounded-[2rem] border bg-card p-7 shadow-sm sm:p-10">
        <p className="mb-3 font-mono text-muted-foreground text-xs uppercase tracking-[0.22em]">
          Garbage Collector
        </p>
        <h1 className="text-balance font-semibold text-3xl tracking-tight">
          Вход владельца
        </h1>
        <p className="mt-3 text-muted-foreground text-sm leading-6">
          Один владелец управляет подключениями, настройками и жизненным циклом
          PR.
        </p>
        <div className="mt-8 grid gap-5">
          <Field label="Email">
            <input
              id="auth-email"
              type="email"
              autoComplete="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              className={inputClassName}
            />
          </Field>
          <Field label="Пароль">
            <input
              id="auth-password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              className={inputClassName}
            />
          </Field>
          {message ? <ErrorNotice>{message}</ErrorNotice> : null}
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              size="lg"
              disabled={pending !== null}
              onClick={() => submit("signIn")}
            >
              Войти
            </Button>
            <Button
              type="button"
              size="lg"
              variant="outline"
              disabled={pending !== null}
              onClick={() => submit("register")}
            >
              Создать аккаунт
            </Button>
          </div>
        </div>
      </div>
    </section>
  )
}

type SetupWorkspaceProps = {
  readonly active: boolean
  readonly client: ProductionBrowserClient
  readonly onSignOut: () => void
}

function SetupWorkspace({ active, client, onSignOut }: SetupWorkspaceProps) {
  const [setup, setSetup] = useState<SetupPresenterInput | null>(null)
  const [lines, setLines] = useState<readonly PrStatusLine[]>([])
  const [loadError, setLoadError] = useState<string | null>(null)
  const [deviceStarted, setDeviceStarted] = useState(false)
  const [deviceState, setDeviceState] = useState<DeviceLoginState>({
    status: "idle",
  })

  useEffect(() => {
    if (!active) return
    let mounted = true
    client.setup().then(
      (value) => {
        if (mounted && isSetup(value)) setSetup(value)
      },
      (error: unknown) => {
        if (mounted) setLoadError(errorMessage(error))
      }
    )
    const unsubscribe = client.subscribeSetup((value) => {
      if (mounted && isSetup(value)) setSetup(value)
    })
    return () => {
      mounted = false
      unsubscribe()
    }
  }, [active, client])

  const repo = setup?.connectedRepositories[0] ?? null

  useEffect(() => {
    if (!active || !repo) return
    return client.subscribePrStatus(repo, (value) => {
      if (isPrStatus(value)) setLines(value)
    })
  }, [active, client, repo])

  useEffect(() => {
    if (!deviceStarted || !setup) return
    setDeviceState(deviceStateOf(setup))
  }, [deviceStarted, setup])

  async function signOut() {
    await client.signOut()
    onSignOut()
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-5 sm:px-7 sm:py-8">
      <header className="mb-8 flex flex-wrap items-end justify-between gap-4 border-b pb-6">
        <div>
          <p className="font-mono text-muted-foreground text-xs uppercase tracking-[0.22em]">
            Control plane {setup?.tenant ? `/ ${setup.tenant}` : ""}
          </p>
          <h1 className="mt-2 font-semibold text-4xl tracking-tight">
            Настройка
          </h1>
        </div>
        <Button type="button" variant="outline" onClick={signOut}>
          Выйти
        </Button>
      </header>
      {loadError ? <ErrorNotice>{loadError}</ErrorNotice> : null}
      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1.35fr)_minmax(20rem,0.65fr)]">
        <div className="grid gap-6">
          <GitHubPanel setup={setup} />
          <CodexPanel
            client={client}
            deviceState={deviceState}
            onDeviceError={setDeviceState}
            onDeviceStarted={() => setDeviceStarted(true)}
          />
          <SettingsPanel client={client} repo={repo} setup={setup} />
        </div>
        <WhyStuckPanel lines={lines} repo={repo} />
      </div>
    </div>
  )
}

type SetupPanelProps = {
  readonly client: ProductionBrowserClient
  readonly setup: SetupPresenterInput | null
}

function GitHubPanel({
  setup,
}: {
  readonly setup: SetupPresenterInput | null
}) {
  const connected =
    setup?.github.connected === true || githubCallbackSucceeded()
  return (
    <Panel eyebrow="01 / GitHub" title="Подключение repository">
      <div className="rounded-2xl border bg-muted/30 p-5">
        <p className="font-medium">Общий GitHub App</p>
        <p className="mt-1 text-muted-foreground text-sm leading-6">
          Выберите account и repositories на GitHub. Подключённые repositories
          появятся здесь автоматически.
        </p>
        <a
          href="/github/app/connect"
          className="mt-4 inline-flex min-h-10 items-center justify-center rounded-md bg-primary px-4 py-2 font-medium text-primary-foreground text-sm shadow-xs transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          Подключить GitHub
        </a>
        <p className="mt-3 text-muted-foreground text-xs">
          Статус: {connected ? "подключён" : "не подключён"}
        </p>
      </div>
      <div className="mt-7 border-t pt-6">
        <h3 className="font-medium">Подключённые repositories</h3>
        {(setup?.connectedRepositories.length ?? 0) > 0 ? (
          <ul className="mt-3 grid gap-2">
            {setup?.connectedRepositories.map((repo) => (
              <li
                key={repo}
                className="rounded-2xl border px-4 py-3 font-mono text-sm"
              >
                {repo}
              </li>
            ))}
          </ul>
        ) : (
          <p className="mt-3 rounded-2xl border border-dashed px-4 py-3 text-muted-foreground text-sm">
            Repositories пока не подключены
          </p>
        )}
      </div>
    </Panel>
  )
}

function githubCallbackSucceeded(): boolean {
  return (
    typeof window !== "undefined" &&
    new URLSearchParams(window.location.search).get("github") === "connected"
  )
}

type CodexPanelProps = {
  readonly client: ProductionBrowserClient
  readonly deviceState: DeviceLoginState
  readonly onDeviceStarted: () => void
  readonly onDeviceError: (state: DeviceLoginState) => void
}

function CodexPanel({
  client,
  deviceState,
  onDeviceStarted,
  onDeviceError,
}: CodexPanelProps) {
  const [apiKey, setApiKey] = useState("")
  const [pending, setPending] = useState<"api" | "device" | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  async function connectApiKey() {
    if (!apiKey.trim()) {
      setError("OpenAI API key: обязательное поле.")
      return
    }
    setPending("api")
    setError(null)
    try {
      await client.connectCodexWithApiKey(apiKey)
      setApiKey("")
      setMessage("Codex подключён: API key")
    } catch (connectError) {
      setError(errorMessage(connectError))
    } finally {
      setPending(null)
    }
  }

  async function startDeviceLogin() {
    setPending("device")
    setError(null)
    try {
      await client.startCodexDeviceLogin()
      onDeviceStarted()
    } catch (startError) {
      const text = errorMessage(startError)
      onDeviceError(
        /expired/i.test(text)
          ? { status: "expired" }
          : { status: "failed", message: text }
      )
    } finally {
      setPending(null)
    }
  }

  return (
    <Panel eyebrow="02 / Codex" title="Модель и credential">
      <Field label="OpenAI API key">
        <input
          id="openai-api-key"
          type="password"
          value={apiKey}
          onChange={(event) => setApiKey(event.target.value)}
          className={inputClassName}
        />
      </Field>
      <div className="mt-4 flex flex-wrap gap-2">
        <Button
          type="button"
          disabled={pending !== null}
          onClick={connectApiKey}
        >
          Подключить API key
        </Button>
        <Button
          type="button"
          variant="outline"
          disabled={pending !== null}
          onClick={startDeviceLogin}
        >
          Войти по подписке Codex
        </Button>
      </div>
      {error ? <ErrorNotice>{error}</ErrorNotice> : null}
      {message ? <SuccessNotice>{message}</SuccessNotice> : null}
      <DeviceLoginNotice state={deviceState} />
    </Panel>
  )
}

function DeviceLoginNotice({ state }: { readonly state: DeviceLoginState }) {
  if (state.status === "idle") return null
  if (state.status === "connected") {
    return <SuccessNotice>Codex подключён: subscription</SuccessNotice>
  }
  if (state.status === "expired") {
    return <ErrorNotice>Срок device login истёк.</ErrorNotice>
  }
  if (state.status === "failed") {
    return <ErrorNotice>Не удалось войти. Ошибка: {state.message}</ErrorNotice>
  }
  return (
    <div className="mt-4 rounded-2xl border border-amber-500/30 bg-amber-500/5 p-4 text-sm">
      <a
        href={state.deviceUrl}
        target="_blank"
        rel="noreferrer"
        className="font-medium underline underline-offset-4"
      >
        Открыть device login
      </a>
      <p className="mt-2 font-mono">Код: {state.userCode}</p>
    </div>
  )
}

function SettingsPanel({
  client,
  repo,
  setup,
}: SetupPanelProps & { repo: string | null }) {
  const [model, setModel] = useState("gpt-5.6-luna")
  const [reasoning, setReasoning] =
    useState<TenantSettings["reasoning"]>("high")
  const [prompt, setPrompt] = useState("")
  const [openaiBaseUrl, setOpenaiBaseUrl] = useState("")
  const [productionEveryHours, setProductionEveryHours] = useState("1")
  const [openPrLimit, setOpenPrLimit] = useState("12")
  const [closeAfterHours, setCloseAfterHours] = useState("24")
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (setup?.tenantSettings) {
      setModel(setup.tenantSettings.model)
      setReasoning(setup.tenantSettings.reasoning)
      setPrompt(setup.tenantSettings.prompt)
    }
    setOpenaiBaseUrl(setup?.openaiBaseUrl ?? "")
  }, [setup?.openaiBaseUrl, setup?.tenantSettings])

  useEffect(() => {
    const settings = repo ? setup?.repositorySettings?.[repo] : undefined
    if (settings) {
      setProductionEveryHours(String(settings.productionEveryHours))
      setOpenPrLimit(String(settings.openPrLimit))
      setCloseAfterHours(String(settings.closeAfterHours))
    }
  }, [repo, setup?.repositorySettings])

  async function saveTenant(event: FormEvent) {
    event.preventDefault()
    setError(null)
    try {
      await client.saveTenantSettings(
        { model, reasoning, prompt },
        openaiBaseUrl
      )
      setMessage("Настройки сохранены")
    } catch (saveError) {
      setError(errorMessage(saveError))
    }
  }

  async function saveRepository(event: FormEvent) {
    event.preventDefault()
    if (!repo) {
      setError("Сначала выберите repository.")
      return
    }
    const settings: RepositorySettings = {
      productionEveryHours: Number(productionEveryHours),
      openPrLimit: Number(openPrLimit),
      closeAfterHours: Number(closeAfterHours),
    }
    if (
      Object.values(settings).some(
        (value) => !Number.isFinite(value) || value <= 0
      )
    ) {
      setError("Настройки repository должны быть положительными числами.")
      return
    }
    setError(null)
    try {
      await client.saveRepositorySettings(repo, settings)
      setMessage("Настройки сохранены")
    } catch (saveError) {
      setError(errorMessage(saveError))
    }
  }

  return (
    <Panel eyebrow="03 / Policy" title="Темп и пороги">
      <form onSubmit={saveTenant} className="grid gap-4 sm:grid-cols-2">
        <Field label="Модель">
          <select
            id="tenant-model"
            value={model}
            onChange={(event) => setModel(event.target.value)}
            className={inputClassName}
          >
            <option value="gpt-5.6-luna">gpt-5.6-luna</option>
            <option value="gpt-5.4">gpt-5.4</option>
            <option value="gpt-5.3-codex">gpt-5.3-codex</option>
          </select>
        </Field>
        <Field label="Reasoning">
          <select
            id="tenant-reasoning"
            value={reasoning}
            onChange={(event) =>
              setReasoning(event.target.value as TenantSettings["reasoning"])
            }
            className={inputClassName}
          >
            <option value="low">low</option>
            <option value="medium">medium</option>
            <option value="high">high</option>
            <option value="xhigh">xhigh</option>
          </select>
        </Field>
        <div className="sm:col-span-2">
          <Field label="OpenAI Base URL">
            <input
              id="openai-base-url"
              type="url"
              value={openaiBaseUrl}
              onChange={(event) => setOpenaiBaseUrl(event.target.value)}
              placeholder="https://api.openai.com/v1"
              className={inputClassName}
            />
          </Field>
        </div>
        <div className="sm:col-span-2">
          <Field label="Промпт агента">
            <textarea
              id="tenant-prompt"
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              className={`${inputClassName} min-h-28 resize-y`}
            />
          </Field>
        </div>
        <Button type="submit" className="sm:col-span-2 sm:justify-self-start">
          Сохранить настройки тенанта
        </Button>
      </form>
      <form
        onSubmit={saveRepository}
        className="mt-7 grid gap-4 border-t pt-6 sm:grid-cols-3"
      >
        <Field label="Темп, часов">
          <input
            id="repository-cadence"
            type="number"
            min="1"
            value={productionEveryHours}
            onChange={(event) => setProductionEveryHours(event.target.value)}
            className={inputClassName}
          />
        </Field>
        <Field label="Лимит открытых PR">
          <input
            id="repository-limit"
            type="number"
            min="1"
            value={openPrLimit}
            onChange={(event) => setOpenPrLimit(event.target.value)}
            className={inputClassName}
          />
        </Field>
        <Field label="Закрывать через, часов">
          <input
            id="repository-close-age"
            type="number"
            min="1"
            value={closeAfterHours}
            onChange={(event) => setCloseAfterHours(event.target.value)}
            className={inputClassName}
          />
        </Field>
        <Button type="submit" className="sm:col-span-3 sm:justify-self-start">
          Сохранить настройки repository
        </Button>
      </form>
      {error ? <ErrorNotice>{error}</ErrorNotice> : null}
      {message ? <SuccessNotice>{message}</SuccessNotice> : null}
    </Panel>
  )
}

function WhyStuckPanel({
  lines,
  repo,
}: {
  readonly lines: readonly PrStatusLine[]
  readonly repo: string | null
}) {
  return (
    <aside className="sticky top-6 rounded-[2rem] border bg-card p-6 shadow-sm">
      <p className="font-mono text-muted-foreground text-xs uppercase tracking-[0.2em]">
        Live forecast
      </p>
      <h2 className="mt-2 font-semibold text-2xl tracking-tight">
        Почему PR висит
      </h2>
      <p className="mt-2 font-mono text-muted-foreground text-xs">
        {repo ?? "repository не выбран"}
      </p>
      {lines.length === 0 ? (
        <p className="mt-8 rounded-2xl border border-dashed p-5 text-muted-foreground text-sm">
          Открытых PR нет
        </p>
      ) : (
        <div className="mt-6 grid gap-4">
          {lines.map((line) => (
            <article
              key={`${line.pr.repo}-${line.pr.number}`}
              className="rounded-2xl border p-5"
            >
              <div className="flex items-baseline justify-between gap-3">
                <h3 className="font-mono font-semibold text-xl">
                  #{line.pr.number}
                </h3>
                <span className="text-muted-foreground text-xs">
                  {line.ageHours} ч.
                </span>
              </div>
              <p className="mt-4 text-sm leading-6">{reasonText(line)}</p>
              <p className="mt-3 border-t pt-3 text-muted-foreground text-sm leading-6">
                {nextAction(line)}
              </p>
            </article>
          ))}
        </div>
      )}
    </aside>
  )
}

function Panel({
  eyebrow,
  title,
  children,
}: {
  readonly eyebrow: string
  readonly title: string
  readonly children: React.ReactNode
}) {
  return (
    <section className="rounded-[2rem] border bg-card p-6 shadow-sm sm:p-8">
      <p className="font-mono text-muted-foreground text-xs uppercase tracking-[0.2em]">
        {eyebrow}
      </p>
      <h2 className="mt-2 mb-6 font-semibold text-2xl tracking-tight">
        {title}
      </h2>
      {children}
    </section>
  )
}

function Field({
  label,
  children,
}: {
  readonly label: string
  readonly children: React.ReactNode
}) {
  const child = children as React.ReactElement<{ id?: string }>
  return (
    <label htmlFor={child.props.id} className="grid gap-2 font-medium text-sm">
      <span>{label}</span>
      {children}
    </label>
  )
}

function ErrorNotice({ children }: { readonly children: React.ReactNode }) {
  return (
    <p className="mt-4 rounded-2xl border border-destructive/30 bg-destructive/5 px-4 py-3 text-destructive text-sm">
      {children}
    </p>
  )
}

function SuccessNotice({ children }: { readonly children: React.ReactNode }) {
  return (
    <p className="mt-4 rounded-2xl border border-emerald-600/30 bg-emerald-500/5 px-4 py-3 text-emerald-700 text-sm dark:text-emerald-300">
      {children}
    </p>
  )
}

function deviceStateOf(setup: SetupPresenterInput): DeviceLoginState {
  if (
    (setup.codex.connected && setup.codex.method === "subscription") ||
    setup.codex.status === "connected"
  ) {
    return { status: "connected" }
  }
  if (
    setup.codex.status === "expired" ||
    setup.codex.userCode?.toLowerCase() === "expired"
  ) {
    return { status: "expired" }
  }
  if (
    setup.codex.status === "failed" ||
    setup.codex.userCode?.toLowerCase() === "failed"
  ) {
    return { status: "failed", message: "device login failed" }
  }
  if (setup.codex.deviceUrl && setup.codex.userCode) {
    return {
      status: "pending",
      deviceUrl: setup.codex.deviceUrl,
      userCode: setup.codex.userCode,
    }
  }
  return { status: "idle" }
}

function isSetup(value: unknown): value is SetupPresenterInput {
  if (!value || typeof value !== "object") return false
  const candidate = value as Partial<SetupPresenterInput>
  return (
    typeof candidate.tenant === "string" &&
    typeof candidate.github === "object" &&
    candidate.github !== null &&
    typeof candidate.github.connected === "boolean" &&
    Array.isArray(candidate.connectedRepositories) &&
    typeof candidate.codex === "object" &&
    candidate.codex !== null
  )
}

function isPrStatus(value: unknown): value is readonly PrStatusLine[] {
  return (
    Array.isArray(value) &&
    value.every((line) => line && typeof line === "object" && "pr" in line)
  )
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}

const inputClassName =
  "min-h-10 w-full rounded-2xl border bg-background px-3 py-2 text-sm outline-none transition-shadow focus:border-ring focus:ring-3 focus:ring-ring/20"
