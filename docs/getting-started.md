# Getting started

## Prerequisites

- Go **1.26+** (see `back/go.mod`)
- Node.js **20+** and npm
- No external database. Rudy uses SQLite through the pure-Go `modernc.org/sqlite`
  driver, so no C compiler is needed.

## Run the API

```bash
cd back
go run .
```

On first start the server creates the database file (`rudy.db` by default), applies
every embedded migration, and listens on `:8080`.

```text
INFO starting server addr=:8080 db=rudy.db
```

### Backend configuration

The server loads a `.env` file at startup, before it opens the database. When you
run `go run .` from `back/`, it looks for `back/.env`; set `RUDY_ENV_FILE` to use
another path. Variables already set in the process environment always take
precedence over the file, so shell overrides and deployment settings still win.

```bash
cd back
cp .env.example .env   # optional, then edit as needed
go run .
```

| Variable | Default | Meaning |
| --- | --- | --- |
| `RUDY_HOST` | `0.0.0.0` | Bind IP address (use `127.0.0.1` to stay local, `::` for IPv6) |
| `RUDY_PORT` | `PORT` or `8080` | Bind port. Falls back to the platform-standard `PORT`, so most PaaS hosts work automatically. |
| `PORT` | unset | Standard port injected by Render, Railway, Heroku, Fly and similar. Lower priority than `RUDY_PORT`. |
| `RUDY_ADDR` | unset | Full `host:port` override, or a bare port; when set it wins over `RUDY_HOST`/`RUDY_PORT`/`PORT` |
| `RUDY_DB` | `rudy.db` | SQLite file path. Use a temp path for throwaway runs. |
| `RUDY_UPLOAD_DIR` | `uploads` | Directory for chat image attachments. Point this at a persistent volume in production. |
| `RUDY_CORS_ORIGINS` | unset | Comma-separated browser origins allowed to call the API cross-origin (for example `https://rudy.example.com`). Empty keeps the API same-origin only; `*` allows any origin. |
| `RUDY_SESSION_TTL_HOURS` | `720` (30 days) | Session lifetime in hours |
| `RUDY_GOOGLE_CLIENT_ID` | unset | OAuth 2.0 Web client ID for Google sign-in. Unset disables the endpoint. |
| `RUDY_ENV_FILE` | `.env` | Path to the env file to load, relative to the working directory |

Example:

```bash
# bind only on loopback, port 9000
RUDY_HOST=127.0.0.1 RUDY_PORT=9000 RUDY_DB=/tmp/rudy-dev.db go run .

# or use a single full address
RUDY_ADDR=192.168.1.10:9000 go run .

# on a host that injects PORT (Render, Railway, Heroku, Fly)
PORT=3000 RUDY_DB=/data/rudy.db go run .
```

The bind address is resolved in this order: `RUDY_ADDR` (full address or bare
port) wins; otherwise the host comes from `RUDY_HOST` (default `0.0.0.0`) and the
port from `RUDY_PORT`, then `PORT`, then `8080`. On a PaaS, make `RUDY_DB` point
at a persistent volume; the default `rudy.db` only survives as long as the
container filesystem.

The database uses WAL mode, foreign keys, a 5 second busy timeout, and a single
pooled connection (SQLite allows one writer). Delete the file to start over.

### Google sign-in (optional)

1. In the [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
   create an **OAuth 2.0 Client ID** of type **Web application**.
2. Add your frontend origin under **Authorized JavaScript origins**:
   `http://localhost:5173` for local development, plus the production origin.
3. Put the same client ID in both env files:

   ```bash
   # back/.env
   RUDY_GOOGLE_CLIENT_ID=1234567890-abc.apps.googleusercontent.com

   # front/.env
   VITE_GOOGLE_CLIENT_ID=1234567890-abc.apps.googleusercontent.com
   ```

No client secret is needed: the browser receives a short-lived Google ID token
and the Go API verifies it with Google's public keys. The Google button is hidden
when `VITE_GOOGLE_CLIENT_ID` is empty, and the endpoint returns `422` when the
backend is not configured.

### Add the first account

Open the client, choose **Create profile**, and pick **Lead** if you want
to create the first project. Other roles can propose tasks but cannot approve,
assign or complete them.

## Run the client

```bash
cd front
npm install
npm run dev
```

Open `http://127.0.0.1:5173`. Vite proxies `/api` to the origin in
`VITE_PROXY_TARGET`, so there are no CORS settings to configure when you use the
dev server. If the client calls the API directly from another origin, list that
origin in `RUDY_CORS_ORIGINS` on the backend.

### Frontend configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `VITE_API_BASE` | `/api` | Base path for API calls. In development this hits the Vite proxy. |
| `VITE_DEV_PORT` | `5173` | Port for the Vite dev server. |
| `VITE_PROXY_TARGET` | required | Go API origin that Vite proxies `/api` to. Kept in `front/.env`, never hardcoded in `vite.config.ts`. |
| `VITE_GOOGLE_CLIENT_ID` | unset | Same OAuth client ID as `RUDY_GOOGLE_CLIENT_ID`. Empty hides the Google button. |

For a production build served from another origin, set
`VITE_API_BASE=https://api.example.com` and list the frontend origin in the Go
server's `RUDY_CORS_ORIGINS` (for example
`RUDY_CORS_ORIGINS=https://rudy.example.com`). Leave it empty when both are
served from the same origin.

```bash
npm run build
npm run preview
```

## Tests and checks

```bash
# backend
cd back
go test ./...
go vet ./...

# frontend
cd front
npm run typecheck
npm run build
```

Backend tests create isolated SQLite databases in `t.TempDir()` and cover:

- migration idempotency;
- registration, login, bearer authentication and logout revocation;
- approval-before-gates enforcement;
- self-review rejection;
- completion payout and duplicate-completion rejection;
- stats aggregation and preference validation;
- the HTTP router's bearer session flow.

## Project reset

Stop the server, delete the database file, and start again:

```bash
rm back/rudy.db back/rudy.db-shm back/rudy.db-wal
```

## Troubleshooting

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| `401 missing bearer token` | No saved session or token cleared | Sign in again |
| `401 session expired or invalid` | Session TTL elapsed or logged out elsewhere | Sign in again |
| `409 a lead must approve this task before work can begin` | Trying to edit gates/reviewers on a proposal | Approve the task first |
| `409 every reviewer must approve…` | Completion attempted before sign-off | Finish the review queue |
| `415 Content-Type must be application/json` | Request body sent without JSON content type | Send `Content-Type: application/json` |
| Client shows network errors | API not running or wrong `VITE_API_BASE` | Start `go run .`, check the Vite proxy |
