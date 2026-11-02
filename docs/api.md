# API reference

Base URL in development: `http://localhost:8080`. The Vite dev server proxies
`/api/*` to that origin, so the client normally calls `/api/<path>`.

All request and response bodies are JSON. Timestamps are RFC3339 UTC strings
(for example `2026-11-02T08:15:43Z`). Field names are `camelCase` except the two
snake_case request fields called out below (`assignee_id`, `reviewer_id`), which
are kept for compatibility with the original contract.

## Authentication

Rudy uses opaque session tokens.

1. `POST /register` or `POST /login` returns a `token`.
2. Send it on every protected request:

   ```http
   Authorization: Bearer <token>
   ```

3. `POST /logout` revokes the current token.
4. Tokens expire after `RUDY_SESSION_TTL_HOURS` (default 720 hours = 30 days).

Public routes are `GET /health`, `POST /register` and `POST /login`. Everything
else requires a bearer token.

### Auth response

`POST /register` (201) and `POST /login` (200) return:

```json
{
  "user": {
    "id": 1,
    "username": "nova",
    "role": "Project Manager",
    "points": 0,
    "createdAt": "2026-11-02T08:15:43Z"
  },
  "token": "3d9f…",
  "expiresAt": "2026-12-02T08:15:43Z"
}
```

The plaintext token is returned only here. Store it client-side and never log it.

## Error format

Every non-2xx response is:

```json
{ "error": "human readable message" }
```

| Status | When |
| --- | --- |
| 400 | Malformed JSON, unknown JSON fields, bad path/query value, trailing data |
| 401 | Missing, expired or revoked bearer token |
| 403 | Authenticated but the role or relationship does not allow the action |
| 404 | Project, task or user not found |
| 409 | The lifecycle does not allow the action yet (approval-ordered conflicts) |
| 413 | Request body larger than 1 MiB |
| 415 | Body sent without `Content-Type: application/json` |
| 422 | Input validation failed |
| 500 | Unexpected server error; details are logged, not returned |

Requests with no body (logout, approve, submit, complete, delete) do **not**
need a `Content-Type` header. Requests with a body must send
`Content-Type: application/json`. Unknown JSON fields are rejected.

## Roles and task enums

Valid roles:

```
Project Manager, Database Engineer, Frontend Engineer, Backend Engineer,
Fullstack Engineer, QA Engineer, Software Engineer, Data Scientist, UIUX Designer
```

| Enum | Values |
| --- | --- |
| `taskType` | `feature`, `bug`, `fix`, `research` |
| `taskState` | `pending`, `auditing`, `completed` |
| `approvalState` | `proposed`, `approved`, `changes_requested` |
| review `decision` | `pending`, `approved`, `changes_requested` |
| ledger `reason` | `task_completed`, `review_bonus`, `adjustment` |
| `themeMode` | `dark`, `light`, `system` |

## Health

### `GET /health`

Public. Returns `200`:

```json
{ "status": "ok" }
```

## Accounts and sessions

### `POST /register`

Public. Body:

```json
{ "username": "nova", "password": "password123", "role": "Frontend Engineer" }
```

