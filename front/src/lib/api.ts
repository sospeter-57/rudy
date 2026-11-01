import { ApiError, type LeaderboardRow, type LedgerEntry, type Project, type ProjectDraft, type RegisterDraft, type Task, type TaskDraft, type User } from './types'
import { mockApi } from './store'

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
 * The contract the frontend is written against. `mockApi` and `httpApi` both
 * satisfy it, so switching from demo data to the real Go service is a single
 * environment variable.
 */
export interface ApiClient {
  login(username: string, password: string): Promise<User>
  register(draft: RegisterDraft): Promise<User>
  me(userId: number): Promise<User>
  listUsers(): Promise<User[]>
  listProjects(): Promise<Project[]>
  getProject(id: number): Promise<Project>
  createProject(draft: ProjectDraft, actorId: number): Promise<Project>
  listTasks(filter?: TaskFilter): Promise<Task[]>
  getTask(id: number): Promise<Task>
  createTask(projectId: number, draft: TaskDraft, actorId: number): Promise<Task>
  approveTask(taskId: number, actorId: number): Promise<Task>
  rejectTask(taskId: number, reason: string, actorId: number): Promise<Task>
  assignTask(taskId: number, assigneeId: number, actorId: number): Promise<Task>
  setGate(taskId: number, gate: 'code' | 'tests' | 'documentation', value: string, actorId: number): Promise<Task>
  addReviewer(taskId: number, reviewerId: number, actorId: number): Promise<Task>
  removeReviewer(taskId: number, reviewerId: number, actorId: number): Promise<Task>
  submitForReview(taskId: number, actorId: number): Promise<Task>
  decideReview(taskId: number, actorId: number, decision: 'approved' | 'changes_requested', comment?: string): Promise<Task>
  completeTask(taskId: number, actorId: number): Promise<CompletionResult>
  getLedger(userId: number): Promise<LedgerEntry[]>
  leaderboard(): Promise<LeaderboardRow[]>
  resetDemo?(): void
}

const BASE = import.meta.env.VITE_API_BASE ?? '/api'

async function request<T>(path: string, init: RequestInit, actorId?: number): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body) headers.set('Content-Type', 'application/json')
  if (actorId) headers.set('X-User-ID', String(actorId))

  const response = await fetch(`${BASE}${path}`, { ...init, headers })
  const text = await response.text()
  const payload = text ? (JSON.parse(text) as unknown) : null

  if (!response.ok) {
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

export const httpApi: ApiClient = {
  async login(username, password) {
    const data = await request<{ user: User }>('/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    })
    return data.user
  },

  async register(draft) {
    const data = await request<{ user: User }>('/register', {
      method: 'POST',
      body: JSON.stringify(draft),
    })
    return data.user
  },

  me: (userId) => request<User>('/me', { method: 'GET' }, userId),
  listUsers: async () => (await request<{ users: User[] }>('/users', { method: 'GET' })).users,
  listProjects: async () => (await request<{ projects: Project[] }>('/projects', { method: 'GET' })).projects,
  getProject: (id) => request<Project>(`/project/${id}`, { method: 'GET' }),

  async createProject(draft, actorId) {
    const data = await request<{ project: Project }>(
      '/project',
      { method: 'POST', body: JSON.stringify(draft) },
      actorId,
    )
    return data.project
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

  async createTask(projectId, draft, actorId) {
    const data = await request<{ task: Task }>(
      `/project/${projectId}/task`,
      { method: 'POST', body: JSON.stringify(draft) },
      actorId,
    )
    return data.task
  },

  async approveTask(taskId, actorId) {
    return (await request<{ task: Task }>(`/task/${taskId}/approve`, { method: 'POST' }, actorId)).task
  },

  async rejectTask(taskId, reason, actorId) {
    return (
      await request<{ task: Task }>(
        `/task/${taskId}/reject`,
        { method: 'POST', body: JSON.stringify({ reason }) },
        actorId,
      )
    ).task
  },

  async assignTask(taskId, assigneeId, actorId) {
    return (
      await request<{ task: Task }>(
        `/task/${taskId}/assign`,
        { method: 'POST', body: JSON.stringify({ assignee_id: assigneeId }) },
        actorId,
      )
    ).task
  },

  async setGate(taskId, gate, value, actorId) {
    const field = gate === 'code' ? 'code' : gate === 'tests' ? 'tests' : 'documentation'
    return (
      await request<{ task: Task }>(
        `/task/${taskId}/done/${gate}`,
        { method: 'POST', body: JSON.stringify({ [field]: value }) },
        actorId,
      )
    ).task
  },

  async addReviewer(taskId, reviewerId, actorId) {
    return (
      await request<{ task: Task }>(
        `/task/${taskId}/done/reviewer`,
        { method: 'POST', body: JSON.stringify({ reviewer_id: reviewerId }) },
        actorId,
      )
    ).task
  },

  async removeReviewer(taskId, reviewerId, actorId) {
    return (
      await request<{ task: Task }>(
        `/task/${taskId}/reviewer/${reviewerId}`,
        { method: 'DELETE' },
        actorId,
      )
    ).task
  },

  async submitForReview(taskId, actorId) {
    return (await request<{ task: Task }>(`/task/${taskId}/submit`, { method: 'POST' }, actorId)).task
  },

  async decideReview(taskId, actorId, decision, comment) {
    return (
      await request<{ task: Task }>(
        `/task/${taskId}/review`,
        { method: 'POST', body: JSON.stringify({ decision, comment }) },
        actorId,
      )
    ).task
  },

  async completeTask(taskId, actorId) {
    return request<CompletionResult>(`/task/${taskId}/complete`, { method: 'POST' }, actorId)
  },

  async getLedger(userId) {
    return (await request<{ ledger: LedgerEntry[] }>(`/user/${userId}/ledger`, { method: 'GET' }, userId)).ledger
  },

  async leaderboard() {
    return (await request<{ leaderboard: LeaderboardRow[] }>('/leaderboard', { method: 'GET' })).leaderboard
  },
}

export const API_MODE = import.meta.env.VITE_API_MODE === 'http' ? 'http' : 'mock'

export const api: ApiClient = API_MODE === 'http' ? httpApi : mockApi
