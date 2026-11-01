import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Check } from 'lucide-react'
import { api } from '@/lib/api'
import { useMe, useProjects, useTasks } from '@/lib/hooks'
import { TASK_TYPE_LABEL } from '@/lib/permissions'
import { toast } from '@/lib/toast'
import { ROLES, type Role, type TaskType } from '@/lib/types'
import { Button, Dialog, Field, Input, Select, Textarea } from '@/components/ui'
import { cx } from '@/lib/cx'

export function CreateTaskDialog({
  open,
  onClose,
  projectId,
}: {
  open: boolean
  onClose: () => void
  projectId?: number
}) {
  const me = useMe().data
  const queryClient = useQueryClient()
  const { data: projects = [] } = useProjects()

  const [selectedProject, setSelectedProject] = useState<number>(projectId ?? 0)
  const [description, setDescription] = useState('')
  const [taskType, setTaskType] = useState<TaskType>('feature')
  const [role, setRole] = useState<Role>(me?.role ?? 'Frontend Engineer')
  const [points, setPoints] = useState(20)
  const [dependencies, setDependencies] = useState<number[]>([])
  const [error, setError] = useState<string | null>(null)

  const { data: projectTasks = [] } = useTasks({ projectId: selectedProject })

  useEffect(() => {
    if (!open) return
    setSelectedProject(projectId ?? projects[0]?.id ?? 0)
    setError(null)
  }, [open, projectId, projects])

  useEffect(() => {
    if (me) setRole(me.role)
  }, [me])

  const availableDeps = useMemo(
    () => projectTasks.filter((task) => task.taskState !== 'completed'),
    [projectTasks],
  )

  const mutation = useMutation({
    mutationFn: () =>
      api.createTask(
        selectedProject,
        { description, taskType, role, points, dependencies },
        me!.id,
      ),
    onSuccess: (task) => {
      queryClient.invalidateQueries({ queryKey: ['tasks'] })
      queryClient.invalidateQueries({ queryKey: ['project', selectedProject] })
      toast('Proposal sent', 'A Project Manager must approve it before work starts.', 'gold')
      setDescription('')
      setPoints(20)
      setDependencies([])
      onClose()
      void task
    },
    onError: (err) => setError(err instanceof Error ? err.message : 'Could not create the task.'),
  })

  function toggleDep(id: number) {
    setDependencies((prev) => (prev.includes(id) ? prev.filter((d) => d !== id) : [...prev, id]))
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      eyebrow="Anyone can propose"
      title="Put a task on the board"
      wide
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button onClick={() => mutation.mutate()} disabled={mutation.isPending || !me || !selectedProject}>
            {mutation.isPending ? 'Sending…' : 'Submit proposal'}
          </Button>
        </>
      }
    >
      <div className="form-stack">
        <div className="form-grid">
          <Field label="Project" htmlFor="task-project">
            <Select
              id="task-project"
              value={selectedProject}
              onChange={(e) => { setSelectedProject(Number(e.target.value)); setDependencies([]) }}
              disabled={!!projectId}
            >
              <option value={0} disabled>Choose a project</option>
              {projects.map((project) => (
                <option key={project.id} value={project.id}>{project.name}</option>
              ))}
            </Select>
          </Field>
          <Field label="Task type" htmlFor="task-type">
            <Select id="task-type" value={taskType} onChange={(e) => setTaskType(e.target.value as TaskType)}>
              {(Object.keys(TASK_TYPE_LABEL) as TaskType[]).map((type) => (
                <option key={type} value={type}>{TASK_TYPE_LABEL[type]}</option>
              ))}
            </Select>
          </Field>
        </div>

        <Field label="What needs doing?" htmlFor="task-desc">
          <Textarea
            id="task-desc"
            rows={4}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Be specific. The manager approves this as the definition of done."
          />
        </Field>

        <div className="form-grid">
          <Field label="Role needed" htmlFor="task-role">
            <Select id="task-role" value={role} onChange={(e) => setRole(e.target.value as Role)}>
              {ROLES.map((option) => (
                <option key={option} value={option}>{option}</option>
              ))}
            </Select>
          </Field>
          <Field label="Points" hint="1–100 · paid on completion" htmlFor="task-points">
            <Input
              id="task-points"
              type="number"
              min={1}
              max={100}
              value={points}
              onChange={(e) => setPoints(Number(e.target.value))}
            />
          </Field>
        </div>

        {availableDeps.length > 0 ? (
          <div className="dep-picker">
            <span className="field__label">Depends on <span className="field__hint">optional</span></span>
            <div className="dep-picker__list">
              {availableDeps.map((task) => (
                <button
                  type="button"
                  key={task.id}
                  className={cx('dep-chip', dependencies.includes(task.id) && 'is-on')}
                  onClick={() => toggleDep(task.id)}
                >
                  {dependencies.includes(task.id) ? <Check size={12} /> : null}
                  {task.description}
                </button>
              ))}
            </div>
          </div>
        ) : null}

        {error ? <p className="form-error">{error}</p> : null}
        <p className="form-note">
          Proposals start as <strong>Awaiting approval</strong>. The manager decides whether the task is fit to be worked on,
          and only then can it be assigned and started.
        </p>
      </div>
    </Dialog>
  )
}
