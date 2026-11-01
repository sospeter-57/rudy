import type { Task, User } from './types'

/**
 * The point economy
 * -----------------
 * Points are only ever created by the ledger. A user's balance is the sum of
 * their ledger entries, so there is a single source of truth and no drift.
 *
 * Awards happen exactly once, when a Project Manager marks a task completed:
 *
 *   - Assignee  -> task.points                       (reason: task_completed)
 *   - Reviewer  -> round(task.points * REVIEW_RATE)   (reason: review_bonus)
 *
 * A reviewer never earns a review bonus for a task they are assigned to, and
 * the manager earns nothing simply for pressing "complete" — that keeps the
 * economy free of self-dealing. Rejections, proposals and work-in-progress
 * never pay out.
 */

export const REVIEW_RATE = 0.15
export const MIN_REVIEW_BONUS = 2

export function reviewBonusFor(taskPoints: number): number {
  return Math.max(MIN_REVIEW_BONUS, Math.round(taskPoints * REVIEW_RATE))
}

export interface AwardLine {
  userId: number
  reason: 'task_completed' | 'review_bonus'
  points: number
  note: string
}

export interface AwardPlan {
  lines: AwardLine[]
  assigneePoints: number
  reviewerPoints: number
  total: number
}

/** The exact payout a task will produce once a manager completes it. */
export function awardPlanFor(task: Task, users: User[]): AwardPlan {
  const byId = new Map(users.map((u) => [u.id, u]))
  const lines: AwardLine[] = []
  let assigneePoints = 0
  let reviewerPoints = 0

  if (task.assigneeId) {
    const assignee = byId.get(task.assigneeId)
    if (assignee) {
      assigneePoints = task.points
      lines.push({
        userId: assignee.id,
        reason: 'task_completed',
        points: task.points,
        note: `Completed “${task.description}”`,
      })
    }
  }

  const bonus = reviewBonusFor(task.points)
  for (const review of task.acceptance.reviewers) {
    if (review.decision !== 'approved') continue
    if (review.userId === task.assigneeId) continue
    const reviewer = byId.get(review.userId)
    if (!reviewer) continue
    reviewerPoints += bonus
    lines.push({
      userId: reviewer.id,
      reason: 'review_bonus',
      points: bonus,
      note: `Reviewed “${task.description}”`,
    })
  }

  return {
    lines,
    assigneePoints,
    reviewerPoints,
    total: assigneePoints + reviewerPoints,
  }
}

export interface Tier {
  name: string
  min: number
  tone: 'bone' | 'gold' | 'gold-bright' | 'red'
  blurb: string
}

export const TIERS: Tier[] = [
  { name: 'Apprentice', min: 0, tone: 'bone', blurb: 'Learning the craft' },
  { name: 'Journeyman', min: 100, tone: 'gold', blurb: 'Shipping with confidence' },
  { name: 'Artisan', min: 250, tone: 'gold-bright', blurb: 'Trusted with the hard parts' },
  { name: 'Master', min: 500, tone: 'gold-bright', blurb: 'Sets the standard' },
  { name: 'Legend', min: 1000, tone: 'red', blurb: 'The name on the wall' },
]

export function tierFor(points: number): Tier {
  let current = TIERS[0]
  for (const tier of TIERS) {
    if (points >= tier.min) current = tier
  }
  return current
}

export function nextTierFor(points: number): Tier | null {
  return TIERS.find((tier) => tier.min > points) ?? null
}

export function tierProgress(points: number): { current: Tier; next: Tier | null; pct: number; remaining: number } {
  const current = tierFor(points)
  const next = nextTierFor(points)
  if (!next) {
    return { current, next: null, pct: 100, remaining: 0 }
  }
  const span = next.min - current.min
  const pct = Math.min(100, Math.max(0, ((points - current.min) / span) * 100))
  return { current, next, pct, remaining: next.min - points }
}

/** Points a user will collect if every in-flight task finishes cleanly. */
export function pendingRewards(tasks: Task[], userId: number): number {
  return tasks
    .filter(
      (task) =>
        task.assigneeId === userId &&
        task.approvalState === 'approved' &&
        task.taskState !== 'completed',
    )
    .reduce((sum, task) => sum + task.points, 0)
}
