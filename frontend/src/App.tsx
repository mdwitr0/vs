import { BrowserRouter, Routes, Route, NavLink, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from '@/context/AuthContext'
import { ProtectedRoute, AdminRoute } from '@/components/ProtectedRoute'
import { SitesPage } from '@/pages/SitesPage'
import { SiteDetailPage } from '@/pages/SiteDetailPage'
import { TasksPage } from '@/pages/TasksPage'
import { ContentPage } from '@/pages/ContentPage'
import { ContentDetailPage } from '@/pages/ContentDetailPage'
import { LoginPage } from '@/pages/LoginPage'
import { UsersPage } from '@/pages/UsersPage'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30000,
      retry: 1,
    },
  },
})

function NavItem({
  to,
  children,
}: {
  to: string
  children: React.ReactNode
}) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        cn(
          'px-3 py-2 text-sm font-medium transition-colors',
          isActive
            ? 'text-foreground border-b-2 border-foreground'
            : 'text-muted-foreground hover:text-foreground'
        )
      }
    >
      {children}
    </NavLink>
  )
}

function Layout({ children }: { children: React.ReactNode }) {
  const { user, isAdmin, logout } = useAuth()

  return (
    <div className="min-h-screen bg-background">
      <header className="border-b">
        <div className="container mx-auto px-4">
          <div className="flex h-14 items-center justify-between">
            <span className="font-semibold">Video Analytics</span>
            <nav className="flex items-center gap-1">
              <NavItem to="/sites">Сайты</NavItem>
              <NavItem to="/content">Контент</NavItem>
              <NavItem to="/tasks">Задачи</NavItem>
              {isAdmin && <NavItem to="/users">Пользователи</NavItem>}
            </nav>
            <div className="flex items-center gap-3">
              <span className="text-sm text-muted-foreground">{user?.login}</span>
              <Button variant="outline" size="sm" onClick={logout}>
                Выйти
              </Button>
            </div>
          </div>
        </div>
      </header>
      <main className="container mx-auto px-4 py-6">{children}</main>
    </div>
  )
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<ProtectedRoute />}>
        <Route
          path="/"
          element={
            <Layout>
              <Navigate to="/sites" replace />
            </Layout>
          }
        />
        <Route
          path="/sites"
          element={
            <Layout>
              <SitesPage />
            </Layout>
          }
        />
        <Route
          path="/sites/:id"
          element={
            <Layout>
              <SiteDetailPage />
            </Layout>
          }
        />
        <Route
          path="/content"
          element={
            <Layout>
              <ContentPage />
            </Layout>
          }
        />
        <Route
          path="/content/:id"
          element={
            <Layout>
              <ContentDetailPage />
            </Layout>
          }
        />
        <Route
          path="/tasks"
          element={
            <Layout>
              <TasksPage />
            </Layout>
          }
        />
        <Route element={<AdminRoute />}>
          <Route
            path="/users"
            element={
              <Layout>
                <UsersPage />
              </Layout>
            }
          />
        </Route>
      </Route>
    </Routes>
  )
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthProvider>
          <AppRoutes />
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  )
}

export default App
