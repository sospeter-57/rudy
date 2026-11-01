import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  ArrowLeft,
  BadgeCheck,
  Check,
  CheckCircle2,
  Clock,
  Code2,
  FileText,
  Flag,
  MessageSquareWarning,
  Play,
  Send,
  ShieldCheck,
  Sparkles,
  TestTube2,
  Trash2,
  UserPlus,
  X,
} from 'lucide-react'
import { api } from '@/lib/api'
import { useMe, useProject, useTask, useUsers } from '@/lib/hooks'
import {
  canAddReviewer,
  canFinalize,
  canReview,
  canSubmitForReview,
  canWorkOn,
  gatesComplete,
  isAssignee,
  reviewProgress,
} from '@/lib/permissions'
import { awardPlanFor } from '@/lib/points'
import { fullDate, pointsLabel, relativeDate, shortId } from '@/lib/format'
import { toast } from '@/lib/toast'
import { PageHeader } from '@/components/layout'
import { Avatar, Badge, Button, Dialog, EmptyState, Field, Progress, Select, Spinner, Textarea } from '@/components/ui'
import { GateDots, StateBadge, TypeBadge } from '@/components/task'
import { AwardPreview } from '@/components/points'
import { cx } from '@/lib/cx'
import type { Task } from '@/lib/types'

type Gate = 'code' | 'tests' | 'documentation'

const GATE_META: Record<Gate, { label: string; icon: typeof Code2; placeholder: string; hint: string }> = {
  code: {
    label: 'Code implementation',
    icon: Code2,
    placeholder: 'Link the branch or describe what shipped…',
    hint: 'What was built and where to find it.',
  },
  tests: {
    label: 'Tests',
    icon: TestTube2,
    placeholder: 'Describe the tests, coverage and results…',
    hint: 'Proof the work behaves.',
  },
  documentation: {
    label: 'Documentation',
    icon: FileText,
    placeholder: 'Link the doc, runbook or decision record…',
    hint: 'How someone else picks this up.',
  },
}

