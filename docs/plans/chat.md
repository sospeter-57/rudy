# Feature plan — Chat

**Status: implemented in v1.** This document records the shipped design so later
work (admin review, mentions, task threads) can extend it without re-deciding the
core model.

## Goal

Give teams a place to talk where the work lives: private 1:1 messages, project
rooms curated by the project lead, and open groups anyone can discover and join.
Chat access is separate from project/task visibility: every signed-in user can
still read projects and tasks, but project rooms are member-only.

## Scope

### In scope (v1)

- Direct messages between two users, including images.
- Project rooms created and archived by the project lead.
- Explicit `project_members` curated by the lead; project rooms can only include
  project members.
- Open groups with discoverable or private visibility.
- Non-project group creation is **auto-approved for now**; the request, review
  status and invites are persisted in `chat_requests` for a future admin queue.
- Text messages (up to 4000 characters) and up to four images per message.
- Edit/delete your own message; room managers may delete any message.
- Cursor pagination, unread counts, read state, and live updates.
- Image uploads on local disk behind a storage interface.

### Out of scope (v1)

- An admin review UI, admin usernames, or manual approval of open groups.
- Task threads, mentions/notifications, presence and typing indicators.
- Markdown rendering (messages are plain text).
- Voice/video, federation, end-to-end encryption.

## Roles

- **Lead** is the renamed global project-manager role. It can create projects.
- The **project lead** is `projects.creator_id`, not a global role. The lead is
  recorded as the `lead` row in `project_members` and is always included in a
  project room.

## Transport

Server-Sent Events (SSE) for server → client, normal authenticated `POST`
requests for client → server. Native `EventSource` cannot send an `Authorization`
header, so the client reads the stream from a `fetch` body and parses SSE frames.

Flow:

1. Client opens `GET /chat/events` with `Authorization: Bearer …`.
2. The in-process hub subscribes the user to their rooms.
3. A message is committed, then published to the hub.
4. The client refetches the affected queries when an event arrives.
5. On reconnect the client refetches, closing any gap from missed events.

The hub is in-process, so v1 assumes one API instance. Add Redis or Postgres
`LISTEN/NOTIFY` before scaling horizontally.

## Data model

Migrations: `0003_leads_and_project_members.sql` and `0004_chat.sql`.

- `project_members(project_id, user_id, role, added_by, joined_at)` — explicit
  membership; project creation backfills the creator as `lead`.
- `chat_rooms(kind, name, purpose, project_id, created_by, dm_key, visibility,
  status, created_at, archived_at)` with `kind IN ('dm','project','open')`.
  A partial unique index on `dm_key` prevents duplicate DMs; another prevents
  duplicate active room names within a project.
- `chat_members(room_id, user_id, role, joined_at, last_read_message_id)` — room
  membership and read state.
- `chat_messages(room_id, sender_id, body, created_at, edited_at, deleted_at)` —
  soft delete keeps rows for moderation.
- `chat_attachments(room_id, uploader_id, message_id, storage_key, filename,
  mime, size_bytes, width, height, created_at)` — one row per image.
- `chat_requests(requester_id, name, purpose, visibility, status, reviewed_by,
  review_note, room_id, created_at, reviewed_at)` — auto-approved today.
- `chat_request_invites(request_id, user_id)` — who was invited when the group
  was requested.

## Authorization rules

- DMs and open rooms: room members only.
- Project rooms: the project lead creates/archives; only selected project
  members can read or post. Removing someone from the project removes them from
  its rooms.
- Open groups: discoverable rooms can be joined by anyone; private rooms are
  invite-only.
- Attachments are readable only by current members of the attachment's room and
  can only be attached to a message by the uploader, once, in the same room.
- Project and task listings stay open to all authenticated users; membership
  only gates chat.

## API

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/chats` | Rooms the current user belongs to |
| GET | `/chat/{id}` | One room (members only) |
| GET | `/chat/unread` | Total unread messages |
| GET | `/chat/discover` | Discoverable open groups |
| GET | `/chat/requests` | The current user's group requests |
| POST | `/chat` | Create an auto-approved open group |
| POST | `/chat/dm` | Open or reuse a 1:1 room |
| PUT | `/chat/{id}` | Update name, purpose and visibility |
| DELETE | `/chat/{id}` | Archive a project/open room |
| POST | `/chat/{id}/join` | Join a discoverable open group |
| POST | `/chat/{id}/leave` | Leave a room |
| POST/DELETE | `/chat/{id}/members[/{userId}]` | Manager-only membership |
| GET/POST | `/chat/{id}/messages` | History (`before`, `limit`) and send |
| PATCH/DELETE | `/chat/message/{id}` | Edit or soft-delete a message |
| POST | `/chat/{id}/read` | Mark up to a message as read |
| POST | `/chat/{id}/attachments` | Multipart image upload |
| GET | `/attachments/{id}` | Authenticated image bytes |
| GET | `/chat/events` | SSE stream |
| GET/POST | `/project/{id}/members` | List/add project members (lead only for add) |
| DELETE | `/project/{id}/members/{userId}` | Remove a project member |
| GET/POST | `/project/{id}/chats` | List/create project rooms |

## Backend notes

- `ChatService` owns permissions, room lifecycle, messages, uploads and events.
- `transaction/chat_repo.go` holds all chat and project-member SQL. Queries use
  keyset pagination and never hold rows open across a transaction.
- `storage.AttachmentStore` abstracts image bytes; `storage.Local` writes under
  `RUDY_UPLOAD_DIR`, which must be a persistent volume in production.
- `POST /chat/{id}/attachments` is the only multipart endpoint; everything else
  remains JSON.
- Image MIME types and sizes are validated before writing; storage keys are
  generated, never accepted from the client.
- SSE clears the server write deadline per connection. The stream is read-only.

## Frontend notes

- `/chat` and `/chat/:roomId` render the room list, conversation, composer and
  dialogs for DMs, discovery and group creation.
- Authenticated images are fetched with the bearer token and rendered as blob
  URLs, then revoked on unmount.
- `useChatStream` runs once in `AppShell` and invalidates `chats`, `chat-unread`,
  `chat-requests` and the affected room/message queries.
- The nav rail and mobile nav show an unread badge; it polls as a fallback.
- `ProjectDetailPage` exposes lead-only member curation and project-room
  creation, and shows members-only state to everyone else.

## Deferred work

- Admin review queue for `chat_requests` (rows and statuses already exist).
- Task threads, mentions, notifications, presence and typing indicators.
- Moderation tools beyond soft delete.
- Rate limiting and retention/purge jobs.
- Multi-instance SSE fan-out.
