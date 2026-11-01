import { useMemo } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import {
  ArrowUpRight,
  BadgeCheck,
  CircleDollarSign,
  Crown,
  Medal,
  RotateCcw,
  ShieldCheck,
  Sparkles,
  TrendingUp,
} from 'lucide-react'
import { api, API_MODE } from '@/lib/api'
import { useLeaderboard, useLedger, useMe, useProjects, useTasks, useUsers } from '@/lib/hooks'
import { pendingRewards, tierFor, tierProgress, TIERS } from '@/lib/points'
import { pointsLabel, relativeDate } from '@/lib/format'
import { useSession } from '@/lib/session'
import { PageHeader } from '@/components/layout'
import { Avatar, Badge, Button, EmptyState, Progress, Spinner, Stat } from '@/components/ui'
import { Leaderboard, LedgerTable, TierBadge } from '@/components/points'
import { cx } from '@/lib/cx'

export function ProfilePage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const signOut = useSession((state) => state.signOut)
  const { data: me, isLoading } = useMe()
  const { data: ledger = [] } = useLedger(me?.id ?? 0)
  const { data: leaderboard = [] } = useLeaderboard()
  const { data: tasks = [] } = useTasks({})
  const { data: projects = [] } = useProjects()
  const { data: users = [] } = useUsers()

  const stats = useMemo(() => {
    if (!me) return { completed: 0, reviews: 0, month: 0, rank: 0, pending: 0 }
    const myTasks = tasks.filter((task) => task.assigneeId === me.id)
    const completed = myTasks.filter((task) => task.taskState === 'completed').length
    const reviews = tasks.reduce(
      (count, task) =>
        count + task.acceptance.reviewers.filter((r) => r.userId === me.id && r.decision === 'approved').length,
      0,
    )
    const now = new Date()
    const month = ledger
      .filter((entry) => {
        const date = new Date(entry.createdAt)
        return date.getMonth() === now.getMonth() && date.getFullYear() === now.getFullYear()
      })
      .reduce((sum, entry) => sum + entry.points, 0)
    const rank = leaderboard.find((row) => row.user.id === me.id)?.rank ?? 0
    return { completed, reviews, month, rank, pending: pendingRewards(tasks, me.id) }
  }, [me, tasks, ledger, leaderboard])

  if (isLoading || !me) {
    return <div className="loading-pane"><Spinner label="Loading profile" /><span>Opening your record…</span></div>
  }

  const progress = tierProgress(me.points)
  const tier = tierFor(me.points)
  const myRank = leaderboard.find((row) => row.user.id === me.id)

  return (
    <div className="stack">
      <PageHeader
        eyebrow="Profile"
        title={<>{me.username}<span className="title-dot">.</span></>}
        description={
          <>
            {me.role} · {tier.name}. Your points are earned, never given — every entry below is a task that shipped or a review that cleared.
          </>
        }
        meta={
          <>
            <TierBadge points={me.points} />
            <Badge tone="gold"><Sparkles size={11} /> {pointsLabel(me.points)}</Badge>
            <span className="meta-quiet">Rank #{stats.rank || '—'} of {leaderboard.length}</span>
          </>
        }
        actions={<Link to="/tasks" className="btn btn--outline btn--md">Find work <ArrowUpRight size={14} /></Link>}
      />

      <section className="profile-hero">
        <div className="profile-hero__identity">
          <Avatar name={me.username} size="xl" />
          <div>
            <span className="eyebrow">Member since {new Date(me.createdAt).getFullYear()}</span>
            <h2>{me.username}</h2>
            <p>{me.role}</p>
          </div>
        </div>
        <div className="profile-hero__points">
          <span className="profile-hero__points-label">Lifetime points</span>
          <strong>{me.points}</strong>
          <span className="profile-hero__points-tier"><Crown size={13} /> {tier.name}</span>
        </div>
        <div className="profile-hero__progress">
          <div className="profile-hero__progress-head">
            <span>{tier.name}</span>
            <span>{progress.next ? `${progress.remaining} to ${progress.next.name}` : 'Top tier'}</span>
          </div>
          <Progress value={progress.pct} tone={tier.tone === 'red' ? 'red' : 'gold'} />
          <div className="tier-ladder">
            {TIERS.map((entry) => (
              <span key={entry.name} className={cx('tier-ladder__step', me.points >= entry.min && 'is-reached')}>
                <span className="tier-ladder__dot" />
                {entry.name}
                <em>{entry.min}</em>
              </span>
            ))}
          </div>
        </div>
      </section>

      <section className="stat-row">
        <Stat label="Tasks completed" value={stats.completed} hint="Paid from the ledger" tone="gold" />
        <Stat label="Reviews cleared" value={stats.reviews} hint="Sign-offs you gave" />
        <Stat label="Earned this month" value={stats.month} hint="Recent momentum" tone="red" />
        <Stat label="At stake" value={stats.pending} hint="If current work lands" />
      </section>

      <div className="profile-grid">
        <div className="profile-main">
          <section className="panel">
            <header className="panel__head">
              <div>
                <span className="eyebrow">The ledger</span>
                <h2 className="panel__title">Every point, accounted for</h2>
              </div>
              <Badge tone="muted">{ledger.length} entries</Badge>
            </header>
            <LedgerTable entries={ledger} tasks={tasks} projects={projects} />
          </section>

          <section className="panel">
            <header className="panel__head">
              <div>
                <span className="eyebrow">Standings</span>
                <h2 className="panel__title">Leaderboard</h2>
              </div>
              <Medal size={18} className="panel__icon" />
            </header>
            <Leaderboard rows={leaderboard} currentUserId={me.id} />
          </section>
        </div>

        <aside className="profile-side">
          <section className="panel panel--tight">
            <header className="panel__head">
              <div>
                <span className="eyebrow">How it pays</span>
                <h2 className="panel__title">The point economy</h2>
              </div>
              <CircleDollarSign size={18} className="panel__icon" />
            </header>
            <ul className="economy-list">
              <li>
                <span className="economy-list__icon economy-list__icon--gold"><BadgeCheck size={15} /></span>
                <div>
                  <strong>Assignee earns the full task value</strong>
                  <p>When a manager completes the task, its points land on your profile.</p>
                </div>
              </li>
              <li>
                <span className="economy-list__icon economy-list__icon--bone"><ShieldCheck size={15} /></span>
                <div>
                  <strong>Reviewers earn 15%</strong>
                  <p>Each approving reviewer earns a bonus. The assignee cannot review their own work.</p>
                </div>
              </li>
              <li>
                <span className="economy-list__icon economy-list__icon--red"><Crown size={15} /></span>
                <div>
                  <strong>Managers earn nothing for clicking</strong>
                  <p>Approval and completion are duties, not payouts. No self-dealing.</p>
                </div>
              </li>
              <li>
                <span className="economy-list__icon"><TrendingUp size={15} /></span>
                <div>
                  <strong>Paid exactly once</strong>
                  <p>Completed tasks cannot be paid twice. The ledger is the source of truth.</p>
                </div>
              </li>
            </ul>
          </section>

          <section className="panel panel--tight">
            <header className="panel__head">
              <div>
                <span className="eyebrow">Your standing</span>
                <h2 className="panel__title">By the numbers</h2>
              </div>
            </header>
            {myRank ? (
              <div className="rank-card">
                <span className="rank-card__rank">#{myRank.rank}</span>
                <div>
                  <strong>{myRank.user.points} points</strong>
                  <span>{myRank.completedTasks} tasks · {myRank.reviews} reviews</span>
                </div>
              </div>
            ) : (
              <p className="muted-copy">Unranked for now.</p>
            )}
            <div className="mini-kv">
              <div><span>Role</span><strong>{me.role}</strong></div>
              <div><span>Tier</span><strong>{tier.name}</strong></div>
              <div><span>Next tier</span><strong>{progress.next?.name ?? '—'}</strong></div>
              <div><span>Points to go</span><strong>{progress.next ? progress.remaining : 0}</strong></div>
            </div>
            {API_MODE === 'mock' ? (
              <div className="demo-reset">
                <p>Demo data lives in this browser only. Reset it to walk the whole story again.</p>
                <Button
                  variant="quiet"
                  size="sm"
                  onClick={() => {
                    if (!window.confirm('Reset the demo? Tasks, points and sessions will be restored to the seed.')) return
                    api.resetDemo?.()
                    queryClient.clear()
                    signOut()
                    navigate('/')
                  }}
                >
                  <RotateCcw size={13} /> Reset demo data
                </Button>
              </div>
            ) : null}
          </section>

          <section className="panel panel--tight">
            <header className="panel__head">
              <div>
                <span className="eyebrow">Recent</span>
                <h2 className="panel__title">Last movement</h2>
              </div>
            </header>
            {ledger.slice(0, 3).length === 0 ? (
              <EmptyState title="No movement yet" body="Complete a task to open your ledger." />
            ) : (
              <div className="mini-list">
                {ledger.slice(0, 3).map((entry) => (
                  <div key={entry.id} className="mini-row">
                    <div>
                      <strong>{entry.note}</strong>
                      <span>{relativeDate(entry.createdAt)}</span>
                    </div>
                    <Badge tone={entry.points >= 0 ? 'gold' : 'red'}>{entry.points >= 0 ? '+' : ''}{entry.points}</Badge>
                  </div>
                ))}
              </div>
            )}
          </section>

          <section className="panel panel--tight">
            <header className="panel__head">
              <div>
                <span className="eyebrow">Roster</span>
                <h2 className="panel__title">People</h2>
              </div>
            </header>
            <div className="people-list">
              {users.map((user) => (
                <div key={user.id} className={cx('people-row', user.id === me.id && 'is-me')}>
                  <Avatar name={user.username} size="sm" />
                  <div>
                    <strong>{user.username}{user.id === me.id ? <span className="leaderboard__you">you</span> : null}</strong>
                    <span>{user.role}</span>
                  </div>
                  <span className="people-row__points">{user.points}</span>
                </div>
              ))}
            </div>
          </section>
        </aside>
      </div>
    </div>
  )
}
