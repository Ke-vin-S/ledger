# SplitLedger

Personal and team expense tracking. Split bills, manage loans, settle debts, and keep a permanent audit trail of every financial event — including corrections.

---

## Features

- **Expense splitting** — equal or custom share amounts, with server-enforced sum validation
- **Team management** — invite-link and approval-based membership, role-based access
- **Loans** — direct peer-to-peer loans tracked separately from group expenses
- **Settlements** — pay off balances; settlement cannot exceed outstanding debt
- **Flags** — dispute or annotate any expense for review
- **Audit log** — append-only record of every create, correct, and void event
- **Corrections** — snapshot-based; past data is never mutated
- **Notifications** — activity feed with 30s polling

---

## Stack

| Layer          | Tech                                                                                            |
| -------------- | ----------------------------------------------------------------------------------------------- |
| API            | Go 1.25, Chi, pgx v5, gqlgen, Redis                                                             |
| Frontend       | Next.js 15 (App Router), Tailwind v4, shadcn/ui, React Query v5, Zustand                        |
| Infrastructure | AWS CDK (TypeScript), Lambda Function URL, ElastiCache, S3, SSM SecureString (`ap-southeast-1`) |
| Database       | PostgreSQL (Aiven)                                                                              |
| Auth           | RS256 JWT + refresh token rotation, Google OAuth                                                |
| Logging        | go.uber.org/zap — JSON stdout, forwardable to CloudWatch / Datadog / Loki                       |

---

## Monorepo Structure

```
splitleger/
├── api/        Go REST + GraphQL backend
├── web/        Next.js 15 frontend
├── infra/      AWS CDK infrastructure (4 stacks)
└── docs/       Planning docs — spec, data model, API layer, frontend plan
```

Each sub-project has its own `CLAUDE.md` with commands, conventions, and gotchas.

---

## Getting Started (Local)

### Prerequisites

- Go 1.25+
- Node.js 20+ and pnpm
- Docker (for Postgres + Redis)
- `air` for hot reload: `go install github.com/air-verse/air@latest`
- `golangci-lint` for linting

### 1. Start local dependencies

```bash
cd api
docker compose up -d
```

### 2. API

```bash
cd api
cp .env.example .env        # fill in JWT keys and other required vars
go run ./cmd/migrate up     # run DB migrations
air                         # hot reload dev server on :8080
```

Or without hot reload:

```bash
go run ./cmd/api
```

### 3. Frontend

```bash
cd web
pnpm install
# create .env.local with:
# NEXT_PUBLIC_API_URL=http://localhost:8080/v1
# NEXT_PUBLIC_GRAPHQL_URL=http://localhost:8080/graphql
pnpm dev                    # dev server on :3000
```

---

## Environment Variables

### API (`api/.env`)

| Variable               | Required | Description                                            |
| ---------------------- | -------- | ------------------------------------------------------ |
| `DATABASE_URL`         | yes      | PostgreSQL connection string                           |
| `REDIS_URL`            | yes      | Redis connection string                                |
| `JWT_PRIVATE_KEY`      | yes      | RS256 PEM private key                                  |
| `JWT_PUBLIC_KEY`       | yes      | RS256 PEM public key                                   |
| `PORT`                 | no       | Default `8080`                                         |
| `ENV`                  | no       | `local` \| `production`. Default `local`               |
| `LOG_LEVEL`            | no       | `debug` \| `info` \| `warn` \| `error`. Default `info` |
| `S3_BUCKET`            | no       | Receipts bucket name                                   |
| `AWS_REGION`           | no       | Default `ap-southeast-1`                               |
| `GOOGLE_CLIENT_ID`     | no       | Google OAuth client ID                                 |
| `GOOGLE_CLIENT_SECRET` | no       | Google OAuth client secret                             |
| `FRONTEND_URL`         | no       | Allowed CORS origin. Default `http://localhost:3000`   |

Generate RS256 keys:

```bash
openssl genrsa -out private.pem 2048
openssl rsa -in private.pem -pubout -out public.pem
```

### Frontend (`web/.env.local`)

```
NEXT_PUBLIC_API_URL=http://localhost:8080/v1
NEXT_PUBLIC_GRAPHQL_URL=http://localhost:8080/graphql
```

Production values are set in the Vercel dashboard — never commit them.

---

## Commands Reference

### API

