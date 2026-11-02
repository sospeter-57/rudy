# Feature plan — Chat

## Goal

Give teams a place to talk where the work lives. Chat should sit beside the board:
a project channel for general coordination, and a thread on every task for
decisions, links and review context. Discussion should stop leaking into external
tools and disappearing.

## Scope

### In scope (v1)

- One channel per project, created lazily.
- Task threads.
- Text messages with links and basic Markdown (bold, italic, code, lists).
- Edit and delete your own message.
- Mentions (`@username`) that create notifications.
- Message history with cursor pagination.
- Unread counts per channel/thread.
- A simple presence indicator ("3 online in this project").

### Out of scope (v1)

- Direct messages between users.
- File uploads and image attachments.
- Voice/video.
- Cross-workspace federation.
- End-to-end encryption.

## Transport

Use **Server-Sent Events (SSE)** for server → client and normal `POST` requests
for client → server.

Why not WebSockets first: SSE is plain HTTP, works with the existing `net/http`
router and bearer middleware, reconnects natively, and needs no new dependency.
It is one-way, which is all a chat feed needs; sending is a regular authenticated
POST. If typing indicators or presence need lower latency later, upgrade to
WebSockets behind the same message service.

Flow:

1. Client opens `GET /events` with `Authorization: Bearer …` and `Last-Event-ID`.
2. Server subscribes the connection to channels the user can see.
3. New messages are appended to the database, then published to an in-process hub.
4. Hub fans out as SSE frames `id`, `event`, `data`.
5. On reconnect, the client replays from `Last-Event-ID`; the server backfills
   from the database.

Because the hub is in-process, v1 assumes a single API instance. Document this.
For multiple instances, add a Postgres `LISTEN/NOTIFY` or Redis pub/sub bridge.

## Data model

Migration `0003_chat.sql`:

```sql
CREATE TABLE chat_channels (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  kind        TEXT NOT NULL,              -- project | task
  project_id  INTEGER REFERENCES projects(id) ON DELETE CASCADE,
  task_id     INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
  name        TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL,
  UNIQUE(project_id, kind),
  UNIQUE(task_id, kind)
);

CREATE TABLE chat_messages (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  channel_id  INTEGER NOT NULL REFERENCES chat_channels(id) ON DELETE CASCADE,
  author_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  body        TEXT NOT NULL,
  edited_at   TEXT,
  deleted_at  TEXT,
  created_at  TEXT NOT NULL
);

CREATE INDEX chat_messages_channel
  ON chat_messages(channel_id, id DESC);

CREATE TABLE chat_reads (
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  channel_id  INTEGER NOT NULL REFERENCES chat_channels(id) ON DELETE CASCADE,
  last_read_id INTEGER NOT NULL DEFAULT 0,
  updated_at  TEXT NOT NULL,
  PRIMARY KEY(user_id, channel_id)
);

CREATE TABLE chat_mentions (
  message_id  INTEGER NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at  TEXT NOT NULL,
  PRIMARY KEY(message_id, user_id)
);
```

All authenticated users can currently see all projects, so channel membership is
implicit. If workspaces are added later, add a `channel_members` table rather than
changing message authorization in handlers.

## API

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/projects/{id}/channel` | Get or lazily create the project channel |
| GET | `/tasks/{id}/thread` | Get or lazily create the task thread |
| GET | `/channels/{id}/messages?before=&limit=` | Paginated history, newest first |
| POST | `/channels/{id}/messages` | Body `{ "body": "…", "clientId": "uuid" }` |
| PATCH | `/messages/{id}` | Edit your own message |
| DELETE | `/messages/{id}` | Soft-delete your own message; managers may delete any |
| POST | `/channels/{id}/read` | Body `{ "lastMessageId": 42 }` |
| GET | `/channels/unread` | Unread counts for the current user |
| GET | `/events` | SSE stream |
| GET | `/channels/{id}/presence` | Online users (optional, in-memory) |

Message shape:

```json
{
  "id": 42,
  "channelId": 3,
  "author": { "id": 7, "username": "ari", "role": "Frontend Engineer" },
  "body": "Pushed the fix, ready for review.",
  "mentions": [4],
  "editedAt": null,
  "deletedAt": null,
  "createdAt": "2026-11-02T08:15:43Z"
}
```

Validation:

- body 1–4000 characters after trimming;
- reject empty or whitespace-only messages;
- `clientId` is idempotency for retries (unique per author);
- mentions must resolve to real usernames;
- edits keep an `editedAt` marker but no full revision history in v1.

## Backend work

1. `ChatService` with channel resolution, message CRUD, read state, mention
   parsing.
2. Repository methods with keyset pagination (`WHERE id < ? ORDER BY id DESC`).
3. A `Hub` with per-channel subscriber sets, a mutex, and buffered send channels.
   Drop slow consumers rather than blocking the writer.
4. SSE handler:
   - flush headers immediately;
   - send a heartbeat comment every 20–30 seconds;
   - honour `Last-Event-ID` by replaying missed rows;
   - stop on request context cancellation.
5. Publish after commit, same rule as notifications.
6. Strip/escape content on render. Store Markdown source, render on the client.
7. Rate-limit sending: for example 30 messages/minute/user.
8. Emit `chat.mention` notifications through the notification service.

## Frontend work

- A **Chat** entry in the project page (tab or side panel) and a **Discussion**
  tab on task detail.
- `useChannel`, `useMessages` (infinite query), `useSendMessage`, `useReadState`.
- An `EventSource` managed in one module; reconnect with exponential backoff; on
  reconnect send `Last-Event-ID`.
- Optimistic send with a temporary id, reconciled on response; failed messages
  stay visible with a retry affordance.
- Compose box with Enter to send, Shift+Enter for newline, `@` autocomplete from
  `GET /users`.
- Day separators, author grouping, edit/delete menus.
- Unread badge on the project card and the nav item.
- Keep the message list virtualised or capped if a channel grows large.

## Rollout

1. Schema, service, repository, plain REST history and send. No realtime.
2. SSE hub + live updates for a single instance.
3. Mentions and notification integration.
4. Read state and unread badges.
5. Edit/delete and moderation.
6. Presence and typing indicators only if users ask.

## Risks

| Risk | Mitigation |
| --- | --- |
| SQLite write contention with chat volume | Small inserts, WAL, single writer, rate limits; revisit Postgres if needed |
| Lost events on reconnect | SSE `Last-Event-ID` + database backfill |
| Memory growth from dead connections | Heartbeats, write deadlines, context cancellation, subscriber cap |
| XSS via Markdown | Render with a strict allow-list, sanitise HTML, never `dangerouslySetInnerHTML` raw |
| Spam and abuse | Rate limits, delete controls, report flow later |
| Multi-instance deployment | Document single-instance hub; add pub/sub before scaling out |
| Privacy of old messages | Define retention; soft-delete keeps rows, add a purge job later |

## Success measures

- Messages per active project per week.
- Share of review decisions preceded by task-thread discussion.
- Median time to answer a mention.
- Reduction in external chat links in task descriptions.
