# Запускаемый продукт

Готовый результат — не библиотека и не только зелёный in-process harness. После реализации
человек заполняет `.env`, запускает одну команду Docker Compose, открывает браузер и проходит
все шесть сценариев без ручных вызовов Convex functions или worker internals.

## Официальный способ запуска

Поддерживается один обязательный topology: Docker Compose.

```bash
just up
just status
```

Стек содержит:

- `web` — production build админки;
- `convex` — self-hosted Convex с HTTP routes, auth и persistent data;
- `worker` — отдельный процесс с `git`, Codex SDK и tenant-scoped workspaces;
- reverse proxy/TLS boundary либо документированный внешний TLS terminator.

`just up` строит образы и поднимает стек. `just status` завершается успешно только когда web,
Convex и worker готовы обслуживать запросы. `just down` не удаляет persistent volumes.

## Переменные окружения

Репозиторий содержит `.env.example` без секретов. Production startup валидирует env до
запуска серверов и печатает одну понятную ошибку на каждое отсутствующее или неверное значение.

Обязательные категории:

- public web URL, public Convex URL и internal Convex URL;
- Better Auth secret и allowed origin;
- master encryption key;
- worker credential;
- persistent paths/volumes для Convex, tenant `CODEX_HOME` и временных workspaces;
- параметры реального smoke: owner email/password, GitHub App, installation, disposable repo,
  base branch и Codex credential/mode.

Секретные значения не попадают в image layers, browser bundle, journal, task payloads, public
DTO, health responses или logs.

## Вход и tenant bootstrap

Первая версия использует Better Auth email/password.

1. Успешная регистрация атомарно создаёт tenant и owner relation; повтор одного registration
   result идемпотентен, конкурентная регистрация не может создать двух owners.
2. Повторный вход восстанавливает сессию после reload.
3. Все setup mutations и status queries требуют session и ownership.
4. Logout закрывает доступ; пользователь другого tenant не читает и не меняет данные.

## Browser flow

Админка — интерактивное React-приложение, а не строковый presenter. Через браузер доступны:

1. регистрация, вход, восстановление session и logout;
2. ввод GitHub App ID, installation ID, private key и webhook secret;
3. discovery repositories и выбор подключённых repositories;
4. подключение Codex API key;
5. запуск device-code login, показ URL/code, live переход в connected/expired/failed;
6. tenant settings: model, reasoning и prompt;
7. repository settings: cadence, open PR limit и close age;
8. live why-stuck список с причиной и следующим действием.

Формы имеют label, validation, pending, success и error states. После сохранения и после нового
webhook данные обновляются без полного restart приложения.

## Worker transport

Внешний worker не вызывает Convex internal functions напрямую. Convex предоставляет
защищённый worker protocol для claim, lease renewal, completion и reports. Каждый запрос
аутентифицируется worker credential; missing/invalid credential получает отказ и не читает
task/secret state.

Task/report persistence принадлежит production control plane. Acceptance может подменять
только transport и внешние leaf ports, но не очередь, lease или journal append.

Worker renew lease до половины её срока, на idle ждёт, на transient failure использует bounded
backoff с jitter, на `SIGTERM` перестаёт брать новые задачи и завершает либо безопасно
освобождает текущую lease.

## Credentials для worker

Worker получает decrypted GitHub/Codex credentials just-in-time отдельной task-scoped
операцией защищённого control-plane protocol. Lease token доказывает право worker на tenant и
task. Credentials не лежат в journal или task snapshot. Кэш ограничен временем жизни короткого
внешнего token; rotation/revocation действует на следующем запросе credentials.

Для каждого tenant используется отдельный persistent `CODEX_HOME`. Временные git workspaces
очищаются отдельно и не содержат долгоживущих credentials.

## Agent → PR handoff

Production contracts типизированы и передают значения явно:

- workspace prepare: directory, base branch и новая branch;
- Codex run: outcome и usage;
- commit/push: branch и head SHA;
- PR create: repo, base, head branch, title/body и stable idempotency key;
- PR result: number, branch и head SHA для `ReportPrCreated`.

Пустое изменение не создаёт PR и журналируется как завершённый agent run без выдуманного PR.
Hermetic integration использует реальный `git` и временный bare repository.

## Health и persistence

Health endpoints не раскрывают секреты и различают liveness/readiness. Worker readiness
означает: env валиден, `git` доступен, Convex reachable и claim protocol авторизован.

После restart сохраняются journal, tasks, owner relation, settings, encrypted secrets и tenant
`CODEX_HOME`. Истёкшая lease снова доступна; один task не порождает два семантических
результата.

## Обязательный реальный smoke

Задача реализации не завершена без успешного credentialed smoke. Smoke работает только с
явно выделенным disposable GitHub repository и имеет право создавать, мержить и закрывать PR.
До запуска он сверяет numeric repository ID, owner/name и disposable marker topic/fixture.

```bash
just smoke-release api_key
just smoke-release subscription
```

Оба запуска обязательны. Они проходят production browser и transport boundaries:

1. clean Compose startup и health;
2. email/password registration или login, reload session;
3. real GitHub App credential validation и repository discovery;
4. real Codex connection и минимальный model call;
5. real clone, Codex change, commit, push и PR creation;
6. signed webhook через public route;
7. required-check lookup и live why-stuck UI;
8. owner-authenticated operator endpoints запускают production/sweep ticks без ожидания часов
   и пишут boundary audit record;
9. merge и age-close paths;
10. restart Convex и worker с сохранением state;
11. logout и negative auth checks.

Subscription smoke интерактивен: CLI печатает device URL/code, ждёт подтверждение оператора с
настраиваемым timeout и продолжает только после реального connected state. Он стартует с чистым
tenant `CODEX_HOME`, затем проверяет сохранение login после restart.

Если credentials, public HTTPS или disposable repository недоступны, implementation-сессия
заблокирована и не может объявить задачу завершённой.
