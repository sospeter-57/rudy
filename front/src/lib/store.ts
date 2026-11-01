import {
  ApiError,
  type ApprovalState,
  type LeaderboardRow,
  type LedgerEntry,
  type Project,
  type ProjectDraft,
  type RegisterDraft,
  type Review,
  type Role,
  type Task,
  type TaskDraft,
  type User,
} from './types'
import { awardPlanFor } from './points'
import { ROLES } from './types'

const STORAGE_KEY = 'rudy.state.v1'
const SESSION_KEY = 'rudy.session.v1'

interface StoredUser extends User {
  password: string
}

interface AppState {
  users: StoredUser[]
  projects: Project[]
  tasks: Task[]
  ledger: LedgerEntry[]
  seq: { user: number; project: number; task: number; ledger: number }
}

function daysAgo(days: number): string {
  return new Date(Date.now() - days * 86_400_000).toISOString()
}

function hoursAgo(hours: number): string {
  return new Date(Date.now() - hours * 3_600_000).toISOString()
}

function review(userId: number, decision: Review['decision'], comment?: string): Review {
  return {
    userId,
    decision,
    comment,
    decidedAt: decision === 'pending' ? undefined : daysAgo(2),
  }
}

function seed(): AppState {
  const users: StoredUser[] = [
    { id: 1, username: 'nova', password: 'password123', role: 'Project Manager', points: 0, createdAt: daysAgo(120) },
    { id: 2, username: 'ari', password: 'password123', role: 'Frontend Engineer', points: 0, createdAt: daysAgo(96) },
    { id: 3, username: 'kai', password: 'password123', role: 'Backend Engineer', points: 0, createdAt: daysAgo(88) },
    { id: 4, username: 'mira', password: 'password123', role: 'QA Engineer', points: 0, createdAt: daysAgo(64) },
    { id: 5, username: 'sol', password: 'password123', role: 'UIUX Designer', points: 0, createdAt: daysAgo(41) },
  ]

  const projects: Project[] = [
    {
      id: 1,
      name: 'Aurora Commerce',
      description:
        'A storefront rebuild focused on a checkout that feels effortless, fast on mid-range phones, and accessible to everyone.',
      ttl: 1440,
      creatorId: 1,
      createdAt: daysAgo(70),
    },
    {
      id: 2,
      name: 'Atlas Mobile',
      description:
        'Companion app for field teams: offline-first sync, crisp onboarding, and native-feeling interactions.',
      ttl: 2880,
      creatorId: 1,
      createdAt: daysAgo(45),
    },
  ]

  const tasks: Task[] = [
    {
      id: 1,
      projectId: 1,
      description: 'Redesign the checkout flow',
      taskType: 'feature',
      taskState: 'completed',
      approvalState: 'approved',
      creatorId: 5,
      assigneeId: 2,
      role: 'Frontend Engineer',
      points: 40,
      dependencies: [],
      acceptance: {
        code: 'Checkout reworked as a three-step flow with optimistic totals and skeleton states.',
        tests: 'Playwright covers card, wallet and failure paths; 94% branch coverage on the reducer.',
        documentation: 'Decision record in /docs/checkout-v2.md with rollout and rollback notes.',
        reviewers: [review(4, 'approved', 'Verified all failure states.'), review(1, 'approved')],
      },
      createdAt: daysAgo(40),
      completedAt: daysAgo(32),
    },
    {
      id: 2,
      projectId: 1,
      description: 'Harden payment webhook retries',
      taskType: 'bug',
      taskState: 'auditing',
      approvalState: 'approved',
      creatorId: 3,
      assigneeId: 3,
      role: 'Backend Engineer',
      points: 30,
      dependencies: [1],
      acceptance: {
        code: 'Exponential backoff with jitter, dead-letter queue and replay tooling.',
        tests: 'Chaos suite simulates 5xx bursts and duplicate deliveries; all green.',
        documentation: 'Runbook added for replaying dead-letter events safely.',
        reviewers: [review(4, 'pending'), review(1, 'pending')],
      },
      createdAt: daysAgo(12),
    },
    {
      id: 3,
      projectId: 1,
      description: 'Product card hover states',
      taskType: 'feature',
      taskState: 'pending',
      approvalState: 'approved',
      creatorId: 5,
      assigneeId: null,
      role: 'Frontend Engineer',
      points: 15,
      dependencies: [],
      acceptance: {
        code: '',
        tests: '',
        documentation: '',
        reviewers: [],
      },
      createdAt: daysAgo(6),
    },
    {
      id: 4,
      projectId: 1,
      description: 'Accessibility audit of cart',
      taskType: 'research',
      taskState: 'pending',
      approvalState: 'proposed',
      creatorId: 4,
      assigneeId: null,
      role: 'QA Engineer',
      points: 25,
      dependencies: [],
      acceptance: { code: '', tests: '', documentation: '', reviewers: [] },
      createdAt: daysAgo(2),
    },
    {
      id: 5,
      projectId: 2,
      description: 'Offline sync engine',
      taskType: 'feature',
      taskState: 'pending',
      approvalState: 'approved',
      creatorId: 3,
      assigneeId: 3,
      role: 'Backend Engineer',
      points: 60,
      dependencies: [],
      acceptance: {
        code: 'CRDT-based queue with conflict resolution for field notes.',
        tests: 'Property tests cover merge ordering; 12h soak test passed.',
        documentation: 'Architecture diagram and failure matrix included.',
        reviewers: [review(2, 'pending')],
      },
      createdAt: daysAgo(9),
    },
    {
      id: 6,
      projectId: 2,
      description: 'Onboarding carousel',
      taskType: 'feature',
      taskState: 'completed',
      approvalState: 'approved',
      creatorId: 5,
      assigneeId: 2,
      role: 'UIUX Designer',
      points: 20,
      dependencies: [],
      acceptance: {
        code: 'Three-panel carousel with reduced-motion fallback.',
        tests: 'Snapshot tests at 320/375/414 widths.',
        documentation: 'Copy deck and motion spec linked in the design file.',
        reviewers: [review(1, 'approved', 'Copy is sharp. Ship it.')],
      },
      createdAt: daysAgo(28),
      completedAt: daysAgo(21),
    },
    {
      id: 7,
      projectId: 2,
      description: 'Crash on cold start',
      taskType: 'bug',
      taskState: 'pending',
      approvalState: 'proposed',
      creatorId: 4,
      assigneeId: null,
      role: 'Backend Engineer',
      points: 35,
      dependencies: [5],
      acceptance: { code: '', tests: '', documentation: '', reviewers: [] },
      createdAt: hoursAgo(20),
    },
    {
      id: 8,
      projectId: 2,
      description: 'Add haptics to primary actions',
      taskType: 'fix',
      taskState: 'pending',
      approvalState: 'changes_requested',
      creatorId: 2,
      assigneeId: null,
      role: 'Frontend Engineer',
      points: 10,
      dependencies: [],
      acceptance: { code: '', tests: '', documentation: '', reviewers: [] },
      rejectionReason: 'Scope is unclear — link this to the haptics spec before we approve it.',
      createdAt: daysAgo(4),
    },
  ]

  const ledger: LedgerEntry[] = [
    { id: 1, userId: 2, taskId: 1, projectId: 1, reason: 'task_completed', points: 40, note: 'Completed “Redesign the checkout flow”', createdAt: daysAgo(32) },
    { id: 2, userId: 4, taskId: 1, projectId: 1, reason: 'review_bonus', points: 6, note: 'Reviewed “Redesign the checkout flow”', createdAt: daysAgo(32) },
    { id: 3, userId: 1, taskId: 1, projectId: 1, reason: 'review_bonus', points: 6, note: 'Reviewed “Redesign the checkout flow”', createdAt: daysAgo(32) },
    { id: 4, userId: 2, taskId: 6, projectId: 2, reason: 'task_completed', points: 20, note: 'Completed “Onboarding carousel”', createdAt: daysAgo(21) },
    { id: 5, userId: 1, taskId: 6, projectId: 2, reason: 'review_bonus', points: 3, note: 'Reviewed “Onboarding carousel”', createdAt: daysAgo(21) },
    { id: 6, userId: 2, taskId: null, projectId: null, reason: 'adjustment', points: 120, note: 'Sprint 01 legacy credit', createdAt: daysAgo(60) },
    { id: 7, userId: 3, taskId: null, projectId: null, reason: 'adjustment', points: 110, note: 'Sprint 01 legacy credit', createdAt: daysAgo(60) },
    { id: 8, userId: 1, taskId: null, projectId: null, reason: 'adjustment', points: 60, note: 'Sprint 01 legacy credit', createdAt: daysAgo(60) },
    { id: 9, userId: 4, taskId: null, projectId: null, reason: 'adjustment', points: 45, note: 'Sprint 01 legacy credit', createdAt: daysAgo(60) },
    { id: 10, userId: 5, taskId: null, projectId: null, reason: 'adjustment', points: 30, note: 'Sprint 01 legacy credit', createdAt: daysAgo(60) },
  ]

  const state: AppState = {
    users,
    projects,
    tasks,
    ledger,
    seq: { user: 5, project: 2, task: 8, ledger: 10 },
  }
  syncPoints(state)
  return state
}

