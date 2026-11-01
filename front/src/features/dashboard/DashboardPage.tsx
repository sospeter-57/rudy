import { Link, useNavigate } from 'react-router-dom'
import {
  ArrowRight,
  ArrowUpRight,
  CircleDollarSign,
  ClipboardCheck,
  Crown,
  FolderKanban,
  Plus,
  ShieldCheck,
  Sparkles,
  Trophy,
} from 'lucide-react'
import { useMemo } from 'react'
import { useLeaderboard, useLedger, useMe, useProjects, useTasks, useUsers } from '@/lib/hooks'
import { canCreateProject, canFinalize, reviewProgress } from '@/lib/permissions'
import { pendingRewards, tierFor, tierProgress } from '@/lib/points'
import { pointsLabel, relativeDate, shortId } from '@/lib/format'
import { PageHeader } from '@/components/layout'
import { Badge, Button, EmptyState, Progress, Spinner, Stat } from '@/components/ui'
import { GateDots, ReviewPips, StateBadge } from '@/components/task'
import { Leaderboard, LedgerTable, TierBadge } from '@/components/points'
import type { Task } from '@/lib/types'

function greeting(): string {
  const hour = new Date().getHours()
  if (hour < 5) return 'Still up'
  if (hour < 12) return 'Good morning'
  if (hour < 18) return 'Good afternoon'
  return 'Good evening'
}

