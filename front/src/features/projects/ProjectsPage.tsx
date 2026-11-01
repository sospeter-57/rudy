import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { ArrowUpRight, FolderKanban, Plus, Users } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { canCreateProject } from '@/lib/permissions'
import { useMe, useProjects, useTasks, useUsers } from '@/lib/hooks'
import { minutesLabel, shortId } from '@/lib/format'
import { toast } from '@/lib/toast'
import { PageHeader } from '@/components/layout'
import { Badge, Button, Dialog, EmptyState, Field, Input, Progress, Spinner, Textarea } from '@/components/ui'

export function CreateProjectDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const me = useMe().data
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [ttl, setTtl] = useState(1440)
  const [error, setError] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: () => api.createProject({ name, description, ttl }, me!.id),
    onSuccess: (project) => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      toast('Project created', `${project.name} is on the board.`, 'gold')
      setName('')
      setDescription('')
      setTtl(1440)
      onClose()
    },
    onError: (err) => setError(err instanceof Error ? err.message : 'Could not create the project.'),
  })

  return (
    <Dialog
      open={open}
      onClose={onClose}
      eyebrow="Project Manager only"
      title="Open a new project"
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button onClick={() => mutation.mutate()} disabled={mutation.isPending || !me}>
            {mutation.isPending ? 'Creating…' : 'Create project'}
          </Button>
        </>
      }
    >
      <div className="form-stack">
        <Field label="Project name" hint="3–24 characters" htmlFor="project-name">
          <Input id="project-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Aurora Commerce" maxLength={24} />
        </Field>
        <Field label="Description" hint="What is this project for?" htmlFor="project-desc">
          <Textarea id="project-desc" rows={4} value={description} onChange={(e) => setDescription(e.target.value)} placeholder="A short brief the whole team can rally around." />
        </Field>
        <Field label="Time to live" hint="Minutes a task may stay open" htmlFor="project-ttl">
          <Input id="project-ttl" type="number" min={30} step={30} value={ttl} onChange={(e) => setTtl(Number(e.target.value))} />
        </Field>
        {error ? <p className="form-error">{error}</p> : null}
        <p className="form-note">Only Project Managers can create projects. Everyone can propose tasks inside them.</p>
      </div>
    </Dialog>
  )
}

export function ProjectsPage() {
  const me = useMe().data
  const { data: projects = [], isLoading } = useProjects()
  const { data: tasks = [] } = useTasks({})
  const { data: users = [] } = useUsers()
  const [params, setParams] = useSearchParams()
  const navigate = useNavigate()
  const [dialogOpen, setDialogOpen] = useState(false)

  useEffect(() => {
    if (params.get('new') === '1' && canCreateProject(me)) {
      setDialogOpen(true)
      params.delete('new')
      setParams(params, { replace: true })
    }
  }, [params, setParams, me])

  const userById = useMemo(() => new Map(users.map((u) => [u.id, u])), [users])

  return (
    <div className="stack">
      <PageHeader
        eyebrow="Portfolio"
        title={<>The <em>projects</em>.</>}
        description="Every board is a living contract: tasks proposed, approved, audited, and paid."
        meta={<Badge tone="gold">{projects.length} projects</Badge>}
        actions={
          canCreateProject(me) ? (
            <Button onClick={() => setDialogOpen(true)}><Plus size={15} /> New project</Button>
          ) : (
            <Badge tone="muted">Only a Project Manager can open a project</Badge>
          )
        }
      />

      {isLoading ? (
        <div className="loading-pane"><Spinner label="Loading projects" /><span>Pulling the boards…</span></div>
      ) : projects.length === 0 ? (
        <EmptyState
          icon={<FolderKanban size={22} />}
          title="No projects yet"
          body={canCreateProject(me) ? 'Open the first project and let the team propose work.' : 'A Project Manager has not opened a project yet.'}
          action={canCreateProject(me) ? <Button onClick={() => setDialogOpen(true)}><Plus size={15} /> Create the first project</Button> : undefined}
        />
      ) : (
        <div className="project-grid">
          {projects.map((project) => {
            const projectTasks = tasks.filter((task) => task.projectId === project.id)
            const done = projectTasks.filter((task) => task.taskState === 'completed').length
            const paid = projectTasks.filter((task) => task.taskState === 'completed').reduce((sum, t) => sum + t.points, 0)
            const open = projectTasks.filter((task) => task.taskState !== 'completed').reduce((sum, t) => sum + t.points, 0)
            const pct = projectTasks.length === 0 ? 0 : (done / projectTasks.length) * 100
            const creator = userById.get(project.creatorId)
            return (
              <article key={project.id} className="project-card">
                <div className="project-card__top">
                  <span className="mono-quiet">{shortId(project.id)}</span>
                  <Badge tone="muted">{minutesLabel(project.ttl)} ttl</Badge>
                </div>
                <h2 className="project-card__name">{project.name}</h2>
                <p className="project-card__desc">{project.description}</p>
                <div className="project-card__progress">
                  <Progress value={pct} tone={pct === 100 ? 'gold' : 'red'} />
                  <span>{done}/{projectTasks.length} complete</span>
                </div>
                <div className="project-card__stats">
                  <div><span>Open points</span><strong>{open}</strong></div>
                  <div><span>Paid out</span><strong>{paid}</strong></div>
                  <div><span>Tasks</span><strong>{projectTasks.length}</strong></div>
                </div>
                <div className="project-card__foot">
                  <span className="project-card__creator"><Users size={13} /> {creator?.username ?? '—'}</span>
                  <button className="text-link" onClick={() => navigate(`/projects/${project.id}`)}>
                    Open board <ArrowUpRight size={13} />
                  </button>
                </div>
              </article>
            )
          })}
        </div>
      )}

      <CreateProjectDialog open={dialogOpen} onClose={() => setDialogOpen(false)} />
    </div>
  )
}
