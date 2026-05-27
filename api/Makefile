GO ?= $(shell which go 2>/dev/null || echo /usr/local/go/bin/go)

.PHONY: run dev test test-short test-pkg lint fmt build migrate-up migrate-down \
        db-up db-down db-reset generate tidy

# ── Dev ───────────────────────────────────────────────────────────────────────

run:
	$(GO) run ./cmd/api

dev:
	air

build:
	$(GO) build -o api ./cmd/api

# ── Test ──────────────────────────────────────────────────────────────────────

test:
	$(GO) test ./... -race -count=1

# Skip slow testcontainer-based integration tests
test-short:
	$(GO) test ./... -race -count=1 -short

# Run a single package: make test-pkg PKG=./internal/domain/expense/...
test-pkg:
	$(GO) test $(PKG) -race -v

# ── Code quality ──────────────────────────────────────────────────────────────

lint:
	golangci-lint run ./...

fmt:
	$(shell dirname $(GO))/gofmt -w .

tidy:
	$(GO) mod tidy

generate:
	$(GO) generate ./...

# ── DB migrations ─────────────────────────────────────────────────────────────

migrate-up:
	$(GO) run ./cmd/migrate up

migrate-down:
	$(GO) run ./cmd/migrate down 1

# ── Local dependencies ────────────────────────────────────────────────────────

db-up:
	docker compose up -d

db-down:
	docker compose down

db-reset: db-down db-up migrate-up
