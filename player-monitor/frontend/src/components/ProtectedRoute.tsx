import { Navigate, Outlet } from 'react-router-dom'
import { useAuth } from '@/context/AuthContext'

export function ProtectedRoute() {
  const { isAuthenticated, isLoading } = useAuth()

  if (isLoading) {
    return <div className="flex h-screen items-center justify-center">Загрузка...</div>
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  return <Outlet />
}

export function AdminRoute() {
  const { isAdmin, isLoading } = useAuth()

  if (isLoading) {
    return <div className="flex h-screen items-center justify-center">Загрузка...</div>
  }

  if (!isAdmin) {
    return <Navigate to="/" replace />
  }

  return <Outlet />
}
