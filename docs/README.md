# Rudy documentation

Rudy is a points-based project and task command deck. This folder is the entry
point for developers, operators and users.

## Start here

| Document | Audience | What it covers |
| --- | --- | --- |
| [Getting started](getting-started.md) | Developers | Prerequisites, run commands, configuration, tests |
| [Architecture](architecture.md) | Developers | Layers, database schema, auth, lifecycle, transactions |
| [API reference](api.md) | Developers / integrations | Endpoints, auth, request and response shapes |
| [Frontend guide](frontend.md) | Frontend developers | Routes, state, permissions, theming, charts |
| [User guide](user-guide.md) | End users | Signing up, workflow, roles, points, settings |
| [Roadmap](roadmap.md) | Product / developers | Prioritised future work |
| [Feature plans](plans/) | Developers | Detailed specs for notifications, chat, articles, AI |

## Product in one paragraph

Anyone can propose a task inside a project. A Project Manager approves it as fit
to be worked on, assigns a builder, and the builder records code, tests and
documentation evidence. Reviewers approve or request changes. When a manager marks
the task complete, the ledger mints the task value to the assignee and a review
bonus to every approving reviewer. A user's balance is always the sum of their
ledger entries.

## Repository layout

```
rudy/
  back/                  Go API
    main.go              configuration, DB open, migrations, HTTP server
    internal/models/     API models and error kinds
    internal/transaction/ SQLite, migration runner, repository
    internal/services/   domain rules and transactions
    internal/routes/     HTTP handlers, auth middleware, JSON mapping
  front/                 React client
    src/lib/             API client, session, hooks, permissions, theme
    src/features/        pages grouped by domain
  docs/                  this documentation
```

## Current status

Implemented end to end:

- Registration, login, logout and opaque bearer-token sessions.
- Project creation (Project Manager only) and project listing/detail.
- Task proposals, approval-first gates, assignment, review, completion.
- Ledger-backed point balances with idempotent payout.
- Leaderboard and per-user ledger.
- `/stats` aggregations and a Recharts statistics page.
- Account-level theme and palette preferences applied through CSS custom properties.
- End-to-end backend tests for migrations, sessions, gates, self-review, payout, stats and preferences.
