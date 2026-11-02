# Roadmap

Rudy already closes the core loop: propose → approve → build → review → get paid.
The work below extends that loop without weakening its invariants. Priorities
reflect impact on collaboration first, then knowledge sharing, then automation.

## Guiding principles

1. **The ledger stays append-only.** New features may read balances, never write
   them directly.
2. **Approval-first stays intact.** Notifications and automation may prompt
   actions but never bypass a lifecycle gate.
3. **Backend is the source of truth.** Any new client state must have an API
   behind it.
4. **Ship in vertical slices.** Each plan starts with schema + API + tests, then
   UI, then delivery channels.
5. **Privacy by default.** Chat, articles and AI features handle user content;
   make visibility and retention explicit.

## Now → next → later

| Horizon | Theme | Items |
| --- | --- | --- |
| Now | Collaboration | In-app notifications, task blocking visibility, digest email |
| Next | Conversation | Project chat, task threads, presence |
| Next | Knowledge | Articles and experience posts, reactions, comments |
| Later | Intelligence | Stats insights, proposal drafting, review checklists, semantic search |
| Continuous | Hardening | CORS allow-list, rate limiting, observability, pagination, frontend tests |

## Milestone 1 — Notifications

**Problem:** users only learn about progress by refreshing the board. Blocked
tasks and pending reviews stall silently.

**Scope**

- `notifications` table and per-user read state.
- Events emitted from existing services: task proposed, approved, rejected,
  assigned, submitted, review decided, completed, reviewer added, points minted.
- Derived "blocked on" notifications when a dependency is incomplete.
- In-app bell with unread count, notification list and deep links.
- Daily/weekly digest email (opt-in).
- Per-user notification preferences.

**Definition of done**

- An event in any service produces at most one notification per recipient.
- Notifications never block the originating transaction; emit after commit.
- Users can mark one or all as read and mute categories.
- Tests cover fan-out, deduplication and authorization.

Details: [plans/notifications.md](plans/notifications.md).

## Milestone 2 — Chat

**Problem:** discussion about a task lives outside the tool, so decisions and
context are lost.

**Scope**

- Project channels and task threads.
- Server-Sent Events or WebSocket transport.
- Message history, edit/delete, mentions.
- Mentions generate notifications.
- Basic moderation controls.

**Definition of done**

- Messages are durable and paginated.
- Only project members (all authenticated users, initially) can read a channel.
- Reconnect resumes without gaps.
- Chat load does not contend with the single SQLite writer for long.

Details: [plans/chat.md](plans/chat.md).

## Milestone 3 — Articles and experience posts

**Problem:** knowledge gained while building is trapped in chat and heads.

**Scope**

- Markdown posts with title, summary, body, tags and visibility.
- Draft → published lifecycle.
- Reactions and comments.
- Author profile and tag filtering.
- Optional link from a post to a task or project.

**Definition of done**

- Drafts are private; published posts are readable by signed-in users.
- Sanitised rendering, no raw HTML injection.
- Pagination and tag filters are server-side.

Details: [plans/articles.md](plans/articles.md).

## Milestone 4 — AI assistance

**Problem:** managers and builders spend time on summaries, estimates and
review checklists that a model can draft.

**Scope (in order)**

1. **Stats narratives** — turn `/stats` into a short weekly insight summary.
2. **Proposal assist** — suggest description, type, points and acceptance gates
   from a short brief.
3. **Review checklist** — generate questions from a task and its evidence.
4. **Semantic search** — search tasks, articles and chat by meaning.
5. **Assistant panel** — scoped Q&A over the current project.

**Definition of done**

- AI output is always labelled, dismissible and never auto-applied.
- No user content is sent to a provider without an explicit org-level setting.
- Every AI action is rate-limited and auditable.

Details: [plans/ai.md](plans/ai.md).

## Continuous hardening

These are small but should land alongside feature work.

### Security and operations

- CORS allow-list driven by `RUDY_ALLOWED_ORIGINS`.
- Per-IP and per-session rate limits on auth and write endpoints.
- Security headers (`CSP`, `X-Content-Type-Options`, `Referrer-Policy`).
- Structured request IDs and duration histograms.
- Backup/restore procedure for the SQLite file.
- Optional Postgres driver behind the repository interface.

### API and data

- Cursor pagination for tasks, ledger, leaderboard and notifications.
- Consistent wrapped response shapes (some endpoints return bare objects today).
- Soft-delete or archive for projects.
- Audit log for manager actions (approve, reject, assign, complete).
- Export a user's ledger as CSV.

### Frontend

- Unit tests for `permissions.ts`, `points.ts` and `api.ts`.
- Component tests for the task lifecycle.
- Error boundaries and retry affordances.
- Accessibility pass: focus traps, labels, keyboard navigation, contrast in
  both themes.
- Optimistic updates for review decisions, with rollback.

### Product

- Team/workspace boundaries if Rudy is used by more than one group.
- Saved filters and board views.
- Bulk assignment for managers.
- Task templates.
- Time tracking alongside points.

## Sequencing rationale

1. Notifications are cheap, depend only on existing events, and fix the biggest
   collaboration gap.
2. Chat and articles both create user-generated content; build moderation and
   notification plumbing first so they can reuse it.
3. AI depends on clean data and good search; ship it last and keep it advisory.
4. Hardening items are scheduled continuously rather than as a phase, because
   each feature adds attack surface and load.

## Not planned

- Paying real money for points.
- Letting managers edit balances directly.
- Bypassing review for "urgent" tasks by default.
- Public anonymous access. Rudy is a team tool; every read is authenticated.
