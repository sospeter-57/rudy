import type { Role, Task, User } from './types'

export function isLead(user: User | null | undefined): boolean {
  return user?.role === 'Lead'
}

export function canCreateProject(user: User | null | undefined): boolean {
  return isLead(user)
}

export function canApprove(user: User | null | undefined): boolean {
  return isLead(user)
}

export function canComplete(user: User | null | undefined): boolean {
  return isLead(user)
}

export function isAssignee(task: Task, user: User | null | undefined): boolean {
  return !!user && task.assigneeId === user.id
}

export function canWorkOn(task: Task, user: User | null | undefined): boolean {
  return !!user && (isAssignee(task, user) || isLead(user))
}

/**
 * Acceptance evidence and reviewers may only be edited after a lead has
 * approved the task and before the assignee submits it for review. This is the
 * approval-first rule the UI previously ignored.
 */
export function canEditAcceptance(task: Task, user: User | null | undefined): boolean {
  return canWorkOn(task, user) && task.approvalState === 'approved' && task.taskState === 'pending'
}

export function canReview(task: Task, user: User | null | undefined): boolean {
  return !!user && task.acceptance.reviewers.some((review) => review.userId === user.id)
}

export function canAddReviewer(task: Task, user: User | null | undefined): boolean {
  return canEditAcceptance(task, user)
}

export function gatesComplete(task: Task): boolean {
  return (
    task.acceptance.code.trim().length > 0 &&
    task.acceptance.tests.trim().length > 0 &&
    task.acceptance.documentation.trim().length > 0
  )
}

export function allReviewersDecided(task: Task): boolean {
  if (task.acceptance.reviewers.length === 0) return false
  return task.acceptance.reviewers.every((review) => review.decision !== 'pending')
}

export function allReviewersApproved(task: Task): boolean {
  return task.acceptance.reviewers.every((review) => review.decision === 'approved')
}

export function canSubmitForReview(task: Task, user: User | null | undefined): boolean {
  return (
    canWorkOn(task, user) &&
    task.approvalState === 'approved' &&
    task.taskState === 'pending' &&
    gatesComplete(task) &&
    task.acceptance.reviewers.length > 0
  )
}

export function canFinalize(task: Task, user: User | null | undefined): boolean {
  if (!canComplete(user)) return false
  if (task.approvalState !== 'approved') return false
  if (task.taskState !== 'pending' && task.taskState !== 'auditing') return false
  if (!gatesComplete(task)) return false
  if (task.acceptance.reviewers.length > 0 && !allReviewersApproved(task)) return false
  return true
}

export function reviewProgress(task: Task): { done: number; total: number; pct: number } {
  const total = task.acceptance.reviewers.length
  const done = task.acceptance.reviewers.filter((r) => r.decision !== 'pending').length
  return { done, total, pct: total === 0 ? 0 : (done / total) * 100 }
}

export const TASK_TYPE_LABEL: Record<Task['taskType'], string> = {
  feature: 'Feature',
  bug: 'Bug',
  fix: 'Fix',
  research: 'Research',
}

export const STATE_LABEL: Record<Task['taskState'], string> = {
  pending: 'Pending',
  auditing: 'Auditing',
  completed: 'Completed',
}

export const APPROVAL_LABEL: Record<Task['approvalState'], string> = {
  proposed: 'Awaiting approval',
  approved: 'Approved',
  changes_requested: 'Changes requested',
}

export function roleOptions(): Role[] {
  return [
    'Lead',
    'Database Engineer',
    'Frontend Engineer',
    'Backend Engineer',
    'Fullstack Engineer',
    'QA Engineer',
    'Software Engineer',
    'Data Scientist',
    'UIUX Designer',
  ]
}
