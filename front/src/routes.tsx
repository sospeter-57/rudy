import { Navigate, Route, Routes } from 'react-router-dom'
import { useSession } from '@/lib/session'
import { AppShell } from '@/components/layout'
import { LandingPage } from '@/features/auth/LandingPage'
import { DashboardPage } from '@/features/dashboard/DashboardPage'
import { ProjectsPage } from '@/features/projects/ProjectsPage'
import { ProjectDetailPage } from '@/features/projects/ProjectDetailPage'
import { TasksPage } from '@/features/tasks/TasksPage'
import { TaskDetailPage } from '@/features/tasks/TaskDetailPage'
import { ReviewPage } from '@/features/review/ReviewPage'
import { ProfilePage } from '@/features/profile/ProfilePage'
import { NotFoundPage } from '@/features/NotFoundPage'

function Protected({ children }: { children: React.ReactNode }) {
  const user = useSession((state) => state.user)
  if (!user) return <Navigate to="/" replace />
  return <>{children}</>
}

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<LandingPage />} />
      <Route
        element={
          <Protected>
            <AppShell />
          </Protected>
        }
      >
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/projects" element={<ProjectsPage />} />
        <Route path="/projects/:id" element={<ProjectDetailPage />} />
        <Route path="/tasks" element={<TasksPage />} />
        <Route path="/tasks/:id" element={<TaskDetailPage />} />
        <Route path="/review" element={<ReviewPage />} />
        <Route path="/profile" element={<ProfilePage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  )
}
