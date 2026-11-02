# Architecture

## Overview

Rudy is a two-tier application with a deliberately small operational surface.

```
Browser (React)
   │  fetch + Authorization: Bearer <opaque token>
   ▼
Go HTTP API (net/http, Go 1.22+ routing)
   │  services / repository
   ▼
SQLite (modernc.org/sqlite, WAL, foreign keys)
```

No mock backend exists. The client's only data source is `front/src/lib/api.ts`,
which talks to the Go service through `/api` in development.

## Backend layers

```
back/
  main.go                     config, DB open, Migrate, service wiring, graceful shutdown
  internal/models/            JSON DTOs, roles, states, AppError kinds
  internal/transaction/
    db.go                     Open + embedded migration runner
    migrations/*.sql          numbered schema files
    repo.go                   all SQL queries and transaction helpers
  internal/services/
    user.go                   register/login/logout/authenticate/users/leaderboard
    project.go                project rules
    task.go                   lifecycle, gates, reviews, completion transaction
    stats.go                  SQL aggregations for /stats
    preferences.go            account theme preferences
  internal/routes/
    routes.go                 dependency wiring and route table
    middleware.go             bearer auth, logging, panic recovery, content type
    handlers.go               HTTP handlers
    json.go                   decode/encode and AppError → status mapping
    contracts.go              request DTOs
```

Dependencies point downward only: routes → services → transaction/repository →
database. Services never import routes.

### Timestamps

All timestamps are stored as fixed-width UTC RFC3339 strings. Fixed width matters
because session expiry is compared lexicographically in SQLite. Use
`transaction.TimeLayout()` rather than hard-coding a layout.

## Database schema

The schema is created by embedded migrations (`0001_init.sql`,
`0003_leads_and_project_members.sql`, `0004_chat.sql`). Migrations are embedded
in the binary, sorted by numeric filename prefix, and each runs in its own
transaction. Applied versions are recorded in `schema_migrations`.

| Table | Purpose |
| --- | --- |
| `users` | Account, bcrypt password hash, role, cached point balance |
| `sessions` | Opaque session id, SHA-256 token hash, expiry, revocation |
| `projects` | Project metadata and creator |
| `project_members` | Explicit membership curated by the project lead |
| `tasks` | Task state, approval state, assignee, role, points, timestamps |
| `task_acceptance` | One row per task: code, tests, documentation evidence |
| `task_reviewers` | Reviewer list with decision, comment and decided time |
| `task_dependencies` | Many-to-many task dependency edges |
| `ledger_entries` | Append-only point awards; source of truth for balances |
| `user_preferences` | Theme mode and palette per user |
| `chat_rooms` | `dm`, `project` or `open` room metadata |
| `chat_members` | Room membership and last-read pointer |
| `chat_messages` | Plain-text messages with soft delete |
| `chat_attachments` | Image metadata and storage keys |
| `chat_requests` | Group requests, auto-approved today |
| `chat_request_invites` | Invitees recorded with a group request |

Indexes cover sessions by user and expiry, tasks by project/assignee/state/
approval state, reviewers by user, and ledger by user and creation time.

A partial unique index enforces one award per `(task_id, user_id, reason)` so a
task cannot be paid twice even if application checks are bypassed.

## Authentication and sessions

1. `POST /register` or `POST /login` validates credentials.
2. Passwords are hashed with bcrypt (`golang.org/x/crypto/bcrypt`).
3. The server generates two random 32-byte values: a session id and the token.
4. Only `sha256(token)` is stored in `sessions.token_hash`. The plaintext token is
   returned once, in the auth response.
5. Clients send `Authorization: Bearer <token>`.
6. Middleware hashes the token, joins `sessions` to `users`, and rejects missing,
   revoked or expired sessions.
7. `POST /logout` sets `revoked_at`; the token is immediately unusable.
8. Expired and revoked sessions are cleaned up on each new login/register.

Tokens are opaque: they carry no user data and cannot be forged by inspecting the
client. A database leak does not expose usable tokens because only hashes are
stored.

Public routes are `GET /health`, `POST /register`, `POST /login` and
`POST /auth/google`. Every other route requires a bearer token.

## Domain rules

### Roles

Any registered user can propose tasks. Only a user whose role is exactly
`Lead` can create projects, approve/reject proposals, assign tasks, or
complete tasks.

### Task lifecycle

```
proposed ──approve──▶ approved ──assign + gates + reviewers──▶ auditing
   ▲                    │                                        │
   │                    └──────────── complete (lead) ────────┤
   │                                                             │
   └──── changes_requested ◀── reject / request changes ◀────────┘

auditing ──all reviewers approved + lead completes──▶ completed
```

