# Frontend guide

React 19 + Vite + TypeScript. There is no mock backend and no local seed data:
every screen reads and writes through `src/lib/api.ts`, which talks to the Go
service. TanStack Query owns server state; Zustand owns the session; component
state stays local.

## Running and building

```bash
npm install
npm run dev        # http://127.0.0.1:5173, proxies /api to :8080
npm run typecheck  # tsc -b --noEmit
npm run build      # tsc -b && vite build
npm run preview    # serve dist/
```

Configuration lives in `src/lib/config.ts` and comes from `VITE_*` variables
(see `.env.example`). `VITE_API_BASE` defaults to `/api`.
`VITE_PROXY_TARGET` (in `front/.env`) sets the Go API origin that the Vite dev
server proxies `/api` to; `VITE_DEV_PORT` sets the dev port.

## Routes

| Route | Component | Notes |
| --- | --- | --- |
| `/` | `auth/LandingPage` | Public. Login and registration. Redirects signed-in users to `/dashboard`. |
| `/dashboard` | `dashboard/DashboardPage` | Personal deck: my tasks, approvals waiting, review queue, points at stake. |
| `/projects` | `projects/ProjectsPage` | Project grid and create-project dialog (leads only). |
| `/projects/:id` | `projects/ProjectDetailPage` | One project's board, tasks, members and project rooms. |
| `/tasks` | `tasks/TasksPage` | Cross-project task list with filters and the proposal dialog. |
| `/tasks/:id` | `tasks/TaskDetailPage` | Gates, reviewers, approvals, submission and completion. |
| `/chat` | `features/chat/ChatPage` | Room list, DMs, discovery and group creation. |
| `/chat/:roomId` | `features/chat/ChatPage` | One conversation with text and image messages. |
| `/review` | `review/ReviewPage` | Tasks where the current user is a reviewer. |
| `/stats` | `stats/StatsPage` | Recharts dashboards from `GET /stats`. |
| `/profile` | `profile/ProfilePage` | `Builder <username>`, balance, tier, ledger, leaderboard. |
| `/settings` | `settings/SettingsPage` | Theme mode and palette saved to the account. |
| `*` | `NotFoundPage` | Inside the authenticated shell. |

Every route except `/` is wrapped in `Protected` (`src/routes.tsx`). Without a
session it redirects to `/`. The shell (`src/components/layout.tsx`) renders the
side rail, mobile nav, topbar, and a `ThemeSync` component that applies saved
preferences.

## State model

### Session (`src/lib/session.ts`, `src/lib/storage.ts`)

- `useSession` is a Zustand store holding `user`, `ready`, and actions.
- `setSession(user, token)` persists both. `signOut()` clears both.
- The token lives in `localStorage` under `rudy.token.v1`; the cached user under
  `rudy.user.v1`.
- `api.ts` attaches `Authorization: Bearer <token>` to every request.
- On a `401`, the client clears storage and dispatches `rudy:signout`; the
  store listens and resets, sending the user back to the landing page.
- Signing out calls `POST /logout` first so the token is revoked server-side,
  then clears local state.

Only the token and a cached user are persisted. There is no offline cache of
tasks or projects.

### Server state (`src/lib/hooks.ts`)

| Hook | Query key | Endpoint |
| --- | --- | --- |
| `useMe()` | `['me', userId]` | `GET /me`, refreshes the cached user when points change |
| `useProjects()` | `['projects']` | `GET /projects` |
| `useProject(id)` | `['project', id]` | `GET /project/{id}` |
| `useTasks(filter)` | `['tasks', filter]` | `GET /tasks?…` |
| `useTask(id)` | `['task', id]` | `GET /task/{id}` |
| `useUsers()` | `['users']` | `GET /users` |
| `useLedger(userId)` | `['ledger', userId]` | `GET /user/{id}/ledger` |
| `useLeaderboard()` | `['leaderboard']` | `GET /leaderboard` |
| `usePreferences()` | `['preferences']` | `GET /me/preferences` |
| `useStats(range)` | `['stats', range]` | `GET /stats?range=7d\|30d\|90d\|all` |
| `useChats()` / `useChat(id)` | `['chats']`, `['chat', id]` | Your rooms / one room |
| `useChatMessages(id)` | `['chat-messages', id]` | `GET /chat/{id}/messages` |
| `useChatUnread()` | `['chat-unread']` | `GET /chat/unread`, polls every 30s |
| `useChatRequests()` | `['chat-requests']` | `GET /chat/requests` |
| `useProjectMembers(id)` | `['project-members', id]` | `GET /project/{id}/members` |
| `useProjectChats(id)` | `['project-chats', id]` | `GET /project/{id}/chats` |
| `useChatStream()` | n/a | Authenticated SSE; invalidates chat queries |

`useChatStream` runs once in `AppShell`. It reconnects after a dropped stream and
refetches on reconnect, so a client that missed events still converges.

Mutations call `api.*` directly and then invalidate the relevant keys. Keep this
pattern: the API is the source of truth, and optimistic data is never invented
in the client.

### API client (`src/lib/api.ts`)

`api` implements the `ApiClient` interface. It:

- prefixes `config.apiBase`;
- sets `Content-Type: application/json` only for non-`FormData` bodies;
- attaches the bearer token when present;
- parses JSON and throws `ApiError(status, message)` for non-2xx responses;
- triggers the global sign-out on `401`;
- fetches chat images as authenticated blobs and reads the SSE stream from a
  `fetch` body (native `EventSource` cannot send the bearer header).

