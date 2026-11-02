# User guide

Rudy is a shared project board where finished work earns points. Anyone can
propose a task; a Project Manager decides what is fit to build; builders record
evidence; reviewers sign off; completing a task mints points to the people who
did the work and reviewed it.

This guide walks through the product as a user. For how it is built, see
[Architecture](architecture.md).

## 1. Create an account

Open the app and choose **Create profile**.

- **Username** — unique, at least 3 characters.
- **Password** — at least 8 characters.
- **Role** — pick the role that matches the work you do. This is permanent for
  the account and decides what you can unlock.

If you already have an account, use **Sign in**. Sessions last 30 days by
default; signing out revokes the token immediately.

> Only the **Project Manager** role can create projects, approve proposals,
> assign tasks and complete them. Everyone else proposes, builds and reviews.

## 2. Learn the roles

| Role | Can propose | Can build | Can review | Can approve / assign / complete |
| --- | --- | --- | --- | --- |
| Project Manager | Yes | Yes | If listed | Yes |
| Any builder role | Yes | If assigned | If listed | No |

A Project Manager is not automatically a reviewer and earns no points for
approving or completing work. Reviewers are chosen deliberately per task.

## 3. Open or find a project

- **Projects** shows every project. A manager can create one with
  **New project**: name, description, and a TTL (time-to-live in minutes, used
  as a planning hint).
- Click a project to see its board, tasks and people.

Everyone can see all projects. Projects do not hide work from teammates.

## 4. Propose a task

On a project page or from **Tasks**, choose **Propose task**.

| Field | What it means |
| --- | --- |
| Description | What needs to happen. At least 4 characters. |
| Type | `Feature`, `Bug`, `Fix` or `Research`. |
| Role | The role best suited to the work. |
| Points | 1–100. This is the full reward the assignee receives. |
| Dependencies | Other tasks in the same project that should finish first. Completed tasks cannot be dependencies. |

A new task starts as **Awaiting approval** and **Pending**. It is not ready to
build yet.

### What happens next

A manager reviews the proposal:

- **Approve** — the task becomes approved and can be assigned.
- **Request changes** — the task returns to the creator with a reason. Edit the
  proposal or discuss it, then ask for another review.

## 5. Prepare the task (after approval)

Once a task is approved, the assignee or a manager can set it up. These controls
are hidden until approval — this is deliberate.

1. **Assign** a builder (manager only).
2. Fill the three acceptance gates:
   - **Code** — link to the branch, pull request or commit.
   - **Tests** — how the work was verified.
   - **Documentation** — where the docs live.
3. **Add reviewers** — at least one. The assignee cannot be a reviewer on their
   own task.

All three gates and at least one reviewer are required before review.

## 6. Submit for review

The assignee chooses **Submit for review**. The task moves to **Auditing**, and
every reviewer decision resets to **Pending**.

Each listed reviewer opens the task from **Review** and chooses:

- **Approve** — sign off on the evidence.
- **Request changes** — send it back with a comment. The task returns to
  **Pending**, and the assignee can revise and resubmit.

Reviewers cannot be the assignee. If evidence changes after a resubmission,
everyone reviews again.

## 7. Complete and earn

A manager completes the task once:

- it is approved;
- all gates are filled;
- every reviewer has approved;
- an assignee exists.

Completion mints points exactly once:

| Who | Points |
| --- | --- |
| Assignee | The full task value |
| Each approving reviewer | `max(2, round(task points × 0.15))` |

Example: a 20-point task pays the assignee 20 and each approving reviewer 3.
Managers earn nothing for pressing complete.

Points are recorded in an append-only ledger. The balance shown on your profile
is always the sum of that ledger, so it cannot drift.

## 8. Use the dashboard

**Dashboard** is your personal deck:

- tasks assigned to you and what they are waiting on;
- proposals waiting for a manager;
- your review queue;
- points at stake in approved but unfinished work;
- recent activity and the leaderboard.

Use it as the first screen each day.

## 9. Track your progress

**Profile** addresses you as **Builder &lt;username&gt;** and shows:

- current balance and tier;
- progress to the next tier;
- your ledger, with the reason and note for every award;
- the leaderboard.

Tiers are presentation only; they do not change permissions.

| Tier | Points |
| --- | --- |
| Apprentice | 0 |
| Journeyman | 100 |
| Artisan | 250 |
| Master | 500 |
| Legend | 1000 |

## 10. Read the stats page

**Stats** aggregates the database, not a local cache:

- totals for builders, accounts, projects, tasks and points;
- completion rate and average cycle time;
- review approval rate;
- tasks by state, approval state and type;
- per-project completion and points;
- points by ledger reason;
- the builder leaderboard;
- daily proposals, completions and reviews.

Use the **30 days / 90 days / all time** switch to change the window. Project
totals and trends honour the selected range.

## 11. Make Rudy yours

**Settings** stores appearance preferences on your account, so they follow you
to any browser:

- **Theme** — dark, light, or follow the system.
- **Palette** — pick a preset or choose custom accent and blend colours.
- A live preview shows the result before saving.

Choose **Save appearance** to persist. Unsaved changes only affect the current
page.

## 12. Sign out

Use **Sign out** at the bottom of the side rail. The client revokes the session
on the server and clears the local token. If a session expires or is revoked
elsewhere, the next request signs you out automatically.

## Rules to remember

1. **Approval comes first.** Gates and reviewers cannot be edited until a
   manager approves the task, and only while it is pending.
2. **Evidence is the price of review.** Code, tests and documentation are all
   required.
3. **You cannot review your own work.** The assignee is excluded from reviewers.
4. **One payout per task.** Completion is idempotent; points are never minted
   twice.
5. **Balances come from the ledger.** There is no manual balance editing in the
   UI.
6. **Managers do not earn for gatekeeping.** Approving, assigning and completing
   pay nothing by themselves.

## Troubleshooting

| Message | What it means | What to do |
| --- | --- | --- |
| `a manager must approve this task first` | You tried to edit gates, reviewers or submit before approval | Ask a manager to approve the proposal |
| `this task is not pending` | The task is already in review or completed | Check its state on the task page |
| `code, tests and documentation are all required before review` | A gate is empty | Fill all three gates |
| `add at least one reviewer before requesting review` | No reviewers listed | Add a teammate |
| `the assignee cannot review their own work` | You tried to add yourself | Choose someone else |
| `every reviewer must approve before this task can be completed` | A review is pending or changes were requested | Finish the review queue |
| `assign this task before completing it` | Completion needs an owner | Assign a builder |
| `this task has already been paid out` | Completion was attempted twice | Refresh; the points are already in the ledger |
| Signed out unexpectedly | Session expired or was revoked | Sign in again |

If a button is missing, it is usually because the lifecycle does not allow that
action yet — the UI hides what the backend would reject.
