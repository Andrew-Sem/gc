default:
    @just --list

check: lint test

test:
    go test ./...

spec:
    go test -count=1 -v ./specs/...

lint:
    gofmt -l .
    go vet ./...

fmt:
    gofmt -w .

build:
    go build -o gc ./cmd/gc

run:
    go run ./cmd/gc

smoke:
    go test -tags smoke -count=1 -v ./internal/app/...
