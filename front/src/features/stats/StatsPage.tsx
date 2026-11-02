import { useMemo, useState } from 'react'
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { Activity, BarChart3, CircleDollarSign, PieChart as PieIcon, TrendingUp } from 'lucide-react'
import { useStats } from '@/lib/hooks'
import { APPROVAL_LABEL, STATE_LABEL, TASK_TYPE_LABEL } from '@/lib/permissions'
import { pointsLabel, relativeDate, shortId } from '@/lib/format'
import { PageHeader } from '@/components/layout'
import { Badge, Button, EmptyState, Spinner, Stat } from '@/components/ui'
import { cx } from '@/lib/cx'
import type { StatsRange, TaskType } from '@/lib/types'

const RANGES: Array<{ value: StatsRange; label: string }> = [
  { value: '30d', label: '30 days' },
  { value: '90d', label: '90 days' },
  { value: 'all', label: 'All time' },
]

const COLORS = {
  gold: '#d4a72c',
  goldBright: '#f0c75e',
  red: '#e01e37',
  bone: '#f6f1e7',
  boneDim: '#b9b2a6',
  muted: '#7d766c',
  ink: '#0f0e12',
}

const PIE_COLORS = [COLORS.gold, COLORS.red, COLORS.boneDim, COLORS.goldBright, COLORS.muted]

const tooltipStyle = {
  background: 'var(--ink-2)',
  border: '1px solid var(--line-strong)',
  borderRadius: 3,
  color: 'var(--bone)',
  fontFamily: 'var(--font-mono)',
  fontSize: 11,
}

function reasonLabel(reason: string): string {
  if (reason === 'task_completed') return 'Task completion'
  if (reason === 'review_bonus') return 'Review bonus'
  if (reason === 'adjustment') return 'Adjustment'
  return reason
}

function shortPeriod(period: string): string {
  const date = new Date(`${period}T00:00:00Z`)
  if (Number.isNaN(date.getTime())) return period
  return `${date.getUTCMonth() + 1}/${date.getUTCDate()}`
}

