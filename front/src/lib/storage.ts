import type { User } from './types'

const TOKEN_KEY = 'rudy.token.v1'
const USER_KEY = 'rudy.user.v1'

export const SIGN_OUT_EVENT = 'rudy:signout'

function read(key: string): string | null {
  try {
    return window.localStorage.getItem(key)
  } catch {
    return null
  }
}

function write(key: string, value: string | null) {
  try {
    if (value === null) window.localStorage.removeItem(key)
    else window.localStorage.setItem(key, value)
  } catch {
    // Storage can be unavailable in private browsing; the session simply
    // lasts for the current page in that case.
  }
}

export function getToken(): string | null {
  return read(TOKEN_KEY)
}

export function setToken(token: string) {
  write(TOKEN_KEY, token)
}

export function readUser(): User | null {
  const raw = read(USER_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as User
  } catch {
    return null
  }
}

export function writeUser(user: User | null) {
  write(USER_KEY, user === null ? null : JSON.stringify(user))
}

export function clearStoredSession() {
  write(TOKEN_KEY, null)
  write(USER_KEY, null)
}

/** Announce that a request proved the session invalid so the UI can reset. */
export function notifySignOut() {
  window.dispatchEvent(new Event(SIGN_OUT_EVENT))
}
