import {
  ApiError,
  type AuthSession,
  type ChatAttachment,
  type ChatMessage,
  type ChatRequest,
  type ChatRoom,
  type LeaderboardRow,
  type LedgerEntry,
  type OpenRoomResult,
  type Preferences,
  type PreferencesInput,
  type Project,
  type ProjectDraft,
  type ProjectMember,
  type RegisterDraft,
  type Role,
  type RoomVisibility,
  type Stats,
  type StatsRange,
  type Task,
  type TaskDraft,
  type User,
} from './types'
import { config } from './config'
import { clearStoredSession, getToken, notifySignOut } from './storage'

export interface TaskFilter {
  projectId?: number
  assigneeId?: number
  reviewerId?: number
  state?: Task['taskState']
}

export interface CompletionResult {
  task: Task
  awards: LedgerEntry[]
}

/** One server-sent chat event, as delivered by `GET /chat/events`. */
export interface ChatStreamEvent {
  type: string
  data: unknown
}

export interface OpenRoomDraft {
  name: string
  purpose: string
  visibility: RoomVisibility
  inviteIds?: number[]
}

export interface ProjectRoomDraft {
  name: string
  purpose: string
  memberIds?: number[]
}

/**
 * The contract the frontend is written against. The real Go service is the only
 * implementation: every call goes through the bearer-token session.
 */
export interface ApiClient {
  login(username: string, password: string): Promise<AuthSession>
  register(draft: RegisterDraft): Promise<AuthSession>
  googleLogin(credential: string, role?: Role): Promise<AuthSession>
  logout(): Promise<void>
  me(): Promise<User>
  listUsers(): Promise<User[]>
  listProjects(): Promise<Project[]>
  getProject(id: number): Promise<Project>
  createProject(draft: ProjectDraft): Promise<Project>
  listTasks(filter?: TaskFilter): Promise<Task[]>
  getTask(id: number): Promise<Task>
  createTask(projectId: number, draft: TaskDraft): Promise<Task>
  approveTask(taskId: number): Promise<Task>
  rejectTask(taskId: number, reason: string): Promise<Task>
  assignTask(taskId: number, assigneeId: number): Promise<Task>
  setGate(taskId: number, gate: 'code' | 'tests' | 'documentation', value: string): Promise<Task>
  addReviewer(taskId: number, reviewerId: number): Promise<Task>
  removeReviewer(taskId: number, reviewerId: number): Promise<Task>
  submitForReview(taskId: number): Promise<Task>
  decideReview(taskId: number, decision: 'approved' | 'changes_requested', comment?: string): Promise<Task>
  completeTask(taskId: number): Promise<CompletionResult>
  getLedger(userId: number): Promise<LedgerEntry[]>
  leaderboard(): Promise<LeaderboardRow[]>
  getPreferences(): Promise<Preferences>
  updatePreferences(draft: PreferencesInput): Promise<Preferences>
  getStats(range: StatsRange): Promise<Stats>

  // Projects: explicit membership, curated by the project lead.
  listProjectMembers(projectId: number): Promise<ProjectMember[]>
  addProjectMember(projectId: number, userId: number): Promise<ProjectMember[]>
  removeProjectMember(projectId: number, userId: number): Promise<ProjectMember[]>
  listProjectChats(projectId: number): Promise<ChatRoom[]>
  createProjectChat(projectId: number, draft: ProjectRoomDraft): Promise<ChatRoom>

