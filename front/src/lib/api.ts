import {
  ApiError,
  type AuthSession,
  type LeaderboardRow,
  type LedgerEntry,
  type Preferences,
  type PreferencesInput,
  type Project,
  type ProjectDraft,
  type RegisterDraft,
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

/**
 * The contract the frontend is written against. The real Go service is the only
 * implementation: every call goes through the bearer-token session.
 */
export interface ApiClient {
  login(username: string, password: string): Promise<AuthSession>
  register(draft: RegisterDraft): Promise<AuthSession>
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
}

const BASE = config.apiBase

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body) headers.set('Content-Type', 'application/json')

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
}
