import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { api } from './api'
import { useSession } from './session'
import type { StatsRange, Task } from './types'

export function useMe() {
  const user = useSession((state) => state.user)
  const refreshUser = useSession((state) => state.refreshUser)
  const query = useQuery({
    queryKey: ['me', user?.id],
    queryFn: () => api.me(),
    enabled: !!user,
    staleTime: 5_000,
  })

  useEffect(() => {
    if (query.data && user && query.data.points !== user.points) {
      refreshUser(query.data)
    }
  }, [query.data, user, refreshUser])

  return query
}

export function useProjects() {
  return useQuery({ queryKey: ['projects'], queryFn: () => api.listProjects() })
}

export function useProject(id: number) {
  return useQuery({ queryKey: ['project', id], queryFn: () => api.getProject(id), enabled: id > 0 })
}

export function useTasks(filter: {
  projectId?: number
  assigneeId?: number
  reviewerId?: number
  state?: Task['taskState']
}) {
  return useQuery({
    queryKey: ['tasks', filter],
    queryFn: () => api.listTasks(filter),
  })
}

export function useTask(id: number) {
  return useQuery({ queryKey: ['task', id], queryFn: () => api.getTask(id), enabled: id > 0 })
}

export function useUsers() {
  return useQuery({ queryKey: ['users'], queryFn: () => api.listUsers() })
}

export function useLedger(userId: number) {
  return useQuery({ queryKey: ['ledger', userId], queryFn: () => api.getLedger(userId), enabled: userId > 0 })
}

export function useLeaderboard() {
  return useQuery({ queryKey: ['leaderboard'], queryFn: () => api.leaderboard() })
}

export function usePreferences() {
  return useQuery({
    queryKey: ['preferences'],
    queryFn: () => api.getPreferences(),
    staleTime: 60_000,
  })
}

export function useStats(range: StatsRange) {
  return useQuery({
    queryKey: ['stats', range],
    queryFn: () => api.getStats(range),
    staleTime: 15_000,
  })
}
