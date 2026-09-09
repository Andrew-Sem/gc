# Что покрывается таблицами

Справка, а не инструкция. Правила для агентов живут в `AGENTS.md`.

## Три уровня

| Уровень | Что доказывает |
|---|---|
| Доменные таблицы | Решение из журнала и команды. |
| Таблицы адаптеров | Внешний формат превращается в команды без скрытых выводов. |
| Vertical acceptance | Production entry points действительно соединены в рабочий путь. |
| Browser acceptance | Человек проходит auth/setup/settings/why-stuck через actual React App. |
| Release smoke | Реальные Compose, GitHub, Codex и browser работают вместе. |

Граница проходит по чистому преобразованию и production handoff, а не по названию слоя.

| Кусок | Чем проверяется |
|---|---|
| Admission и merge-gate | доменные таблицы |
| Report idempotency и out-of-order observations | доменные таблицы |
| Разбор webhook 0..N | таблицы GitHub adapter |
| Global webhook signature до tenant resolution | boundary integration + release smoke |
| Полный required-check lookup | таблицы GitHub adapter |
| Reconciliation response | таблицы GitHub adapter |
| События в эффекты | доменные таблицы |
| Журнал в модель экрана | доменные таблицы |
| Расписание кронов и routes | декларативные таблицы |
| Опции запуска Codex | таблицы чистой конфигурации |
| GitHub → Convex → worker → GitHub → UI | шесть vertical acceptance строк через production wiring |
| Auth и четыре человеческих сценария | DOM browser acceptance через actual `App` |
| Shared App install state, OAuth state, PKCE, pagination и atomic bind | boundary integration |
| Installation lifecycle и repository reconciliation | boundary/vertical integration |
| Optional OpenAI Base URL normalization и `RunAgent` snapshot | boundary/vertical integration |
| Lease-scoped installation token без private key у worker | worker protocol integration |
| Реальный git handoff | `just integration`: временный bare repository + production workspace adapter |
| Compose/GitHub/Codex/browser | обязательный credentialed release smoke |

## Vertical acceptance

`packages/acceptance/src/production-wiring.spec.ts` содержит по одной representative строке на
каждый юзкейс. Harness обязан использовать:

- реальный HTTP route и реальные Convex functions под `convex-test`;
- реальный `runWorkerOnce`, вокруг которого production main строит цикл;
- реальные публичные query, которые читает админка;
- production presenter экрана «почему PR висит»;
- fake ports только для сети, Codex, process/filesystem и clock.

В harness запрещены отдельные `JournalPort`, `DecisionPort`, in-memory event log и повторная
реализация приложения. Тест не вызывает напрямую `decide`, `effectsOf`, `projectOpenPrs`,
`commandsOf` или `codexOptions`: иначе он повторяет нижний уровень вместо проверки handoff.

Acceptance проверяет только разрывы между слоями: route → journal, journal → task, task →
worker, worker → external port, report → journal, public query → UI. Он не повторяет
матрицы порогов, refusal variants и полные DTO из доменных таблиц.

Все шесть строк идут в одном Vitest process с новым `convexTest` на строку. Кроны не ждём:
вызываем реальные targets с явным временем. Real sleeps и polling запрещены; concurrency
проверяется latch promises. Вся `just spec` остаётся около пяти секунд.

DOM/browser acceptance, настоящие subprocess, локальный bare git и container checks не входят
в быструю петлю. Они остаются обязательными и запускаются через `just integration` перед
release smoke.

## Browser acceptance

`packages/acceptance/src/browser-app.spec.tsx` запускается через обязательный `just integration`,
монтирует production `App` в jsdom и действует
как пользователь: вводит email/password, кликает единственную ссылку `Подключить GitHub`,
заполняет Codex/settings forms, запускает device login, делает logout и читает live why-stuck.
Тест не вводит App ID/installation ID/private key/webhook secret и не выбирает repositories
второй раз: он проверяет redirect boundary и отображение набора, выбранного на fake GitHub.
Тест не вызывает setup mutations напрямую. Он передаёт production web-client factory только
fake Better Auth, Convex и navigation transports.

