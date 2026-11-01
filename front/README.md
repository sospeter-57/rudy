# Rudy — frontend

A project/task command deck where completed work earns points. Built with **React 19 + Vite + TypeScript**, TanStack Query, Zustand and React Router.

The UI is designed against a documented API contract and ships with a fully seeded **mock API** so the whole product can be used with no backend running. The Go service under `../back` is untouched by this frontend.

---

## Quick start

```bash
cd front
npm install
npm run dev        # http://127.0.0.1:5173
```

Other scripts:

```bash
npm run build      # typecheck + production build
npm run preview    # serve the production build
npm run typecheck  # tsc only
```

### Demo accounts

All demo users use the password **`password123`**.

| Username | Role              | What they show                                   |
| -------- | ----------------- | ------------------------------------------------ |
| `nova`   | Project Manager   | Approvals, assignment, completion/payout          |
| `ari`    | Frontend Engineer | Assigned work, proposals, review bonus            |
| `kai`    | Backend Engineer  | Assignee payout from a completed task             |
| `mira`   | QA Engineer       | Reviewer queue and review bonus                   |
| `sol`    | UIUX Designer     | Proposals and a second review perspective         |

The landing page has one-click sign-in for **Nova** and **Ari**; the other accounts can be used through the normal form. New profiles can be created with any valid role. Everyone joins with the same profile type — the **role** decides permissions.

Demo state is stored in `localStorage`:

- `rudy.state.v1` — users, projects, tasks, ledger
- `rudy.session.v1` — current user id

The **Reset demo data** button on the Profile page restores the seed.

---

## API modes

Set these in `front/.env` (see `.env.example`):

```bash
VITE_API_MODE=mock   # default: seeded localStorage store, no backend needed
VITE_API_BASE=/api   # base path used when VITE_API_MODE=http
```

- **`mock`** — `src/lib/store.ts`. Every rule (permissions, lifecycle, payout) is enforced in the mock, so the demo is honest about what the real API must do.
- **`http`** — `src/lib/api.ts` `httpApi`. Requests go through the Vite dev proxy `/api → http://localhost:8080`, so the Go server needs no CORS changes.

The two implementations share the `ApiClient` interface, so switching modes does not touch UI code.

---

## Product rules

- **One profile type.** Anyone can register. `role` determines what they may do.
- **Only a Project Manager can create a project.**
- **Anyone can propose a task** inside a project.
- **A Project Manager must approve a proposal** before it is “fit to be worked on”. Rejection sends it back with a reason (`changes_requested`).
- Only an **approved** task can be assigned and started.
- The assignee (or a manager) records three acceptance gates: **code**, **tests**, **documentation**.
- The assignee adds **reviewers** (never themselves) and sends the task to **auditing**.
- Reviewers approve or request changes. A change request returns the task to the assignee.
- A Project Manager may mark a task **completed** from `pending` or `auditing` once all gates are filled, an assignee exists, and every reviewer has approved.

### Lifecycle

```
proposed ──approve──▶ approved ──assign + gates + reviewers──▶ auditing
   ▲                    │                                        │
   │                    └──────────── complete (manager) ────────┤
   │                                                             │
   └──── changes_requested ◀── reject / request changes ◀────────┘

auditing ──all reviewers approved + manager completes──▶ completed  (points minted)
```

### Permission matrix

| Action                       | Project Manager | Assignee | Reviewer | Other member |
| ---------------------------- | :-------------: | :------: | :------: | :----------: |
| Create project               |        ✅        |    —     |    —     |      —       |
| Propose task                 |        ✅        |    ✅     |    ✅    |      ✅       |
| Approve / reject task        |        ✅        |    —     |    —     |      —       |
| Assign task                  |        ✅        |    —     |    —     |      —       |
| Record gates / add reviewers |        ✅        |    ✅     |    —     |      —       |
| Send to review               |        ✅        |    ✅     |    —     |      —       |
| Review (approve / changes)   |    if added     |    —     |    ✅    |      —       |
| Complete + mint points       |        ✅        |    —     |    —     |      —       |

---

## The point economy

Points are **earned**, never granted by clicking a button. The design goals were: one source of truth, no double payouts, no self-dealing, and a visible profile payoff after every finished task.

### Ledger is the source of truth

