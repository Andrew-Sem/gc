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

Всё, кроме админки в браузере, крутится на одном VPS в двух контейнерах: Convex и воркер.

```
GitHub ──вебхуки──▶ Convex ──задача──▶ Воркер ──▶ Codex ──▶ API модели
                      │  ▲                │
                   журнал└────результат───┘
                      │
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

До проверки подписи HTTP route читает только минимальный installation envelope, чтобы найти
secret установки. Затем он проверяет `X-Hub-Signature-256`; dedupe, canonical tenant
resolution и domain parser выполняются только после успешной проверки. Невалидная подпись не
резолвит tenant, не вызывает parser и не пишет журнал.

Это первая линия. Вторая — форма самого домена: `decide` выпускает `PrStateChanged` только
если снимок отличается от известного. `updatedAt` не даёт поздней доставке старого состояния
откатить новое. Даже если дубль просочится, факта не появится.

Одна check delivery даёт 0..N команд по уникальным PR associations. Required-check inventory
в webhook не приходит и там не вычисляется.

HTTP boundary резолвит installation ID в внутренний canonical tenant и передаёт его в
`ParseContext`. Чистый parser не объявляет installation ID идентификатором тенанта и не
переписывает уже созданные domain commands после разбора.

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

## Стек

### Production topology

Официальный запуск — `compose.yaml`: production web image, self-hosted Convex, отдельный worker
и TLS/reverse-proxy boundary. Convex data и tenant `CODEX_HOME` лежат в разных persistent
volumes; временные workspaces не являются credential storage. Все services имеют liveness и
readiness health checks.

Browser использует Better Auth email/password и reactive Convex client. Первый user создаёт
tenant и становится owner. Setup forms вызывают production actions/mutations, why-stuck
подписан на live query.

Внешний worker общается с control plane только через `/worker/protocol` с bearer credential.
Internal cron/tick functions снаружи недоступны. Credentials выдаются worker just-in-time и не
попадают в task payload. Worker loop ждёт на idle, делает bounded backoff, renew lease и
gracefully drains на `SIGTERM`.

Release gate включает два destructive smoke в designated disposable GitHub repository:
API-key и subscription Codex. Без обоих успешных smoke приложение не считается готовым.

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
