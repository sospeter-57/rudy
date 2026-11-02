# Feature plans

Detailed, implementation-ready plans for the next major features. The
prioritisation and rationale live in [../roadmap.md](../roadmap.md). Each plan
covers the problem, data model, APIs, backend and frontend work, rollout and
risks.

| Plan | Summary | Depends on |
| --- | --- | --- |
| [Notifications](notifications.md) | In-app and email events for task progress, blocked work and stale reviews | Existing task events |
| [Chat](chat.md) | Project channels and task threads over SSE, with mentions and unread state | Notification service for mentions |
| [Articles](articles.md) | Markdown experience posts, tags, reactions and comments | Notifications for publish/comment events |
| [AI](ai.md) | Grounded stats insights, proposal drafts, review checklists and search | Clean stats data; optional FTS5 |

## Common delivery pattern

Every plan follows the same sequence:

1. **Schema** — a numbered embedded migration, reversible by restoring a backup
   during development.
2. **Repository + service** — business rules in services, SQL in the repository.
3. **Tests** — service tests for rules, router tests for auth and status codes.
4. **HTTP API** — camelCase JSON, `AppError` kinds mapped to status codes.
5. **Frontend** — types, API client, query hooks, page, route, nav, styles.
6. **Delivery** — feature flag, then default on after one release cycle.
7. **Docs** — update `docs/api.md`, `docs/frontend.md`, and the user guide.

## Shared conventions

- New tables use `INTEGER PRIMARY KEY AUTOINCREMENT`, UTC RFC3339 timestamps, and
  `REFERENCES … ON DELETE CASCADE` where ownership is clear.
- Add indexes for every feed or lookup path used by a list endpoint.
- Paginate with keyset cursors (`before`/`after` + `limit`), not offsets.
- Emit side effects (notifications, chat events) after the transaction commits.
- Respect the single SQLite writer: keep writes short and never hold rows open
  while starting a transaction.
- Soft-delete user content (`deleted_at`) so moderation and recovery are possible.
- Feature-flag every new surface with a `RUDY_*` environment variable.

## Before starting any plan

- Confirm the feature flag name and default.
- Add the migration number after the latest existing file.
- Sketch the response shapes and update `docs/api.md` first.
- Decide the retention and privacy rules for any user-generated content.
- Add at least one failure-path test (unauthorized, conflict, empty input).
