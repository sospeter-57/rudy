import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useEffect, useMemo } from 'react'
import {
  FolderKanban,
  BarChart3,
  LayoutDashboard,
  ListChecks,
  LogOut,
  Plus,
  Settings,
  ShieldCheck,
  Sparkles,
  UserRound,
} from 'lucide-react'
import { api } from '@/lib/api'
import { cx } from '@/lib/cx'
import { useMe, usePreferences } from '@/lib/hooks'
import { tierFor, tierProgress } from '@/lib/points'
import { useSession } from '@/lib/session'
import { applyTheme, watchSystemTheme } from '@/lib/theme'
import { Avatar, Button } from '@/components/ui'

const NAV = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/projects', label: 'Projects', icon: FolderKanban },
  { to: '/tasks', label: 'Tasks', icon: ListChecks },
  { to: '/review', label: 'Review', icon: ShieldCheck },
  { to: '/stats', label: 'Stats', icon: BarChart3 },
  { to: '/profile', label: 'Profile', icon: UserRound },
  { to: '/settings', label: 'Settings', icon: Settings },
]

function useCrumbs(): string[] {
  const { pathname } = useLocation()
  if (pathname.startsWith('/projects/')) return ['Projects', 'Project']
  if (pathname.startsWith('/tasks/')) return ['Tasks', 'Task']
  if (pathname.startsWith('/projects')) return ['Projects']
  if (pathname.startsWith('/tasks')) return ['Tasks']
  if (pathname.startsWith('/review')) return ['Review']
  if (pathname.startsWith('/stats')) return ['Stats']
  if (pathname.startsWith('/profile')) return ['Profile']
  if (pathname.startsWith('/settings')) return ['Settings']
  return ['Dashboard']
}

function PointsChip({ points, compact }: { points: number; compact?: boolean }) {
  const tier = tierFor(points)
  return (
    <Link to="/profile" className={cx('points-chip', compact && 'points-chip--compact')} title={`${tier.name} · ${points} points`}>
      <Sparkles size={14} />
      <span className="points-chip__value">{points}</span>
      {!compact ? <span className="points-chip__tier">{tier.name}</span> : null}
    </Link>
  )
}

export function SideRail() {
  const user = useSession((state) => state.user)
  const signOut = useSession((state) => state.signOut)
  const navigate = useNavigate()
  const { data: me } = useMe()
  const current = me ?? user

  const progress = useMemo(() => tierProgress(current?.points ?? 0), [current?.points])

  if (!current) return null

  return (
    <aside className="rail">
      <Link to="/dashboard" className="rail__brand" aria-label="Rudy home">
        <span className="rail__mark">R</span>
        <span className="rail__word">RUDY</span>
      </Link>

      <nav className="rail__nav" aria-label="Primary">
        {NAV.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) => cx('rail__link', isActive && 'rail__link--active')}
          >
            <Icon size={17} strokeWidth={1.6} />
            <span>{label}</span>
          </NavLink>
        ))}
      </nav>

      <div className="rail__foot">
        <div className="rail__tier">
          <div className="rail__tier-head">
            <span className="eyebrow">{progress.current.name}</span>
            <span className="rail__tier-points">{current.points} pts</span>
          </div>
          <div className="rail__tier-track">
            <span style={{ width: `${progress.pct}%` }} />
          </div>
          <span className="rail__tier-hint">
            {progress.next ? `${progress.remaining} pts to ${progress.next.name}` : 'Top tier reached'}
          </span>
        </div>
        <button
          className="rail__signout"
          onClick={async () => {
            try {
              await api.logout()
            } catch {
              // The local session is cleared either way.
            }
            signOut()
            navigate('/')
          }}
        >
          <LogOut size={15} />
          <span>Sign out</span>
        </button>
      </div>
    </aside>
  )
}

export function TopBar() {
  const user = useSession((state) => state.user)
  const { data: me } = useMe()
  const current = me ?? user
  const crumbs = useCrumbs()
  const navigate = useNavigate()

  if (!current) return null

  return (
    <header className="topbar">
      <div className="topbar__crumbs">
        {crumbs.map((crumb, index) => (
          <span key={`${crumb}-${index}`} className={cx('topbar__crumb', index === crumbs.length - 1 && 'topbar__crumb--last')}>
            {crumb}
            {index < crumbs.length - 1 ? <span className="topbar__sep">/</span> : null}
          </span>
        ))}
      </div>

      <div className="topbar__actions">
        <Button size="sm" variant="outline" onClick={() => navigate('/tasks?new=1')}>
          <Plus size={15} />
          <span>New task</span>
        </Button>
        <PointsChip points={current.points} compact />
        <Link to="/profile" className="topbar__profile">
          <Avatar name={current.username} size="sm" />
          <span className="topbar__name">
            {current.username}
            <small>{current.role}</small>
          </span>
        </Link>
      </div>
    </header>
  )
}

export function MobileNav() {
  return (
    <nav className="mobile-nav" aria-label="Primary mobile">
      {NAV.map(({ to, label, icon: Icon }) => (
        <NavLink key={to} to={to} className={({ isActive }) => cx('mobile-nav__link', isActive && 'mobile-nav__link--active')}>
          <Icon size={18} strokeWidth={1.6} />
          <span>{label}</span>
        </NavLink>
      ))}
    </nav>
  )
}

function ThemeSync() {
  const { data } = usePreferences()

  useEffect(() => {
    if (!data) return
    applyTheme(data)
    return watchSystemTheme(data, () => applyTheme(data))
  }, [data])

  return null
}

export function AppShell() {
  return (
    <div className="shell">
      <ThemeSync />
      <SideRail />
      <div className="shell__main">
        <TopBar />
        <main className="page">
          <Outlet />
        </main>
        <MobileNav />
      </div>
    </div>
  )
}

export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
  meta,
}: {
  eyebrow?: string
  title: React.ReactNode
  description?: React.ReactNode
  actions?: React.ReactNode
  meta?: React.ReactNode
}) {
  return (
    <header className="page-head">
      <div className="page-head__main">
        {eyebrow ? <span className="eyebrow">{eyebrow}</span> : null}
        <h1 className="page-head__title">{title}</h1>
        {description ? <p className="page-head__desc">{description}</p> : null}
        {meta ? <div className="page-head__meta">{meta}</div> : null}
      </div>
      {actions ? <div className="page-head__actions">{actions}</div> : null}
    </header>
  )
}
