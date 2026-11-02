import { useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, Clock, MessageSquare, Plus, Trash2, UserPlus, Users } from 'lucide-react'
import { useMe, useProject, useProjectChats, useProjectMembers, useTasks, useUsers } from '@/lib/hooks'
import { minutesLabel, shortId } from '@/lib/format'
import { PageHeader } from '@/components/layout'
import { Badge, Button, Dialog, EmptyState, Field, Input, Select, Spinner, Stat, Textarea } from '@/components/ui'
import { TaskBoard } from '@/components/task'
import { CreateTaskDialog } from '@/features/tasks/CreateTaskDialog'
import { api } from '@/lib/api'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from '@/lib/toast'
import type { ProjectMember } from '@/lib/types'

export function ProjectDetailPage() {
  const { id } = useParams()
  const projectId = Number(id)
  const me = useMe().data
  const { data: project, isLoading } = useProject(projectId)
  const { data: tasks = [], isLoading: tasksLoading } = useTasks({ projectId })
  const { data: users = [] } = useUsers()
  const { data: members = [] } = useProjectMembers(projectId)
  const { data: chats = [] } = useProjectChats(projectId, !!me)
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [memberOpen, setMemberOpen] = useState(false)
  const [chatOpen, setChatOpen] = useState(false)

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
  const isLead = !!me && project.creatorId === me.id
  const isMember = !!me && members.some((member) => member.userId === me.id)
  const memberIDs = new Set(members.map((member) => member.userId))
  const addableUsers = users.filter((user) => !memberIDs.has(user.id))

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
          body="Propose the first task. A Lead will approve it before work begins."
          action={me ? <Button onClick={() => setDialogOpen(true)}><Plus size={15} /> Propose a task</Button> : undefined}
        />
      ) : (
        <TaskBoard tasks={tasks} users={users} />
      )}

      <section className="panel">
        <header className="panel__head">
          <div>
            <span className="eyebrow">Team</span>
            <h2 className="panel__title">Project members</h2>
          </div>
          {isLead ? (
            <Button size="sm" variant="outline" onClick={() => setMemberOpen(true)}>
              <UserPlus size={14} /> Add member
            </Button>
          ) : null}
        </header>
        <p className="form-note">
          Membership controls project chat access. Projects and tasks stay visible to every signed-in user.
        </p>
        {members.length === 0 ? (
          <EmptyState title="No members yet" body="The lead curates who joins this project." />
        ) : (
          <div className="member-list">
            {members.map((member) => (
              <div key={member.userId} className="member-list__row">
                <Users size={15} />
                <span className="member-list__who">
                  <strong>{member.username}</strong>
                  <small>{member.role}</small>
                </span>
                <Badge tone={member.projectRole === 'lead' ? 'gold' : 'muted'}>
                  {member.projectRole === 'lead' ? 'Project lead' : 'Member'}
                </Badge>
                {isLead && member.projectRole !== 'lead' ? (
                  <button
                    type="button"
                    className="icon-btn"
                    title={`Remove ${member.username}`}
                    aria-label={`Remove ${member.username}`}
                    onClick={async () => {
                      try {
                        await api.removeProjectMember(project.id, member.userId)
                        toast('Member removed', member.username, 'bone')
                        void queryClient.invalidateQueries({ queryKey: ['project-members', project.id] })
                        void queryClient.invalidateQueries({ queryKey: ['project-chats', project.id] })
                      } catch (err) {
                        toast('Could not remove member', err instanceof Error ? err.message : undefined, 'red')
                      }
                    }}
                  >
                    <Trash2 size={14} />
                  </button>
                ) : null}
              </div>
            ))}
          </div>
        )}
      </section>

      <section className="panel">
        <header className="panel__head">
          <div>
            <span className="eyebrow">Chat</span>
            <h2 className="panel__title">Project rooms</h2>
          </div>
          {isLead ? (
            <Button size="sm" variant="outline" onClick={() => setChatOpen(true)}>
              <MessageSquare size={14} /> New room
            </Button>
          ) : null}
        </header>
        {!isMember ? (
          <EmptyState
            title="Members only"
            body="Project rooms are curated by the lead. Ask the lead to add you to the project."
          />
        ) : chats.length === 0 ? (
          <EmptyState
            title="No project rooms yet"
            body={isLead ? 'Open a room for a slice of the project and pick who joins.' : 'The lead has not opened a project room yet.'}
          />
        ) : (
          <div className="chat-list">
            {chats.map((room) => (
              <Link key={room.id} to={`/chat/${room.id}`} className="chat-list__row">
                <span className="chat-list__icon"><MessageSquare size={15} /></span>
                <span className="chat-list__main">
                  <span className="chat-list__top">
                    <strong>{room.name}</strong>
                    <small>{room.members?.length ?? 0} members</small>
                  </span>
                  <span className="chat-list__hint">{room.purpose || 'Project conversation'}</span>
                </span>
              </Link>
            ))}
          </div>
        )}
      </section>

      <CreateTaskDialog open={dialogOpen} onClose={() => setDialogOpen(false)} projectId={project.id} />
      <AddMemberDialog
        open={memberOpen}
        onClose={() => setMemberOpen(false)}
        projectId={project.id}
        candidates={addableUsers}
      />
      <CreateProjectChatDialog
        open={chatOpen}
        onClose={() => setChatOpen(false)}
        projectId={project.id}
        members={members}
        leadID={project.creatorId}
      />
    </div>
  )
}

