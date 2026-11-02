# Rudy — frontend

React 19 + Vite + TypeScript client for the Rudy API. It contains no mock data:
every screen calls the Go service in `../back` through the bearer-token client in
`src/lib/api.ts`.

For the full documentation set start at [`../docs/README.md`](../docs/README.md).

## Run

```bash
npm install
npm run dev        # http://127.0.0.1:5173
```

The Vite dev server proxies `/api` to `VITE_PROXY_TARGET` in `front/.env`
(for example `http://localhost:8080`); the API origin is never hardcoded in
`vite.config.ts`.

```bash
npm run typecheck  # tsc only
npm run build      # typecheck + production build
npm run preview    # serve the build
```

## Configuration

```bash
VITE_API_BASE=/api
VITE_DEV_PORT=5173
VITE_PROXY_TARGET=http://localhost:8080
VITE_GOOGLE_CLIENT_ID=1234567890-abc.apps.googleusercontent.com   # optional
```

`VITE_API_BASE` is the only required variable. `VITE_GOOGLE_CLIENT_ID` enables
the Google sign-in button; it must match `RUDY_GOOGLE_CLIENT_ID` in `back/.env`.
All other tunables have safe defaults in `src/lib/config.ts`.

## Routes

| Route | Page | Purpose |
| --- | --- | --- |
| `/` | Landing | Login and registration |
| `/dashboard` | Dashboard | Personal deck, approvals, review queue, at-stake points |
| `/projects` | Projects | Portfolio and project creation |
| `/projects/:id` | Project detail | Board, tasks and team for one project |
| `/tasks` | Tasks | Cross-project task list and proposal dialog |
| `/tasks/:id` | Task detail | Gates, reviewers, approvals, completion |
| `/review` | Review | Reviewer queue and history |
| `/stats` | Stats | Aggregated charts from `GET /stats` |
| `/profile` | Profile | Ledger, tier progress, leaderboard |
| `/settings` | Settings | Theme and palette stored on the account |

## Source map

```
src/
  components/        UI primitives, layout shell, task and points widgets
  features/
    auth/            landing, login and registration
    dashboard/       command deck
    projects/        portfolio, project board, create-project dialog
    tasks/           task table, detail page, proposal dialog
    review/          reviewer queue
    stats/           Recharts dashboards
    settings/        appearance and account preferences
    profile/         balance, ledger, leaderboard
  lib/
    api.ts           typed HTTP client (the only data source)
    session.ts       Zustand session store
    storage.ts       token + user persistence
    hooks.ts         TanStack Query hooks
    permissions.ts   role and lifecycle rules
    points.ts        economy helpers and tiers
    theme.ts         CSS custom-property theming
```

## Session

Login and registration return `{ user, token, expiresAt }`. The token is stored in
`localStorage` under `rudy.token.v1` and sent as `Authorization: Bearer <token>`.
A `401` clears the stored session and returns the user to the landing page. Logout
calls `POST /logout`, which revokes the token server-side.