function syncPoints(state: AppState): void {
  const totals = new Map<number, number>()
  for (const entry of state.ledger) {
    totals.set(entry.userId, (totals.get(entry.userId) ?? 0) + entry.points)
  }
  for (const user of state.users) {
    user.points = Math.max(0, totals.get(user.id) ?? 0)
  }
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

function load(): AppState {
  if (typeof localStorage === 'undefined') return seed()
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      const fresh = seed()
      localStorage.setItem(STORAGE_KEY, JSON.stringify(fresh))
      return fresh
    }
    const parsed = JSON.parse(raw) as AppState
    if (!parsed?.users?.length || !parsed?.seq) return seed()
    syncPoints(parsed)
    return parsed
  } catch {
    return seed()
  }
}

let state: AppState = load()

function persist(): void {
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  }
}

function nextId(kind: keyof AppState['seq']): number {
  state.seq[kind] += 1
  return state.seq[kind]
}

function publicUser(user: StoredUser): User {
  const { password: _password, ...rest } = user
  return rest
}

function findUser(id: number): StoredUser {
  const user = state.users.find((u) => u.id === id)
  if (!user) throw new ApiError(401, 'Session expired. Sign in again.')
  return user
}

function requireManager(actorId: number): StoredUser {
  const user = findUser(actorId)
  if (user.role !== 'Project Manager') {
    throw new ApiError(403, 'Only a Project Manager can do that.')
  }
  return user
}

