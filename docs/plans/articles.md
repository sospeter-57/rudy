# Feature plan — Articles and experience posts

## Goal

Give people a place to write down what they learned. Rudy already produces a
steady stream of decisions, reviews and shipped work; articles turn that into
durable knowledge instead of chat scrollback. The tone is a personal article or
field note, not formal documentation: what I built, what broke, what I would do
differently.

## Scope

### In scope (v1)

- Markdown posts with a title, summary, body, tags and optional cover image URL.
- Draft → published lifecycle, plus archived.
- Public-to-the-team reading (every signed-in user can read published posts).
- Author profile page and tag filtering.
- Reactions (a small fixed set) and threaded comments.
- Optional link to a project or task that inspired the post.
- Reading time estimate and publish date.

### Out of scope (v1)

- Public internet publishing / SEO pages.
- Rich WYSIWYG editing. Markdown plus preview is enough.
- File uploads. Use an external image URL; add uploads later.
- Paid content or gating.
- Full version history.

## Data model

Migration `0004_articles.sql`:

```sql
CREATE TABLE articles (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  author_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title         TEXT NOT NULL,
  slug          TEXT NOT NULL UNIQUE,
  summary       TEXT NOT NULL DEFAULT '',
  body          TEXT NOT NULL,
  cover_url     TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'draft', -- draft | published | archived
  project_id    INTEGER REFERENCES projects(id) ON DELETE SET NULL,
  task_id       INTEGER REFERENCES tasks(id) ON DELETE SET NULL,
  published_at  TEXT,
  created_at    TEXT NOT NULL,
  updated_at    TEXT NOT NULL
);

CREATE INDEX articles_feed ON articles(status, published_at DESC);
CREATE INDEX articles_author ON articles(author_id, created_at DESC);

CREATE TABLE article_tags (
  article_id  INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  tag         TEXT NOT NULL,
  PRIMARY KEY(article_id, tag)
);

CREATE INDEX article_tags_tag ON article_tags(tag, article_id);

CREATE TABLE article_reactions (
  article_id  INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind        TEXT NOT NULL, -- insight | useful | agree | celebrate
  created_at  TEXT NOT NULL,
  PRIMARY KEY(article_id, user_id, kind)
);

CREATE TABLE article_comments (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  article_id  INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  author_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  parent_id   INTEGER REFERENCES article_comments(id) ON DELETE CASCADE,
  body        TEXT NOT NULL,
  deleted_at  TEXT,
  created_at  TEXT NOT NULL
);

CREATE INDEX article_comments_article ON article_comments(article_id, created_at);
```

Slug generation: lowercase, ASCII, hyphen-separated, suffix `-2`, `-3` on
collision. Allow an explicit slug override but keep it unique.

## API

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/articles?tag=&author=&cursor=&limit=` | Published feed, newest first |
| GET | `/articles/{slug}` | One published article (or own draft) |
| POST | `/articles` | Create a draft |
| PUT | `/articles/{id}` | Update title/body/tags/links/status |
| DELETE | `/articles/{id}` | Archive or delete own article |
| GET | `/articles/mine` | Own drafts and published posts |
| POST | `/articles/{id}/reactions` | Toggle a reaction kind |
| GET | `/articles/{id}/comments` | Comment tree |
| POST | `/articles/{id}/comments` | Add a comment or reply |
| DELETE | `/comments/{id}` | Soft-delete own comment |
| GET | `/tags` | Tag list with counts |

Article shape:

```json
{
  "id": 9,
  "slug": "what-i-learned-shipping-the-settings-page",
  "title": "What I learned shipping the settings page",
  "summary": "A short note on theme tokens, previews and one bad assumption.",
  "body": "## The assumption\n\n…",
  "coverUrl": "",
  "status": "published",
  "author": { "id": 3, "username": "ari", "role": "Frontend Engineer" },
  "tags": ["frontend", "theming"],
  "projectId": 2,
  "taskId": 7,
  "readingMinutes": 4,
  "reactions": { "insight": 3, "useful": 5 },
  "commentCount": 2,
  "publishedAt": "2026-11-02T08:15:43Z",
  "updatedAt": "2026-11-02T08:15:43Z"
}
```

Validation:

- title 4–140 characters;
- summary at most 240 characters;
- body 1–50,000 characters;
- at most 5 tags, each 1–24 characters, lowercase, `[a-z0-9-]`;
- cover URL must be `https://` or empty;
- only the author may edit or delete (managers may archive abusive posts).

## Markdown and safety

- Store raw Markdown. Render on the client with a CommonMark renderer.
- Disable raw HTML by default. If raw HTML is ever allowed, run it through a
  strict sanitiser.
- Linkify bare URLs, open external links with `rel="noopener noreferrer"`.
- Syntax-highlight fenced code blocks with a small, lazy-loaded highlighter.
- Generate a table of contents from `##` headings.
- Compute reading time from word count at ~220 words per minute.

## Backend work

1. `ArticleService` with create, update, publish, archive, feed, detail, tags,
   reactions and comments.
2. Slug allocation inside the write transaction.
3. Feed query uses keyset pagination on `(published_at, id)`.
4. Store tags in `article_tags`; update by diff, not delete-all, to avoid churn.
5. Emit notifications: `article.published` to followers/teammates (see
   [notifications](notifications.md)), `article.comment` to the author and
   parent commenter, `article.reaction` optionally batched.
6. Rate-limit creates and comments.
7. Add `GET /articles/mine` so drafts never leak into the public feed.

## Frontend work

- New routes: `/articles`, `/articles/:slug`, `/articles/new`,
  `/articles/:id/edit`, `/articles/mine`.
- A **Write** action in the topbar or rail.
- Feed cards with cover, title, summary, tags, author, reading time and reaction
  counts.
- Article page with Markdown rendering, tag chips, reactions, comments and an
  author card.
- Editor with split Markdown/preview, autosave draft every few seconds, explicit
  **Publish** and **Save draft** actions, and a preview of the slug.
- Tag filter and author filter on the feed.
- Empty states that invite the first post.
- Use existing tokens and typography so articles feel like part of Rudy, not a
  separate blog.

## Rollout

1. Schema + service + tests for drafts, publishing and slug collisions.
2. Read-only feed and article page seeded through the API.
3. Editor with preview and autosave.
4. Tags, reactions and comments.
5. Notifications and author profile integration.
6. Optional image upload and richer embeds.

## Risks

| Risk | Mitigation |
| --- | --- |
| XSS through Markdown | Disable raw HTML, sanitise, escape code, CSP |
| Spam or low-quality posts | Rate limits, report/archive controls, team-only visibility |
| Draft privacy | Every query scopes by author unless published |
| Large bodies | 50k character cap, pagination, lazy syntax highlighting |
| Comment toxicity | Soft-delete, manager archive, report flow later |
| Notification noise | Batch reactions, notify comments and publishes only |
| Scope creep into a CMS | Keep v1 to Markdown posts with tags and comments |

## Success measures

- Number of published posts per month.
- Share of active users who read at least one post per month.
- Comments per post and reaction rate.
- Posts linked to a task or project (signal that knowledge comes from real work).
- Repeat authors (more than one post).