State fields:

- `approval_state`: `proposed`, `approved`, `changes_requested`
- `task_state`: `pending`, `auditing`, `completed`

### Approval-first invariant

Acceptance gates and reviewers may only be edited when:

- the actor is the assignee or a Lead, **and**
- `approval_state === 'approved'`, **and**
- `task_state === 'pending'`.

This rule exists in `services.requirePreparation` and is mirrored in the frontend
by `canEditAcceptance` in `front/src/lib/permissions.ts`.

Assignment also requires approval. Submission requires approval, pending state,
all three gates, and at least one reviewer.

### Reviews

- Only listed reviewers may decide.
- The assignee may not be added as a reviewer and may not submit a review.
- `changes_requested` returns the task to `pending`.
- Re-submitting resets every reviewer decision to `pending` so the new evidence is
  reviewed again.
- `completed` requires every reviewer to be `approved` when reviewers exist.

### Completion and payout

Completion is a single database transaction:

1. Re-read the task inside the transaction.
2. Reject if it is already completed, not approved, not `pending`/`auditing`,
   missing gate evidence, missing an assignee, or has a non-approved reviewer.
3. Insert the assignee ledger entry for the full task value
   (`task_completed`).
4. Insert one ledger entry per approving reviewer (`review_bonus`).
5. Mark the task completed.
6. Recompute affected users' cached balances as `SUM(ledger_entries.points)`.

The unique ledger index and the completed-state check make payout idempotent.
Leads earn nothing for approvals or completion.

Review bonus: `max(2, round(task.points × 0.15))`, implemented as
`models.ReviewBonus`.

### Chat and project membership

- Creating a project inserts a `project_members` row for the creator with role
  `lead`. Adding project members is lead-only; removing one also removes them
  from the project's chat rooms in the same transaction.
- Global `Lead` (project creation and task approval) is separate from the
  per-project lead, which is always `projects.creator_id`.
- Project chat rooms are created and archived by the project lead. Only selected
  `project_members` are added, and only current room members can read messages
  or attachments.
- DMs are keyed by the sorted pair of user IDs; a partial unique index guarantees
  one room per pair. Open groups are member-owned and may be discoverable.
- Open group creation is auto-approved in v1. The room is active immediately and
  a `chat_requests` row with `status = 'approved'` plus its invites is stored for
  a future admin review page.
- Attachments are stored through `storage.AttachmentStore`; the local
  implementation writes under `RUDY_UPLOAD_DIR`. Attachment rows are room-scoped
  and can only be attached to a message by their uploader, once.
- Chat events are published after commit through an in-process hub and delivered
  over SSE. The hub assumes a single API instance.

## Error mapping

Services return `*models.AppError` with a kind. The HTTP layer maps it:

| Kind | HTTP status |
| --- | --- |
| `not_found` | 404 |
| `unauthorized` | 401 |
| `forbidden` | 403 |
| `conflict` | 409 |
| `validation` | 422 |
| anything else | 500 (logged, details hidden) |

Malformed JSON or bad path/query parameters return 400. A body with the wrong
content type returns 415.

## Stats

`GET /stats?range=7d|30d|90d|all` aggregates:

- account, builder, project and task totals;
- completion rate and average cycle time;
- review approval rate;
- task counts by state, approval state and type;
- role distribution;
- per-project task/point totals;
- per-user completed tasks and reviews;
- daily proposal/completion/review/point trends;
- ledger totals by reason.

All aggregation happens in SQL. The frontend only shapes the arrays for Recharts.

## Operational notes

- The HTTP listener binds to `RUDY_HOST`:`RUDY_PORT` (default `0.0.0.0:8080`).
  `RUDY_PORT` falls back to the platform `PORT`, and `RUDY_ADDR` overrides both
  when a single address or bare port is more convenient.
- Chat images live under `RUDY_UPLOAD_DIR` (default `uploads`); mount a
  persistent volume for it in production.
- SQLite runs with `foreign_keys=1`, `busy_timeout=5000` and `journal_mode=WAL`.
- The pool is limited to one connection to keep writes predictable; do not hold
  result sets open while starting a transaction (the migration runner closes its
  `schema_migrations` rows before applying migrations for this reason).
- There is no background worker. Session cleanup happens inline during auth writes.
- CORS is an explicit allow-list from `RUDY_CORS_ORIGINS`; no origin is
  hardcoded. Development uses the Vite proxy, so the variable can stay empty
  there. Set it when the client and API are served from different origins.