function findTask(id: number): Task {
  const task = state.tasks.find((t) => t.id === id)
  if (!task) throw new ApiError(404, 'Task not found.')
  return task
}

function findProject(id: number): Project {
  const project = state.projects.find((p) => p.id === id)
  if (!project) throw new ApiError(404, 'Project not found.')
  return project
}

function assertWorkable(task: Task, actorId: number): void {
  const actor = findUser(actorId)
  if (task.assigneeId !== actorId && actor.role !== 'Project Manager') {
    throw new ApiError(403, 'Only the assignee or a Project Manager can change this task.')
  }
  if (task.taskState === 'completed') {
    throw new ApiError(409, 'This task is already completed.')
  }
}

export const session = {
  get(): User | null {
    if (typeof localStorage === 'undefined') return null
    const raw = localStorage.getItem(SESSION_KEY)
    if (!raw) return null
    try {
      const id = Number(JSON.parse(raw))
      const user = state.users.find((u) => u.id === id)
      return user ? publicUser(user) : null
    } catch {
      return null
    }
  },
  set(user: User | null): void {
    if (typeof localStorage === 'undefined') return
    if (user) localStorage.setItem(SESSION_KEY, JSON.stringify(user.id))
    else localStorage.removeItem(SESSION_KEY)
  },
}

