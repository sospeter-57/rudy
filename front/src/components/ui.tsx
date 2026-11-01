import { createPortal } from 'react-dom'
import { useEffect, useId, type ReactNode } from 'react'
import { X } from 'lucide-react'
import { cx } from '@/lib/cx'
import { initials } from '@/lib/format'
import { useToasts } from '@/lib/toast'

type ButtonVariant = 'primary' | 'danger' | 'outline' | 'ghost' | 'quiet'
type ButtonSize = 'sm' | 'md' | 'lg'

export function Button({
  variant = 'primary',
  size = 'md',
  className,
  children,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant
  size?: ButtonSize
}) {
  return (
    <button className={cx('btn', `btn--${variant}`, `btn--${size}`, className)} {...props}>
      {children}
    </button>
  )
}

export function Field({
  label,
  hint,
  error,
  children,
  htmlFor,
}: {
  label: string
  hint?: string
  error?: string
  children: ReactNode
  htmlFor?: string
}) {
  return (
    <label className="field" htmlFor={htmlFor}>
      <span className="field__label">
        {label}
        {hint ? <span className="field__hint">{hint}</span> : null}
      </span>
      {children}
      {error ? <span className="field__error">{error}</span> : null}
    </label>
  )
}

export function Input({ className, ...props }: React.InputHTMLAttributes<HTMLInputElement>) {
  return <input className={cx('input', className)} {...props} />
}

export function Textarea({ className, ...props }: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={cx('input', 'textarea', className)} {...props} />
}

export function Select({ className, children, ...props }: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select className={cx('input', 'select', className)} {...props}>
      {children}
    </select>
  )
}

type Tone = 'gold' | 'red' | 'bone' | 'muted' | 'green'

export function Badge({ tone = 'muted', children, className }: { tone?: Tone; children: ReactNode; className?: string }) {
  return <span className={cx('badge', `badge--${tone}`, className)}>{children}</span>
}

export function Avatar({ name, size = 'md', tone }: { name: string; size?: 'sm' | 'md' | 'lg' | 'xl'; tone?: Tone }) {
  const tones: Tone[] = ['gold', 'red', 'bone', 'green']
  const picked = tone ?? tones[name.length % tones.length]
  return (
    <span className={cx('avatar', `avatar--${size}`, `avatar--${picked}`)} aria-hidden="true">
      {initials(name)}
    </span>
  )
}

export function Progress({ value, tone = 'gold', label }: { value: number; tone?: Tone; label?: string }) {
  const pct = Math.max(0, Math.min(100, value))
  return (
    <div className={cx('progress', `progress--${tone}`)} role="progressbar" aria-valuenow={Math.round(pct)} aria-valuemin={0} aria-valuemax={100} aria-label={label}>
      <span className="progress__fill" style={{ width: `${pct}%` }} />
    </div>
  )
}

export function Stat({
  label,
  value,
  hint,
  tone = 'bone',
}: {
  label: string
  value: ReactNode
  hint?: ReactNode
  tone?: 'bone' | 'gold' | 'red'
}) {
  return (
    <div className={cx('stat', `stat--${tone}`)}>
      <span className="stat__label">{label}</span>
      <span className="stat__value">{value}</span>
      {hint ? <span className="stat__hint">{hint}</span> : null}
    </div>
  )
}

export function EmptyState({
  icon,
  title,
  body,
  action,
}: {
  icon?: ReactNode
  title: string
  body?: string
  action?: ReactNode
}) {
  return (
    <div className="empty">
      {icon ? <div className="empty__icon">{icon}</div> : null}
      <h3 className="empty__title">{title}</h3>
      {body ? <p className="empty__body">{body}</p> : null}
      {action ? <div className="empty__action">{action}</div> : null}
    </div>
  )
}

export function Spinner({ label = 'Loading' }: { label?: string }) {
  return (
    <span className="spinner" role="status" aria-label={label}>
      <span />
    </span>
  )
}

export function Dialog({
  open,
  onClose,
  title,
  eyebrow,
  children,
  footer,
  wide,
}: {
  open: boolean
  onClose: () => void
  title: string
  eyebrow?: string
  children: ReactNode
  footer?: ReactNode
  wide?: boolean
}) {
  const titleId = useId()

  useEffect(() => {
    if (!open) return
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = previous
    }
  }, [open, onClose])

  if (!open) return null

  return createPortal(
    <div className="dialog" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <div className={cx('dialog__panel', wide && 'dialog__panel--wide')} role="dialog" aria-modal="true" aria-labelledby={titleId}>
        <header className="dialog__head">
          <div>
            {eyebrow ? <span className="eyebrow">{eyebrow}</span> : null}
            <h2 id={titleId} className="dialog__title">
              {title}
            </h2>
          </div>
          <button className="icon-btn" onClick={onClose} aria-label="Close dialog">
            <X size={18} />
          </button>
        </header>
        <div className="dialog__body">{children}</div>
        {footer ? <footer className="dialog__foot">{footer}</footer> : null}
      </div>
    </div>,
    document.body,
  )
}

export function Toaster() {
  const toasts = useToasts((state) => state.toasts)
  const dismiss = useToasts((state) => state.dismiss)

  if (toasts.length === 0) return null

  return createPortal(
    <div className="toasts" aria-live="polite">
      {toasts.map((item) => (
        <div key={item.id} className={cx('toast', `toast--${item.tone}`)}>
          <div>
            <strong>{item.title}</strong>
            {item.detail ? <p>{item.detail}</p> : null}
          </div>
          <button className="icon-btn" onClick={() => dismiss(item.id)} aria-label="Dismiss">
            <X size={14} />
          </button>
        </div>
      ))}
    </div>,
    document.body,
  )
}