function AddMemberDialog({
  open,
  onClose,
  projectId,
  candidates,
}: {
  open: boolean
  onClose: () => void
  projectId: number
  candidates: { id: number; username: string; role: string }[]
}) {
  const queryClient = useQueryClient()
  const [userID, setUserID] = useState<number | ''>('')
  const [busy, setBusy] = useState(false)

  async function add() {
    if (!userID) return
    setBusy(true)
    try {
      await api.addProjectMember(projectId, Number(userID))
      toast('Member added', undefined, 'gold')
      void queryClient.invalidateQueries({ queryKey: ['project-members', projectId] })
      setUserID('')
      onClose()
    } catch (err) {
      toast('Could not add member', err instanceof Error ? err.message : undefined, 'red')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Add a project member"
      eyebrow="Curated membership"
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button onClick={() => void add()} disabled={busy || !userID}>
            {busy ? <Spinner label="Adding" /> : <UserPlus size={15} />} Add member
          </Button>
        </>
      }
    >
      <Field label="Person" htmlFor="project-member">
        <Select id="project-member" value={userID} onChange={(event) => setUserID(event.target.value ? Number(event.target.value) : '')}>
          <option value="">Choose someone…</option>
          {candidates.map((user) => (
            <option key={user.id} value={user.id}>{user.username} — {user.role}</option>
          ))}
        </Select>
      </Field>
      <p className="form-note">Only people you add here can enter this project's chats.</p>
    </Dialog>
  )
}

function CreateProjectChatDialog({
  open,
  onClose,
  projectId,
  members,
  leadID,
}: {
  open: boolean
  onClose: () => void
  projectId: number
  members: ProjectMember[]
  leadID: number
}) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [purpose, setPurpose] = useState('')
  const [selected, setSelected] = useState<number[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const candidates = members.filter((member) => member.userId !== leadID)

  function toggle(id: number) {
    setSelected((current) => (current.includes(id) ? current.filter((value) => value !== id) : [...current, id]))
  }

  async function create() {
    setBusy(true)
    setError(null)
    try {
      const room = await api.createProjectChat(projectId, { name, purpose, memberIds: selected })
      void queryClient.invalidateQueries({ queryKey: ['project-chats', projectId] })
      void queryClient.invalidateQueries({ queryKey: ['chats'] })
      onClose()
      setName('')
      setPurpose('')
      setSelected([])
      navigate(`/chat/${room.id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not create the room.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Open a project room"
      eyebrow="Lead curated"
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button onClick={() => void create()} disabled={busy}>
            {busy ? <Spinner label="Creating" /> : <MessageSquare size={15} />} Create room
          </Button>
        </>
      }
    >
      <Field label="Name" htmlFor="project-chat-name" hint="3–48 characters">
        <Input id="project-chat-name" value={name} onChange={(event) => setName(event.target.value)} placeholder="API design" />
      </Field>
      <Field label="Purpose" htmlFor="project-chat-purpose">
        <Textarea id="project-chat-purpose" rows={3} value={purpose} onChange={(event) => setPurpose(event.target.value)} placeholder="Decisions, reviews and open questions for this slice." />
      </Field>
      <div className="chat-member-picker">
        <span className="field__label">Who joins?</span>
        {candidates.length === 0 ? (
          <p className="meta-quiet">Add project members before opening a room.</p>
        ) : (
          candidates.map((member) => (
            <label key={member.userId} className="chat-member-picker__row">
              <input type="checkbox" checked={selected.includes(member.userId)} onChange={() => toggle(member.userId)} />
              <span>
                <strong>{member.username}</strong>
                <small>{member.role}</small>
              </span>
            </label>
          ))
        )}
      </div>
      <p className="form-note">Only selected members can read this room. The lead is always included.</p>
      {error ? <p className="field__error">{error}</p> : null}
    </Dialog>
  )
}
