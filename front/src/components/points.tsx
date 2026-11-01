import { ArrowDownRight, ArrowUpRight, Crown, Sparkles } from 'lucide-react'
import { cx } from '@/lib/cx'
import { fullDate, pointsLabel, relativeDate } from '@/lib/format'
import { awardPlanFor, tierFor, type AwardPlan } from '@/lib/points'
import type { LeaderboardRow, LedgerEntry, Task, User } from '@/lib/types'
import { Avatar, Badge, Progress } from '@/components/ui'

export function TierBadge({ points }: { points: number }) {
  const tier = tierFor(points)
  return (
    <span className={cx('tier-badge', `tier-badge--${tier.tone}`)}>
      <Sparkles size={12} />
      {tier.name}
    </span>
  )
}

export function AwardPreview({ task, users }: { task: Task; users: User[] }) {
  const plan: AwardPlan = awardPlanFor(task, users)
  const byId = new Map(users.map((u) => [u.id, u]))

  if (plan.lines.length === 0) {
    return <p className="award-empty">No points will be awarded — assign the task first.</p>
  }

  return (
    <div className="award-preview">
      {plan.lines.map((line) => {
        const user = byId.get(line.userId)
        return (
          <div key={`${line.userId}-${line.reason}`} className="award-preview__row">
            <Avatar name={user?.username ?? '?'} size="sm" />
            <div className="award-preview__who">
              <strong>{user?.username ?? 'Unknown'}</strong>
              <span>{line.reason === 'task_completed' ? 'Assignee payout' : 'Review bonus · 15%'}</span>
            </div>
            <span className="award-preview__points">+{pointsLabel(line.points)}</span>
          </div>
        )
      })}
      <div className="award-preview__total">
        <span>Total minted</span>
        <strong>+{pointsLabel(plan.total)}</strong>
      </div>
    </div>
  )
}

export function LedgerTable({
  entries,
  tasks,
  projects,
  compact,
}: {
  entries: LedgerEntry[]
  tasks: Task[]
  projects: Array<{ id: number; name: string }>
  compact?: boolean
}) {
  const taskById = new Map(tasks.map((t) => [t.id, t]))
  const projectById = new Map(projects.map((p) => [p.id, p]))

  if (entries.length === 0) {
    return <p className="ledger-empty">No points yet. Finish a task to open your account.</p>
  }

  return (
    <div className={cx('ledger', compact && 'ledger--compact')}>
      <div className="ledger__head">
        <span>Activity</span>
        <span>Reason</span>
        <span>Project</span>
        <span>When</span>
        <span className="ledger__points-head">Points</span>
      </div>
      {entries.map((entry) => {
        const task = entry.taskId ? taskById.get(entry.taskId) : undefined
        const project = entry.projectId ? projectById.get(entry.projectId) : undefined
        return (
          <div key={entry.id} className="ledger__row">
            <div className="ledger__activity">
              <span className={cx('ledger__mark', entry.points >= 0 ? 'is-credit' : 'is-debit')}>
                {entry.points >= 0 ? <ArrowUpRight size={13} /> : <ArrowDownRight size={13} />}
              </span>
              <div>
                <strong>{entry.note}</strong>
                {task ? <span className="ledger__task">{task.description}</span> : null}
              </div>
            </div>
            <span>
              <Badge tone={entry.reason === 'review_bonus' ? 'bone' : entry.reason === 'adjustment' ? 'muted' : 'gold'}>
                {entry.reason === 'task_completed' ? 'Task payout' : entry.reason === 'review_bonus' ? 'Review' : 'Credit'}
              </Badge>
            </span>
            <span className="ledger__project">{project?.name ?? '—'}</span>
            <span className="ledger__when" title={fullDate(entry.createdAt)}>
              {relativeDate(entry.createdAt)}
            </span>
            <span className="ledger__points">{entry.points >= 0 ? '+' : ''}{entry.points}</span>
          </div>
        )
      })}
    </div>
  )
}

export function Leaderboard({
  rows,
  currentUserId,
  limit,
}: {
  rows: LeaderboardRow[]
  currentUserId?: number
  limit?: number
}) {
  const visible = limit ? rows.slice(0, limit) : rows
  return (
    <div className="leaderboard">
      {visible.map((row) => {
        const isMe = row.user.id === currentUserId
        return (
          <div key={row.user.id} className={cx('leaderboard__row', isMe && 'leaderboard__row--me', row.rank <= 3 && 'leaderboard__row--podium')}>
            <span className={cx('leaderboard__rank', row.rank === 1 && 'is-first')}>
              {row.rank === 1 ? <Crown size={14} /> : row.rank}
            </span>
            <Avatar name={row.user.username} size="sm" />
            <div className="leaderboard__who">
              <strong>
                {row.user.username}
                {isMe ? <span className="leaderboard__you">you</span> : null}
              </strong>
              <span>{row.user.role}</span>
            </div>
            <div className="leaderboard__stats">
              <span>{row.completedTasks} done</span>
              <span>{row.reviews} reviews</span>
            </div>
            <div className="leaderboard__points">
              <strong>{row.user.points}</strong>
              <span>pts</span>
            </div>
            <Progress value={Math.min(100, (row.user.points / Math.max(1, rows[0]?.user.points ?? 1)) * 100)} />
          </div>
        )
      })}
    </div>
  )
}