export const mockApi = {
  resetDemo(): void {
    state = seed()
    persist()
    session.set(null)
  },

  async login(username: string, password: string): Promise<User> {
    const user = state.users.find((u) => u.username.toLowerCase() === username.trim().toLowerCase())
    if (!user || user.password !== password) {
      throw new ApiError(401, 'Invalid username or password.')
    }
    return publicUser(user)
  },

  async register(draft: RegisterDraft): Promise<User> {
    const username = draft.username.trim()
    if (username.length < 3 || username.length > 24) {
      throw new ApiError(422, 'Username must be between 3 and 24 characters.')
    }
    if (draft.password.length < 8) {
      throw new ApiError(422, 'Password must be at least 8 characters.')
    }
    if (!ROLES.includes(draft.role)) {
      throw new ApiError(422, 'Choose a valid role.')
    }
    if (state.users.some((u) => u.username.toLowerCase() === username.toLowerCase())) {
      throw new ApiError(409, 'That username is already taken.')
    }
    const user: StoredUser = {
      id: nextId('user'),
      username,
      password: draft.password,
      role: draft.role,
      points: 0,
      createdAt: new Date().toISOString(),
    }
    state.users.push(user)
    persist()
    return publicUser(user)
  },

  async me(userId: number): Promise<User> {
    return publicUser(findUser(userId))
  },

  async listUsers(): Promise<User[]> {
    return state.users.map(publicUser)
  },

  async listProjects(): Promise<Project[]> {
    return clone([...state.projects].sort((a, b) => b.createdAt.localeCompare(a.createdAt)))
  },

  async getProject(id: number): Promise<Project> {
    return clone(findProject(id))
  },

  async createProject(draft: ProjectDraft, actorId: number): Promise<Project> {
    requireManager(actorId)
    const name = draft.name.trim()
    if (name.length < 3) throw new ApiError(422, 'Project name must be at least 3 characters.')
    if (!draft.description.trim()) throw new ApiError(422, 'A short description is required.')
    if (draft.ttl <= 0) throw new ApiError(422, 'Time to live must be positive.')
    const project: Project = {
      id: nextId('project'),
      name,
      description: draft.description.trim(),
      ttl: draft.ttl,
      creatorId: actorId,
      createdAt: new Date().toISOString(),
    }
    state.projects.push(project)
    persist()
    return clone(project)
  },

  async listTasks(filter?: {
    projectId?: number
    assigneeId?: number
    reviewerId?: number
    state?: Task['taskState']
  }): Promise<Task[]> {
    let tasks = [...state.tasks]
    if (filter?.projectId) tasks = tasks.filter((t) => t.projectId === filter.projectId)
    if (filter?.assigneeId) tasks = tasks.filter((t) => t.assigneeId === filter.assigneeId)
    if (filter?.reviewerId) {
      tasks = tasks.filter((t) => t.acceptance.reviewers.some((r) => r.userId === filter.reviewerId))
    }
    if (filter?.state) tasks = tasks.filter((t) => t.taskState === filter.state)
    return clone(tasks.sort((a, b) => b.createdAt.localeCompare(a.createdAt)))
  },

  async getTask(id: number): Promise<Task> {
    return clone(findTask(id))
  },

  async createTask(projectId: number, draft: TaskDraft, actorId: number): Promise<Task> {
    findUser(actorId)
    findProject(projectId)
    const description = draft.description.trim()
    if (description.length < 4) throw new ApiError(422, 'Describe the task in a little more detail.')
    if (draft.points < 1 || draft.points > 100) {
      throw new ApiError(422, 'Points must be between 1 and 100.')
    }
    const task: Task = {
      id: nextId('task'),
      projectId,
      description,
      taskType: draft.taskType,
      taskState: 'pending',
      approvalState: 'proposed',
      creatorId: actorId,
      assigneeId: null,
      role: draft.role,
      points: draft.points,
      dependencies: draft.dependencies,
      acceptance: { code: '', tests: '', documentation: '', reviewers: [] },
      createdAt: new Date().toISOString(),
    }
    state.tasks.push(task)
    persist()
    return clone(task)
  },

  async approveTask(taskId: number, actorId: number): Promise<Task> {
    requireManager(actorId)
    const task = findTask(taskId)
    if (task.approvalState === 'approved') {
      throw new ApiError(409, 'This task is already approved.')
    }
    if (task.taskState === 'completed') {
      throw new ApiError(409, 'Completed tasks cannot be re-approved.')
    }
    task.approvalState = 'approved'
    task.rejectionReason = undefined
    persist()
    return clone(task)
  },

  async rejectTask(taskId: number, reason: string, actorId: number): Promise<Task> {
    requireManager(actorId)
    const task = findTask(taskId)
    if (!reason.trim()) throw new ApiError(422, 'Give the creator a reason.')
    if (task.taskState === 'completed') throw new ApiError(409, 'Completed tasks cannot be rejected.')
    task.approvalState = 'changes_requested'
    task.rejectionReason = reason.trim()
    persist()
    return clone(task)
  },

  async assignTask(taskId: number, assigneeId: number, actorId: number): Promise<Task> {
    requireManager(actorId)
    const task = findTask(taskId)
    if (task.approvalState !== 'approved') {
      throw new ApiError(409, 'Approve the task before assigning it.')
    }
    if (task.taskState === 'completed') throw new ApiError(409, 'Completed tasks are locked.')
    findUser(assigneeId)
    task.assigneeId = assigneeId
    persist()
    return clone(task)
  },

  async setGate(
    taskId: number,
    gate: 'code' | 'tests' | 'documentation',
    value: string,
    actorId: number,
  ): Promise<Task> {
    const task = findTask(taskId)
    assertWorkable(task, actorId)
    task.acceptance[gate] = value.trim()
    persist()
    return clone(task)
  },

  async addReviewer(taskId: number, reviewerId: number, actorId: number): Promise<Task> {
    const task = findTask(taskId)
    assertWorkable(task, actorId)
    if (task.assigneeId === reviewerId) {
      throw new ApiError(409, 'The assignee cannot review their own work.')
    }
    findUser(reviewerId)
    if (task.acceptance.reviewers.some((r) => r.userId === reviewerId)) {
      throw new ApiError(409, 'That person is already a reviewer.')
    }
    task.acceptance.reviewers.push({ userId: reviewerId, decision: 'pending' })
    persist()
    return clone(task)
  },

  async removeReviewer(taskId: number, reviewerId: number, actorId: number): Promise<Task> {
    const task = findTask(taskId)
    assertWorkable(task, actorId)
    task.acceptance.reviewers = task.acceptance.reviewers.filter((r) => r.userId !== reviewerId)
    persist()
    return clone(task)
  },

  async submitForReview(taskId: number, actorId: number): Promise<Task> {
    const task = findTask(taskId)
    assertWorkable(task, actorId)
    if (task.approvalState !== 'approved') {
      throw new ApiError(409, 'A manager must approve this task first.')
    }
    if (!task.acceptance.code.trim() || !task.acceptance.tests.trim() || !task.acceptance.documentation.trim()) {
      throw new ApiError(409, 'Code, tests and documentation are all required before review.')
    }
    if (task.acceptance.reviewers.length === 0) {
      throw new ApiError(409, 'Add at least one reviewer before requesting review.')
    }
    task.taskState = 'auditing'
    task.acceptance.reviewers = task.acceptance.reviewers.map((r) => ({
      userId: r.userId,
      decision: 'pending',
    }))
    persist()
    return clone(task)
  },

  async decideReview(
    taskId: number,
    actorId: number,
    decision: 'approved' | 'changes_requested',
    comment?: string,
  ): Promise<Task> {
    findUser(actorId)
    const task = findTask(taskId)
    if (task.taskState !== 'auditing') {
      throw new ApiError(409, 'This task is not in review.')
    }
    const slot = task.acceptance.reviewers.find((r) => r.userId === actorId)
    if (!slot) throw new ApiError(403, 'You are not a reviewer on this task.')
    slot.decision = decision
    slot.comment = comment?.trim() || undefined
    slot.decidedAt = new Date().toISOString()
    if (decision === 'changes_requested') {
      task.taskState = 'pending'
    }
    persist()
    return clone(task)
  },

  async completeTask(taskId: number, actorId: number): Promise<{ task: Task; awards: LedgerEntry[] }> {
    requireManager(actorId)
    const task = findTask(taskId)
    if (task.taskState === 'completed') {
      throw new ApiError(409, 'This task is already completed.')
    }
    if (task.approvalState !== 'approved') {
      throw new ApiError(409, 'A task must be approved before it can be completed.')
    }
    if (task.taskState !== 'pending' && task.taskState !== 'auditing') {
      throw new ApiError(409, 'This task is not in a completable state.')
    }
    const gatesFilled =
      task.acceptance.code.trim() &&
      task.acceptance.tests.trim() &&
      task.acceptance.documentation.trim()
    if (!gatesFilled) {
      throw new ApiError(409, 'Code, tests and documentation must all be present.')
    }
    if (task.acceptance.reviewers.length > 0) {
      const outstanding = task.acceptance.reviewers.filter((r) => r.decision !== 'approved')
      if (outstanding.length > 0) {
        throw new ApiError(409, 'Every reviewer must approve before this task can be completed.')
      }
    }
    if (!task.assigneeId) {
      throw new ApiError(409, 'Assign this task before completing it so the points have an owner.')
    }

    const users = state.users.map(publicUser)
    const plan = awardPlanFor(task, users)
    const now = new Date().toISOString()
    const awards: LedgerEntry[] = plan.lines.map((line) => ({
      id: nextId('ledger'),
      userId: line.userId,
      taskId: task.id,
      projectId: task.projectId,
      reason: line.reason,
      points: line.points,
      note: line.note,
      createdAt: now,
    }))
    state.ledger.push(...awards)
    task.taskState = 'completed'
    task.completedAt = now
    syncPoints(state)
    persist()
    return { task: clone(task), awards: clone(awards) }
  },

  async getLedger(userId: number): Promise<LedgerEntry[]> {
    return clone(
      state.ledger
        .filter((entry) => entry.userId === userId)
        .sort((a, b) => b.createdAt.localeCompare(a.createdAt)),
    )
  },

  async leaderboard(): Promise<LeaderboardRow[]> {
    const rows = state.users.map((user) => {
      const completedTasks = state.tasks.filter(
        (task) => task.assigneeId === user.id && task.taskState === 'completed',
      ).length
      const reviews = state.tasks.reduce(
        (count, task) =>
          count +
          task.acceptance.reviewers.filter((r) => r.userId === user.id && r.decision === 'approved').length,
        0,
      )
      return { user: publicUser(user), rank: 0, completedTasks, reviews }
    })
    rows.sort((a, b) => b.user.points - a.user.points || b.completedTasks - a.completedTasks)
    rows.forEach((row, index) => {
      row.rank = index + 1
    })
    return clone(rows)
  },
}

export function resetDemoData(): void {
  mockApi.resetDemo()
}

export type MockApi = typeof mockApi
export type { ApprovalState, Role }
