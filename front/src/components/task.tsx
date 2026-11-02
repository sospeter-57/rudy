import { Link } from 'react-router-dom'
import { ArrowUpRight, Check, CircleDashed, GitPullRequest, MessageSquareWarning, ShieldCheck } from 'lucide-react'
import { cx } from '@/lib/cx'
import { pointsLabel } from '@/lib/format'
import { APPROVAL_LABEL, STATE_LABEL, TASK_TYPE_LABEL, allReviewersApproved, gatesComplete } from '@/lib/permissions'
import type { Task, User } from '@/lib/types'
import { Avatar, Badge } from '@/components/ui'

export function TypeBadge({ type }: { type: Task['taskType'] }) {
  return <Badge tone={type === 'bug' ? 'red' : type === 'research' ? 'bone' : 'muted'}>{TASK_TYPE_LABEL[type]}</Badge>
}

export function StateBadge({ task }: { task: Task }) {
  if (task.taskState === 'completed') return <Badge tone="gold">Completed</Badge>
  if (task.taskState === 'auditing') return <Badge tone="red">Auditing</Badge>
  if (task.approvalState === 'proposed') return <Badge tone="muted">Awaiting approval</Badge>
  if (task.approvalState === 'changes_requested') return <Badge tone="red">Changes requested</Badge>
  return <Badge tone="bone">{STATE_LABEL[task.taskState]}</Badge>
}

export function GateDots({ task, showLabels }: { task: Task; showLabels?: boolean }) {
  const gates = [
    { key: 'code', label: 'Code', done: task.acceptance.code.trim().length > 0 },
    { key: 'tests', label: 'Tests', done: task.acceptance.tests.trim().length > 0 },
    { key: 'docs', label: 'Docs', done: task.acceptance.documentation.trim().length > 0 },
    {
      key: 'reviews',
      label: 'Reviews',
      done: task.acceptance.reviewers.length > 0 && allReviewersApproved(task),
    },
  ]

  if (showLabels) {
    return (
      <div className="gates">
        {gates.map((gate) => (
          <span key={gate.key} className={cx('gate', gate.done && 'gate--done')}>
            {gate.done ? <Check size={12} /> : <CircleDashed size={12} />}
            {gate.label}
          </span>
        ))}
      </div>
    )
  }

  return (
    <span className="gate-dots" title={gates.map((g) => `${g.label}: ${g.done ? 'done' : 'open'}`).join(' · ')}>
      {gates.map((gate) => (
        <span key={gate.key} className={cx('gate-dot', gate.done && 'gate-dot--done')} />
      ))}
    </span>
  )
}

export function ReviewPips({ task }: { task: Task }) {
  const total = task.acceptance.reviewers.length
  if (total === 0) return <span className="review-pips review-pips--empty">No reviewers</span>
  const approved = task.acceptance.reviewers.filter((r) => r.decision === 'approved').length
  return (
    <span className="review-pips" title={`${approved} of ${total} reviewers approved`}>
      <ShieldCheck size={12} />
      {approved}/{total}
    </span>
  )
}

function accentFor(task: Task): string {
  if (task.taskState === 'completed') return 'gold'
  if (task.taskState === 'auditing') return 'red'
  if (task.approvalState === 'proposed') return 'muted'
  if (task.approvalState === 'changes_requested') return 'red'
  return 'bone'
}

export function TaskCard({ task, users }: { task: Task; users: User[] }) {
  const assignee = users.find((u) => u.id === task.assigneeId)
  const accent = accentFor(task)

  return (
    <Link to={`/tasks/${task.id}`} className={cx('task-card', `task-card--${accent}`)}>
      <div className="task-card__top">
        <TypeBadge type={task.taskType} />
        <span className="task-card__points">+{task.points}</span>
      </div>
      <h3 className="task-card__title">{task.description}</h3>
      <div className="task-card__role">{task.role}</div>
      <div className="task-card__foot">
        <span className="task-card__assignee">
          {assignee ? <Avatar name={assignee.username} size="sm" /> : <span className="task-card__unassigned">Unassigned</span>}
          {assignee ? <span>{assignee.username}</span> : null}
        </span>
        <ReviewPips task={task} />
      </div>
      <div className="task-card__gates">
        <GateDots task={task} showLabels />
      </div>
      <ArrowUpRight className="task-card__arrow" size={15} />
    </Link>
  )
}

const COLUMNS: Array<{ key: string; title: string; caption: string; match: (task: Task) => boolean }> = [
  {
    key: 'proposals',
    title: 'Proposals',
    caption: 'Waiting for the lead',
    match: (task) => task.approvalState === 'proposed' || task.approvalState === 'changes_requested',
  },
  {
    key: 'approved',
    title: 'Approved',
    caption: 'Fit to be worked on',
    match: (task) => task.approvalState === 'approved' && task.taskState === 'pending',
  },
  {
    key: 'auditing',
    title: 'Auditing',
    caption: 'Reviewers deciding',
    match: (task) => task.taskState === 'auditing',
  },
  {
    key: 'completed',
    title: 'Completed',
    caption: 'Points paid out',
    match: (task) => task.taskState === 'completed',
  },
]

export function TaskBoard({ tasks, users }: { tasks: Task[]; users: User[] }) {
  return (
    <div className="board">
      {COLUMNS.map((column) => {
        const items = tasks.filter(column.match)
        return (
          <section key={column.key} className="board__col">
            <header className="board__head">
              <div>
                <h2 className="board__title">{column.title}</h2>
                <p className="board__caption">{column.caption}</p>
              </div>
              <span className="board__count">{items.length}</span>
            </header>
            <div className="board__list">
              {items.length === 0 ? (
                <div className="board__empty">
                  {column.key === 'completed' ? <Check size={16} /> : <CircleDashed size={16} />}
                  <span>Nothing here yet</span>
                </div>
              ) : (
                items.map((task) => <TaskCard key={task.id} task={task} users={users} />)
              )}
            </div>
          </section>
        )
      })}
    </div>
  )
}

export function TaskStateSummary({ task }: { task: Task }) {
  const label =
    task.taskState === 'completed'
      ? 'Completed'
      : task.taskState === 'auditing'
        ? 'In review'
        : APPROVAL_LABEL[task.approvalState]

  return (
    <div className="task-summary">
      <span className={cx('task-summary__dot', task.taskState === 'completed' && 'is-done', task.taskState === 'auditing' && 'is-live')} />
      <span>{label}</span>
      {task.taskState === 'auditing' ? <GitPullRequest size={13} /> : null}
      {task.approvalState === 'changes_requested' ? <MessageSquareWarning size={13} /> : null}
      {gatesComplete(task) ? <Check size={13} /> : null}
    </div>
  )
}

export function pointsForTask(task: Task): string {
  return pointsLabel(task.points)
}