All return types are in `src/lib/types.ts`. When adding an endpoint, update
`ApiClient`, the `api` object, the types, and the query hook together.

## Permissions and lifecycle (`src/lib/permissions.ts`)

The UI mirrors backend rules. Do not hand-roll permission checks in components.

| Helper | Meaning |
| --- | --- |
| `isLead` | Role is exactly `Lead` |
| `canCreateProject`, `canApprove`, `canComplete` | Lead-only actions |
| `isAssignee` | Current user owns the task |
| `canWorkOn` | Assignee or lead |
| `canEditAcceptance` | `canWorkOn` **and** task approved **and** pending |
| `canAddReviewer` | Same as `canEditAcceptance` |
| `canSubmitForReview` | Approved + pending + all gates + at least one reviewer |
| `canReview` | Current user is a listed reviewer |
| `canFinalize` | Lead + approved + pending/auditing + gates + all reviewers approved |
| `gatesComplete` | Code, tests and documentation all non-empty |
| `reviewProgress` | Decided / total reviewers for the progress meter |

`canEditAcceptance` is the fix for the original approval-before-gates bug. Gate
fields and reviewer add/remove controls render only when it is true. If you add
a new editing surface, use this helper rather than comparing states inline.

Labels for states, approval states and task types are exported from the same
module so the board, detail page and stats page agree.

## Point economy (`src/lib/points.ts`)

`points.ts` is a presentation layer for rules the backend enforces:

- `reviewBonusFor(points)` = `max(minReviewBonus, round(points × reviewRate))`,
  defaulting to 15% with a floor of 2.
- `awardPlanFor(task, users)` previews the exact payout shown in the completion
  panel.
- `TIERS`, `tierFor`, `nextTierFor`, `tierProgress` drive the profile.
- `pendingRewards(tasks, userId)` sums approved, unfinished work.

Never compute a balance from these helpers. Balances come from `/me` and the
ledger. The helpers only preview what the server will mint.

## Theming

Preferences are account-level and backend-stored.

1. `SettingsPage` edits theme mode (`dark`, `light`, `system`), an accent colour,
   and two blend colours, with a live preview.
2. `usePreferences()` loads them; `ThemeSync` in the shell calls `applyTheme`.
3. `applyTheme` (`src/lib/theme.ts`) sets `data-theme` on `<html>` and writes
   `--gold`, `--red` and `--gold-bright` custom properties.
4. `watchSystemTheme` re-applies when the OS theme changes while mode is
   `system`.
5. `src/styles/globals.css` defines the dark palette on `:root` and a full
   light-theme override under `html[data-theme='light']`.

The rest of the UI consumes design tokens (`--ink`, `--bone`, `--gold`,
`--line`, …). To add a themed surface, use the tokens and add a light override
if the surface uses a dark-only translucent background.

## Charts (`StatsPage`)

`StatsPage` uses Recharts:

- area chart for proposals, completions and reviews over time;
- pie for tasks by state;
- bar charts for approval state, task type, ledger reasons;
- stacked bar plus a table for per-project totals;
- leaderboard list for people.

Charts only reshape API data with `useMemo`; they do not compute aggregates.
Colours reference the CSS custom properties where possible so charts follow the
theme, with fixed fallbacks for SVG fills. Range tabs (`30d`, `90d`, `all`) feed
`useStats(range)`.

## Styling conventions

- Global CSS in `src/styles/globals.css`, organised by feature blocks.
- Components use plain class names (`panel`, `stat-row`, `stats-grid`,
  `theme-option`, …), composed with the `cx` helper.
- Use existing tokens instead of raw colours. `rgba(...)` backgrounds are used
  for translucent dark surfaces; add a light-theme equivalent when introducing
  one.
- Icons come from `lucide-react`.
- Layout is responsive: the side rail collapses to a bottom mobile nav at
  `980px`; settings and stats grids collapse to one column.

## Source map

```
src/
  components/        UI primitives, shell layout, task/points widgets
  features/
    auth/            landing, sign in, registration
    dashboard/       personal command deck
    projects/        portfolio, project detail, create dialog
    tasks/           list, detail, proposal dialog
    review/          reviewer queue
    stats/           Recharts statistics page
    settings/        appearance preferences
    profile/         identity, ledger, tiers, leaderboard
  lib/
    api.ts           typed HTTP client
    session.ts       Zustand session store
    storage.ts       token/user persistence + sign-out event
    hooks.ts         TanStack Query hooks
    permissions.ts   role and lifecycle rules
    points.ts        economy helpers and tiers
    theme.ts         CSS custom-property theming
    config.ts        VITE_* configuration
    types.ts         shared API types
    cx.ts            class-name helper
    format.ts        dates, points, ids
```

## Adding a page

1. Add the type and API method in `lib/types.ts` and `lib/api.ts`.
2. Add a query hook in `lib/hooks.ts`.
3. Create the page under `features/<domain>/`.
4. Add the route in `routes.tsx`.
5. Add a nav item in `components/layout.tsx` if it belongs in the rail.
6. Add styles to `globals.css` using existing tokens.
7. Run `npm run typecheck && npm run build`.

## Testing

There is no frontend unit test suite yet. `npm run typecheck` and `npm run build`
are the required gates. Backend behaviour is covered by `go test ./...`; see
[Getting started](getting-started.md#tests-and-checks).
