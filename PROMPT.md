# Задание реализующему агенту

**Перед запуском этой сессии человек выполняет `just freeze-on`.**

Это режим 2 — реализация. Авторингом спек в этой сессии не занимаемся.

## Задача

Сделать запускаемое production-приложение, которым можно пользоваться через браузер. Зелёные
спеки обязательны, но недостаточны. Результат завершён только когда Docker Compose поднимает
полный стек и два credentialed release smoke (`api_key` и `subscription`) проходят реальный
путь `browser → GitHub → Convex journal/tasks → worker/Codex → GitHub → live UI`.

## Сначала прочитай целиком

1. `AGENTS.md`
2. `specs/use-cases.md`
3. `specs/glossary.md`
4. `specs/facts.md`
5. `specs/runtime.md`
6. `docs/architecture.md`
7. `docs/executable-specs.md`
8. Контракты и все `*.spec.ts` в `packages/domain`, `packages/github`,
   `packages/acceptance`

Требования берутся только оттуда. Если таблицы противоречат друг другу — остановись и скажи
человеку. Не меняй таблицу под удобную реализацию.

## Что реализовать

| Место | Ответственность |
|---|---|
| `packages/domain/src/decider.ts`, `packages/domain/src/index.ts` | Полный `decide/evolve`: settings per tick, report idempotency, out-of-order observations, full sweep evidence; production export того же decider API. |
| `packages/domain/src/effects.ts` | Чистое отображение фактов, включая `ReconcilePr`. |
| `packages/domain/src/projections.ts` | Живой why-stuck forecast из полного inventory и новых webhooks. |
| `packages/domain/src/wiring.ts` | Безопасные Codex options. |
| `packages/github/src/webhook.ts` | Webhook `0..N`, actions, validation, timestamps. |
| `packages/github/src/required-checks.ts` | Полный required inventory + все attempts; partial response только failure. |
| `packages/github/src/reconciliation.ts` | PR first, затем runs текущего head независимо от associations. |
| `packages/convex/**` | Schema, journal/tasks, auth ownership, encrypted secrets, webhook, protected worker protocol, queries/actions, health и deployment config. |
| `apps/worker/**` | Typed production ports, real GitHub App client, real git workspaces, Codex API/device auth, leases, backoff/shutdown и `runWorkerOnce`. |
| `apps/web/src/**` | Полное интерактивное приложение: auth, setup forms, settings и live why-stuck. Сохраняй FSD и используй shadcn/ui. |
| `packages/acceptance/**` | Production wiring, DOM browser acceptance, real-git integration и credentialed release smoke runner. |
| `Dockerfile*`, `compose.yaml`, `.env.example`, runtime config | Официальный one-command stack, health checks, volumes и fail-fast env validation. |

Создавай implementation-файлы свободно. Не создавай вторую in-memory реализацию journal,
decision engine, task queue или UI projection: acceptance harness должен вызывать production
код.

## Что менять запрещено

- `packages/*/src/contract.ts`
- любые `*.spec.ts` и `fixtures.ts`
- `packages/domain/src/framework/`
- `freeze-manifest.json`
- всё в `specs/`, `docs/`, уже существующие конфиги, `AGENTS.md`, `PROMPT.md`

Новые Dockerfiles, `compose.yaml`, `.env.example`, runtime modules и smoke implementation files
создавать можно. Существующие frozen config/package files менять нельзя: необходимые scripts
и dependencies уже утверждены авторингом.

Красный `freeze.spec.ts` означает правку запрещённого файла. Верни файл, манифест не правь.

## Нормативные ограничения

- Неприменимая или дублирующая команда: `{ ok: true, events: [] }`.
- `decide` не бросает и не отклоняет команды; `DomainError = never`.
- Порядок событий нормативен.
- Checks evidence текущего свипа терминален: полный snapshot или failure, не оба.
- Старое evidence не разрешает новый свип.
- `LookupIncomplete` не закрывает PR и блокирует только свой свип.
- Последняя attempt выбирается доменом по `createdAt`, затем `id`.
- Passing conclusions: `success`, `neutral`, `skipped`.
- Required identity: `context + appId`.
- Delivery ID защищает от дубля; `updatedAt` — от позднего старого состояния.
- HTTP webhook проверяет `X-Hub-Signature-256` до dedupe и parser.
- Tenant settings для агента берутся из due-тика и сохраняются в task snapshot.
- Секреты никогда не попадают в журнал или public DTO.
- Exactly-once относится к task consumer/outbox, не к `effectsOf`.
- `main` воркера должен вызывать тот же `runWorkerOnce`, что acceptance.
- Повтор внешнего вызова после process crash допустим только с тем же стабильным idempotency
  key либо через effect-specific reconciliation; один task не может породить два
  семантических результата.
- `just spec` должен оставаться около пяти секунд; real sleeps запрещены.
- Better Auth использует email/password; первый user становится owner tenant.
- Browser acceptance кликает actual `App`; string presenters не заменяют UI.
- Worker использует authenticated external protocol, а не Convex internal references.
- Production loop имеет idle wait, bounded backoff, lease renewal и graceful `SIGTERM` drain.
- Официальный runtime — Docker Compose с persistent volumes и health checks.

## Петля обратной связи

```bash
just check
just lint
just integration
just up
just status
just smoke-release api_key
just smoke-release subscription
```

Работай до полностью зелёных `just check`, `just lint`, `just integration` и успешных двух real
smoke. Отсутствие credentials, public HTTPS или disposable repository — blocker, а не
допустимый остаток.

## Отчёт по завершении

- сколько прогонов потребовалось;
- какие production seams созданы;
- где контракт допускал несколько реализаций и какой выбор сделан;
- URL поднятого приложения и health status;
- доказательство обоих release smoke, созданные PR и проверенные restart paths;
- какие значения env требуются оператору без публикации самих секретов.
