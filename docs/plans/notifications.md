# Feature plan — Notifications

## Goal

Keep people in the loop without making them refresh the board. When a task moves,
when someone is waiting on them, or when their work is blocked, Rudy should say so
— in the app first, by email as an opt-in second.

## User stories

- As a builder, I want to know when my proposal is approved or sent back, so I can
  act without polling.
- As a builder, I want to know which of my tasks are blocked and by whom, so I can
  unblock myself or ask for help.
- As a reviewer, I want a reminder when a task is waiting on my decision.
- As a lead, I want a digest of proposals awaiting approval and reviews that
  have stalled.
- As a user, I want to mute categories I do not care about.

## Event catalogue

| Event | Recipients | Default channel |
| --- | --- | --- |
| `task.proposed` | Project leads | In-app |
| `task.approved` | Creator, assignee | In-app |
| `task.changes_requested` | Creator | In-app |
| `task.assigned` | Assignee | In-app + email |
| `task.reviewer_added` | Reviewer | In-app |
| `task.submitted` | Reviewers, leads | In-app + email |
| `review.approved` | Assignee, leads | In-app |
| `review.changes_requested` | Assignee | In-app + email |
| `task.completed` | Assignee, reviewers, creator | In-app |
| `points.minted` | Awarded users | In-app |
| `task.blocked` | Assignee | In-app (derived) |
| `review.overdue` | Reviewer | In-app + email (derived) |

Derived notifications are produced by a scheduled sweep, not by a user action.

## Data model

New migration `0002_notifications.sql`:

```sql
CREATE TABLE notifications (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind         TEXT NOT NULL,
  title        TEXT NOT NULL,
  body         TEXT NOT NULL DEFAULT '',
  project_id   INTEGER REFERENCES projects(id) ON DELETE CASCADE,
  task_id      INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
  actor_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
  dedupe_key   TEXT,
  read_at      TEXT,
  created_at   TEXT NOT NULL
);

CREATE UNIQUE INDEX notifications_dedupe
  ON notifications(user_id, dedupe_key)
  WHERE dedupe_key IS NOT NULL;

CREATE INDEX notifications_user_created
  ON notifications(user_id, created_at DESC);
CREATE INDEX notifications_user_unread
  ON notifications(user_id, read_at);
```

Add per-user delivery preferences:

```sql
CREATE TABLE notification_preferences (
  user_id       INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  in_app        INTEGER NOT NULL DEFAULT 1,
  email         INTEGER NOT NULL DEFAULT 0,
  digest        TEXT NOT NULL DEFAULT 'daily', -- off | daily | weekly
  muted_kinds   TEXT NOT NULL DEFAULT '',      -- comma-separated kinds
  updated_at    TEXT NOT NULL
);
```

`dedupe_key` examples: `review_reminder:task:7:user:4:2026-11-02`. A unique
partial index makes the sweep safe to retry.

## Backend work

1. Add notification models and repository methods:
   `InsertNotification`, `ListNotifications(userID, cursor, limit)`,
   `UnreadCount(userID)`, `MarkRead(userID, ids)`, `MarkAllRead(userID)`,
   `NotificationPreferences`, `UpsertNotificationPreferences`.
2. Add a `Notifier` interface in `internal/services` with a no-op implementation,
   so task services depend on an interface rather than SMTP or SQL details.
3. Emit events **after** the transaction commits. Never send email or write a
   second row inside the payout transaction.
4. Fan-out rules live in one table-driven function so recipients are testable.
5. Skip self-notifications (`actor_id == user_id`) by default.
6. Add endpoints:

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/notifications?cursor=&limit=&unread=` | Paginated list |
| GET | `/notifications/unread-count` | Badge count |
| POST | `/notifications/read` | Body `{ "ids": [1,2] }` |
| POST | `/notifications/read-all` | Clear badge |
| GET | `/me/notification-preferences` | Delivery settings |
| PUT | `/me/notification-preferences` | Update settings |

7. Add a `Notifier` implementation that writes in-app rows. Email sits behind the
   same interface.
8. Add an in-process scheduler (a goroutine with a ticker, stopped on shutdown)
   for:
   - `task.blocked`: assignee has an approved, incomplete task whose dependency
     is still incomplete;
   - `review.overdue`: task in `auditing` with a pending reviewer decision older
     than N hours;
   - daily/weekly digest.
9. Add `RUDY_NOTIFICATIONS_ENABLED`, `RUDY_SMTP_*`, `RUDY_DIGEST_HOUR` config.

## Email delivery

- Use `net/smtp` or a provider SDK behind a `Mailer` interface.
- Render plain text plus a minimal HTML template.
- Batch digests: one email per user per period, grouped by project.
- Include an unsubscribe token or a link to preferences. Never require login to
  unsubscribe.
- Retry with capped backoff; record failures without failing the request.

## Frontend work

- Add `useNotifications`, `useUnreadCount`, `useNotificationPreferences`.
- Bell icon in the topbar with an unread badge; opening it shows the latest 20
  with a "View all" link.
- Deep-link each notification to its task, project or profile.
- Mark read on open, with an explicit "Mark all read".
- Add a Notifications section to **Settings**: in-app, email, digest cadence, and
  category mutes.
- Poll the unread count every 30–60 seconds initially. Replace with SSE when chat
  lands (see [chat](chat.md)).
- Keep notifications out of the task cache: they are their own query key.

## Rollout

1. Schema + repository + tests (no UI). Verify fan-out with service tests.
2. In-app only, feature-flagged by `RUDY_NOTIFICATIONS_ENABLED`.
3. Bell + list + preferences UI.
4. Derived sweeps and digests.
5. Email provider integration last, after preference controls exist.

## Risks

| Risk | Mitigation |
| --- | --- |
| Notification spam | Default to in-app only, dedupe keys, category mutes, digest batching |
| Event emission inside transactions | Emit after commit through a small outbox or post-commit hook |
| Email deliverability | Use a reputable provider, SPF/DKIM, bounce handling, plain-text fallback |
| Sweep duplicates | Unique partial index on `(user_id, dedupe_key)` |
| Single SQLite writer contention | Keep writes small and batched; run sweeps at low-traffic times |
| Privacy leaks in notification bodies | Store ids, render content from the task at read time where possible |

## Success measures

- Median time from `task.submitted` to first reviewer action.
- Share of proposals approved within 24 hours.
- Weekly active users with at least one notification opened.
- Unsubscribe/mute rate below an agreed threshold.