export function DashboardPage() {
  const navigate = useNavigate()
  const { data: me, isLoading: meLoading } = useMe()
  const { data: projects = [], isLoading: projectsLoading } = useProjects()
  const { data: tasks = [], isLoading: tasksLoading } = useTasks({})
  const { data: users = [] } = useUsers()
  const { data: leaderboard = [] } = useLeaderboard()
  const { data: ledger = [] } = useLedger(me?.id ?? 0)

  const derived = useMemo(() => {
    if (!me) {
      return {
        myTasks: [] as Task[],
        reviewQueue: [] as Task[],
        approvals: [] as Task[],
        payouts: [] as Task[],
        completed: 0,
        rank: 0,
        pending: 0,
      }
    }
    const myTasks = tasks.filter((task) => task.assigneeId === me.id)
    const reviewQueue = tasks.filter(
      (task) =>
        task.taskState === 'auditing' &&
        task.acceptance.reviewers.some((r) => r.userId === me.id && r.decision === 'pending'),
    )
    const approvals = tasks.filter(
      (task) => task.approvalState === 'proposed' || task.approvalState === 'changes_requested',
    )
    const payouts = myTasks.filter((task) => task.taskState !== 'completed' && task.approvalState === 'approved')
    const completed = myTasks.filter((task) => task.taskState === 'completed').length
    const rank = leaderboard.find((row) => row.user.id === me.id)?.rank ?? 0
    const pending = pendingRewards(tasks, me.id)
    return { myTasks, reviewQueue, approvals, payouts, completed, rank, pending }
  }, [me, tasks, leaderboard])

  if (meLoading || !me) {
    return (
      <div className="loading-pane">
        <Spinner label="Loading your deck" />
        <span>Opening the deck…</span>
      </div>
    )
  }

  const tier = tierFor(me.points)
  const progress = tierProgress(me.points)
  const projectById = new Map(projects.map((p) => [p.id, p]))
  const loading = projectsLoading || tasksLoading

  return (
    <div className="stack">
      <PageHeader
        eyebrow="Command deck"
        title={<>{greeting()}, <em>{me.username}</em>.</>}
        description={
          <>
            You are a <strong>{me.role}</strong> on the {tier.name} tier.
            {progress.next
              ? ` ${progress.remaining} points stand between you and ${progress.next.name}.`
              : ' You are at the top tier — keep the standard high.'}
          </>
        }
        meta={
          <>
            <TierBadge points={me.points} />
            <Badge tone="gold">
              <Sparkles size={11} /> {pointsLabel(me.points)}
            </Badge>
            <span className="meta-quiet">Rank #{derived.rank || '—'} of {leaderboard.length}</span>
          </>
        }
        actions={
          <>
            <Button variant="outline" onClick={() => navigate('/tasks?new=1')}>
              <Plus size={15} /> New task
            </Button>
            {canCreateProject(me) ? (
              <Button onClick={() => navigate('/projects?new=1')}>
                <FolderKanban size={15} /> New project
              </Button>
            ) : null}
          </>
        }
      />

      <section className="stat-row">
        <Stat label="Total points" value={me.points} hint="Lifetime, from the ledger" tone="gold" />
        <Stat label="Completed" value={derived.completed} hint="Tasks you finished" />
        <Stat label="At stake" value={derived.pending} hint="If current work lands" tone="red" />
        <Stat label="Tier progress" value={`${Math.round(progress.pct)}%`} hint={progress.next ? `to ${progress.next.name}` : 'Maxed out'} />
      </section>

      <div className="dash-grid">
        <div className="dash-main">
          <section className="panel">
            <header className="panel__head">
              <div>
                <span className="eyebrow">Rewards in motion</span>
                <h2 className="panel__title">At stake</h2>
              </div>
              <CircleDollarSign size={18} className="panel__icon" />
            </header>
            {derived.payouts.length === 0 ? (
              <EmptyState
                icon={<CircleDollarSign size={20} />}
                title="Nothing at stake yet"
                body="Claim or get assigned an approved task and the points will show up here."
                action={<Link to="/tasks" className="btn btn--outline btn--sm">Find work <ArrowRight size={14} /></Link>}
              />
            ) : (
              <div className="payout-list">
                {derived.payouts.map((task) => {
                  const review = reviewProgress(task)
                  const ready = canFinalize(task, { ...me, role: 'Project Manager' })
                  return (
                    <Link key={task.id} to={`/tasks/${task.id}`} className="payout-row">
                      <div className="payout-row__main">
                        <span className="mono-quiet">{shortId(task.id)}</span>
                        <strong>{task.description}</strong>
                        <span className="payout-row__project">{projectById.get(task.projectId)?.name}</span>
                      </div>
                      <div className="payout-row__meta">
                        <StateBadge task={task} />
                        <ReviewPips task={task} />
                        {task.taskState === 'auditing' ? (
                          <span className="payout-row__reviews">{review.done}/{review.total} decided</span>
                        ) : null}
                      </div>
                      <div className="payout-row__points">
                        <strong>+{task.points}</strong>
                        <span>pts</span>
                      </div>
                      {ready ? <Badge tone="gold">Ready</Badge> : <GateDots task={task} />}
                    </Link>
                  )
                })}
              </div>
            )}
          </section>

          <section className="panel">
            <header className="panel__head">
              <div>
                <span className="eyebrow">Your ledger</span>
                <h2 className="panel__title">Recent point activity</h2>
              </div>
              <Link to="/profile" className="text-link">Open profile <ArrowUpRight size={13} /></Link>
            </header>
            <LedgerTable entries={ledger.slice(0, 5)} tasks={tasks} projects={projects} compact />
          </section>

          <section className="panel">
            <header className="panel__head">
              <div>
                <span className="eyebrow">On the board</span>
                <h2 className="panel__title">Projects</h2>
              </div>
              <Link to="/projects" className="text-link">All projects <ArrowUpRight size={13} /></Link>
            </header>
            {loading ? (
              <div className="panel__loading"><Spinner /></div>
            ) : (
              <div className="project-strip">
                {projects.map((project) => {
                  const projectTasks = tasks.filter((task) => task.projectId === project.id)
                  const done = projectTasks.filter((task) => task.taskState === 'completed').length
                  const pct = projectTasks.length === 0 ? 0 : (done / projectTasks.length) * 100
                  return (
                    <Link key={project.id} to={`/projects/${project.id}`} className="project-strip__card">
                      <div className="project-strip__top">
                        <span className="mono-quiet">{shortId(project.id)}</span>
                        <span className="project-strip__ttl">{project.ttl}m ttl</span>
                      </div>
                      <h3>{project.name}</h3>
                      <p>{project.description}</p>
                      <Progress value={pct} tone={pct === 100 ? 'gold' : 'red'} />
                      <div className="project-strip__foot">
                        <span>{done}/{projectTasks.length} completed</span>
                        <span>{projectTasks.reduce((sum, t) => sum + (t.taskState !== 'completed' ? t.points : 0), 0)} pts open</span>
                      </div>
                    </Link>
                  )
                })}
              </div>
            )}
          </section>
        </div>

        <aside className="dash-side">
          <section className="panel panel--tight">
            <header className="panel__head">
              <div>
                <span className="eyebrow">Manager desk</span>
                <h2 className="panel__title">Approvals</h2>
              </div>
              <ClipboardCheck size={18} className="panel__icon" />
            </header>
            {derived.approvals.length === 0 ? (
              <p className="muted-copy">The queue is clear. Nice.</p>
            ) : (
              <div className="mini-list">
                {derived.approvals.slice(0, 4).map((task) => (
                  <Link key={task.id} to={`/tasks/${task.id}`} className="mini-row">
                    <div>
                      <strong>{task.description}</strong>
                      <span>{task.role} · +{task.points} pts</span>
                    </div>
                    <Badge tone={task.approvalState === 'changes_requested' ? 'red' : 'muted'}>
                      {task.approvalState === 'changes_requested' ? 'Revise' : 'Review'}
                    </Badge>
                  </Link>
                ))}
              </div>
            )}
            <Link to="/tasks" className="panel__link">Open the approval queue <ArrowRight size={13} /></Link>
          </section>

          <section className="panel panel--tight">
            <header className="panel__head">
              <div>
                <span className="eyebrow">Your turn</span>
                <h2 className="panel__title">Review queue</h2>
              </div>
              <ShieldCheck size={18} className="panel__icon" />
            </header>
            {derived.reviewQueue.length === 0 ? (
              <p className="muted-copy">No reviews waiting on you.</p>
            ) : (
              <div className="mini-list">
                {derived.reviewQueue.slice(0, 4).map((task) => (
                  <Link key={task.id} to={`/tasks/${task.id}`} className="mini-row">
                    <div>
                      <strong>{task.description}</strong>
                      <span>{reviewProgress(task).done}/{reviewProgress(task).total} decided</span>
                    </div>
                    <Badge tone="red">Audit</Badge>
                  </Link>
                ))}
              </div>
            )}
          </section>

          <section className="panel panel--tight">
            <header className="panel__head">
              <div>
                <span className="eyebrow">Standings</span>
                <h2 className="panel__title">Leaderboard</h2>
              </div>
              <Trophy size={18} className="panel__icon" />
            </header>
            <Leaderboard rows={leaderboard} currentUserId={me.id} limit={5} />
            <Link to="/profile" className="panel__link">See the full table <ArrowRight size={13} /></Link>
          </section>

          <section className="panel panel--tight tier-panel">
            <div className="tier-panel__head">
              <Crown size={18} />
              <span className="eyebrow">Tier ladder</span>
            </div>
            <h2>{tier.name}</h2>
            <p>{tier.blurb}</p>
            <Progress value={progress.pct} tone={tier.tone === 'red' ? 'red' : 'gold'} />
            <span className="tier-panel__hint">
              {progress.next ? `${progress.remaining} pts to ${progress.next.name}` : 'Top of the ladder'}
            </span>
          </section>
        </aside>
      </div>
    </div>
  )
}