  // Chat.
  listChats(): Promise<ChatRoom[]>
  getChat(roomId: number): Promise<ChatRoom>
  chatUnread(): Promise<number>
  discoverChats(): Promise<ChatRoom[]>
  listChatRequests(): Promise<ChatRequest[]>
  openDM(userId: number): Promise<ChatRoom>
  createOpenRoom(draft: OpenRoomDraft): Promise<OpenRoomResult>
  updateChat(roomId: number, draft: { name: string; purpose: string; visibility: RoomVisibility }): Promise<ChatRoom>
  archiveChat(roomId: number): Promise<void>
  joinChat(roomId: number): Promise<ChatRoom>
  leaveChat(roomId: number): Promise<void>
  addChatMember(roomId: number, userId: number): Promise<ChatRoom>
  removeChatMember(roomId: number, userId: number): Promise<ChatRoom>
  listChatMessages(roomId: number, before?: number, limit?: number): Promise<ChatMessage[]>
  sendChatMessage(roomId: number, body: string, attachmentIds?: number[]): Promise<ChatMessage>
  editChatMessage(messageId: number, body: string): Promise<ChatMessage>
  deleteChatMessage(messageId: number): Promise<void>
  markChatRead(roomId: number, messageId: number): Promise<void>
  uploadChatAttachment(roomId: number, file: File): Promise<ChatAttachment>
  fetchChatAttachment(attachmentId: number): Promise<Blob>
  streamChatEvents(onEvent: (event: ChatStreamEvent) => void, signal: AbortSignal): Promise<void>
}

const BASE = config.apiBase

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  // FormData sets its own multipart boundary; everything else is JSON.
  if (init.body && !(init.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }

  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const response = await fetch(`${BASE}${path}`, { ...init, headers })
  const text = await response.text()
  let payload: unknown = null
  if (text) {
    try {
      payload = JSON.parse(text) as unknown
    } catch {
      payload = null
    }
  }

  if (!response.ok) {
    if (response.status === 401 && token) {
      clearStoredSession()
      notifySignOut()
    }
    const message =
      payload && typeof payload === 'object' && 'error' in payload
        ? String((payload as { error: unknown }).error)
        : `Request failed (${response.status})`
    throw new ApiError(response.status, message)
  }

  return payload as T
}

/** Fetches an authenticated binary response and returns it as a blob. */
async function requestBlob(path: string): Promise<Blob> {
  const token = getToken()
  const headers = new Headers()
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const response = await fetch(`${BASE}${path}`, { headers })
  if (!response.ok) {
    if (response.status === 401 && token) {
      clearStoredSession()
      notifySignOut()
    }
    throw new ApiError(response.status, `Request failed (${response.status})`)
  }
  return response.blob()
}

/**
 * Reads the authenticated chat event stream. Native EventSource cannot send a
 * bearer header, so this parses the SSE frames from a fetch body instead.
 */
async function streamChatEvents(
  onEvent: (event: ChatStreamEvent) => void,
  signal: AbortSignal,
): Promise<void> {
  const token = getToken()
  const headers = new Headers({ Accept: 'text/event-stream' })
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const response = await fetch(`${BASE}/chat/events`, { headers, signal })
  if (!response.ok || !response.body) {
    if (response.status === 401 && token) {
      clearStoredSession()
      notifySignOut()
    }
    throw new ApiError(response.status, `Chat stream failed (${response.status})`)
  }

  const reader = response.body.pipeThrough(new TextDecoderStream()).getReader()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) return
    buffer += value

    let boundary = buffer.indexOf('\n\n')
    while (boundary !== -1) {
      const frame = buffer.slice(0, boundary)
      buffer = buffer.slice(boundary + 2)
      boundary = buffer.indexOf('\n\n')

      let type = 'message'
      const dataLines: string[] = []
      for (const line of frame.split('\n')) {
        if (line.startsWith('event:')) type = line.slice(6).trim()
        else if (line.startsWith('data:')) dataLines.push(line.slice(5).trim())
      }
      if (dataLines.length === 0) continue

      const raw = dataLines.join('\n')
      let data: unknown = raw
      try {
        data = JSON.parse(raw) as unknown
      } catch {
        // Keep the raw payload; the caller only needs a reason to refetch.
      }
      onEvent({ type, data })
    }
  }
}

function query(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') search.set(key, String(value))
  }
  const qs = search.toString()
  return qs ? `?${qs}` : ''
}

