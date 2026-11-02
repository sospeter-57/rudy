import { useQuery, useQueryClient } from '@tanstack/react-query'
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

// ---------- projects and chat ----------

export function useProjectMembers(projectId: number) {
  return useQuery({
    queryKey: ['project-members', projectId],
    queryFn: () => api.listProjectMembers(projectId),
    enabled: projectId > 0,
  })
}

export function useProjectChats(projectId: number, enabled = true) {
  return useQuery({
    queryKey: ['project-chats', projectId],
    queryFn: () => api.listProjectChats(projectId),
    enabled: projectId > 0 && enabled,
    retry: false,
  })
}

export function useChats() {
  return useQuery({ queryKey: ['chats'], queryFn: () => api.listChats() })
}

export function useChat(roomId: number) {
  return useQuery({
    queryKey: ['chat', roomId],
    queryFn: () => api.getChat(roomId),
    enabled: roomId > 0,
  })
}

export function useChatMessages(roomId: number) {
  return useQuery({
    queryKey: ['chat-messages', roomId],
    queryFn: () => api.listChatMessages(roomId, undefined, 50),
    enabled: roomId > 0,
  })
}

export function useDiscoverChats(enabled = true) {
  return useQuery({
    queryKey: ['chat-discover'],
    queryFn: () => api.discoverChats(),
    enabled,
  })
}

export function useChatRequests() {
  return useQuery({ queryKey: ['chat-requests'], queryFn: () => api.listChatRequests() })
}

export function useChatUnread() {
  return useQuery({
    queryKey: ['chat-unread'],
    queryFn: () => api.chatUnread(),
    refetchInterval: 30_000,
  })
}

function eventRoomID(data: unknown): number | null {
  if (!data || typeof data !== 'object') return null
  const record = data as Record<string, unknown>
  if (typeof record.roomId === 'number') return record.roomId
  if (typeof record.id === 'number') return record.id
  return null
}

/**
 * Subscribes to the live chat stream and refreshes the matching queries. The
 * stream is re-established after a dropped connection; a refetch on reconnect
 * keeps a client that slept through events in sync.
 */
export function useChatStream() {
  const user = useSession((state) => state.user)
  const queryClient = useQueryClient()

  useEffect(() => {
    if (!user) return

    const controller = new AbortController()
    let retry: ReturnType<typeof setTimeout> | undefined
    let closed = false

    const refresh = (roomId: number | null) => {
      void queryClient.invalidateQueries({ queryKey: ['chats'] })
      void queryClient.invalidateQueries({ queryKey: ['chat-unread'] })
      void queryClient.invalidateQueries({ queryKey: ['chat-requests'] })
      if (roomId) {
        void queryClient.invalidateQueries({ queryKey: ['chat', roomId] })
        void queryClient.invalidateQueries({ queryKey: ['chat-messages', roomId] })
      }
    }

    const connect = async () => {
      try {
        await api.streamChatEvents((event) => {
          refresh(eventRoomID(event.data))
        }, controller.signal)
      } catch {
        // Aborts are expected on unmount; anything else retries.
      }
      if (closed) return
      refresh(null)
      retry = setTimeout(() => void connect(), 3_000)
    }

    void connect()

    return () => {
      closed = true
      controller.abort()
      if (retry) clearTimeout(retry)
    }
  }, [user, queryClient])
}
