import { Outlet, Link, useLocation, Navigate } from 'react-router-dom'
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { Bot, LayoutDashboard, Users, LogOut } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'
import { useAuth } from '@/hooks/useAuth'
import { deanApi, DeanMe } from '@/api/client'

interface DeanContextType {
  faculty: DeanMe | null
}

const DeanContext = createContext<DeanContextType>({ faculty: null })

export function useDean() {
  return useContext(DeanContext)
}

const navItems = [
  { to: '/dean/dashboard', label: 'Дашборд', icon: LayoutDashboard, exact: true },
  { to: '/dean/students', label: 'Студенты', icon: Users, exact: false },
]

export default function DeanLayout() {
  const { pathname } = useLocation()
  const { logout } = useAuth()
  const [faculty, setFaculty] = useState<DeanMe | null>(null)
  const [loading, setLoading] = useState(true)
  const [notDean, setNotDean] = useState(false)

  useEffect(() => {
    deanApi.getMe()
      .then(setFaculty)
      .catch(() => setNotDean(true))
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center">
        <div className="text-muted-foreground text-sm">Загрузка...</div>
      </div>
    )
  }

  if (notDean) {
    return <Navigate to="/admin/plugins" replace />
  }

  const isActive = (to: string, exact: boolean) =>
    exact ? pathname === to : pathname.startsWith(to)

  return (
    <DeanContext.Provider value={{ faculty }}>
      <TooltipProvider delayDuration={150}>
        <div className="min-h-screen bg-background text-foreground flex flex-col">
          <Toaster />
          <header className="border-b bg-background px-4 sm:px-6">
            <div className="max-w-5xl mx-auto flex items-center justify-between">
              <Link
                to="/dean/dashboard"
                className="flex items-center gap-2 text-xl font-semibold tracking-tight hover:opacity-80 transition-opacity py-3"
              >
                <Bot className="h-6 w-6 text-primary" />
                <span>Деканат</span>
                {faculty && (
                  <span className="text-sm font-normal text-muted-foreground hidden sm:inline">
                    — {faculty.faculty_name}
                  </span>
                )}
              </Link>
              <nav className="flex items-center gap-1">
                {navItems.map((item) => {
                  const active = isActive(item.to, item.exact)
                  const Icon = item.icon
                  return (
                    <Button
                      key={item.to}
                      variant="ghost"
                      size="sm"
                      asChild
                      className={cn(
                        'relative rounded-none py-5',
                        active
                          ? 'text-primary after:absolute after:bottom-0 after:left-0 after:right-0 after:h-0.5 after:bg-primary'
                          : 'text-muted-foreground hover:text-foreground',
                      )}
                    >
                      <Link to={item.to}>
                        <Icon className="h-4 w-4 sm:mr-1.5" />
                        <span className="hidden sm:inline">{item.label}</span>
                      </Link>
                    </Button>
                  )
                })}
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={logout}
                  className="text-muted-foreground hover:text-foreground rounded-none py-5"
                >
                  <LogOut className="h-4 w-4 sm:mr-1.5" />
                  <span className="hidden sm:inline">Выйти</span>
                </Button>
              </nav>
            </div>
          </header>
          <main className="max-w-5xl mx-auto w-full px-4 sm:px-6 py-8 flex-1">
            <Outlet />
          </main>
        </div>
      </TooltipProvider>
    </DeanContext.Provider>
  )
}

export function DeanProviderOnly({ children }: { children: ReactNode }) {
  return <DeanContext.Provider value={{ faculty: null }}>{children}</DeanContext.Provider>
}
