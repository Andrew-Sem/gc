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
