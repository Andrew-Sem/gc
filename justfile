default:
    @just --list

check: lint test

generate:
    go tool templ generate

test: generate
    go test ./...

spec: generate
    go test -count=1 -v ./specs/...

lint: generate
    test -z "$(gofmt -l .)" || (gofmt -l . && exit 1)
    go vet ./...
    go tool templ fmt -fail .

fmt:
    gofmt -w .
    go tool templ fmt .

build: generate
    go build -o gc ./cmd/gc

run addr="localhost:8080" db="gc.db": generate
    set -a && . ./.env && set +a && GC_ADDR={{addr}} GC_DB={{db}} go run ./cmd/gc
