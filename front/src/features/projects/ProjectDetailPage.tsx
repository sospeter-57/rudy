import { useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ArrowLeft, Clock, Plus, Users } from 'lucide-react'
import { useMe, useProject, useTasks, useUsers } from '@/lib/hooks'
import { minutesLabel, shortId } from '@/lib/format'
import { PageHeader } from '@/components/layout'
import { Badge, Button, EmptyState, Spinner, Stat } from '@/components/ui'
import { TaskBoard } from '@/components/task'
import { CreateTaskDialog } from '@/features/tasks/CreateTaskDialog'

export function ProjectDetailPage() {
  const { id } = useParams()
  const projectId = Number(id)
  const me = useMe().data
  const { data: project, isLoading } = useProject(projectId)
  const { data: tasks = [], isLoading: tasksLoading } = useTasks({ projectId })
  const { data: users = [] } = useUsers()
  const [dialogOpen, setDialogOpen] = useState(false)

  const creator = useMemo(() => users.find((u) => u.id === project?.creatorId), [users, project])

  if (isLoading) {
    return <div className="loading-pane"><Spinner label="Loading project" /><span>Opening the board…</span></div>
  }

  if (!project) {
    return (
      <EmptyState
        icon={<Clock size={22} />}
        title="Project not found"
        body="It may have been archived, or the link is wrong."
        action={<Link to="/projects" className="btn btn--outline btn--sm"><ArrowLeft size={14} /> Back to projects</Link>}
      />
    )
  }

  const completed = tasks.filter((task) => task.taskState === 'completed')
  const openPoints = tasks.filter((task) => task.taskState !== 'completed').reduce((sum, task) => sum + task.points, 0)
  const paidPoints = completed.reduce((sum, task) => sum + task.points, 0)
  const proposed = tasks.filter((task) => task.approvalState === 'proposed' || task.approvalState === 'changes_requested')

  return (
    <div className="stack">
      <Link to="/projects" className="back-link"><ArrowLeft size={14} /> All projects</Link>

      <PageHeader
        eyebrow={`Project ${shortId(project.id)}`}
        title={<>{project.name}</>}
        description={project.description}
        meta={
          <>
            <Badge tone="muted"><Clock size={11} /> {minutesLabel(project.ttl)} ttl</Badge>
            <Badge tone="bone"><Users size={11} /> {creator?.username ?? '—'}</Badge>
            <Badge tone={proposed.length > 0 ? 'red' : 'gold'}>
              {proposed.length > 0 ? `${proposed.length} awaiting approval` : 'No pending proposals'}
            </Badge>
          </>
        }
        actions={
          me ? <Button onClick={() => setDialogOpen(true)}><Plus size={15} /> New task</Button> : null
        }
      />

      <section className="stat-row">
        <Stat label="Tasks" value={tasks.length} hint="On this board" />
        <Stat label="Completed" value={completed.length} hint="Paid out" tone="gold" />
        <Stat label="Open points" value={openPoints} hint="Still on the table" tone="red" />
        <Stat label="Points paid" value={paidPoints} hint="Minted to profiles" tone="gold" />
      </section>

      {tasksLoading ? (
        <div className="loading-pane"><Spinner label="Loading tasks" /></div>
      ) : tasks.length === 0 ? (
        <EmptyState
          icon={<Plus size={22} />}
          title="A blank board"
          body="Propose the first task. A Project Manager will approve it before work begins."
          action={me ? <Button onClick={() => setDialogOpen(true)}><Plus size={15} /> Propose a task</Button> : undefined}
        />
      ) : (
        <TaskBoard tasks={tasks} users={users} />
      )}

      <CreateTaskDialog open={dialogOpen} onClose={() => setDialogOpen(false)} projectId={project.id} />
    </div>
  )
}
