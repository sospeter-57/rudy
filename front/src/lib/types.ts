export const ROLES = [
  'Project Manager',
  'Database Engineer',
  'Frontend Engineer',
  'Backend Engineer',
  'Fullstack Engineer',
  'QA Engineer',
  'Software Engineer',
  'Data Scientist',
  'UIUX Designer',
] as const

export type Role = (typeof ROLES)[number]

export interface User {
  id: number
  username: string
  email?: string
  role: Role
  points: number
  createdAt: string
}

export interface Project {
  id: number
  name: string
  description: string
  ttl: number
  creatorId: number
  createdAt: string
}

export type TaskType = 'feature' | 'bug' | 'fix' | 'research'
export type TaskState = 'pending' | 'auditing' | 'completed'
export type ApprovalState = 'proposed' | 'approved' | 'changes_requested'
export type ReviewDecision = 'pending' | 'approved' | 'changes_requested'

export interface Review {
  userId: number
  decision: ReviewDecision
  comment?: string
  decidedAt?: string
}

export interface Acceptance {
  code: string
  tests: string
  documentation: string
  reviewers: Review[]
}

export interface Task {
  id: number
  projectId: number
  description: string
  taskType: TaskType
  taskState: TaskState
  approvalState: ApprovalState
  creatorId: number
  assigneeId: number | null
  role: Role
  points: number
  dependencies: number[]
  acceptance: Acceptance
  rejectionReason?: string
  createdAt: string
  completedAt?: string
}

export type LedgerReason = 'task_completed' | 'review_bonus' | 'adjustment'

export interface LedgerEntry {
  id: number
  userId: number
  taskId: number | null
  projectId: number | null
  reason: LedgerReason
  points: number
  note: string
  createdAt: string
}

export interface LeaderboardRow {
  user: User
  rank: number
  completedTasks: number
  reviews: number
}

export interface AuthSession {
  user: User
  token: string
  expiresAt: string
}

export type ThemeMode = 'dark' | 'light' | 'system'

export interface Preferences {
  themeMode: ThemeMode
  accent: string
  blendA: string
  blendB: string
  updatedAt?: string
}

export interface PreferencesInput {
  themeMode: ThemeMode
  accent: string
  blendA: string
  blendB: string
}

export type StatsRange = '30d' | '90d' | 'all'

export interface StatsTotals {
  users: number
  builders: number
  projects: number
  tasks: number
  completedTasks: number
  completionRate: number
  pointsMinted: number
  reviewsApproved: number
  reviewsDecided: number
  reviewApprovalPct: number
  avgCycleHours: number
}

export interface ProjectStats {
  projectId: number
  name: string
  totalTasks: number
  proposed: number
  pending: number
  auditing: number
  completed: number
  openPoints: number
  paidPoints: number
  completionPct: number
}

export interface UserStats {
  user: User
  completedTasks: number
  reviews: number
}

export interface TrendPoint {
  period: string
  proposals: number
  completions: number
  reviews: number
  pointsMinted: number
}

export interface ReasonTotal {
  reason: string
  points: number
  count: number
}

export interface Stats {
  range: string
  totals: StatsTotals
  tasksByState: Record<string, number>
  tasksByApprovalState: Record<string, number>
  tasksByType: Record<string, number>
  roleDistribution: Record<string, number>
  projects: ProjectStats[]
  users: UserStats[]
  trends: TrendPoint[]
  ledgerByReason: ReasonTotal[]
}

export interface TaskDraft {
  description: string
  taskType: TaskType
  role: Role
  points: number
  dependencies: number[]
}

export interface ProjectDraft {
  name: string
  description: string
  ttl: number
}

export interface RegisterDraft {
  username: string
  password: string
  role: Role
}

export interface ApiErrorShape {
  status: number
  message: string
}

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}
