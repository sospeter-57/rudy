import { useEffect, useState } from 'react'
import { Check, Monitor, Moon, Palette, Save, Sun } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { usePreferences } from '@/lib/hooks'
import { applyTheme, DEFAULT_PREFERENCES } from '@/lib/theme'
import { toast } from '@/lib/toast'
import { PageHeader } from '@/components/layout'
import { Button, Field, Spinner } from '@/components/ui'
import { cx } from '@/lib/cx'
import type { Preferences, ThemeMode } from '@/lib/types'

const THEME_OPTIONS: Array<{ value: ThemeMode; label: string; hint: string; icon: typeof Moon }> = [
  { value: 'dark', label: 'Dark', hint: 'Ink and gold', icon: Moon },
  { value: 'light', label: 'Light', hint: 'Paper and ink', icon: Sun },
  { value: 'system', label: 'System', hint: 'Follow the OS', icon: Monitor },
]

const PRESETS: Array<{ name: string; accent: string; blendA: string; blendB: string }> = [
  { name: 'Ember', accent: '#d4a72c', blendA: '#e01e37', blendB: '#f0c75e' },
  { name: 'Jade', accent: '#3fae7c', blendA: '#1f6feb', blendB: '#7ee2b8' },
  { name: 'Violet', accent: '#a78bfa', blendA: '#ec4899', blendB: '#c4b5fd' },
  { name: 'Steel', accent: '#94a3b8', blendA: '#38bdf8', blendB: '#e2e8f0' },
]

export function SettingsPage() {
  const queryClient = useQueryClient()
  const { data, isLoading } = usePreferences()
  const [draft, setDraft] = useState<Preferences>(DEFAULT_PREFERENCES)

  useEffect(() => {
    if (data) setDraft(data)
  }, [data])

  useEffect(() => {
    applyTheme(draft)
  }, [draft])

  const mutation = useMutation({
    mutationFn: () =>
      api.updatePreferences({
        themeMode: draft.themeMode,
        accent: draft.accent,
        blendA: draft.blendA,
        blendB: draft.blendB,
      }),
    onSuccess: (preferences) => {
      queryClient.setQueryData(['preferences'], preferences)
      applyTheme(preferences)
      toast('Appearance saved', 'Your palette follows you on every device.', 'gold')
    },
    onError: (err) => toast('Could not save', err instanceof Error ? err.message : 'Try again.', 'red'),
  })

  if (isLoading) {
    return <div className="loading-pane"><Spinner label="Loading settings" /><span>Reading your preferences…</span></div>
  }

  return (
    <div className="stack">
      <PageHeader
        eyebrow="Settings"
        title={<>Make it <em>yours</em>.</>}
        description="Appearance is stored on your account, so your palette and theme follow you between devices."
        meta={<span className="meta-quiet">Preferences are saved by the Go API.</span>}
      />

      <div className="settings-grid">
        <section className="panel">
          <header className="panel__head">
            <div>
              <span className="eyebrow">Theme</span>
              <h2 className="panel__title">Light or dark</h2>
            </div>
            <Palette size={18} className="panel__icon" />
          </header>

          <div className="theme-options">
            {THEME_OPTIONS.map(({ value, label, hint, icon: Icon }) => (
              <button
                key={value}
                type="button"
                className={cx('theme-option', draft.themeMode === value && 'is-active')}
                onClick={() => setDraft((current) => ({ ...current, themeMode: value }))}
              >
                <Icon size={18} />
                <span>
                  <strong>{label}</strong>
                  <small>{hint}</small>
                </span>
                {draft.themeMode === value ? <Check size={15} /> : null}
              </button>
            ))}
          </div>

          <div className="settings-section">
            <span className="eyebrow">Palette</span>
            <div className="preset-row">
              {PRESETS.map((preset) => {
                const active =
                  draft.accent === preset.accent &&
                  draft.blendA === preset.blendA &&
                  draft.blendB === preset.blendB
                return (
                  <button
                    key={preset.name}
                    type="button"
                    className={cx('preset', active && 'is-active')}
                    onClick={() =>
                      setDraft((current) => ({
                        ...current,
                        accent: preset.accent,
                        blendA: preset.blendA,
                        blendB: preset.blendB,
                      }))
                    }
                  >
                    <span className="preset__swatches" aria-hidden="true">
                      <i style={{ background: preset.accent }} />
                      <i style={{ background: preset.blendA }} />
                      <i style={{ background: preset.blendB }} />
                    </span>
                    <span>{preset.name}</span>
                  </button>
                )
              })}
            </div>
          </div>

          <div className="settings-section form-grid">
            <Field label="Accent" hint="Highlights and progress" htmlFor="accent">
              <div className="color-input">
                <input
                  id="accent"
                  type="color"
                  value={draft.accent}
                  onChange={(e) => setDraft((current) => ({ ...current, accent: e.target.value }))}
                />
                <code>{draft.accent}</code>
              </div>
            </Field>
            <Field label="Signal" hint="Danger and alerts" htmlFor="blend-a">
              <div className="color-input">
                <input
                  id="blend-a"
                  type="color"
                  value={draft.blendA}
                  onChange={(e) => setDraft((current) => ({ ...current, blendA: e.target.value }))}
                />
                <code>{draft.blendA}</code>
              </div>
            </Field>
            <Field label="Bright accent" hint="Hover and emphasis" htmlFor="blend-b">
              <div className="color-input">
                <input
                  id="blend-b"
                  type="color"
                  value={draft.blendB}
                  onChange={(e) => setDraft((current) => ({ ...current, blendB: e.target.value }))}
                />
                <code>{draft.blendB}</code>
              </div>
            </Field>
          </div>

          <div className="settings-actions">
            <Button onClick={() => mutation.mutate()} disabled={mutation.isPending}>
              <Save size={15} /> {mutation.isPending ? 'Saving…' : 'Save appearance'}
            </Button>
            <Button
              variant="quiet"
              onClick={() => setDraft((current) => ({ ...DEFAULT_PREFERENCES, themeMode: current.themeMode }))}
              disabled={mutation.isPending}
            >
              Reset colours
            </Button>
          </div>
        </section>

        <aside className="panel panel--tight">
          <header className="panel__head">
            <div>
              <span className="eyebrow">Preview</span>
              <h2 className="panel__title">The deck at a glance</h2>
            </div>
          </header>
          <div className="theme-preview">
            <div className="theme-preview__bar">
              <span />
              <span />
              <span />
            </div>
            <div className="theme-preview__body">
              <span className="eyebrow">Command deck</span>
              <h3>Build Responsibly Purposefully</h3>
              <p>
                Propose the work, clear approval, ship it and collect. This preview uses the same
                tokens as every screen.
              </p>
              <div className="theme-preview__chips">
                <span className="theme-preview__chip theme-preview__chip--gold">Approved</span>
                <span className="theme-preview__chip theme-preview__chip--red">Changes</span>
                <span className="theme-preview__chip">Pending</span>
              </div>
              <div className="theme-preview__progress"><span /></div>
              <div className="theme-preview__stats">
                <div><strong>250</strong><span>points</span></div>
                <div><strong>12</strong><span>tasks</span></div>
                <div><strong>4</strong><span>reviews</span></div>
              </div>
            </div>
          </div>
          <p className="settings-footnote">
            Themes apply instantly as a preview. Save to persist them to your account.
          </p>
        </aside>
      </div>
    </div>
  )
}