Every award is an append-only `LedgerEntry`. A user's balance is the **sum of their ledger entries** — `user.points` is a cached projection, recomputed from the ledger after each write. There is no path that increments a balance directly, so balances cannot drift from history.

```
balance(user) = Σ ledger[user].points
```

### When points are minted

Payout happens **exactly once**, at the moment a Project Manager marks a task `completed`:

| Recipient              | Award                                  | Ledger reason    |
| ---------------------- | -------------------------------------- | ---------------- |
| Assignee               | full `task.points`                     | `task_completed` |
| Each approving reviewer | `max(2, round(task.points × 0.15))`    | `review_bonus`   |
| Manager                | **0** for approving or completing       | —                |

Rules that keep it honest:

- The **assignee cannot review their own task**, so they cannot farm review bonuses.
- Only reviewers whose decision is `approved` earn the bonus. Pending or change-requested reviewers earn nothing.
- Managers earn nothing for administrative actions — approval and completion are duties, not payouts.
- Proposals, rejections, assignments and work-in-progress never pay out.
- A completed task is locked: it cannot be completed or paid a second time.
- Payout is idempotent because the task transitions to `completed` in the same operation that appends the ledger rows. The real backend must perform both in **one transaction** with a uniqueness guard on `(task_id, user_id, reason)`.

### Worked example

Task worth **30 points**, assignee **Kai**, reviewers **Mira** and **Nova**, both approve:

| Person | Role     | Calculation          | Points |
| ------ | -------- | -------------------- | -----: |
| Kai    | Assignee | full task value      |  +30   |
| Mira   | Reviewer | `round(30 × 0.15)=5` |   +5   |
| Nova   | Reviewer | `round(30 × 0.15)=5` |   +5   |
| —      | Manager  | duty only            |    0   |
|        |          | **total minted**     | **40** |

This is exactly the demo flow: after completing task `#002`, Nova's profile goes **69 → 74** and Kai's goes **110 → 140**, with matching ledger entries.

### Tiers

Balances feed a tier ladder shown on the profile and rail:

| Tier       | Threshold |
| ---------- | --------: |
| Apprentice |         0 |
| Journeyman |       100 |
| Artisan    |       250 |
| Master     |       500 |
| Legend     |      1000 |

### Where the design lives

- `src/lib/points.ts` — rates, `awardPlanFor(task, users)`, tiers, `pendingRewards`
- `src/lib/store.ts` — ledger, completion transaction, balance projection, mock rules
- `src/features/tasks/TaskDetailPage.tsx` — payout preview + completion dialog
- `src/features/profile/ProfilePage.tsx` — balance, tier progress, full ledger, economy explainer

`awardPlanFor` is used both for the “Point split” preview and for the actual mock payout, so what the manager sees is what gets minted.

---

## Backend contract

The frontend is written against the interface in `src/lib/api.ts`. All JSON is expected in **camelCase** and all timestamps are ISO-8601 strings. Authenticated requests send `X-User-ID: <id>` (matching the existing Go middleware convention); the real service should replace this with a session token without changing the UI.

### Already present in `back/`

| Method | Path                          | Body                          | Response              |
| ------ | ----------------------------- | ----------------------------- | --------------------- |
| POST   | `/register`                   | `{username,password,role}`    | `{user}`              |
| POST   | `/login`                      | `{username,password}`         | `{user}`              |
| GET    | `/project/{id}`               | —                             | `project`             |
| GET    | `/user/tasks`                 | — (`X-User-ID`)               | `{tasks}`             |
| POST   | `/task/{id}/done/code`        | `{code}`                      | `{task}`              |
| POST   | `/task/{id}/done/tests`       | `{tests}`                     | `{task}`              |
| POST   | `/task/{id}/done/reviewer`    | `{reviewer_id}`               | `{task}`              |
| POST   | `/task/{id}/done/documentation` | `{documentation}`           | `{task}`              |

> The existing Go models use snake_case JSON tags (`task_type`, `task_state`, `created_at`, `acceptance_criteria`, …). Either emit camelCase or add a response mapper; the frontend currently consumes camelCase directly.

### Requested additions

**Users & session**

| Method | Path        | Body | Response                    |
| ------ | ----------- | ---- | --------------------------- |
| GET    | `/me`       | —    | `User`                      |
| GET    | `/users`    | —    | `{users: User[]}`           |
| GET    | `/leaderboard` | — | `{leaderboard: LeaderboardRow[]}` |
| GET    | `/user/{id}/ledger` | — | `{ledger: LedgerEntry[]}` |

