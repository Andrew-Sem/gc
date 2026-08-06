default:
    @just --list

# Дев-сервер админки
dev:
    pnpm dev

# Исполняемые спеки домена
spec:
    pnpm test

# Типы по всем пакетам
types:
    pnpm typecheck

# Петля обратной связи для реализующего агента: типы, затем спеки
check: types spec

# Полный линт: стиль, харнесс-политика, границы слоёв FSD
lint: lint-biome lint-harness lint-fsd

lint-biome:
    pnpm exec biome check .

lint-harness:
    pnpm exec harnesslint .

lint-fsd:
    pnpm exec steiger apps/web/src

# Безопасные автофиксы Biome и харнесс-политики. Границы FSD правятся руками.
lint-fix:
    pnpm exec biome check --write .
    pnpm exec harnessfix .

# Добавить компонент shadcn/ui. Уезжает в packages/ui, а не в приложение.
# Пример: just ui-add button dialog
ui-add +components:
    pnpm dlx shadcn@latest add {{components}} -c apps/web

# Включить заморозку: запретить правку спек и контрактов в этой сессии.
# Выполняется человеком перед запуском реализующей сессии по PROMPT.md.
freeze-on:
    cp .claude/freeze-deny.json .claude/settings.local.json
    @echo "Заморозка включена. Спеки, контракты и манифесты защищены от правки."

# Выключить заморозку: вернуться в режим авторинга спек.
freeze-off:
    rm -f .claude/settings.local.json
    @echo "Заморозка выключена. Режим авторинга."
