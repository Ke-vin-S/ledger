# SplitLedger — Go API

Go REST + GraphQL backend. Chi router, pgx v5, gqlgen, Redis, S3.

## Commands

```bash
# Run (with hot reload)
air

# Run (without hot reload)
go run ./cmd/api

# Test (all, with race detector)
go test ./... -race -count=1

# Test (single package)
go test ./internal/domain/expense/... -race

# Lint
golangci-lint run ./...

# Format
gofmt -w .

# DB migrations
go run ./cmd/migrate up
go run ./cmd/migrate down 1

# Regenerate gqlgen code (after schema changes)
go generate ./...

# Local dependencies (Postgres + Redis)
docker compose up -d
docker compose down
```

## Environment

Copy `.env.example` to `.env` for local dev. Required vars:

```
PORT=8080
ENV=local
DATABASE_URL=postgresql://dev:dev@localhost:5432/splitleger?sslmode=disable
REDIS_URL=redis://localhost:6379
JWT_PRIVATE_KEY=<RS256 PEM>
JWT_PUBLIC_KEY=<RS256 PEM>
S3_BUCKET=splitleger-receipts-dev
AWS_REGION=ap-southeast-1
GOOGLE_CLIENT_ID=<from Google Cloud Console>
GOOGLE_CLIENT_SECRET=<from Google Cloud Console>
```

## Structure

```
cmd/
  api/main.go        # entrypoint — wire deps, start server
  migrate/main.go    # migration runner
internal/
  auth/              # JWT RS256, refresh rotation, Chi middleware
  config/            # load from env/SSM
  db/                # pgxpool setup, migrations/
  domain/            # pure business logic — no HTTP, no DB imports
    user/            # model.go, service.go, repository.go (interface)
    team/
    expense/
    split/
    settlement/
    flag/
    notification/
  handler/           # HTTP handlers — thin, delegate to domain services
  graph/             # gqlgen schema + resolvers (3 queries only)
  middleware/        # cors, request_id, rate_limit
  repository/        # concrete pgx implementations of domain interfaces
  storage/           # S3 pre-signed URL generation
  audit/             # append-only audit log writer
```

## Architecture

- `domain/` has zero imports from `handler/`, `repository/`, or `graph/`. Services depend on repository interfaces only — never concrete implementations. This is the hard boundary.
- `handler/` calls domain services, never touches pgx directly.
- `repository/` implements domain interfaces with raw SQL (pgx v5). No ORM.
- All money stored as `int64` minor units (cents/paisa). Never float64.
- All IDs are UUID v4 strings. Never auto-increment integers in the API.
- Corrections create new snapshot rows — never mutate existing expense or split rows. See @docs/data-model.md §7.
- Audit log is append-only. IMPORTANT: never issue UPDATE or DELETE against audit_log.

## Conventions

- Error responses always use the envelope: `{ "error": { "code": "...", "message": "...", "field": "..." } }`. See @docs/api-layer.md §1.3 for full format.
- GraphQL is read-only. All mutations go through REST. Do not add mutations to the GraphQL schema.
- `split_method = equal`: share_units is ignored. Server computes equal division with remainder on the first participant.
- YOU MUST validate `SUM(split.share_amount) == expense.amount` before committing any split. Reject with 422 `INVALID_SPLIT_SUM` if it fails.
- Settlement amount must not exceed outstanding balance. Reject with 409 `SETTLEMENT_EXCEEDS_DEBT`.
- Anonymous user claim must run in a single DB transaction. No partial migrations.

## Gotchas

- Aiven PostgreSQL enforces TLS. Local dev uses `sslmode=disable`; production uses `sslmode=require`. The connection string differs — don't use the same URL for both.
- pgxpool free tier limit: Aiven free tier has ~25 max connections. Set `pool_max_conns=10` in the DATABASE_URL to avoid exhausting them.
- gqlgen regenerates `graph/generated.go` — never edit that file manually. Edit `graph/schema.graphqls` and run `go generate ./...`.
- `golang-migrate` expects migration files named `NNN_description.up.sql` / `NNN_description.down.sql`. Wrong naming silently skips them.
- Air watches `.go` files only by default — add `*.graphqls` to the watch list in `.air.toml` if you want hot reload on schema changes.
- testcontainers in integration tests spin up a real Postgres. They require Docker running locally. They are slow — run with `-short` to skip them: `go test -short ./...`.

## Never Do

- Never hard-delete from `audit_log`, `expenses`, `expense_splits`, or `settlements`. Use soft deletes (`deleted_at`, `is_void`).
- Never store money as `float64`. Always `int64` minor units.
- Never expose auto-increment IDs in API responses. UUIDs only.
- Never import `handler/` or `repository/` from `domain/`.

## Reference

- @docs/api-layer.docx — full endpoint list, request/response shapes, error codes
- @docs/data-model.docx — full schema, all tables, enums, integrity invariants
