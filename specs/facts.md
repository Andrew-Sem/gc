# Факты

Семнадцать фактов в четырёх группах. Формальные типы — в
`packages/domain/src/contract.ts`, он источник правды. Здесь смысл и границы.

## Наблюдения от GitHub

| Факт | Когда возникает |
|---|---|
| `PrStateChanged` | Увидели более новый снимок PR, отличающийся от известного. Несёт head, целевую ветку, драфт, открыт, mergeable, даты создания и обновления. |
| `CheckRunStateChanged` | Увидели более новое состояние check run. Несёт сырьё: id, required identity, status, conclusion и времена. Обязательность из webhook не выводится. |
| `RequiredChecksObserved` | Полностью получили required inventory и все attempts для одного PR, head и `sweepId`. Частичный ответ этим фактом быть не может. |
| `LookupIncomplete` | Lookup оборвался: rate limit, нет прав, ошибка апстрима или предел пагинации. Checks lookup несёт `sweepId` и блокирует только эту попытку свипа. |

`PrStateChanged` и `CheckRunStateChanged` выпускаются только для наших PR и только если
наблюдение новее известного. HTTP boundary сначала проверяет `X-Hub-Signature-256`; delivery ID
отбрасывает точный дубль на приёме, `updatedAt` защищает от поздней доставки старого состояния,
сравнение снимков — от содержательного дубля. Подпись проверяется глобальным webhook secret
общего GitHub App до tenant resolution, dedupe и parser; только после этого подписанный
installation ID резолвится в tenant.

Webhook-наблюдения поддерживают живой экран, но не доказывают полноту required checks. Мерж
разрешается только по `RequiredChecksObserved` текущего свипа.

## Наши решения и результаты работы

| Факт | Когда возникает |
|---|---|
| `PrCreated` | Агент завёл PR. Несёт `reportId`, номер, ветку и первый коммит. Запускает reconciliation. |
| `AgentRunFinished` | Заход агента закончился. Несёт `reportId`, tenant/repo, optional PR, исход и расход токенов. Провал до создания PR не выдумывает номер. |
| `AdmissionApproved` | Admission разрешил заводить новую работу. |
| `MergeApproved` | Merge-gate текущего `sweepId` разрешил мерж. Это решение, а не свершившийся факт. |
| `CloseApproved` | Свип решил закрыть PR по возрасту. |

Успешный мерж отдельным фактом не пишется: он приедет наблюдением. Человек закрыл наш PR
руками — это тоже `PrStateChanged` с `open: false`.

`AdmissionApproved` несёт tenant settings из due-тика. Это снимок конкретной попытки, а не
ссылка на mutable latest settings.

## Провалы и отказы

| Факт | Когда возникает |
|---|---|
| `MergeFailed` | Эффект конкретного `sweepId` вызвал GitHub, мерж не прошёл. Несёт `reportId`. |
| `AdmissionDeclined` | Due-тик прошёл, PR не завели. Несёт причину и числа. |
| `MergeDeclined` | Свип прошёл, PR не смержили. Несёт `sweepId` и причину. |

Провал закрытия отдельным фактом не заводим: закрытие практически не отказывает.

## Подключения и настройки

| Факт | Когда возникает |
|---|---|
| `GitHubAppConnected` | Setup flow доказал доступ вошедшего GitHub user к installation через session-bound install state, отдельный OAuth+PKCE flow и `/user/installations`, а принадлежность installation общему App — независимым App-JWT lookup. В факте только App ID общего App и installation ID. |
| `RepositoryConnected` | Repository входит в полностью полученный набор, выбранный человеком на GitHub для проверенной installation. Второго выбора в приложении нет. |
| `CodexConnected` | Завершён вход API key или subscription. Токенов и содержимого `auth.json` в факте нет. |
| `TenantSettingsChanged` | Сохранены модель, reasoning и промпт. |
| `RepositorySettingsChanged` | Сохранены темп производства, максимум открытых и возраст закрытия. |

`GitHubAppConnected` и все `RepositoryConnected` одного setup callback записываются атомарно и
в указанном порядке. Installation ID уникально принадлежит не более чем одному tenant. Retry
завершённого callback не создаёт вторых фактов, а partial repository discovery не создаёт ни
одного. `RepositoryConnected` остаётся историческим фактом после GitHub-side removal. Текущая
eligibility для новых ticks и credentials вычисляется вместе с mutable installation boundary;
удаление, suspension или deletion не переписывают append-only журнал и не требуют нового типа
доменного факта.

Каждая report-команда и соответствующий факт несут `reportId`. Повтор одного `type + reportId`
ничего не порождает. Разные report ID означают разные реальные попытки, даже если payload
совпал, но не отменяют семантическую неприменимость команды: второй `PrCreated` для уже
известного PR всё равно ничего не порождает и не запускает reconciliation повторно. Tenant
secrets живут только в зашифрованном mutable storage и никогда не попадают в журнал. Private
key, client secret и webhook secret общего GitHub App являются operator env, не tenant secrets,
и не попадают также в browser, task payloads или logs. OpenAI Base URL является boundary
setting: оно может быть в immutable task snapshot, но не является фактом и не попадает в
журнал.