export const api: ApiClient = {
  login: (username, password) =>
    request<AuthSession>('/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  register: (draft) =>
    request<AuthSession>('/register', {
      method: 'POST',
      body: JSON.stringify(draft),
    }),

  googleLogin: (credential, role) =>
    request<AuthSession>('/auth/google', {
      method: 'POST',
      body: JSON.stringify({ credential, role }),
    }),

  logout: () => request<void>('/logout', { method: 'POST' }),

  me: () => request<User>('/me', { method: 'GET' }),
  listUsers: async () => (await request<{ users: User[] }>('/users', { method: 'GET' })).users,
  listProjects: async () => (await request<{ projects: Project[] }>('/projects', { method: 'GET' })).projects,
  getProject: (id) => request<Project>(`/project/${id}`, { method: 'GET' }),

  async createProject(draft) {
    return (await request<{ project: Project }>('/project', { method: 'POST', body: JSON.stringify(draft) })).project
  },

  async listTasks(filter) {
    const data = await request<{ tasks: Task[] }>(
      `/tasks${query({
        project_id: filter?.projectId,
        assignee: filter?.assigneeId,
        reviewer: filter?.reviewerId,
        state: filter?.state,
      })}`,
      { method: 'GET' },
    )
    return data.tasks
  },

  getTask: (id) => request<Task>(`/task/${id}`, { method: 'GET' }),

  async createTask(projectId, draft) {
    return (
      await request<{ task: Task }>(`/project/${projectId}/task`, {
        method: 'POST',
        body: JSON.stringify(draft),
      })
    ).task
  },

  async approveTask(taskId) {
    return (await request<{ task: Task }>(`/task/${taskId}/approve`, { method: 'POST' })).task
  },

  async rejectTask(taskId, reason) {
    return (
      await request<{ task: Task }>(`/task/${taskId}/reject`, {
        method: 'POST',
        body: JSON.stringify({ reason }),
      })
    ).task
  },

  async assignTask(taskId, assigneeId) {
    return (
      await request<{ task: Task }>(`/task/${taskId}/assign`, {
        method: 'POST',
        body: JSON.stringify({ assignee_id: assigneeId }),
      })
    ).task
  },

  async setGate(taskId, gate, value) {
    const field = gate === 'code' ? 'code' : gate === 'tests' ? 'tests' : 'documentation'
    return (
      await request<{ task: Task }>(`/task/${taskId}/done/${gate}`, {
        method: 'POST',
        body: JSON.stringify({ [field]: value }),
      })
    ).task
  },

  async addReviewer(taskId, reviewerId) {
    return (
      await request<{ task: Task }>(`/task/${taskId}/done/reviewer`, {
        method: 'POST',
        body: JSON.stringify({ reviewer_id: reviewerId }),
      })
    ).task
  },

  async removeReviewer(taskId, reviewerId) {
    return (await request<{ task: Task }>(`/task/${taskId}/reviewer/${reviewerId}`, { method: 'DELETE' })).task
  },

  async submitForReview(taskId) {
    return (await request<{ task: Task }>(`/task/${taskId}/submit`, { method: 'POST' })).task
  },

  async decideReview(taskId, decision, comment) {
    return (
      await request<{ task: Task }>(`/task/${taskId}/review`, {
        method: 'POST',
        body: JSON.stringify({ decision, comment }),
      })
    ).task
  },

  completeTask: (taskId) => request<CompletionResult>(`/task/${taskId}/complete`, { method: 'POST' }),

  async getLedger(userId) {
    return (await request<{ ledger: LedgerEntry[] }>(`/user/${userId}/ledger`, { method: 'GET' })).ledger
  },

  async leaderboard() {
    return (await request<{ leaderboard: LeaderboardRow[] }>('/leaderboard', { method: 'GET' })).leaderboard
  },

  async getPreferences() {
    return (await request<{ preferences: Preferences }>('/me/preferences', { method: 'GET' })).preferences
  },

  async updatePreferences(draft) {
    return (
      await request<{ preferences: Preferences }>('/me/preferences', {
        method: 'PUT',
        body: JSON.stringify(draft),
      })
    ).preferences
  },

  async getStats(range) {
    return (await request<{ stats: Stats }>(`/stats${query({ range })}`, { method: 'GET' })).stats
  },

  async listProjectMembers(projectId) {
    return (await request<{ members: ProjectMember[] }>(`/project/${projectId}/members`, { method: 'GET' })).members
  },

  async addProjectMember(projectId, userId) {
    return (
      await request<{ members: ProjectMember[] }>(`/project/${projectId}/members`, {
        method: 'POST',
        body: JSON.stringify({ userId }),
      })
    ).members
  },

  async removeProjectMember(projectId, userId) {
    return (
      await request<{ members: ProjectMember[] }>(`/project/${projectId}/members/${userId}`, {
        method: 'DELETE',
      })
    ).members
  },

  async listProjectChats(projectId) {
    return (await request<{ rooms: ChatRoom[] }>(`/project/${projectId}/chats`, { method: 'GET' })).rooms
  },

  async createProjectChat(projectId, draft) {
    return (
      await request<{ room: ChatRoom }>(`/project/${projectId}/chats`, {
        method: 'POST',
        body: JSON.stringify(draft),
      })
    ).room
  },

  async listChats() {
    return (await request<{ rooms: ChatRoom[] }>('/chats', { method: 'GET' })).rooms
  },

  async getChat(roomId) {
    return (await request<{ room: ChatRoom }>(`/chat/${roomId}`, { method: 'GET' })).room
  },

  async chatUnread() {
    return (await request<{ unread: number }>('/chat/unread', { method: 'GET' })).unread
  },

  async discoverChats() {
    return (await request<{ rooms: ChatRoom[] }>('/chat/discover', { method: 'GET' })).rooms
  },

  async listChatRequests() {
    return (await request<{ requests: ChatRequest[] }>('/chat/requests', { method: 'GET' })).requests
  },

  async openDM(userId) {
    return (await request<{ room: ChatRoom }>('/chat/dm', { method: 'POST', body: JSON.stringify({ userId }) })).room
  },

  createOpenRoom: (draft) =>
    request<OpenRoomResult>('/chat', { method: 'POST', body: JSON.stringify(draft) }),

  async updateChat(roomId, draft) {
    return (
      await request<{ room: ChatRoom }>(`/chat/${roomId}`, {
        method: 'PUT',
        body: JSON.stringify(draft),
      })
    ).room
  },

  archiveChat: (roomId) => request<void>(`/chat/${roomId}`, { method: 'DELETE' }),
  async joinChat(roomId) {
    return (await request<{ room: ChatRoom }>(`/chat/${roomId}/join`, { method: 'POST' })).room
  },
  leaveChat: (roomId) => request<void>(`/chat/${roomId}/leave`, { method: 'POST' }),

  async addChatMember(roomId, userId) {
    return (
      await request<{ room: ChatRoom }>(`/chat/${roomId}/members`, {
        method: 'POST',
        body: JSON.stringify({ userId }),
      })
    ).room
  },

  async removeChatMember(roomId, userId) {
    return (await request<{ room: ChatRoom }>(`/chat/${roomId}/members/${userId}`, { method: 'DELETE' })).room
  },

  async listChatMessages(roomId, before, limit) {
    return (
      await request<{ messages: ChatMessage[] }>(
        `/chat/${roomId}/messages${query({ before, limit })}`,
        { method: 'GET' },
      )
    ).messages
  },

  async sendChatMessage(roomId, body, attachmentIds) {
    return (
      await request<{ message: ChatMessage }>(`/chat/${roomId}/messages`, {
        method: 'POST',
        body: JSON.stringify({ body, attachmentIds }),
      })
    ).message
  },

  async editChatMessage(messageId, body) {
    return (
      await request<{ message: ChatMessage }>(`/chat/message/${messageId}`, {
        method: 'PATCH',
        body: JSON.stringify({ body }),
      })
    ).message
  },

  deleteChatMessage: (messageId) => request<void>(`/chat/message/${messageId}`, { method: 'DELETE' }),

  markChatRead: (roomId, messageId) =>
    request<void>(`/chat/${roomId}/read`, { method: 'POST', body: JSON.stringify({ messageId }) }),

  async uploadChatAttachment(roomId, file) {
    const form = new FormData()
    form.append('file', file)
    return (
      await request<{ attachment: ChatAttachment }>(`/chat/${roomId}/attachments`, {
        method: 'POST',
        body: form,
      })
    ).attachment
  },

  fetchChatAttachment: (attachmentId) => requestBlob(`/attachments/${attachmentId}`),

  streamChatEvents,
}
