# Архитектура

Точка входа для человека. Агентам — `AGENTS.md`. Что система делает — `specs/use-cases.md`.

## Что это

Event-driven control plane над жизненным циклом PR. Переписывание GC-автоматики из Nucex в
отдельный переиспользуемый продукт. Multi-tenant: Nucex — первый тенант, а не владелец логики.

Система собирается из **исполняемых спек**: поведение домена описано таблицами
given/when/then, реализация пишется под них.

## Части

| Часть | Ответственность | Есть в репозитории |
|---|---|---|
| `packages/domain` | Чистое ядро: словарь фактов, `decide`, `evolve`. Не знает ни про Convex, ни про GitHub. | да |
| `packages/github` | Формат GitHub: разбор доставок вебхуков в команды домена. | да |
| `packages/ui` | Компоненты shadcn/ui. | да |
| `apps/web` | Админка: подключение репозитория, настройки, экран «почему PR висит». | каркас |
| `packages/convex` | Журнал, outbox/tasks, decisions, два wake-up, webhook, auth, public queries. | нет, требуется acceptance |
| `apps/worker` | Отдельный процесс. Исполняет task через production `runWorkerOnce`. | нет, требуется acceptance |
| `packages/acceptance` | Шесть вертикальных строк через production wiring и fake I/O ports. | контракт и красные таблицы |

Production stack крутится на одном VPS под Docker Compose: web/reverse proxy, self-hosted Convex
и отдельный worker. GitHub App один на deployment и зарегистрирован оператором заранее.

```
GitHub ──вебхуки──▶ reverse proxy ──▶ Convex ──задача──▶ Воркер ──▶ Codex ──▶ API модели
                         ▲              │  ▲                │
                         │           журнал└────результат────┘
                      Админка
```

## Решения и почему

### Журнал append-only — единственный источник правды

Всё, что произошло, лежит одной последовательностью фактов. Записи не изменяются и не
удаляются. Текущее доменное состояние — не отдельная таблица, а свёртка журнала. Auth
ownership и credentials относятся к boundary state, а не к доменному журналу.

Отсюда два следствия. Первое: секреты в журнал не попадают никогда — переписать его нечем,
и утёкший туда ключ останется навсегда. Секреты живут в отдельной мутабельной таблице,
зашифрованные мастер-ключом из env; в журнал идёт только факт ротации.

Второе: append-only лог **сам по себе является наблюдаемостью**. Метрики выводимы из него
задним числом, обратное — нет. Поэтому на старте мы не поднимаем Grafana вовсе.

### Дедупликация по webhook delivery id

GitHub может доставить один и тот же вебхук дважды. Каждая доставка несёт уникальный
идентификатор (`X-GitHub-Delivery`). Мы его запоминаем и повторную доставку отбрасываем на
приёме.

Deployment имеет один webhook secret общего GitHub App. HTTP route проверяет
`X-Hub-Signature-256` этим глобальным secret **до** tenant lookup, dedupe и parser. Только после
успешной проверки подписанный installation ID резолвится в canonical tenant. Невалидная подпись
не резолвит tenant, не вызывает parser и не пишет журнал.

Это первая линия. Вторая — форма самого домена: `decide` выпускает `PrStateChanged` только
если снимок отличается от известного. `updatedAt` не даёт поздней доставке старого состояния
откатить новое. Даже если дубль просочится, факта не появится.

Одна check delivery даёт 0..N команд по уникальным PR associations. Required-check inventory
в webhook не приходит и там не вычисляется.

HTTP boundary резолвит installation ID в внутренний canonical tenant и передаёт его в
`ParseContext`. Чистый parser не объявляет installation ID идентификатором тенанта и не
переписывает уже созданные domain commands после разбора.

### Один GitHub App на deployment

Оператор один раз регистрирует shared GitHub App и задаёт через env `GITHUB_APP_ID`,
`GITHUB_APP_CLIENT_ID`, `GITHUB_APP_CLIENT_SECRET`, `GITHUB_APP_PRIVATE_KEY`,
`GITHUB_APP_SLUG` и `GITHUB_APP_WEBHOOK_SECRET`. `request_oauth_on_install` выключен. Permissions:
`administration: read`, `checks: read`, `contents: write`, `metadata: read`,
`pull_requests: write`; administration read требуется для branch protection. Events:
`pull_request`, `check_run`, `installation` и `installation_repositories`.

Tenant onboarding не создаёт App и не принимает credentials. Owner идёт по одной same-origin
ссылке `Подключить GitHub` на `/github/app/connect`. Control plane создаёт install state, связанный с Better Auth
subject+tenant, и делает `303` на GitHub installation page. GitHub остаётся единственным местом
выбора account/organization и all/selected repositories.

