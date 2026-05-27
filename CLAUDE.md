# SplitLedger

Personal and team expense tracking platform. Users track income/expenses, split bills across teams, manage direct loans, and maintain a full audit trail of every financial event including corrections.

Monorepo: `api/` (Go), `web/` (Next.js), `infra/` (AWS CDK TypeScript). Each sub-project has its own `CLAUDE.md`.

## Structure

```
splitleger/
├── api/          # Go REST + GraphQL backend
├── web/          # Next.js 15 frontend
├── infra/        # AWS CDK TypeScript infrastructure
└── docs/         # Planning documents (spec, data model, API layer, tech stack, frontend plan)
```

## Sub-project CLAUDE.mds

Read the relevant one before working in that directory:

- @api/CLAUDE.md — Go API: commands, conventions, architecture
- @web/CLAUDE.md — Next.js frontend: commands, conventions, patterns
- @infra/CLAUDE.md — CDK infra: deploy commands, stack structure

## Reference Docs

Full planning documents in docs/ — read on demand, not upfront:

- @docs/spec.docx — product requirements and feature scope
- @docs/data-model.docx — PostgreSQL schema, all tables, enums, invariants
- @docs/api-layer.docx — REST endpoints, GraphQL queries, auth flow, error codes
- @docs/tech-stack.docx — stack decisions and rationale
- @docs/frontend-plan.docx — page inventory, component designs, state management

## Repo

GitHub: https://github.com/Ke-vin-S/ledger
