import { useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { CheckCircle2, ClipboardList, Filter, Plus, Search, ShieldCheck, Sparkles } from 'lucide-react'
import { useMe, useProjects, useTasks, useUsers } from '@/lib/hooks'
import { allReviewersApproved, gatesComplete, reviewProgress } from '@/lib/permissions'
import { pointsLabel, shortId } from '@/lib/format'
import { PageHeader } from '@/components/layout'
import { Badge, Button, EmptyState, Input, Spinner } from '@/components/ui'
import { GateDots, ReviewPips, StateBadge, TypeBadge } from '@/components/task'
import { CreateTaskDialog } from './CreateTaskDialog'
import { cx } from '@/lib/cx'

type View = 'all' | 'mine' | 'reviewing' | 'approvals' | 'completed'

const VIEW_LABEL: Record<View, string> = {
  all: 'All tasks',
  mine: 'Mine',
  reviewing: 'To review',
  approvals: 'Approvals',
  completed: 'Completed',
}

export function TasksPage() {
  const me = useMe().data
  const { data: tasks = [], isLoading } = useTasks({})
  const { data: projects = [] } = useProjects()
  const { data: users = [] } = useUsers()
  const [params, setParams] = useSearchParams()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [search, setSearch] = useState('')

  const view = (params.get('view') as View) || 'all'

  useEffect(() => {
    if (params.get('new') === '1' && me) {
      setDialogOpen(true)
      params.delete('new')
      setParams(params, { replace: true })
    }
  }, [params, setParams, me])

  const projectById = useMemo(() => new Map(projects.map((p) => [p.id, p])), [projects])
  const userById = useMemo(() => new Map(users.map((u) => [u.id, u])), [users])

  const buckets = useMemo(() => {
    if (!me) return { all: [] as typeof tasks, mine: [] as typeof tasks, reviewing: [] as typeof tasks, approvals: [] as typeof tasks, completed: [] as typeof tasks }
    const mine = tasks.filter((task) => task.assigneeId === me.id)
    const reviewing = tasks.filter(
      (task) =>
        task.taskState === 'auditing' &&
        task.acceptance.reviewers.some((r) => r.userId === me.id && r.decision === 'pending'),
    )
    const approvals = tasks.filter(
      (task) => task.approvalState === 'proposed' || task.approvalState === 'changes_requested',
    )
    const completed = tasks.filter((task) => task.taskState === 'completed')
    return { all: tasks, mine, reviewing, approvals, completed }
  }, [tasks, me])

  const visible = (buckets[view] ?? tasks).filter((task) => {
    if (!search.trim()) return true
    const needle = search.toLowerCase()
    return (
      task.description.toLowerCase().includes(needle) ||
      task.role.toLowerCase().includes(needle) ||
      (projectById.get(task.projectId)?.name.toLowerCase().includes(needle) ?? false)
    )
  })

  const tabs: View[] = me?.role === 'Lead'
    ? ['all', 'mine', 'reviewing', 'approvals', 'completed']
    : ['all', 'mine', 'reviewing', 'completed']

  return (
    <div className="stack">
      <PageHeader
        eyebrow="Work queue"
        title={<>Every <em>task</em>, one ledger.</>}
        description="Propose anything. A lead approves what is fit to build. Reviewers sign off. Points land when it ships."
        meta={
          <>
            <Badge tone="gold"><Sparkles size={11} /> {tasks.filter((t) => t.taskState !== 'completed').reduce((s, t) => s + t.points, 0)} pts open</Badge>
            <Badge tone="muted"><CheckCircle2 size={11} /> {buckets.completed.length} completed</Badge>
          </>
        }
        actions={me ? <Button onClick={() => setDialogOpen(true)}><Plus size={15} /> New task</Button> : null}
      />

      <div className="task-toolbar">
        <div className="task-tabs">
          {tabs.map((tab) => (
            <button
              key={tab}
              className={cx('task-tab', view === tab && 'is-active')}
              onClick={() => {
                const next = new URLSearchParams(params)
                if (tab === 'all') next.delete('view')
                else next.set('view', tab)
                setParams(next, { replace: true })
              }}
            >
              {VIEW_LABEL[tab]}
              <span className="task-tab__count">{buckets[tab].length}</span>
            </button>
          ))}
        </div>
        <label className="search">
          <Search size={15} />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search tasks, roles, projects"
            aria-label="Search tasks"
          />
        </label>
      </div>

      {isLoading ? (
        <div className="loading-pane"><Spinner label="Loading tasks" /><span>Reading the ledger…</span></div>
      ) : visible.length === 0 ? (
        <EmptyState
          icon={view === 'reviewing' ? <ShieldCheck size={22} /> : view === 'approvals' ? <ClipboardList size={22} /> : <Filter size={22} />}
          title={view === 'reviewing' ? 'No reviews waiting' : view === 'approvals' ? 'Nothing to approve' : 'No tasks here'}
          body={
            view === 'reviewing'
              ? 'When a task you review enters auditing, it will appear here.'
              : view === 'approvals'
                ? 'Proposals from the team will queue up here for a lead decision.'
                : 'Try another filter, or propose a new task.'
          }
          action={me ? <Button variant="outline" onClick={() => setDialogOpen(true)}><Plus size={15} /> Propose a task</Button> : undefined}
        />
      ) : (
        <div className="task-table">
          <div className="task-table__head">
            <span>Task</span>
            <span>Project</span>
            <span>State</span>
            <span>Owner</span>
            <span>Gates</span>
            <span>Reviews</span>
            <span className="task-table__points-head">Points</span>
          </div>
          {visible.map((task) => {
            const assignee = task.assigneeId ? userById.get(task.assigneeId) : undefined
            const review = reviewProgress(task)
            return (
              <Link key={task.id} to={`/tasks/${task.id}`} className="task-row">
                <div className="task-row__task">
                  <span className="mono-quiet">{shortId(task.id)}</span>
                  <div>
                    <strong>{task.description}</strong>
                    <span className="task-row__role">{task.role} · <TypeBadge type={task.taskType} /></span>
                  </div>
                </div>
                <span className="task-row__project">{projectById.get(task.projectId)?.name ?? '—'}</span>
                <span><StateBadge task={task} /></span>
                <span className="task-row__owner">{assignee ? assignee.username : <span className="muted-copy">Unassigned</span>}</span>
                <span>{gatesComplete(task) && allReviewersApproved(task) ? <Badge tone="gold">All clear</Badge> : <GateDots task={task} />}</span>
                <span>{review.total > 0 ? <ReviewPips task={task} /> : <span className="muted-copy">—</span>}</span>
                <span className="task-row__points">+{pointsLabel(task.points)}</span>
              </Link>
            )
          })}
        </div>
      )}

      <CreateTaskDialog open={dialogOpen} onClose={() => setDialogOpen(false)} />
    </div>
  )
}