Setup callback `/github/app/setup/callback` не доверяет query-параметру `installation_id`. После проверки той же session и
install state он запускает отдельный GitHub App OAuth flow со свежими one-time state и PKCE.
OAuth callback `/github/app/oauth/callback` обменивает code, полностью пагинирует `/user/installations`, независимо читает
`/app/installations/{id}` с App JWT, проверяет permissions/suspension и полностью получает
repositories. Только после всех доказательств одна транзакция уникально связывает installation
с tenant и записывает `GitHubAppConnected`, затем `RepositoryConnected` для каждого GitHub-side
selected repository. Временная user authorization сразу удаляется или отзывается. После
успешного code exchange flow хранит code hash и зашифрованную временную authorization: точный
retry той же session/state/code продолжает pagination без второго exchange. Завершённый retry
идемпотентен; incomplete discovery не оставляет частичную связь.

Global private key, client secret и webhook secret никогда не входят в tenant storage, browser,
journal, task или logs. `/github/app/` исключён из proxy access log, потому что URL callback
несёт code/state. Installation lifecycle меняет доступность через reconciliation: repository
updates синхронизируют mutable eligibility, suspension/deletion блокируют выдачу tokens.
Append-only `RepositoryConnected` остаётся историей: removed repository исключается из новых
ticks и credentials пересечением journal projection с текущим installation boundary, без нового
типа доменного факта.

### Полнота evidence принадлежит свипу

Webhook observations полезны для живого экрана, но не доказывают, что отсутствующая required
identity действительно отсутствует. Перед решением каждый PR получает полный
`RequiredChecksObserved` или `LookupIncomplete` с тем же `sweepId`. Partial pagination
порождает только failure. Старое evidence не переиспользуется следующим свипом.

Каждая попытка lookup сначала читает свежий PR и только затем branch protection и runs его
текущего head. Это же обновляет nullable `mergeable`: одноразовая reconciliation после
`PrCreated` восстанавливает ранние webhooks, но не заменяет refresh перед каждым свипом.

Adapter передаёт полный inventory и все attempts. Последнюю попытку по `createdAt`, затем `id`,
а также passing conclusions вычисляет домен.

### Создание PR всегда запускает reconciliation

Webhook check run может прийти до создания PR и без associations. Поэтому `PrCreated`
отображается в `ReconcilePr`: GitHub adapter читает текущий PR и все runs его head, после чего
отправляет обычные `ObservePr` и `ObserveCheckRun`. Отдельной параллельной модели фактов для
reconciliation нет.

### Время попадает в домен только тиком

Домен не читает часы. Текущее время приезжает командой `TickProduction` или `TickSweep`.
Иначе спеки становятся невоспроизводимыми: таблица, зависящая от `Date.now()`, зелёная
сегодня и красная завтра.

Долговечных таймеров нет: расписание держат кроны Convex. Условие обратимости этого
решения — **`decide` не возвращает эффект «разбуди меня в момент T»**. Как только такой
эффект появится, знание о планировании времени протечёт в домен, и вернуться будет дорого.

Cron targets не принимают repository arguments: `wakeUp:production` перечисляет подключённые
репозитории и вызывает due `tick:productionRepository`, а `wakeUp:sweep` запускает
`tick:sweepRepository` для каждого подключённого репозитория.

### Шов между Convex и доменом — заметка на потом

Декларативных констант недостаточно: route может быть смонтирован, но не писать журнал, а
worker может запускать отдельную тестовую реализацию. Поэтому `packages/acceptance` вызывает
production Convex functions под `convex-test`, production `runWorkerOnce`, public queries и UI
presenter. Fake только I/O ports. Шесть строк проверяют handoff, не повторяя доменные матрицы.

Зелёный unit-уровень при красном vertical acceptance означает незавершённое приложение.

### Воркер — отдельная программа

Функция Convex умирает через 10 минут и получает 512 MB памяти. У неё нет постоянной
файловой системы, нет `git`, нельзя порождать процессы. Self-hosted это не меняет.

Агенту нужно склонировать репозиторий, погонять модель, сделать коммит и запушить. В эти
рамки он не влезает. Поэтому воркер — отдельный процесс в своём контейнере, а Convex с ним
общается через реактивную подписку.

Изоляция двойная: контейнер как граница ОС и песочница Codex (`workspace-write`) как граница
агента. Одно не заменяет другое.

Task consumer отвечает за lease, retry и semantic idempotency. После process crash внешний
вызов повторяется с тем же стабильным idempotency key либо сначала сверяется с внешней
системой; два результата одного task недопустимы. Чистая `effectsOf` этого не обеспечивает.
`main` только собирает production ports и повторяет `runWorkerOnce`; acceptance вызывает тот
же entry point с fake GitHub, Codex, workspace и clock.

Lookup required checks и последующий merge/close являются продолжением **той же durable task**,
той же lease и стабильных idempotency keys. Терминальный lookup report атомарно сохраняет
continuation; replay того же report возвращает ту же continuation, а worker завершает task
сразу после последней фазы. Нельзя создать отдельную незавязанную task между lookup и effect:
это потеряло бы fencing и допустило двойной merge/close.

### Настройки и секреты

Tenant создаётся атомарно при успешной email/password registration: Better Auth subject
становится его единственным owner. Эта связь хранится в mutable auth storage; она не является
доменным фактом и проверяется на каждом public query/action/mutation. Системные cron/tick
functions внутренние и клиентскому API не экспортируются. Release smoke использует отдельные
owner-authenticated operator actions, которые вызывают internal ticks и пишут boundary audit
record.

