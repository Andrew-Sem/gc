default:
    @just --list

# Петля обратной связи: сначала vet, потом тесты
check: lint test

# Все тесты
test:
    go test ./...

# Только доменные таблицы. Держим в пределах секунды.
spec:
    go test ./internal/domain/...

# Форматирование и статические проверки
lint:
    gofmt -l .
    go vet ./...

# Форматирование по месту
fmt:
    gofmt -w .

# Собрать бинарник
build:
    go build -o gc ./cmd/gc

# Запустить локально
run:
    go run ./cmd/gc

# Реальный smoke: настоящие GitHub и Codex, только disposable repository.
smoke:
    go test -tags smoke -count=1 -v ./internal/app/...
