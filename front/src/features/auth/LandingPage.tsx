import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArrowRight } from 'lucide-react'
import { api } from '@/lib/api'
import { ROLES, type Role } from '@/lib/types'
import { useSession } from '@/lib/session'
import { Button, Field, Input, Select } from '@/components/ui'
import { cx } from '@/lib/cx'

type Mode = 'login' | 'register'

export function LandingPage() {
  const user = useSession((state) => state.user)
  const setSession = useSession((state) => state.setSession)
  const navigate = useNavigate()

  const [mode, setMode] = useState<Mode>('login')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<Role>('Frontend Engineer')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (user) navigate('/dashboard', { replace: true })
  }, [user, navigate])

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    setError(null)
    setBusy(true)
    try {
      const next = mode === 'login'
        ? await api.login(username, password)
        : await api.register({ username, password, role })
      setSession(next.user, next.token)
      navigate('/dashboard', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="auth">
      <section className="auth__hero">
        <div className="auth__hero-top">
          <span className="auth__mark">R</span>
          <span className="eyebrow">Project command deck</span>
        </div>

        <div className="auth__hero-main">
          <h1 className="auth__word">RUDY</h1>
          <p className="auth__tagline">
            Build <em>Responsibly</em> Purposefully
          </p>
          <p className="auth__copy">
            Propose the work. A manager approves what is fit to build. Ship it, get reviewed,
            and watch your profile grow. Every completed task mints points — for the maker and
            the reviewers who signed off.
          </p>

          <div className="auth__flow">
            <div className="auth__flow-step">
              <span>01</span>
              <div>
                <strong>Propose</strong>
                <p>Anyone can put a task on the board.</p>
              </div>
            </div>
            <div className="auth__flow-step">
              <span>02</span>
              <div>
                <strong>Approve</strong>
                <p>A manager decides if it is fit to be worked on.</p>
              </div>
            </div>
            <div className="auth__flow-step">
              <span>03</span>
              <div>
                <strong>Earn</strong>
                <p>Complete it, clear review, and bank the points.</p>
              </div>
            </div>
          </div>
        </div>

        <div className="auth__hero-foot">
          <div className="auth__palette" aria-hidden="true">
            <span className="swatch swatch--ink" />
            <span className="swatch swatch--red" />
            <span className="swatch swatch--gold" />
            <span className="swatch swatch--bone" />
          </div>
          <span>Black / Red / Gold / Bone</span>
          <span className="auth__foot-dot">·</span>
          <span>Points are minted once, on completion</span>
        </div>
      </section>

      <section className="auth__panel">
        <div className="auth__panel-inner">
          <div className="auth__tabs" role="tablist">
            <button
              role="tab"
              aria-selected={mode === 'login'}
              className={cx('auth__tab', mode === 'login' && 'is-active')}
              onClick={() => { setMode('login'); setError(null) }}
            >
              Sign in
            </button>
            <button
              role="tab"
              aria-selected={mode === 'register'}
              className={cx('auth__tab', mode === 'register' && 'is-active')}
              onClick={() => { setMode('register'); setError(null) }}
            >
              Create profile
            </button>
          </div>

          <div className="auth__intro">
            <span className="eyebrow">{mode === 'login' ? 'Welcome back' : 'One profile type'}</span>
            <h2>{mode === 'login' ? 'Pick up the thread.' : 'Make your name.'}</h2>
            <p>
              {mode === 'login'
                ? 'Your points, your tasks, your review queue — exactly where you left them.'
                : 'Everyone joins the same way. Your role decides what you can unlock.'}
            </p>
          </div>

          <form className="auth__form" onSubmit={submit}>
            <Field label="Username" htmlFor="username">
              <Input
                id="username"
                autoComplete="username"
                placeholder="e.g. nova"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                required
              />
            </Field>

            <Field label="Password" hint={mode === 'register' ? '8+ characters' : undefined} htmlFor="password">
              <Input
                id="password"
                type="password"
                autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
                placeholder="••••••••"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                required
              />
            </Field>

            {mode === 'register' ? (
              <Field label="Role" hint="This decides your permissions" htmlFor="role">
                <Select id="role" value={role} onChange={(event) => setRole(event.target.value as Role)}>
                  {ROLES.map((option) => (
                    <option key={option} value={option}>
                      {option}
                    </option>
                  ))}
                </Select>
              </Field>
            ) : null}

            {error ? <p className="auth__error">{error}</p> : null}

            <Button type="submit" size="lg" disabled={busy} className="auth__submit">
              {busy ? 'Working…' : mode === 'login' ? 'Enter the deck' : 'Create profile'}
              {!busy ? <ArrowRight size={16} /> : null}
            </Button>
          </form>
        </div>
      </section>
    </div>
  )
}