export function TaskDetailPage() {
  const { id } = useParams()
  const taskId = Number(id)
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const me = useMe().data
  const { data: task, isLoading } = useTask(taskId)
  const { data: users = [] } = useUsers()
  const { data: project } = useProject(task?.projectId ?? 0)

  const [gateDialog, setGateDialog] = useState<{ gate: Gate; value: string } | null>(null)
  const [rejectOpen, setRejectOpen] = useState(false)
  const [rejectReason, setRejectReason] = useState('')
  const [assignee, setAssignee] = useState(0)
  const [reviewerOpen, setReviewerOpen] = useState(false)
  const [reviewerId, setReviewerId] = useState(0)
  const [completeOpen, setCompleteOpen] = useState(false)
  const [reviewOpen, setReviewOpen] = useState(false)
  const [reviewDecision, setReviewDecision] = useState<'approved' | 'changes_requested'>('approved')
  const [reviewComment, setReviewComment] = useState('')

  const userById = useMemo(() => new Map(users.map((u) => [u.id, u])), [users])
  const creator = task ? userById.get(task.creatorId) : undefined
  const assigneeUser = task?.assigneeId ? userById.get(task.assigneeId) : undefined

  useEffect(() => {
    if (task) setAssignee(task.assigneeId ?? 0)
  }, [task])

  function invalidate() {
    queryClient.invalidateQueries({ queryKey: ['task', taskId] })
    queryClient.invalidateQueries({ queryKey: ['tasks'] })
    queryClient.invalidateQueries({ queryKey: ['project', task?.projectId] })
  }

  const setGateMutation = useMutation({
    mutationFn: ({ gate, value }: { gate: Gate; value: string }) => api.setGate(taskId, gate, value, me!.id),
    onSuccess: () => { invalidate(); setGateDialog(null); toast('Gate updated', 'The acceptance criteria moved forward.', 'gold') },
    onError: (err) => toast('Could not save', err instanceof Error ? err.message : 'Try again.', 'red'),
  })

  const approveMutation = useMutation({
    mutationFn: () => api.approveTask(taskId, me!.id),
    onSuccess: () => { invalidate(); toast('Task approved', 'It is now fit to be worked on.', 'gold') },
    onError: (err) => toast('Could not approve', err instanceof Error ? err.message : 'Try again.', 'red'),
  })

  const rejectMutation = useMutation({
    mutationFn: () => api.rejectTask(taskId, rejectReason, me!.id),
    onSuccess: () => { invalidate(); setRejectOpen(false); setRejectReason(''); toast('Sent back for changes', 'The creator has your reason.', 'red') },
    onError: (err) => toast('Could not reject', err instanceof Error ? err.message : 'Try again.', 'red'),
  })

  const assignMutation = useMutation({
    mutationFn: () => api.assignTask(taskId, assignee, me!.id),
    onSuccess: (updated) => { invalidate(); toast('Assigned', `${userById.get(updated.assigneeId ?? 0)?.username ?? 'Someone'} owns this now.`, 'gold') },
    onError: (err) => toast('Could not assign', err instanceof Error ? err.message : 'Try again.', 'red'),
  })

  const reviewerMutation = useMutation({
    mutationFn: () => api.addReviewer(taskId, reviewerId, me!.id),
    onSuccess: () => { invalidate(); setReviewerOpen(false); toast('Reviewer added', 'They will be asked to sign off.', 'gold') },
    onError: (err) => toast('Could not add reviewer', err instanceof Error ? err.message : 'Try again.', 'red'),
  })

  const removeReviewerMutation = useMutation({
    mutationFn: (userId: number) => api.removeReviewer(taskId, userId, me!.id),
    onSuccess: () => { invalidate(); toast('Reviewer removed', undefined, 'bone') },
    onError: (err) => toast('Could not remove reviewer', err instanceof Error ? err.message : 'Try again.', 'red'),
  })

  const submitMutation = useMutation({
    mutationFn: () => api.submitForReview(taskId, me!.id),
    onSuccess: () => { invalidate(); toast('Sent to review', 'Every reviewer has been notified.', 'gold') },
    onError: (err) => toast('Could not submit', err instanceof Error ? err.message : 'Try again.', 'red'),
  })

  const decideMutation = useMutation({
    mutationFn: () => api.decideReview(taskId, me!.id, reviewDecision, reviewComment),
    onSuccess: () => {
      invalidate()
      setReviewOpen(false)
      setReviewComment('')
      toast(
        reviewDecision === 'approved' ? 'Review approved' : 'Changes requested',
        reviewDecision === 'approved' ? 'Your sign-off is recorded.' : 'The assignee will see your notes.',
        reviewDecision === 'approved' ? 'gold' : 'red',
      )
    },
    onError: (err) => toast('Could not record review', err instanceof Error ? err.message : 'Try again.', 'red'),
  })

  const completeMutation = useMutation({
    mutationFn: () => api.completeTask(taskId, me!.id),
    onSuccess: (result) => {
      invalidate()
      queryClient.invalidateQueries({ queryKey: ['me'] })
      queryClient.invalidateQueries({ queryKey: ['ledger'] })
      queryClient.invalidateQueries({ queryKey: ['leaderboard'] })
      setCompleteOpen(false)
      const total = result.awards.reduce((sum, award) => sum + award.points, 0)
      const names = result.awards
        .map((award) => `${userById.get(award.userId)?.username ?? 'Someone'} +${award.points}`)
        .join(' · ')
      toast(`+${pointsLabel(total)} minted`, names || 'Task completed.', 'gold')
    },
    onError: (err) => toast('Could not complete', err instanceof Error ? err.message : 'Try again.', 'red'),
  })

  if (isLoading) {
    return <div className="loading-pane"><Spinner label="Loading task" /><span>Opening the task…</span></div>
  }

  if (!task || !me) {
    return (
      <EmptyState
        icon={<Flag size={22} />}
        title="Task not found"
        body="It may have been removed, or the link is wrong."
        action={<Link to="/tasks" className="btn btn--outline btn--sm"><ArrowLeft size={14} /> Back to tasks</Link>}
      />
    )
  }

  const myReview = task.acceptance.reviewers.find((r) => r.userId === me.id)
  const review = reviewProgress(task)
  const gatesOk = gatesComplete(task)
  const finalizable = canFinalize(task, me) && !!task.assigneeId
  const plan = awardPlanFor(task, users)
  const stage = task.taskState === 'completed' ? 3 : task.taskState === 'auditing' ? 2 : task.approvalState === 'approved' ? 1 : 0
  const steps = ['Proposed', 'Approved', 'Auditing', 'Completed']

  return (
    <div className="stack">
      <Link to={`/projects/${task.projectId}`} className="back-link">
        <ArrowLeft size={14} /> {project?.name ?? 'Project'}
      </Link>

      <PageHeader
        eyebrow={`Task ${shortId(task.id)} · ${project?.name ?? ''}`}
        title={<>{task.description}</>}
        description={
          <>
            Proposed by <strong>{creator?.username ?? '—'}</strong> for a <strong>{task.role}</strong>.
            {task.assigneeId ? <> Owned by <strong>{assigneeUser?.username ?? '—'}</strong>.</> : ' Not yet assigned.'}
          </>
        }
        meta={
          <>
            <StateBadge task={task} />
            <TypeBadge type={task.taskType} />
            <Badge tone="gold"><Sparkles size={11} /> {pointsLabel(task.points)}</Badge>
            <span className="meta-quiet">Created {relativeDate(task.createdAt)}</span>
          </>
        }
        actions={
          <div className="head-actions">
            {canReview(task, me) && task.taskState === 'auditing' && myReview?.decision === 'pending' ? (
              <Button onClick={() => { setReviewDecision('approved'); setReviewOpen(true) }}>
                <ShieldCheck size={15} /> Review
              </Button>
            ) : null}
            {canSubmitForReview(task, me) ? (
              <Button onClick={() => submitMutation.mutate()} disabled={submitMutation.isPending}>
                <Send size={15} /> {submitMutation.isPending ? 'Sending…' : 'Send to review'}
              </Button>
            ) : null}
            {finalizable ? (
              <Button onClick={() => setCompleteOpen(true)}>
                <BadgeCheck size={15} /> Complete task
              </Button>
            ) : null}
          </div>
        }
      />

      {task.approvalState === 'changes_requested' && task.rejectionReason ? (
        <div className="callout callout--red">
          <MessageSquareWarning size={18} />
          <div>
            <strong>Changes requested by the manager</strong>
            <p>{task.rejectionReason}</p>
          </div>
        </div>
      ) : null}

      <div className="detail-grid">
        <div className="detail-main">
          <section className="panel">
            <header className="panel__head">
              <div>
                <span className="eyebrow">The brief</span>
                <h2 className="panel__title">Definition of done</h2>
              </div>
              <GateDots task={task} showLabels />
            </header>
            <div className="brief-grid">
              <div><span>Role</span><strong>{task.role}</strong></div>
              <div><span>Type</span><strong><TypeBadge type={task.taskType} /></strong></div>
              <div><span>Points</span><strong className="gold-text">+{pointsLabel(task.points)}</strong></div>
              <div><span>Created</span><strong>{fullDate(task.createdAt)}</strong></div>
              <div><span>Assignee</span><strong>{assigneeUser?.username ?? 'Unassigned'}</strong></div>
              <div><span>Dependencies</span><strong>{task.dependencies.length ? task.dependencies.map((d) => shortId(d)).join(', ') : 'None'}</strong></div>
            </div>
            {task.dependencies.length > 0 ? (
              <div className="dep-list">
                {task.dependencies.map((depId) => (
                  <Link key={depId} to={`/tasks/${depId}`} className="dep-link">
                    <Clock size={12} /> Depends on {shortId(depId)}
                  </Link>
                ))}
              </div>
            ) : null}
          </section>

          <section className="panel">
            <header className="panel__head">
              <div>
                <span className="eyebrow">Acceptance criteria</span>
                <h2 className="panel__title">Four gates</h2>
              </div>
              <span className={cx('gate-count', gatesOk ? 'is-done' : '')}>
                {[task.acceptance.code, task.acceptance.tests, task.acceptance.documentation].filter((v) => v.trim()).length}/3 submitted
              </span>
            </header>
            <div className="gate-list">
              {(Object.keys(GATE_META) as Gate[]).map((gate) => {
                const meta = GATE_META[gate]
                const Icon = meta.icon
                const value = task.acceptance[gate]
                const done = value.trim().length > 0
                return (
                  <article key={gate} className={cx('gate-card', done && 'is-done')}>
                    <div className="gate-card__head">
                      <span className={cx('gate-card__icon', done && 'is-done')}>
                        {done ? <Check size={15} /> : <Icon size={15} />}
                      </span>
                      <div>
                        <strong>{meta.label}</strong>
                        <span>{meta.hint}</span>
                      </div>
                      {canWorkOn(task, me) && task.taskState !== 'completed' ? (
                        <Button size="sm" variant="ghost" onClick={() => setGateDialog({ gate, value })}>
                          {done ? 'Edit' : 'Add'}
                        </Button>
                      ) : null}
                    </div>
                    <p className={cx('gate-card__body', !done && 'is-empty')}>
                      {done ? value : 'Nothing submitted yet.'}
                    </p>
                  </article>
                )
              })}
            </div>
          </section>

          <section className="panel">
            <header className="panel__head">
              <div>
                <span className="eyebrow">Sign-off</span>
                <h2 className="panel__title">Reviewers</h2>
              </div>
              <div className="panel__head-right">
                <span className="muted-copy">{review.done}/{review.total} decided</span>
                {canAddReviewer(task, me) && task.taskState !== 'completed' ? (
                  <Button size="sm" variant="outline" onClick={() => setReviewerOpen(true)}>
                    <UserPlus size={14} /> Add
                  </Button>
                ) : null}
              </div>
            </header>
            {task.acceptance.reviewers.length === 0 ? (
              <p className="muted-copy">
                No reviewers yet. {canAddReviewer(task, me) ? 'Add at least one before sending the task to review.' : 'The assignee or a manager can add them.'}
              </p>
            ) : (
              <div className="reviewer-list">
                {task.acceptance.reviewers.map((entry) => {
                  const reviewer = userById.get(entry.userId)
                  return (
                    <div key={entry.userId} className={cx('reviewer-row', `reviewer-row--${entry.decision}`)}>
                      <Avatar name={reviewer?.username ?? '?'} size="md" />
                      <div className="reviewer-row__who">
                        <strong>{reviewer?.username ?? 'Unknown'}</strong>
                        <span>{reviewer?.role ?? '—'}</span>
                        {entry.comment ? <p>“{entry.comment}”</p> : null}
                      </div>
                      <Badge tone={entry.decision === 'approved' ? 'gold' : entry.decision === 'changes_requested' ? 'red' : 'muted'}>
                        {entry.decision === 'approved' ? 'Approved' : entry.decision === 'changes_requested' ? 'Changes' : 'Pending'}
                      </Badge>
                      {canAddReviewer(task, me) && task.taskState !== 'completed' ? (
                        <button
                          className="icon-btn"
                          aria-label={`Remove ${reviewer?.username ?? 'reviewer'}`}
                          onClick={() => removeReviewerMutation.mutate(entry.userId)}
                        >
                          <Trash2 size={14} />
                        </button>
                      ) : null}
                    </div>
                  )
                })}
              </div>
            )}
            {task.acceptance.reviewers.length > 0 ? (
              <div className="review-meter">
                <Progress value={review.pct} tone={review.pct === 100 ? 'gold' : 'red'} />
                <span>{review.done === review.total && review.total > 0 ? 'All reviewers have decided' : 'Waiting on reviewers'}</span>
              </div>
            ) : null}
          </section>
        </div>

        <aside className="detail-side">
          <section className="panel panel--tight">
            <header className="panel__head">
              <div>
                <span className="eyebrow">Lifecycle</span>
                <h2 className="panel__title">Where it stands</h2>
              </div>
            </header>
            <ol className="timeline">
              {steps.map((step, index) => (
                <li key={step} className={cx('timeline__step', index <= stage && 'is-done', index === stage && 'is-active')}>
                  <span className="timeline__dot">{index < stage ? <Check size={11} /> : index + 1}</span>
                  <div>
                    <strong>{step}</strong>
                    <span>
                      {index === 0 ? 'Anyone can propose' : index === 1 ? 'Manager approves fitness' : index === 2 ? 'Reviewers decide' : 'Points are minted'}
                    </span>
                  </div>
                </li>
              ))}
            </ol>
            {task.taskState === 'completed' && task.completedAt ? (
              <p className="timeline__foot">Completed {fullDate(task.completedAt)}</p>
            ) : null}
          </section>

          {me.role === 'Project Manager' ? (
            <section className="panel panel--tight manager">
              <header className="panel__head">
                <div>
                  <span className="eyebrow">Manager desk</span>
                  <h2 className="panel__title">Decide</h2>
                </div>
                <Flag size={18} className="panel__icon" />
              </header>

              {task.approvalState === 'proposed' || task.approvalState === 'changes_requested' ? (
                <>
                  <p className="muted-copy">Is this task fit to be worked on?</p>
                  <div className="button-row">
                    <Button onClick={() => approveMutation.mutate()} disabled={approveMutation.isPending}>
                      <Check size={15} /> Approve
                    </Button>
                    <Button variant="danger" onClick={() => setRejectOpen(true)}>
                      <X size={15} /> Reject
                    </Button>
                  </div>
                </>
              ) : (
                <>
                  <div className="manager__assign">
                    <span className="field__label">Assignee</span>
                    <div className="manager__assign-row">
                      <Select value={assignee} onChange={(e) => setAssignee(Number(e.target.value))} disabled={task.taskState === 'completed'}>
                        <option value={0} disabled>Unassigned</option>
                        {users.map((user) => (
                          <option key={user.id} value={user.id}>{user.username} — {user.role}</option>
                        ))}
                      </Select>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => assignMutation.mutate()}
                        disabled={!assignee || assignee === task.assigneeId || task.taskState === 'completed'}
                      >
                        Assign
                      </Button>
                    </div>
                  </div>
                  <div className="manager__complete">
                    <div>
                      <strong>Finalise & pay</strong>
                      <span>
                        {finalizable
                          ? `${plan.total} points ready to mint`
                          : task.taskState === 'completed'
                            ? 'Already completed'
                            : !task.assigneeId
                              ? 'Assign the task first'
                              : !gatesOk
                                ? 'Fill all three gates first'
                                : task.acceptance.reviewers.some((r) => r.decision !== 'approved')
                                  ? 'Waiting on reviewer sign-off'
                                  : 'Not ready yet'}
                      </span>
                    </div>
                    <Button onClick={() => setCompleteOpen(true)} disabled={!finalizable}>
                      <BadgeCheck size={15} /> Complete
                    </Button>
                  </div>
                </>
              )}
            </section>
          ) : null}

          {canReview(task, me) ? (
            <section className="panel panel--tight">
              <header className="panel__head">
                <div>
                  <span className="eyebrow">Your sign-off</span>
                  <h2 className="panel__title">Review</h2>
                </div>
                <ShieldCheck size={18} className="panel__icon" />
              </header>
              {task.taskState !== 'auditing' ? (
                <p className="muted-copy">
                  {task.taskState === 'completed' ? 'This task is complete.' : 'The task is not in review yet.'}
                </p>
              ) : myReview?.decision === 'pending' ? (
                <>
                  <p className="muted-copy">Read the gates, then decide.</p>
                  <div className="button-row">
                    <Button onClick={() => { setReviewDecision('approved'); setReviewOpen(true) }}>
                      <Check size={15} /> Approve
                    </Button>
                    <Button variant="danger" onClick={() => { setReviewDecision('changes_requested'); setReviewOpen(true) }}>
                      <MessageSquareWarning size={15} /> Request changes
                    </Button>
                  </div>
                </>
              ) : (
                <Badge tone={myReview?.decision === 'approved' ? 'gold' : 'red'}>
                  You {myReview?.decision === 'approved' ? 'approved' : 'requested changes'} {myReview?.decidedAt ? relativeDate(myReview.decidedAt) : ''}
                </Badge>
              )}
            </section>
          ) : null}

          <section className="panel panel--tight">
            <header className="panel__head">
              <div>
                <span className="eyebrow">Payout</span>
                <h2 className="panel__title">Point split</h2>
              </div>
              <Sparkles size={18} className="panel__icon" />
            </header>
            {task.taskState === 'completed' ? (
              <div className="paid-stamp">
                <CheckCircle2 size={20} />
                <div>
                  <strong>Paid out</strong>
                  <span>{task.completedAt ? fullDate(task.completedAt) : 'Completed'}</span>
                </div>
              </div>
            ) : null}
            <AwardPreview task={task} users={users} />
            <p className="payout-note">
              {task.taskState === 'completed'
                ? 'The ledger is final. Points are already on the profiles.'
                : 'Points are minted exactly once, when a Project Manager marks the task completed.'}
            </p>
          </section>

          {canWorkOn(task, me) && task.taskState !== 'completed' ? (
            <section className="panel panel--tight">
              <header className="panel__head">
                <div>
                  <span className="eyebrow">{isAssignee(task, me) ? 'Your work' : 'Assignee work'}</span>
                  <h2 className="panel__title">Move it forward</h2>
                </div>
                <Play size={18} className="panel__icon" />
              </header>
              <p className="muted-copy">
                {canSubmitForReview(task, me)
                  ? 'Everything is in place. Send it to the reviewers.'
                  : task.taskState === 'auditing'
                    ? 'Waiting on reviewers. Nothing else to do.'
                    : !gatesOk
                      ? 'Fill code, tests and documentation.'
                      : task.acceptance.reviewers.length === 0
                        ? 'Add at least one reviewer.'
                        : 'This task must be approved first.'}
              </p>
              <Button
                className="full-width"
                onClick={() => submitMutation.mutate()}
                disabled={!canSubmitForReview(task, me) || submitMutation.isPending}
              >
                <Send size={15} /> {submitMutation.isPending ? 'Sending…' : 'Send to review'}
              </Button>
            </section>
          ) : null}
        </aside>
      </div>

      <Dialog
        open={!!gateDialog}
        onClose={() => setGateDialog(null)}
        eyebrow="Acceptance criteria"
        title={gateDialog ? GATE_META[gateDialog.gate].label : ''}
        footer={
          <>
            <Button variant="ghost" onClick={() => setGateDialog(null)}>Cancel</Button>
            <Button
              onClick={() => gateDialog && setGateMutation.mutate({ gate: gateDialog.gate, value: gateDialog.value })}
              disabled={setGateMutation.isPending}
            >
              {setGateMutation.isPending ? 'Saving…' : 'Save gate'}
            </Button>
          </>
        }
      >
        {gateDialog ? (
          <Field label="Evidence" hint={GATE_META[gateDialog.gate].hint} htmlFor="gate-value">
            <Textarea
              id="gate-value"
              rows={6}
              value={gateDialog.value}
              onChange={(e) => setGateDialog({ ...gateDialog, value: e.target.value })}
              placeholder={GATE_META[gateDialog.gate].placeholder}
            />
          </Field>
        ) : null}
      </Dialog>

      <Dialog
        open={rejectOpen}
        onClose={() => setRejectOpen(false)}
        eyebrow="Manager decision"
        title="Send this back for changes"
        footer={
          <>
            <Button variant="ghost" onClick={() => setRejectOpen(false)}>Cancel</Button>
            <Button variant="danger" onClick={() => rejectMutation.mutate()} disabled={rejectMutation.isPending || !rejectReason.trim()}>
              {rejectMutation.isPending ? 'Sending…' : 'Request changes'}
            </Button>
          </>
        }
      >
        <Field label="Reason" hint="Be specific so the creator can fix it" htmlFor="reject-reason">
          <Textarea
            id="reject-reason"
            rows={4}
            value={rejectReason}
            onChange={(e) => setRejectReason(e.target.value)}
            placeholder="What needs to change before this is fit to be worked on?"
          />
        </Field>
      </Dialog>

      <Dialog
        open={reviewerOpen}
        onClose={() => setReviewerOpen(false)}
        eyebrow="Sign-off"
        title="Add a reviewer"
        footer={
          <>
            <Button variant="ghost" onClick={() => setReviewerOpen(false)}>Cancel</Button>
            <Button onClick={() => reviewerMutation.mutate()} disabled={!reviewerId || reviewerMutation.isPending}>
              {reviewerMutation.isPending ? 'Adding…' : 'Add reviewer'}
            </Button>
          </>
        }
      >
        <Field label="Person" hint="They cannot be the assignee" htmlFor="reviewer-select">
          <Select id="reviewer-select" value={reviewerId} onChange={(e) => setReviewerId(Number(e.target.value))}>
            <option value={0} disabled>Choose a person</option>
            {users
              .filter((user) => user.id !== task.assigneeId && !task.acceptance.reviewers.some((r) => r.userId === user.id))
              .map((user) => (
                <option key={user.id} value={user.id}>{user.username} — {user.role}</option>
              ))}
          </Select>
        </Field>
      </Dialog>

      <Dialog
        open={reviewOpen}
        onClose={() => setReviewOpen(false)}
        eyebrow="Your sign-off"
        title={reviewDecision === 'approved' ? 'Approve this task' : 'Request changes'}
        footer={
          <>
            <Button variant="ghost" onClick={() => setReviewOpen(false)}>Cancel</Button>
            <Button
              variant={reviewDecision === 'approved' ? 'primary' : 'danger'}
              onClick={() => decideMutation.mutate()}
              disabled={decideMutation.isPending || (reviewDecision === 'changes_requested' && !reviewComment.trim())}
            >
              {decideMutation.isPending ? 'Recording…' : reviewDecision === 'approved' ? 'Approve' : 'Request changes'}
            </Button>
          </>
        }
      >
        <Field
          label="Comment"
          hint={reviewDecision === 'approved' ? 'Optional' : 'Required'}
          htmlFor="review-comment"
        >
          <Textarea
            id="review-comment"
            rows={4}
            value={reviewComment}
            onChange={(e) => setReviewComment(e.target.value)}
            placeholder={reviewDecision === 'approved' ? 'Anything worth noting?' : 'What needs to change?'}
          />
        </Field>
      </Dialog>

      <Dialog
        open={completeOpen}
        onClose={() => setCompleteOpen(false)}
        eyebrow="Manager desk"
        title="Complete and mint points"
        wide
        footer={
          <>
            <Button variant="ghost" onClick={() => setCompleteOpen(false)}>Cancel</Button>
            <Button onClick={() => completeMutation.mutate()} disabled={completeMutation.isPending}>
              <BadgeCheck size={15} /> {completeMutation.isPending ? 'Minting…' : 'Complete task'}
            </Button>
          </>
        }
      >
        <div className="form-stack">
          <div className="callout callout--gold">
            <Sparkles size={18} />
            <div>
              <strong>This is the payout moment</strong>
              <p>Completing the task writes the ledger entries below. It cannot be paid twice.</p>
            </div>
          </div>
          <AwardPreview task={task} users={users} />
          <p className="form-note">
            The assignee earns the task's points. Each approving reviewer earns a 15% review bonus.
            The manager earns nothing for pressing the button — no self-dealing.
          </p>
        </div>
      </Dialog>
    </div>
  )
}
