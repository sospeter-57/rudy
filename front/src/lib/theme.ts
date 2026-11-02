import type { Preferences } from './types'

export const DEFAULT_PREFERENCES: Preferences = {
  themeMode: 'dark',
  accent: '#d4a72c',
  blendA: '#e01e37',
  blendB: '#f0c75e',
}

function resolvedTheme(mode: Preferences['themeMode']): 'dark' | 'light' {
  if (mode !== 'system') return mode
  if (typeof window === 'undefined') return 'dark'
  return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

/**
 * Push preferences into CSS custom properties so every screen follows the
 * selected palette. The backend remains the source of truth for the values.
 */
export function applyTheme(preferences: Preferences) {
  if (typeof document === 'undefined') return
  const root = document.documentElement
  root.dataset.theme = resolvedTheme(preferences.themeMode)
  root.style.setProperty('--gold', preferences.accent)
  root.style.setProperty('--red', preferences.blendA)
  root.style.setProperty('--gold-bright', preferences.blendB)
}

export function watchSystemTheme(preferences: Preferences, onChange: () => void) {
  if (preferences.themeMode !== 'system' || typeof window === 'undefined') return () => {}
  const media = window.matchMedia('(prefers-color-scheme: light)')
  media.addEventListener('change', onChange)
  return () => media.removeEventListener('change', onChange)
}