export function StatsPage() {
  const [range, setRange] = useState<StatsRange>('30d')
  const { data: stats, isLoading } = useStats(range)

  const stateData = useMemo(
    () =>
      Object.entries(stats?.tasksByState ?? {}).map(([key, value]) => ({
        name: STATE_LABEL[key as keyof typeof STATE_LABEL] ?? key,
        value,
      })),
    [stats],
  )

  const approvalData = useMemo(
    () =>
      Object.entries(stats?.tasksByApprovalState ?? {}).map(([key, value]) => ({
        name: APPROVAL_LABEL[key as keyof typeof APPROVAL_LABEL] ?? key,
        value,
      })),
    [stats],
  )

  const typeData = useMemo(
    () =>
      Object.entries(stats?.tasksByType ?? {}).map(([key, value]) => ({
        name: TASK_TYPE_LABEL[key as TaskType] ?? key,
        value,
      })),
    [stats],
  )

  const ledgerData = useMemo(
    () => (stats?.ledgerByReason ?? []).map((row) => ({ name: reasonLabel(row.reason), points: row.points })),
    [stats],
  )

  const trendData = useMemo(
    () =>
      (stats?.trends ?? []).map((point) => ({
        ...point,
        label: shortPeriod(point.period),
      })),
    [stats],
  )

  const projectData = useMemo(
    () =>
      (stats?.projects ?? []).map((project) => ({
        name: project.name,
        completed: project.completed,
        open: Math.max(0, project.totalTasks - project.completed),
      })),
    [stats],
  )

  if (isLoading || !stats) {
    return <div className="loading-pane"><Spinner label="Loading stats" /><span>Crunching the ledger…</span></div>
  }

  const totals = stats.totals
  const maxProjectTasks = Math.max(1, ...stats.projects.map((project) => project.totalTasks))

  return (
    <div className="stack">
      <PageHeader
        eyebrow="Stats"
        title={<>The <em>shape</em> of the work.</>}
        description="Every number is aggregated straight from the database: the board, the review queue, and the ledger."
        meta={
          <>
            <Badge tone="gold">{RANGES.find((item) => item.value === stats.range)?.label ?? stats.range}</Badge>
            <span className="meta-quiet">Completion rate {totals.completionRate}%</span>
          </>
        }
        actions={
          <div className="range-switch">
            {RANGES.map((item) => (
              <Button
                key={item.value}
                size="sm"
                variant={range === item.value ? 'primary' : 'outline'}
                onClick={() => setRange(item.value)}
              >
                {item.label}
              </Button>
            ))}
          </div>
        }
      />

      <section className="stat-row">
        <Stat label="Builders" value={totals.builders} hint={`${totals.users} accounts · ${totals.projects} projects`} />
        <Stat label="Tasks" value={totals.tasks} hint={`${totals.completedTasks} completed`} tone="gold" />
        <Stat label="Completion rate" value={`${totals.completionRate}%`} hint="Tasks finished vs proposed" />
        <Stat label="Points minted" value={pointsLabel(totals.pointsMinted)} hint="Ledger awards in range" tone="red" />
      </section>

      <div className="stat-chip-row">
        <div className="stat-chip">
          <TrendingUp size={15} />
          <div><strong>{totals.avgCycleHours}h</strong><span>Avg cycle time</span></div>
        </div>
        <div className="stat-chip">
          <Activity size={15} />
          <div><strong>{totals.reviewsApproved}/{totals.reviewsDecided}</strong><span>Reviews approved</span></div>
        </div>
        <div className="stat-chip">
          <PieIcon size={15} />
          <div><strong>{totals.reviewApprovalPct}%</strong><span>Review approval rate</span></div>
        </div>
        <div className="stat-chip">
          <CircleDollarSign size={15} />
          <div><strong>{stats.ledgerByReason.length}</strong><span>Ledger reasons</span></div>
        </div>
      </div>

      <div className="stats-grid">
        <section className="panel stats-grid__wide">
          <header className="panel__head">
            <div>
              <span className="eyebrow">Momentum</span>
              <h2 className="panel__title">Proposals, completions and reviews</h2>
            </div>
            <BarChart3 size={18} className="panel__icon" />
          </header>
          {trendData.length === 0 ? (
            <EmptyState title="Nothing in range" body="Widen the range or complete a task to populate the trend." />
          ) : (
            <div className="chart-box">
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={trendData} margin={{ top: 8, right: 8, left: -18, bottom: 0 }}>
                  <defs>
                    <linearGradient id="proposals" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor={COLORS.gold} stopOpacity={0.45} />
                      <stop offset="100%" stopColor={COLORS.gold} stopOpacity={0.02} />
                    </linearGradient>
                    <linearGradient id="completions" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor={COLORS.red} stopOpacity={0.4} />
                      <stop offset="100%" stopColor={COLORS.red} stopOpacity={0.02} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid stroke="var(--line-soft)" strokeDasharray="2 4" vertical={false} />
                  <XAxis dataKey="label" stroke={COLORS.muted} tick={{ fontSize: 10 }} tickLine={false} axisLine={false} minTickGap={24} />
                  <YAxis stroke={COLORS.muted} tick={{ fontSize: 10 }} tickLine={false} axisLine={false} allowDecimals={false} />
                  <Tooltip contentStyle={tooltipStyle} labelStyle={{ color: COLORS.goldBright }} />
                  <Legend wrapperStyle={{ fontSize: 11, color: COLORS.boneDim }} />
                  <Area type="monotone" dataKey="proposals" name="Proposals" stroke={COLORS.gold} fill="url(#proposals)" strokeWidth={2} />
                  <Area type="monotone" dataKey="completions" name="Completions" stroke={COLORS.red} fill="url(#completions)" strokeWidth={2} />
                  <Area type="monotone" dataKey="reviews" name="Reviews" stroke={COLORS.boneDim} fill="transparent" strokeWidth={1.5} strokeDasharray="4 4" />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          )}
        </section>

        <section className="panel">
          <header className="panel__head">
            <div>
              <span className="eyebrow">Board</span>
              <h2 className="panel__title">Tasks by state</h2>
            </div>
          </header>
          <div className="chart-box chart-box--small">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie data={stateData} dataKey="value" nameKey="name" innerRadius={52} outerRadius={82} paddingAngle={4} stroke="none">
                  {stateData.map((entry, index) => (
                    <Cell key={entry.name} fill={PIE_COLORS[index % PIE_COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip contentStyle={tooltipStyle} />
                <Legend wrapperStyle={{ fontSize: 11, color: COLORS.boneDim }} />
              </PieChart>
            </ResponsiveContainer>
          </div>
        </section>

        <section className="panel">
          <header className="panel__head">
            <div>
              <span className="eyebrow">Approval</span>
              <h2 className="panel__title">Fit to build</h2>
            </div>
          </header>
          <div className="chart-box chart-box--small">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={approvalData} margin={{ top: 8, right: 8, left: -22, bottom: 0 }}>
                <CartesianGrid stroke="var(--line-soft)" strokeDasharray="2 4" vertical={false} />
                <XAxis dataKey="name" stroke={COLORS.muted} tick={{ fontSize: 10 }} tickLine={false} axisLine={false} />
                <YAxis stroke={COLORS.muted} tick={{ fontSize: 10 }} tickLine={false} axisLine={false} allowDecimals={false} />
                <Tooltip contentStyle={tooltipStyle} cursor={{ fill: 'rgba(212, 167, 44, 0.08)' }} />
                <Bar dataKey="value" name="Tasks" radius={[2, 2, 0, 0]}>
                  {approvalData.map((entry, index) => (
                    <Cell key={entry.name} fill={PIE_COLORS[index % PIE_COLORS.length]} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        </section>

        <section className="panel">
          <header className="panel__head">
            <div>
              <span className="eyebrow">Kinds of work</span>
              <h2 className="panel__title">Task types</h2>
            </div>
          </header>
          <div className="chart-box chart-box--small">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={typeData} layout="vertical" margin={{ top: 4, right: 16, left: 8, bottom: 0 }}>
                <CartesianGrid stroke="var(--line-soft)" strokeDasharray="2 4" horizontal={false} />
                <XAxis type="number" stroke={COLORS.muted} tick={{ fontSize: 10 }} tickLine={false} axisLine={false} allowDecimals={false} />
                <YAxis type="category" dataKey="name" stroke={COLORS.boneDim} tick={{ fontSize: 11 }} tickLine={false} axisLine={false} width={64} />
                <Tooltip contentStyle={tooltipStyle} cursor={{ fill: 'rgba(212, 167, 44, 0.08)' }} />
                <Bar dataKey="value" name="Tasks" fill={COLORS.gold} radius={[0, 2, 2, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </section>

        <section className="panel stats-grid__wide">
          <header className="panel__head">
            <div>
              <span className="eyebrow">Projects</span>
              <h2 className="panel__title">Where the work sits</h2>
            </div>
            <Badge tone="muted">{stats.projects.length} projects</Badge>
          </header>
          {projectData.length === 0 ? (
            <EmptyState title="No projects yet" body="Open a project to see completion across the board." />
          ) : (
            <>
              <div className="chart-box chart-box--small">
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={projectData} margin={{ top: 8, right: 8, left: -18, bottom: 0 }}>
                    <CartesianGrid stroke="var(--line-soft)" strokeDasharray="2 4" vertical={false} />
                    <XAxis dataKey="name" stroke={COLORS.muted} tick={{ fontSize: 10 }} tickLine={false} axisLine={false} interval={0} angle={-12} height={42} textAnchor="end" />
                    <YAxis stroke={COLORS.muted} tick={{ fontSize: 10 }} tickLine={false} axisLine={false} allowDecimals={false} />
                    <Tooltip contentStyle={tooltipStyle} cursor={{ fill: 'rgba(212, 167, 44, 0.08)' }} />
                    <Legend wrapperStyle={{ fontSize: 11, color: COLORS.boneDim }} />
                    <Bar dataKey="completed" name="Completed" stackId="tasks" fill={COLORS.gold} radius={[0, 0, 0, 0]} />
                    <Bar dataKey="open" name="Open" stackId="tasks" fill={COLORS.red} radius={[2, 2, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              </div>

              <div className="stats-table">
                <div className="stats-table__head">
                  <span>Project</span>
                  <span>Tasks</span>
                  <span>Open</span>
                  <span>Completed</span>
                  <span>Completion</span>
                </div>
                {stats.projects.map((project) => (
                  <div key={project.projectId} className="stats-table__row">
                    <span className="stats-table__name">
                      <span className="mono-quiet">{shortId(project.projectId)}</span>
                      {project.name}
                    </span>
                    <span>{project.totalTasks}</span>
                    <span>{project.totalTasks - project.completed}</span>
                    <span>{project.completed}</span>
                    <span className="stats-table__bar">
                      <span className="stats-table__track">
                        <span style={{ width: `${Math.min(100, (project.totalTasks / maxProjectTasks) * 100)}%` }} />
                      </span>
                      <em>{project.completionPct}%</em>
                    </span>
                  </div>
                ))}
              </div>
            </>
          )}
        </section>

        <section className="panel">
          <header className="panel__head">
            <div>
              <span className="eyebrow">Ledger</span>
              <h2 className="panel__title">Points by reason</h2>
            </div>
          </header>
          {ledgerData.length === 0 ? (
            <EmptyState title="No payouts yet" body="Complete a task to mint the first points." />
          ) : (
            <div className="chart-box chart-box--small">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={ledgerData} layout="vertical" margin={{ top: 4, right: 16, left: 8, bottom: 0 }}>
                  <CartesianGrid stroke="var(--line-soft)" strokeDasharray="2 4" horizontal={false} />
                  <XAxis type="number" stroke={COLORS.muted} tick={{ fontSize: 10 }} tickLine={false} axisLine={false} />
                  <YAxis type="category" dataKey="name" stroke={COLORS.boneDim} tick={{ fontSize: 11 }} tickLine={false} axisLine={false} width={92} />
                  <Tooltip contentStyle={tooltipStyle} cursor={{ fill: 'rgba(212, 167, 44, 0.08)' }} />
                  <Bar dataKey="points" name="Points" fill={COLORS.goldBright} radius={[0, 2, 2, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          )}
        </section>

        <section className="panel">
          <header className="panel__head">
            <div>
              <span className="eyebrow">People</span>
              <h2 className="panel__title">Builders on the board</h2>
            </div>
            <Badge tone="muted">{stats.users.length}</Badge>
          </header>
          {stats.users.length === 0 ? (
            <EmptyState title="No builders yet" body="Invite the team to start proposing work." />
          ) : (
            <div className="stats-people">
              {stats.users.slice(0, 8).map((row, index) => (
                <div key={row.user.id} className="stats-people__row">
                  <span className={cx('stats-people__rank', index < 3 && 'is-top')}>{index + 1}</span>
                  <div className="stats-people__who">
                    <strong>{row.user.username}</strong>
                    <span>{row.user.role}</span>
                  </div>
                  <div className="stats-people__meta">
                    <strong>{row.user.points}</strong>
                    <span>pts</span>
                  </div>
                  <div className="stats-people__meta">
                    <strong>{row.completedTasks}</strong>
                    <span>tasks</span>
                  </div>
                  <div className="stats-people__meta">
                    <strong>{row.reviews}</strong>
                    <span>reviews</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>

      <p className="stats-footnote">
        Aggregated {relativeDate(new Date().toISOString())} · range {stats.range} · database is the source of truth.
      </p>
    </div>
  )
}
