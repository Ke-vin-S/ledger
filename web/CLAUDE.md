# SplitLedger — Next.js Frontend

Next.js 15 (App Router), Tailwind CSS v4, React Query v5, Zustand, shadcn/ui. Deployed on Vercel.

## Commands

```bash
# Install
pnpm install

# Dev server
pnpm dev

# Build
pnpm build

# Typecheck
pnpm tsc --noEmit

# Lint
pnpm lint

# Generate TypeScript types from GraphQL schema
# (requires API running on localhost:8080)
pnpm codegen

# Format
pnpm format
```

## Environment

`.env.local` for local dev (gitignored). Required vars:

```
NEXT_PUBLIC_API_URL=http://localhost:8080/v1
NEXT_PUBLIC_GRAPHQL_URL=http://localhost:8080/graphql
```

Production values are set in Vercel dashboard — never commit them.

## Structure

```
app/
  (auth)/            # public route group — login, register, claim/[token]
  (app)/             # authenticated route group — layout.tsx guards auth
    layout.tsx       # sidebar, FAB, auth guard
    dashboard/
    teams/[teamId]/
    loans/
    notifications/
    settings/
components/
  ui/                # shadcn/ui primitives — do not edit generated files here
  expense/           # ExpenseCard, SplitBuilder, AmountInput
  settlement/        # SettlementSheet, DebtBar
  team/              # ActivityFeed, MemberList, InviteModal
  shared/            # Avatar, CurrencyAmount, DateDisplay
lib/
  api.ts             # typed fetch wrapper with silent token refresh
  auth.ts            # access token memory store, setAccessToken
  graphql/           # graphql-request client, query strings, generated types
  utils.ts           # formatAmount, formatDate
hooks/               # React Query hooks — useTeam, useExpenses, useNotifications
store/
  ui.ts              # Zustand — sidebar, modals, theme
```

## Architecture

- Server Components by default. Add `"use client"` only where interactivity is required.
- React Query owns all server state. Zustand owns UI-only state (sidebar open, modal stack, theme). No exceptions.
- Access token stored in memory (`lib/auth.ts` module scope). Never localStorage, never a cookie accessible to JS. Refresh token is HttpOnly cookie — the browser sends it automatically.
- On 401: `lib/api.ts` silently calls `POST /auth/refresh`, retries the original request once. If refresh fails → redirect to `/login`.
- GraphQL is read-only (activity feed, dashboard aggregates, expense history). All mutations use REST via `lib/api.ts`.
- All money values in API calls are `int64` minor units. `AmountInput` converts display string ↔ minor units. `CurrencyAmount` formats minor units for display. Never pass floats to the API.
- React Query key structure is `["resource", id?, "sub-resource"]`. See @docs/frontend-plan.md §7.1 for the full key map.
- Optimistic updates: cancel in-flight queries → snapshot → mutate → roll back on error. See @docs/frontend-plan.md §7.2 for the pattern.

## Conventions

- Amounts displayed as: `LKR 1,500.00` (ISO code + space + comma-separated + 2 decimal places). Positive amounts owed to user: prefix `+`, green text. Negative: no prefix, red text. Use `CurrencyAmount` component — never format inline.
- Theme: CSS custom properties in `globals.css`. Dark mode via `.dark` class on `<html>`. `ThemeToggle` in Zustand store. IMPORTANT: inline script in `app/layout.tsx` prevents FOUC — do not remove it.
- shadcn components live in `components/ui/`. They are copied source — edit freely. Do not run `shadcn add` without checking for conflicts with existing customisations.
- Route protection: `(app)/layout.tsx` checks auth server-side and redirects. Client components use `useAuth()` hook and show skeletons while loading.
- Notification polling: `useNotifications` hook polls every 30s via `refetchInterval`. Do not add WebSockets or SSE for v1.

## Gotchas

- `graphql-codegen` generates types into `lib/graphql/types.ts`. Run `pnpm codegen` after any GraphQL schema change on the backend. Committing stale types causes TypeScript errors at build time.
- Tailwind v4 reads CSS variables directly — no `tailwind.config.ts` colour extension needed. Adding colours there will be silently ignored.
- `next/font` is configured in `app/layout.tsx`. Do not import fonts anywhere else — causes duplicate font requests.
- shadcn `Sheet` component: always reset the form with `reset()` on close. Stale form values persist between opens otherwise.
- `AmountInput` stores minor units as integer. Zod schema must validate the integer, not the display string. A `z.number().int().positive()` schema is correct; `z.string()` is not.
- Vercel hobby plan: 100 GB bandwidth/month, 12 serverless function executions/month (Next.js API routes). Keep API routes minimal — most calls go directly to the Go backend.

## Never Do

- Never store the access token in localStorage or sessionStorage.
- Never call the API with a float amount. Always convert to minor units first.
- Never add mutations to the GraphQL schema or client — REST only for writes.
- Never import from `app/` inside `components/` or `lib/` — it creates circular dependencies.
- Never use `useEffect` to fetch data — use React Query hooks.

## Reference

- @docs/frontend-plan.docx — page inventory, component designs, state management, build phases
- @docs/api-layer.docx — endpoint contracts, request/response shapes, GraphQL query schemas
