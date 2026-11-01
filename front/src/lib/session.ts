import { create } from 'zustand'
import { session } from './store'
import type { User } from './types'

interface SessionState {
  user: User | null
  ready: boolean
  setUser: (user: User | null) => void
  refreshUser: (user: User) => void
  hydrate: () => void
  signOut: () => void
}

export const useSession = create<SessionState>((set) => ({
  user: null,
  ready: false,
  setUser: (user) => {
    session.set(user)
    set({ user })
  },
  refreshUser: (user) => {
    session.set(user)
    set({ user })
  },
  hydrate: () => {
    set({ user: session.get(), ready: true })
  },
  signOut: () => {
    session.set(null)
    set({ user: null })
  },
}))

export function useCurrentUser(): User | null {
  return useSession((state) => state.user)
}
