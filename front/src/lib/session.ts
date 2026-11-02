import { create } from 'zustand'
import {
  SIGN_OUT_EVENT,
  clearStoredSession,
  readUser,
  setToken,
  writeUser,
} from './storage'
import type { User } from './types'

interface SessionState {
  user: User | null
  ready: boolean
  setSession: (user: User, token: string) => void
  setUser: (user: User | null) => void
  refreshUser: (user: User) => void
  hydrate: () => void
  signOut: () => void
}

export const useSession = create<SessionState>((set) => ({
  user: null,
  ready: false,
  setSession: (user, token) => {
    setToken(token)
    writeUser(user)
    set({ user, ready: true })
  },
  setUser: (user) => {
    writeUser(user)
    set({ user })
  },
  refreshUser: (user) => {
    writeUser(user)
    set({ user })
  },
  hydrate: () => {
    set({ user: readUser(), ready: true })
  },
  signOut: () => {
    clearStoredSession()
    set({ user: null })
  },
}))

if (typeof window !== 'undefined') {
  window.addEventListener(SIGN_OUT_EVENT, () => {
    useSession.getState().signOut()
  })
}

export function useCurrentUser(): User | null {
  return useSession((state) => state.user)
}