DOM строки проверяют labels, validation, pending/error/success и reactive refresh. Вёрстка и
визуальная полировка остаются визуальной проверкой, но отсутствие интерактивного flow — красный
acceptance.

Callback security не доказывается jsdom. Отдельные boundary integration строки обязаны
проверять: `/github/app/connect` и callbacks находятся в secret-safe namespace; install state связан с subject+tenant и одноразов; setup callback считает
`installation_id` недоверенным; OAuth использует отдельные state и PKCE; callback требует ту же
session; `/user/installations` и repository discovery полностью пагинируются; App-JWT lookup
проверяет App, permissions и suspension; bind и все repository facts атомарны; exact retry после
успешного code exchange не обменивает code второй раз, а продолжает сохранённую pagination;
завершённый callback идемпотентен; временный user token удалён/отозван; global secrets отсутствуют в tenant storage,
browser DTO, journal, task и captured logs.

## Release smoke

`just smoke-release api_key` и `just smoke-release subscription` используют production images,
production browser entry и real credentials из env. Это обязательная release gate, а не
необязательная ручная проверка. Smoke работает только против designated disposable repository.

GitHub часть smoke кликает actual `Подключить GitHub` и проходит real install и отдельный
OAuth+PKCE flow. Release gate получает `SMOKE_GITHUB_STORAGE_STATE` с уже аутентифицированной
GitHub session; headed interactive проход нужен для подготовки/обновления этого state, а не
заменяет gate. Smoke не принимает tenant App ID, installation
ID, private key или webhook secret и не подменяет callback прямой setup mutation. Repository
выбирается только на GitHub и должен автоматически появиться в UI. Smoke также проверяет
branch-protection lookup с `administration: read`, signed webhook общим secret, installation
repository update/reconciliation, исключение removed repository из новых ticks/credentials,
suspension denial и отсутствие callback code/state в nginx access log.

Smoke всегда начинает с clean rebuild production images. Публичные URLs, `CONVEX_SITE_URL`,
auth origin, same-origin API/WebSocket/webhook/callback routing и persistent volumes проверяются
на поднятом Compose. Для ngrok допустим только browser-warning bypass header. Отсутствие
operator GitHub session/interactive approval, destructive disposable credential или public HTTPS
является явным blocker, а не поводом вернуться к старому manual credential flow.

## Exactly-once

`effectsOf` — только чистое отображение событий в эффекты и может вернуть тот же эффект при
повторном чтении события. Exactly-once/idempotency проверяется на production task consumer:
один outbox/task id порождает не более одного семантического результата, retry отчёта
отбрасывается по report ID, а незавершённая task остаётся доступной после lease. После crash
внешний вызов может быть повторён только с тем же стабильным idempotency key либо после
effect-specific reconciliation. Это responsibility vertical acceptance, не чистой функции.

Lookup → merge/close проверяется как continuation той же durable task и lease. Terminal lookup
report атомарно сохраняет continuation; replay возвращает ту же continuation; worker выполняет
её с теми же stable idempotency keys и немедленно завершает task до следующего claim. Отдельные
green assertions только на report или только на eventual completion не доказывают этот handoff.

## Конфигурация — значение

Зашитая прямо в вызов настройка непроверяема. Вынесенная в значение или чистую функцию
сравнивается таблицей. То же относится к кронам, routes, Codex options, repository settings и
портам worker orchestration.

OpenAI Base URL проверяется как boundary value: normalization, отказ credentials/query/fragment,
пустое значение как официальный endpoint, immutable snapshot в `RunAgent` task и передача в
Codex без появления в journal.

## Чего мы не делаем в `just spec`

- Не ждём реального срабатывания cron.
- Не вызываем реальный GitHub или Codex в быстрой hermetic петле; их обязательно вызывает
  отдельный release smoke.
- Имена production tables и минимальные durability-поля, которые acceptance читает напрямую,
  входят в frozen handoff; document IDs и конкретный encryption format не проверяем.
- Не дублируем одно доменное правило на acceptance-уровне.
- Не считаем зелёные unit/vertical/browser tests доказательством релиза без real smoke.