Не секретные подключения и настройки журналируются фактами. Секреты остаются в отдельном
зашифрованном mutable storage. Hourly production cron — wake-up; он проверяет репозиторный
`productionEveryHours`. Сохранение темпа начинает новый интервал, а доменный `TickProduction`
вызывается только когда repository due. Каждый due production tick несёт снимки repository и
tenant settings своей попытки; outbox task сохраняет их и worker не перечитывает mutable
настройки перед исполнением.

Optional OpenAI Base URL относится к tenant boundary, а не к доменному журналу. Оно
нормализуется как абсолютный HTTP(S) URL без credentials/query/fragment; пустое значение
означает официальный endpoint. Due tick сохраняет действующее значение в immutable `RunAgent`
task snapshot. Worker получает snapshot через task-scoped credential response и передаёт Codex,
поэтому более позднее изменение не меняет уже одобренную попытку.

Control plane хранит shared App private key и чеканит installation token just-in-time по active
task lease. Worker получает только короткоживущий token, scoped к task/repository и по
возможности permissions; shared private key worker не видит. Lease renewal служит fencing:
провал renewal abort-ит GitHub, git и Codex I/O и запрещает completion. Worker использует
bounded backoff с jitter, graceful `SIGTERM`, tenant-scoped `CODEX_HOME`, allowlist env для Codex
и agent lock. Stable branch и force-with-lease позволяют retry продолжить тот же git handoff.

## Стек

### Production topology

Официальный запуск — `compose.yaml`: production web/reverse-proxy image, self-hosted Convex,
отдельный worker и внешний TLS boundary при необходимости. Convex data и tenant `CODEX_HOME`
лежат в разных persistent volumes; временные workspaces не являются credential storage. Все
services имеют liveness и readiness health checks. Clean release build обязателен: reuse stale
image не является проверкой текущего source tree.

Browser использует Better Auth email/password и reactive Convex client. Первый user создаёт
tenant и становится owner. Setup forms вызывают production actions/mutations, why-stuck
подписан на live query.

Web, auth, Convex API/HTTP, reactive WebSocket, webhook и GitHub callbacks публикуются через
один application origin. Reverse proxy сохраняет WebSocket upgrade, raw webhook body и
forwarded host/proto; `/github/app/` имеет `access_log off`. `PUBLIC_WEB_URL`, public Convex URL,
`CONVEX_SITE_URL`, Better Auth allowed origin и GitHub callback URLs обязаны описывать один и тот
же внешний HTTPS boundary. Для ngrok release browser может передать header обхода interstitial
warning, но не обходить TLS или auth.

Внешний worker общается с control plane только через `/worker/protocol` с bearer credential.
Internal cron/tick functions снаружи недоступны. Lease-scoped credentials выдаются worker
just-in-time и не попадают в task payload; global GitHub App private key никогда не выдаётся.
Worker loop ждёт на idle, делает bounded backoff, renew lease и gracefully drains на `SIGTERM`.

Release gate включает два destructive smoke в designated disposable GitHub repository:
API-key и subscription Codex. Реальный browser кликает shared App connect и проходит GitHub
install плюс отдельный OAuth flow через operator-provided authenticated GitHub browser state или
явное интерактивное подтверждение. Tenant App credentials и второй repository checkbox smoke не
использует. Без обоих успешных smoke приложение не считается готовым.

| Слой | Выбор |
|---|---|
| Язык, сборка, тесты | TypeScript, pnpm, Turborepo, Vitest |
| Линт | Biome, `@andrew-semyonov/harness-linter`, steiger |
| Бэкенд, БД, крон, HTTP | Convex, self-hosted |
| Аутентификация | Better Auth через `@convex-dev/better-auth` |
| Фронт | React 19, Vite, Tailwind 4, shadcn/ui, Feature-Sliced Design |
| Агент | Codex через `@openai/codex-sdk` |
| Деплой | свой VPS, docker |

**Что рассматривали и отклонили:** Hono (его роль играет `httpRouter` Convex), SQLite,
Valibot и Drizzle (у Convex своя БД и своя валидация), OpenTelemetry (wide events дают то же),
GitHub Actions как раннер агента (лимиты съедаются быстро), Vercel Sandbox (нельзя
self-hosted).

## Что переносится из Nucex, а что нет

**Переносится:** раннер исполняемых спек (`docs/exec-specs/framework/runner.ts`), протокол
сессии (`PROMPT.md`, заморозка, случайные фикстуры), wide events из
`apps/convex/_shared/observability.ts`, порог admission.

**Не переносится:** двухтокенная схема (её причина — PAT на руках у LLM-агента, здесь агент
GitHub не трогает); числа 160, 170, шестичасовой дедлайн дрейна и потолок раундов (это обход
лимита джобы GitHub Actions в 150 минут, а не правила домена); lifecycle fingerprint (артефакт
поллинга — при вебхуках перезапуск проверки приезжает событием сам); `baseSha` (уход базы
вперёд не обесценивает зелёный CI на текущем head).
