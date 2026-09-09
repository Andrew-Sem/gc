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

- один public application origin, public Convex/API URL, `CONVEX_SITE_URL` и internal Convex
  URL; browser, auth, Convex HTTP/API/WebSocket и GitHub callbacks обязаны сходиться на
  документированных same-origin routes;
- Better Auth secret и allowed origin;
- master encryption key;
- worker credential;
- operator-registered shared GitHub App:
  `GITHUB_APP_ID`, `GITHUB_APP_CLIENT_ID`, `GITHUB_APP_CLIENT_SECRET`,
  `GITHUB_APP_PRIVATE_KEY`, `GITHUB_APP_SLUG`, `GITHUB_APP_WEBHOOK_SECRET`;
- persistent paths/volumes для Convex, tenant `CODEX_HOME` и временных workspaces;
- параметры реального smoke: owner email/password, disposable repo, base branch, Codex
  credential/mode и `SMOKE_GITHUB_STORAGE_STATE` с уже аутентифицированной GitHub browser
  session. Интерактивный headed проход используется для подготовки/обновления этого state, но
  не заменяет воспроизводимую release gate.

Секретные значения не попадают в image layers, browser bundle, journal, task payloads, public
DTO, health responses или logs. Global GitHub App private key, client secret и webhook secret
не сохраняются в tenant storage. Startup валидирует App slug, numeric App ID, client ID,
private key и непустые secrets до readiness.

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
2. одна same-origin ссылка `Подключить GitHub` на `/github/app/connect`, которая переводит owner
   через GitHub install,
   отдельный GitHub App OAuth flow с PKCE и обратно в приложение;
3. отображение repositories, выбранных на GitHub; второго checkbox selection в приложении нет;
4. подключение Codex API key;
5. запуск device-code login, показ URL/code, live переход в connected/expired/failed;
6. tenant settings: model, reasoning, prompt и optional OpenAI Base URL;
7. repository settings: cadence, open PR limit и close age;
8. live why-stuck список с причиной и следующим действием.

Формы имеют label, validation, pending, success и error states. После сохранения и после нового
webhook данные обновляются без полного restart приложения.

GitHub connect callback routes требуют ту же Better Auth session, что начала flow. Install state
и OAuth state короткоживущие, одноразовые, хранятся только в хешированном либо зашифрованном
boundary state и связаны с subject+tenant. Setup callback не доверяет `installation_id` и не
финализирует связь без отдельного OAuth+PKCE доказательства через полностью пагинированный
`GET /user/installations` и независимый `GET /app/installations/{id}` с App JWT. Если OAuth code
уже обменян, boundary хранит только его hash и зашифрованную временную user authorization, чтобы
точный retry той же session/state/code продолжил pagination без второго exchange. Завершённый
retry идемпотентен; конфликт installation между tenants и повтор с другой session получают отказ.
Временный user access token удаляется или отзывается после финализации и никогда не используется
worker.

OpenAI Base URL нормализуется при сохранении: только абсолютный HTTP(S), без credentials, query
и fragment. Пустое значение означает официальный endpoint OpenAI. Значение сохраняется в
immutable `RunAgent` task snapshot и передаётся Codex; оно не попадает в журнал.

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

Worker получает Codex credentials и короткоживущий GitHub installation token just-in-time
отдельной task-scoped операцией защищённого control-plane protocol. Lease token доказывает право
worker на tenant, task и repository. Control plane с global App private key чеканит или обновляет
installation token непосредственно перед GitHub/git фазой и по возможности сужает его по
repository и permissions. Worker никогда не получает shared private key, client secret или
webhook secret. Credentials не лежат в journal или task snapshot. Кэш ограничен временем жизни
короткого внешнего token; lease loss, rotation, suspension или revocation запрещают следующую
выдачу.

Для каждого tenant используется отдельный persistent `CODEX_HOME`. Временные git workspaces
очищаются отдельно и не содержат долгоживущих credentials.

