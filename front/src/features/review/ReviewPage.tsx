import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { ArrowUpRight, CheckCircle2, ShieldCheck } from 'lucide-react'
import { useMe, useProjects, useTasks, useUsers } from '@/lib/hooks'
import { reviewProgress } from '@/lib/permissions'
import { pointsLabel, relativeDate, shortId } from '@/lib/format'
import { PageHeader } from '@/components/layout'
import { Avatar, Badge, EmptyState, Spinner } from '@/components/ui'
import { GateDots, StateBadge, TypeBadge } from '@/components/task'

export function ReviewPage() {
  const { data: me, isLoading } = useMe()
  const { data: tasks = [] } = useTasks({})
  const { data: users = [] } = useUsers()
  const { data: projects = [] } = useProjects()

  const projectById = useMemo(() => new Map(projects.map((p) => [p.id, p])), [projects])
  const userById = useMemo(() => new Map(users.map((u) => [u.id, u])), [users])

  if (isLoading || !me) {
    return <div className="loading-pane"><Spinner label="Loading review queue" /><span>Checking for sign-offs…</span></div>
  }

  const assigned = tasks.filter((task) => task.acceptance.reviewers.some((r) => r.userId === me.id))
  const pending = assigned.filter(
    (task) => task.taskState === 'auditing' && task.acceptance.reviewers.some((r) => r.userId === me.id && r.decision === 'pending'),
  )
  const history = assigned.filter((task) => !pending.includes(task))

  return (
    <div className="stack">
      <PageHeader
        eyebrow="Sign-off desk"
        title={<>Reviews <em>waiting</em> on you.</>}
        description="Read the gates, then approve or ask for changes. Your sign-off unlocks the lead's final payout."
        meta={
          <>
            <Badge tone={pending.length > 0 ? 'red' : 'gold'}><ShieldCheck size={11} /> {pending.length} pending</Badge>
            <Badge tone="muted"><CheckCircle2 size={11} /> {history.length} in history</Badge>
          </>
        }
      />

      <section className="panel">
        <header className="panel__head">
          <div>
            <span className="eyebrow">Queue</span>
            <h2 className="panel__title">Pending your decision</h2>
          </div>
          <ShieldCheck size={18} className="panel__icon" />
        </header>
        {pending.length === 0 ? (
          <EmptyState
            icon={<CheckCircle2 size={22} />}
            title="Nothing waiting on you"
            body="When a task you review enters auditing, it will land here."
          />
        ) : (
          <div className="review-queue">
            {pending.map((task) => {
              const assignee = task.assigneeId ? userById.get(task.assigneeId) : undefined
              const review = reviewProgress(task)
              return (
                <Link key={task.id} to={`/tasks/${task.id}`} className="review-queue__row">
                  <div className="review-queue__task">
                    <span className="mono-quiet">{shortId(task.id)}</span>
                    <div>
                      <strong>{task.description}</strong>
                      <span>{projectById.get(task.projectId)?.name} · {task.role}</span>
                    </div>
                  </div>
                  <div className="review-queue__meta">
                    <TypeBadge type={task.taskType} />
                    <StateBadge task={task} />
                    <GateDots task={task} showLabels />
                  </div>
                  <div className="review-queue__owner">
                    {assignee ? <Avatar name={assignee.username} size="sm" /> : null}
                    <div>
                      <span>Assignee</span>
                      <strong>{assignee?.username ?? 'Unassigned'}</strong>
                    </div>
                  </div>
                  <div className="review-queue__points">
                    <strong>+{pointsLabel(task.points)}</strong>
                    <span>{review.done}/{review.total} decided</span>
                  </div>
                  <ArrowUpRight size={15} className="review-queue__arrow" />
                </Link>
              )
            })}
          </div>
        )}
      </section>

      <section className="panel">
        <header className="panel__head">
          <div>
            <span className="eyebrow">Record</span>
            <h2 className="panel__title">Reviews you have given</h2>
          </div>
        </header>
        {history.length === 0 ? (
          <p className="muted-copy">No review history yet.</p>
        ) : (
          <div className="review-history">
            {history.map((task) => {
              const myReview = task.acceptance.reviewers.find((r) => r.userId === me.id)
              return (
                <Link key={task.id} to={`/tasks/${task.id}`} className="review-history__row">
                  <span className="mono-quiet">{shortId(task.id)}</span>
                  <div>
                    <strong>{task.description}</strong>
                    <span>{projectById.get(task.projectId)?.name}</span>
                  </div>
                  <Badge tone={myReview?.decision === 'approved' ? 'gold' : myReview?.decision === 'changes_requested' ? 'red' : 'muted'}>
                    {myReview?.decision === 'approved' ? 'Approved' : myReview?.decision === 'changes_requested' ? 'Changes requested' : 'Pending'}
                  </Badge>
                  <span className="review-history__when">
                    {myReview?.decidedAt ? relativeDate(myReview.decidedAt) : '—'}
                  </span>
                </Link>
              )
            })}
          </div>
        )}
      </section>
    </div>
  )
}