Returns `201` with the [auth response](#auth-response). `username` must be
unique (409 if taken) and the role must be one of the roles above (422
otherwise).

### `POST /login`

Public. Body:

```json
{ "username": "nova", "password": "password123" }
```

Returns `200` with the auth response, or `401` for bad credentials.

### `POST /logout`

Auth required. No body. Returns `204`. The token is revoked immediately.

### `GET /me`

Auth required. Returns the current `User` object directly (not wrapped):

```json
{
  "id": 1,
  "username": "nova",
  "role": "Project Manager",
  "points": 42,
  "createdAt": "2026-11-02T08:15:43Z"
}
```

### `GET /users`

Auth required. Returns every account, used for assignee and reviewer pickers.

```json
{ "users": [ { "id": 1, "username": "nova", "role": "Project Manager", "points": 42, "createdAt": "…" } ] }
```

### `GET /leaderboard`

Auth required. Returns users ordered by points with derived counts.

```json
{
  "leaderboard": [
    {
      "user": { "id": 1, "username": "nova", "role": "Project Manager", "points": 42, "createdAt": "…" },
      "rank": 1,
      "completedTasks": 3,
      "reviews": 5
    }
  ]
}
```

### `GET /user/{id}/ledger`

Auth required. Returns the append-only ledger for a user, newest first.

```json
{
  "ledger": [
    {
      "id": 12,
      "userId": 1,
      "taskId": 7,
      "projectId": 2,
      "reason": "task_completed",
      "points": 20,
      "note": "Completed \"Wire the settings page\"",
      "createdAt": "2026-11-02T08:15:43Z"
    }
  ]
}
```

`taskId` and `projectId` are nullable for manual adjustments.

## Preferences

Preferences are stored on the account, so they follow the user across browsers.

### `GET /me/preferences`

Auth required.

```json
{
  "preferences": {
    "themeMode": "dark",
    "accent": "#d4a72c",
    "blendA": "#e01e37",
    "blendB": "#f0c75e",
    "updatedAt": "2026-11-02T08:15:43Z"
  }
}
```

### `PUT /me/preferences`

Auth required. Body (all fields required):

```json
{
  "themeMode": "system",
  "accent": "#d4a72c",
  "blendA": "#e01e37",
  "blendB": "#f0c75e"
}
```

Colours must be 6-digit hex values (`#rrggbb`); `themeMode` must be `dark`,
`light` or `system`. Returns `200` with the same wrapper as the `GET`.

## Stats

### `GET /stats?range=30d|90d|all`

Auth required. `range` defaults to `30d` when omitted or unknown. All
aggregates are computed in SQL. Response shape:

```json
{
  "stats": {
    "range": "30d",
    "totals": {
      "users": 12,
      "builders": 9,
      "projects": 4,
      "tasks": 31,
      "completedTasks": 18,
      "completionRate": 58.1,
      "pointsMinted": 420,
      "reviewsApproved": 22,
      "reviewsDecided": 27,
      "reviewApprovalPct": 81.5,
      "avgCycleHours": 36.4
    },
    "tasksByState": { "pending": 8, "auditing": 5, "completed": 18 },
    "tasksByApprovalState": { "proposed": 7, "approved": 23, "changes_requested": 1 },
    "tasksByType": { "feature": 14, "bug": 8, "fix": 6, "research": 3 },
    "roleDistribution": { "Project Manager": 1, "Frontend Engineer": 5 },
    "projects": [
      {
        "projectId": 2,
        "name": "Aurora",
        "totalTasks": 11,
        "proposed": 2,
        "pending": 3,
        "auditing": 1,
        "completed": 5,
        "openPoints": 90,
        "paidPoints": 120,
        "completionPct": 45.5
      }
    ],
    "users": [
      {
        "user": { "id": 1, "username": "nova", "role": "Frontend Engineer", "points": 42, "createdAt": "…" },
        "completedTasks": 3,
        "reviews": 5
      }
    ],
    "trends": [
      { "period": "2026-11-01", "proposals": 2, "completions": 1, "reviews": 3, "pointsMinted": 20 }
    ],
    "ledgerByReason": [
      { "reason": "task_completed", "points": 300, "count": 15 },
      { "reason": "review_bonus", "points": 120, "count": 20 }
    ]
  }
}
```

`builders` counts users with a non-manager role. `completionRate` is a
percentage. `avgCycleHours` is the mean `completedAt - createdAt` over tasks
completed inside the range.

## Projects

### `GET /projects`

Auth required. Returns all projects newest first.

```json
{ "projects": [ { "id": 2, "name": "Aurora", "description": "…", "ttl": 1440, "creatorId": 1, "createdAt": "…" } ] }
```

### `POST /project`

Auth required, **Project Manager only**. Body:

```json
{ "name": "Aurora", "description": "Internal tooling", "ttl": 1440 }
```

`name` is required and `ttl` must be positive. Returns `201`:

```json
{ "project": { "id": 2, "name": "Aurora", "description": "Internal tooling", "ttl": 1440, "creatorId": 1, "createdAt": "…" } }
```

### `GET /project/{id}`

Auth required. Returns the `Project` object directly.

## Tasks

### Task object

```json
{
  "id": 7,
  "projectId": 2,
  "description": "Wire the settings page",
  "taskType": "feature",
  "taskState": "pending",
  "approvalState": "approved",
  "creatorId": 1,
  "assigneeId": 3,
  "role": "Frontend Engineer",
  "points": 20,
  "dependencies": [5],
  "acceptance": {
    "code": "https://github.com/org/repo/pull/12",
    "tests": "npm test -- settings",
    "documentation": "docs/settings.md",
    "reviewers": [
      { "userId": 4, "decision": "pending" }
    ]
  },
  "rejectionReason": "",
  "createdAt": "2026-11-02T08:15:43Z",
  "completedAt": null
}
```

`assigneeId`, `completedAt`, `rejectionReason`, reviewer `comment` and
`decidedAt` are omitted when empty/null.

### `GET /tasks`

Auth required. Optional query parameters:

| Parameter | Meaning |
| --- | --- |
| `project_id` | Only tasks in this project |
| `assignee` | Only tasks assigned to this user id |
| `reviewer` | Only tasks where this user id is a reviewer |
| `state` | `pending`, `auditing` or `completed` |

Returns `{ "tasks": [ … ] }`.

### `GET /task/{id}`

Auth required. Returns the `Task` object directly.

### `POST /project/{id}/task`

Auth required; any role may propose. Body:

```json
{
  "description": "Wire the settings page",
  "taskType": "feature",
  "role": "Frontend Engineer",
  "points": 20,
  "dependencies": [5]
}
```

Validation:

- description at least 4 characters after trimming;
- `taskType` and `role` must be known values;
- `points` between 1 and 100 inclusive;
- dependencies must exist, belong to the same project and not be completed.

Returns `201` with `{ "task": { … } }`. The task starts as
`approvalState: "proposed"`, `taskState: "pending"`.

### `POST /task/{id}/approve`

Auth required, **Project Manager only**. No body. Marks the proposal
`approved`. Returns `{ "task": { … } }`. `409` if already approved or completed.

### `POST /task/{id}/reject`

Auth required, **Project Manager only**. Body:

```json
{ "reason": "Split this into a frontend and backend task" }
```

`reason` is required (422). Sets `approvalState: "changes_requested"` and stores
the reason. Returns `{ "task": { … } }`.

### `POST /task/{id}/assign`

Auth required, **Project Manager only**. Body uses snake_case:

```json
{ "assignee_id": 3 }
```

The task must already be approved (409). Returns `{ "task": { … } }`.

### `POST /task/{id}/done/{gate}`

Auth required; assignee or Project Manager. `gate` is `code`, `tests` or
`documentation`. The task must be approved and `pending` (409 otherwise). Body:

```json
{ "code": "https://github.com/org/repo/pull/12" }
```

Field name matches the gate (`code`, `tests` or `documentation`). Empty values
are rejected (422). Returns `{ "task": { … } }`.

### `POST /task/{id}/done/reviewer`

Auth required; assignee or Project Manager. Same approval + pending
precondition as gates. Body uses snake_case:

```json
{ "reviewer_id": 4 }
```

The reviewer must exist, must not be the assignee (409), and may not already be
listed (409). Returns `{ "task": { … } }`.

### `DELETE /task/{id}/reviewer/{userId}`

Auth required; assignee or Project Manager, task approved and `pending`. No
body. Returns `{ "task": { … } }`.

### `POST /task/{id}/submit`

Auth required; assignee or Project Manager. No body. Requires:

- task approved and `pending`;
- all three acceptance gates non-empty;
- at least one reviewer.

Moves `taskState` to `auditing` and resets every reviewer decision to `pending`.
Returns `{ "task": { … } }`.

### `POST /task/{id}/review`

Auth required; must be a listed reviewer and not the assignee. The task must be
in `auditing` (409). Body:

```json
{ "decision": "approved", "comment": "Nice work" }
```

`decision` must be `approved` or `changes_requested` (422). A
`changes_requested` decision returns the task to `pending`. Returns
`{ "task": { … } }`.

### `POST /task/{id}/complete`

Auth required, **Project Manager only**. No body. Completion runs in one
database transaction and requires:

- task not already completed;
- `approvalState === "approved"`;
- `taskState` is `pending` or `auditing`;
- all three gates present;
- every reviewer (if any) approved;
- an assignee.

Returns `200`:

```json
{
  "task": { "id": 7, "taskState": "completed", "completedAt": "…", "…": "…" },
  "awards": [
    { "userId": 3, "taskId": 7, "projectId": 2, "reason": "task_completed", "points": 20, "note": "Completed \"…\"", "…": "…" },
    { "userId": 4, "taskId": 7, "projectId": 2, "reason": "review_bonus", "points": 3, "note": "Reviewed \"…\"", "…": "…" }
  ]
}
```

The assignee receives the full task value. Each approving reviewer (excluding
the assignee) receives `max(2, round(points × 0.15))`. Payout is idempotent:
repeating the request returns `409` rather than minting again.

## Example session

```bash
# register and capture the token
TOKEN=$(curl -s localhost:8080/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"nova","password":"password123","role":"Project Manager"}' \
  | jq -r .token)

# create a project
curl -s localhost:8080/project \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Aurora","description":"Internal tooling","ttl":1440}'

# propose a task
curl -s localhost:8080/project/1/task \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"description":"Wire the settings page","taskType":"feature","role":"Frontend Engineer","points":20,"dependencies":[]}'

# stats
curl -s 'localhost:8080/stats?range=30d' -H "Authorization: Bearer $TOKEN"
```
