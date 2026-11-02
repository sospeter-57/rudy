import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useEffect, useMemo, useState } from 'react'
import {
  FolderKanban,
  BarChart3,
  LayoutDashboard,
  ListChecks,
  LogOut,
  MessageSquare,
  Plus,
  Settings,
  ShieldCheck,
  Sparkles,
  UserRound,
  X,
} from 'lucide-react'
import { api } from '@/lib/api'
import { cx } from '@/lib/cx'
import { useChatStream, useChatUnread, useMe, usePreferences } from '@/lib/hooks'
import { tierFor, tierProgress } from '@/lib/points'
import { useSession } from '@/lib/session'
import { applyTheme, watchSystemTheme } from '@/lib/theme'
import { Avatar, Button } from '@/components/ui'

const NAV = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/projects', label: 'Projects', icon: FolderKanban },
  { to: '/tasks', label: 'Tasks', icon: ListChecks },
  { to: '/chat', label: 'Chat', icon: MessageSquare },
  { to: '/review', label: 'Review', icon: ShieldCheck },
  { to: '/stats', label: 'Stats', icon: BarChart3 },
  { to: '/profile', label: 'Profile', icon: UserRound },
  { to: '/settings', label: 'Settings', icon: Settings },
]

function useCrumbs(): string[] {
  const { pathname } = useLocation()
  if (pathname.startsWith('/projects/')) return ['Projects', 'Project']
  if (pathname.startsWith('/tasks/')) return ['Tasks', 'Task']
  if (pathname.startsWith('/chat')) return ['Chat']
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
  const { data: unread = 0 } = useChatUnread()
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
            {to === '/chat' && unread > 0 ? <span className="rail__badge">{unread > 99 ? '99+' : unread}</span> : null}
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

export function TopBar({ onMenuOpen }: { onMenuOpen?: () => void }) {
  const user = useSession((state) => state.user)
  const { data: me } = useMe()
  const current = me ?? user
  const crumbs = useCrumbs()
  const navigate = useNavigate()

  if (!current) return null

  return (
    <header className="topbar">
      <button
        type="button"
        className="mobile-menu-button"
        onClick={onMenuOpen}
        aria-label="Open menu"
      >
        <span />
        <span />
        <span />
      </button>
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

export function MobileDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { data: unread = 0 } = useChatUnread()
  const location = useLocation()
  const navigate = useNavigate()
  const signOut = useSession((state) => state.signOut)

  useEffect(() => {
    if (!open) return
    function handleKey(event: KeyboardEvent) {
      if (event.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [open, onClose])

  useEffect(() => {
    if (open) onClose()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.pathname])

  if (!open) return null

  return (
    <div className="mobile-drawer" role="dialog" aria-modal="true" aria-label="Primary navigation">
      <button type="button" className="mobile-drawer__backdrop" onClick={onClose} aria-label="Close menu" />
      <nav className="mobile-drawer__panel">
        <div className="mobile-drawer__head">
          <span className="rail__mark">R</span>
          <span className="mobile-drawer__title">RUDY</span>
          <button type="button" className="mobile-drawer__close" onClick={onClose} aria-label="Close menu">
            <X size={18} />
          </button>
        </div>
        <div className="mobile-drawer__links">
          {NAV.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              onClick={onClose}
              className={({ isActive }) => cx('mobile-drawer__link', isActive && 'mobile-drawer__link--active')}
            >
              <Icon size={18} strokeWidth={1.6} />
              <span>{label}</span>
              {to === '/chat' && unread > 0 ? <span className="mobile-nav__badge">{unread > 9 ? '9+' : unread}</span> : null}
            </NavLink>
          ))}
        </div>
        <div className="mobile-drawer__foot">
          <button
            type="button"
            className="mobile-drawer__signout"
            onClick={async () => {
              try {
                await api.logout()
              } catch {
                // Local session is cleared either way.
              }
              signOut()
              navigate('/')
              onClose()
            }}
          >
            Sign out
          </button>
        </div>
      </nav>
    </div>
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
  const location = useLocation()
  const [mobileOpen, setMobileOpen] = useState(false)

  useChatStream()

  useEffect(() => {
    setMobileOpen(false)
  }, [location.pathname])

  return (
    <div className="shell">
      <ThemeSync />
      <SideRail />
      <div className="shell__main">
        <TopBar onMenuOpen={() => setMobileOpen(true)} />
        <main className="page">
          <Outlet />
        </main>
        <MobileDrawer open={mobileOpen} onClose={() => setMobileOpen(false)} />
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