Codex subprocess получает только явный allowlist безопасных process env плюс tenant
`CODEX_HOME`; control-plane, auth, encryption и неотносящиеся к задаче model credentials ему не
наследуются. Одновременные agent runs для одной пары tenant/repository сериализуются process- и
file-lock с heartbeat и восстановлением stale lock.

## Agent → PR handoff

Production contracts типизированы и передают значения явно:

- workspace prepare: directory, base branch и новая branch;
- Codex run: outcome и usage;
- commit/push: branch и head SHA;
- PR create: repo, base, head branch, title/body и stable idempotency key;
- PR result: number, branch и head SHA для `ReportPrCreated`.

Пустое изменение не создаёт PR и журналируется как завершённый agent run без выдуманного PR.
Hermetic integration использует реальный `git` и временный bare repository.
Повтор task использует ту же стабильную ветку и безопасно продолжает с уже существующего remote
head; push fenced через ожидаемый remote head, а не создаёт новую ветку или второй PR.

## Health и persistence

Health endpoints не раскрывают секреты и различают liveness/readiness. Worker readiness
означает: env валиден, `git` доступен, Convex reachable и claim protocol авторизован.

После restart сохраняются journal, tasks, owner relation, settings, encrypted secrets и tenant
`CODEX_HOME`. Истёкшая lease снова доступна; один task не порождает два семантических
результата.

Public reverse proxy предоставляет один application origin и маршрутизирует `/api/auth/` в
Better Auth, Convex HTTP/API и reactive WebSocket без потери upgrade headers, `/github/webhook`
в webhook boundary, `/github/app/` в setup callbacks и остальные пути в SPA. Для
`/github/app/` access log выключен, чтобы connect/callback code/state не попадали в журналы proxy.
Webhook body и signature проходят без преобразования. `CONVEX_SITE_URL`, public URLs, allowed
origin и GitHub callback URLs согласованы с внешним HTTPS origin. Если smoke использует ngrok,
browser context явно обходит только interstitial browser warning, не auth или TLS проверки.

Installation lifecycle events обязательны: created/deleted/suspend/unsuspend и repository
added/removed запускают reconciliation привязки и полного выбранного набора repositories.
Removed repository исключается из новых ticks и credentials через mutable boundary, но его
исторические факты не удаляются. Suspended или deleted installation не выдаёт новые tokens.
Ротация global private key, client secret и webhook secret является operator operation с overlap
там, где GitHub это допускает; tenant данные при ротации не переписываются.

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
3. реальный browser click по `Подключить GitHub`, переход через GitHub install и отдельный
   OAuth+PKCE flow, используя предоставленное оператором authenticated GitHub browser state либо
   явное интерактивное подтверждение;
4. GitHub-side all/selected repository choice автоматически появляется в приложении без второго
   in-app выбора; smoke не вводит tenant App ID, installation ID, private key или webhook secret;
5. real Codex connection и минимальный model call, включая configured OpenAI Base URL либо
   официальный endpoint при пустом значении;
6. real clone, Codex change, commit, push и PR creation;
7. signed webhook через public route;
8. required-check lookup, включая branch-protection permission, и live why-stuck UI;
9. owner-authenticated operator endpoints запускают production/sweep ticks без ожидания часов
   и пишут boundary audit record;
10. merge и age-close paths;
11. installation repository update/reconciliation и suspension denial;
12. restart Convex и worker с сохранением state;
13. logout и negative auth checks.

Subscription smoke интерактивен: CLI печатает device URL/code, ждёт подтверждение оператора с
настраиваемым timeout и продолжает только после реального connected state. Он стартует с чистым
tenant `CODEX_HOME`, затем проверяет сохранение login после restart.

Если credentials, public HTTPS или disposable repository недоступны, implementation-сессия
заблокирована и не может объявить задачу завершённой.
