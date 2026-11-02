# Feature plan — AI assistance

## Goal

Use AI to remove friction from the developer journey, not to make decisions.
Rudy has clean structured data — tasks, approvals, reviews, ledger entries, stats
— which makes it a good place for summaries, drafts and checklists. Every AI
output should be advisory, labelled and easy to dismiss.

The flagship surface is the **Stats** page: turn aggregates into a short,
specific narrative that tells a team what changed and what needs attention.

## Principles

1. **Human in the loop.** AI never approves, assigns, reviews or completes a
   task. It may only draft text a person can edit.
2. **Grounded in Rudy data.** Prefer computed aggregates and retrieved records
   over free-form model knowledge. Show the numbers behind a claim.
3. **Opt-in and transparent.** An organisation-level switch controls whether any
   content leaves the server. Label every generated block.
4. **No silent writes.** Generated content is returned to the client; persistence
   only happens when the user saves.
5. **Budgeted.** Per-user and per-org quotas, cached results, and a hard timeout.
6. **Provider-agnostic.** One interface, swap providers through configuration.

## Use cases, in priority order

### 1. Stats narratives

**Where:** Stats page, top panel.

**Input:** the `Stats` JSON for the selected range, plus the previous range for
comparison.

**Output:** 3–5 bullet insights, for example:

- completion rate rose from 48% to 58%, driven by Aurora;
- three tasks have been in review for more than 72 hours;
- review bonuses are 22% of minted points, up from 15%.

**Why first:** read-only, cheap, immediately useful, and easy to evaluate.

**Guardrails:** every bullet must cite a field present in the input. If the model
returns a number not in the payload, drop the bullet. Cache per range and data
version for 15 minutes.

### 2. Proposal assist

**Where:** Propose-task dialog.

**Input:** a one-line brief typed by the user, plus project context (recent task
titles, roles, point distribution).

**Output:** suggested description, task type, role, points, and draft acceptance
gates. The user must accept each field individually. Never auto-submit.

**Guardrails:** clamp points to 1–100; only suggest known roles and types; show a
"why this estimate" line referencing comparable completed tasks.

### 3. Review checklist

**Where:** Task detail, reviewer panel.

**Input:** task description, type, points, acceptance evidence, dependencies.

**Output:** 4–6 review questions tailored to the evidence, for example "The PR
links to a migration but no rollback was described — is that covered?"

**Guardrails:** never produce an approve/reject recommendation. The checklist is
a prompt for the reviewer, not a verdict.

### 4. Semantic search

**Where:** Global search.

**Input:** a natural-language query.

**Output:** ranked tasks, articles and (later) chat messages, with highlighted
snippets.

**Implementation:** start with SQLite FTS5 for keyword search. Add embeddings
only when FTS proves insufficient; store vectors in a side table and compute
cosine similarity in Go for small datasets, or use a vector extension later.

### 5. Assistant panel

**Where:** A collapsible panel, scoped to the current project.

**Input:** "What is blocking Aurora?" or "Summarise this task's review history."

**Output:** an answer with links to the records used. Read-only tools only:
`list_tasks`, `get_task`, `get_stats`, `list_ledger`, `search_articles`.

**Guardrails:** no write tools in v1; cap tool calls; show a trace of which
records were read.

## Architecture

```
HTTP handler
   │
AI service (internal/services/ai.go)
   │  builds a grounded prompt from repository queries
   ├── Provider interface ──► OpenAI / Anthropic / local model
   ├── Cache (ai_cache table)
   └── Usage meter (ai_usage table)
```

```go
type Provider interface {
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

type CompletionRequest struct {
    System      string
    Messages    []Message
    MaxTokens   int
    Temperature float64
    JSONSchema  map[string]any // optional structured output
}
```

Configuration:

| Variable | Meaning |
| --- | --- |
| `RUDY_AI_ENABLED` | Master switch, default off |
| `RUDY_AI_PROVIDER` | `openai`, `anthropic`, `local` |
| `RUDY_AI_MODEL` | Model id |
| `RUDY_AI_API_KEY` | Provider key; never logged |
| `RUDY_AI_TIMEOUT_SECONDS` | Per-request timeout, default 30 |
| `RUDY_AI_DAILY_TOKENS_PER_USER` | Quota |
| `RUDY_AI_ALLOW_CONTENT` | Whether task text may leave the server |

## Data model