**Projects**

| Method | Path        | Body                            | Response              |
| ------ | ----------- | ------------------------------- | --------------------- |
| GET    | `/projects` | —                               | `{projects: Project[]}` |
| POST   | `/project`  | `{name,description,ttl}`        | `{project}` (PM only) |

**Tasks**

| Method | Path                          | Body                                             | Response              |
| ------ | ----------------------------- | ------------------------------------------------ | --------------------- |
| GET    | `/tasks?project_id&assignee&reviewer&state` | —                              | `{tasks: Task[]}`     |
| GET    | `/task/{id}`                  | —                                                | `Task`                |
| POST   | `/project/{id}/task`          | `{description,taskType,role,points,dependencies}` | `{task}`              |
| POST   | `/task/{id}/approve`          | —                                                | `{task}` (PM only)    |
| POST   | `/task/{id}/reject`           | `{reason}`                                       | `{task}` (PM only)    |
| POST   | `/task/{id}/assign`           | `{assignee_id}`                                  | `{task}` (PM only)    |
| DELETE | `/task/{id}/reviewer/{userId}` | —                                               | `{task}`              |
| POST   | `/task/{id}/submit`           | —                                                | `{task}`              |
| POST   | `/task/{id}/review`           | `{decision,comment}` (`approved` \| `changes_requested`) | `{task}`      |
| POST   | `/task/{id}/complete`         | —                                                | `{task, awards: LedgerEntry[]}` (PM only) |

### Expected shapes

```ts
type User = { id: number; username: string; role: Role; points: number; createdAt: string }

type Project = { id: number; name: string; description: string; ttl: number; creatorId: number; createdAt: string }

type Task = {
  id: number
  projectId: number
  description: string
  taskType: 'feature' | 'bug' | 'fix' | 'research'
  taskState: 'pending' | 'auditing' | 'completed'
  approvalState: 'proposed' | 'approved' | 'changes_requested'
  creatorId: number
  assigneeId: number | null
  role: Role
  points: number
  dependencies: number[]
  acceptance: {
    code: string
    tests: string
    documentation: string
    reviewers: Array<{
      userId: number
      decision: 'pending' | 'approved' | 'changes_requested'
      comment?: string
      decidedAt?: string
    }>
  }
  rejectionReason?: string
  createdAt: string
  completedAt?: string
}

type LedgerEntry = {
  id: number
  userId: number
  taskId: number | null
  projectId: number | null
  reason: 'task_completed' | 'review_bonus' | 'adjustment'
  points: number
  note: string
  createdAt: string
}
```

### Server-side invariants the UI depends on

1. Reject completion unless `approvalState === 'approved'`, all three gates are non-empty, an assignee exists, and every reviewer is `approved`.
2. Enforce “assignee may not review their own task” on add-reviewer **and** on review submission.
3. Only a Project Manager may create projects, approve/reject, assign, or complete.
4. Mint points inside the completion transaction; guard against double payout with a unique constraint and a `completed` state check.
5. Compute balances from the ledger; never mutate `points` from an endpoint that is not a ledger append.
6. Return `403` for role violations, `409` for invalid state transitions, `422` for validation failures, `404` for missing records.

---

## Source map

```
src/
  components/        ui primitives, layout shell, task + point widgets
  features/
    auth/            landing, login, register, demo sign-in
    dashboard/       command deck, at-stake payouts, approvals, review queue
    projects/        portfolio, project board, create-project dialog
    tasks/           task table, task detail (gates, reviewers, completion), create-task
    review/          reviewer queue and history
    profile/         balance, tier ladder, full ledger, leaderboard, reset demo
  lib/
    api.ts           ApiClient interface + mock/http selection
    store.ts         seeded mock API, ledger, completion transaction
    points.ts        economy constants, award planning, tiers
    permissions.ts   role + lifecycle rules
    hooks.ts         TanStack Query hooks
    session.ts       Zustand session store
  styles/globals.css “Editorial Ember” design system
```

Design language: full-bleed editorial layout, Instrument Serif display type, Inter for UI, JetBrains Mono for metadata, with a black canvas, crimson signal, gold ledger lines and bone text.