```bash
make dev            # hot reload (air)
make run            # plain go run
make build          # build binary → ./api
make test           # all tests with race detector
make test-short     # skip slow testcontainer integration tests
make lint           # golangci-lint
make fmt            # gofmt
make migrate-up     # apply all pending migrations
make migrate-down   # roll back one migration
make db-up          # docker compose up -d
make db-down        # docker compose down
make db-reset       # down → up → migrate-up
make generate       # regenerate gqlgen code after schema changes
```

### Frontend

```bash
pnpm dev            # dev server
pnpm build          # production build
pnpm tsc --noEmit   # typecheck
pnpm lint           # eslint
pnpm format         # prettier
pnpm codegen        # generate TS types from GraphQL schema (API must be running)
pnpm test:e2e        # Playwright desktop/mobile smoke tests
```

### Infrastructure

```bash
cd infra
cdk diff --all                          # preview changes
cdk deploy --all --require-approval broadening   # deploy all stacks
cdk synth                               # dry run, generate CloudFormation
```

---

## Architecture Notes

**API constraints worth knowing:**

- All money is stored and transmitted as `int64` minor units (cents/paisa). Never float.
- All IDs are UUID v4. No auto-increment integers in the API.
- `domain/` has zero imports from `handler/`, `repository/`, or `graph/`. Services depend on interfaces only.
- GraphQL is read-only (activity feed, dashboard, expense history). All writes go through REST.
- Corrections create new snapshot rows — never update existing expense or split rows.
- The audit log is append-only. No `UPDATE` or `DELETE` against `audit_log`, ever.
- `SUM(split.share_amount)` must equal `expense.amount` or the request is rejected with `INVALID_SPLIT_SUM`.

**Frontend constraints worth knowing:**

- Access token is in memory only (`lib/auth.ts`). Never `localStorage`. Refresh token is HttpOnly cookie.
- On 401, `lib/api.ts` silently refreshes and retries once before redirecting to `/login`.
- React Query owns all server state. Zustand owns UI-only state (sidebar, modals, theme).
- `AmountInput` and `CurrencyAmount` are the only places amounts are formatted — never inline.

---

## Infrastructure (AWS)

Four CDK stacks deploy in dependency order to `ap-southeast-1`:

```
SplitlegerNetwork → SplitlegerData → SplitlegerApp → SplitlegerPipeline
```

- **Network** — VPC, one NAT gateway, private Lambda subnets, Redis/Lambda security groups, S3 gateway endpoint.
- **Data** — ElastiCache Redis, locked-down S3 receipts bucket, encrypted SSM parameters.
- **App** — native Go Lambda Function URL, VPC placement, SSM/KMS/S3/SES permissions, CloudWatch logs.
- **Pipeline** — GitHub OIDC role trusted only by `Ke-vin-S/ledger` on `main`.

The app stack bundles the API locally with Docker. `cdk synth` and CDK tests are safe local checks; `cdk diff` and `cdk deploy` are the commands that contact AWS.

First deployment:

```bash
cd infra
npx cdk synth
npx cdk diff --all
npx cdk bootstrap aws://ACCOUNT_ID/ap-southeast-1  # once
npx cdk deploy --all --require-approval broadening
```

After deployment, replace the `REPLACE_ME` values in the eight encrypted SSM parameters documented in `infra/README.md`. Set Vercel's `NEXT_PUBLIC_API_URL` to the `ApiFunctionUrl` stack output. The deploy workflow builds and publishes the native Lambda `bootstrap` binary through GitHub OIDC; no ECR, ECS, static AWS keys, or setup script is used.

---

## Logs

The API writes structured JSON to stdout. Forward to any service without code changes:

| Destination      | How                                              |
| ---------------- | ------------------------------------------------ |
| AWS CloudWatch   | Lambda logs are captured automatically           |
| Datadog          | `DD_LOGS_ENABLED=true` + container log tag       |
| Grafana Loki     | alloy or promtail on the container log stream    |
| Elastic / Splunk | Filebeat / Universal Forwarder on the log stream |

Set `LOG_LEVEL=debug` for verbose request-level detail.

---

## Docs

Full planning documents live in `docs/` — read on demand:

- `docs/spec.docx` — product requirements and feature scope
- `docs/data-model.docx` — full PostgreSQL schema, enums, invariants
- `docs/api-layer.docx` — REST endpoints, request/response shapes, error codes
- `docs/tech-stack.docx` — stack decisions and rationale
- `docs/frontend-plan.docx` — page inventory, component designs, React Query key map