Migration `0005_ai.sql`:

```sql
CREATE TABLE ai_cache (
  cache_key   TEXT PRIMARY KEY,
  kind        TEXT NOT NULL,
  payload     TEXT NOT NULL,
  created_at  TEXT NOT NULL,
  expires_at  TEXT NOT NULL
);

CREATE TABLE ai_usage (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind          TEXT NOT NULL,
  model         TEXT NOT NULL,
  input_tokens  INTEGER NOT NULL,
  output_tokens INTEGER NOT NULL,
  latency_ms    INTEGER NOT NULL,
  created_at    TEXT NOT NULL
);

CREATE INDEX ai_usage_user_day ON ai_usage(user_id, created_at);
```

Do **not** store prompts or completions by default. Cache only derived,
non-sensitive output (stats narratives) and expire it aggressively. If audit
logging is required, make it opt-in and document retention.

## API

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/ai/status` | Enabled, provider, remaining quota |
| POST | `/ai/stats-insights` | Body `{ "range": "30d" }` → bullet insights |
| POST | `/ai/proposal-draft` | Body `{ "projectId": 2, "brief": "…" }` |
| POST | `/ai/review-checklist` | Body `{ "taskId": 7 }` |
| POST | `/ai/ask` | Body `{ "projectId": 2, "question": "…" }` |
| GET | `/search?q=` | Keyword/semantic search (non-AI fallback) |

Responses include `model`, `generatedAt`, `cached`, and the source ids used. All
endpoints are authenticated and rate-limited. Return `503` with a clear message
when AI is disabled or the provider fails; the rest of the app must keep working.

## Prompting rules

- System prompt states: you are an assistant inside Rudy; never invent numbers;
  answer only from the supplied context; if context is insufficient, say so.
- Pass a compact JSON context, not raw database rows, to reduce tokens and
  accidental PII.
- Request structured JSON where the UI needs fields; validate with a schema and
  discard invalid output.
- Redact usernames only if the org enables it; otherwise keep them for clarity.
- Set temperature low (0–0.3) for insights and estimates.
- Include a request id for debugging without logging content.

## Frontend work

- `useAIStatus`, `useStatsInsights`, `useProposalDraft`, `useReviewChecklist`.
- Stats: an **Insight** panel with skeleton, regenerate, and a "How was this
  generated?" disclosure listing the metrics used.
- Proposal dialog: **Draft with AI** button; suggested fields appear as chips the
  user can apply individually.
- Task detail: **Suggest review questions** in the reviewer panel.
- A global **Ask Rudy** panel with suggested questions and linked sources.
- Every AI block has a small "AI-generated" label and a dismiss control.
- Graceful degradation: hide AI affordances when `/ai/status` says disabled.

## Rollout

1. Provider interface + usage meter + `/ai/status`, no prompts.
2. Stats narratives behind `RUDY_AI_ENABLED`, cached, with gold-standard tests on
   fixed fixtures.
3. Proposal assist, with per-field acceptance.
4. Review checklist.
5. FTS5 search; evaluate embeddings only with real query data.
6. Assistant panel with read-only tools.
7. Org-level privacy settings and spend dashboard.

## Evaluation

- Maintain a fixture of stats payloads and expected claims; assert no invented
  numbers.
- Track acceptance rate of proposal suggestions (fields kept vs discarded).
- Track reviewer checklist usage and comment length.
- Measure cost per active user and cache hit rate.
- Collect thumbs up/down with an optional reason.

## Risks

| Risk | Mitigation |
| --- | --- |
| Hallucinated metrics | Ground every claim in payload fields; validate; drop unsupported bullets |
| Sensitive data leaving the server | Org-level toggle, content minimisation, no prompts stored by default |
| Cost overruns | Per-user quotas, caching, small models, hard timeouts |
| Latency hurting UX | Async panels, skeletons, never block page load |
| Vendor lock-in | Provider interface and model-agnostic prompts |
| Users over-trusting output | Label as AI, advisory-only copy, no auto-apply |
| Prompt injection via task text | Delimit user content, instruct the model to ignore embedded instructions, validate output |
| Silent behaviour changes | Feature flag, audit usage, version the prompt template |

## Explicit non-goals

- Autonomous task completion or approval.
- AI-generated performance ratings or disciplinary signals.
- Sending private articles or chat to a provider without consent.
- Replacing the ledger or any lifecycle rule.
