import { Link, useRouter } from '@tanstack/react-router'
import {
  LayoutDashboard,
  Rocket,
  Key,
  BarChart3,
  Server,
  Settings,
  LogOut,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth'

const navigation = [
  { name: 'Dashboard', href: '/', icon: LayoutDashboard },
  { name: 'Launch', href: '/launch', icon: Rocket },
  { name: 'API Keys', href: '/api-keys', icon: Key },
  { name: 'Usage', href: '/usage', icon: BarChart3 },
  { name: 'Nodes', href: '/nodes', icon: Server },
  { name: 'Settings', href: '/settings', icon: Settings },
]

export function Sidebar() {
  const router = useRouter()
  const { clearAuth } = useAuthStore()

  const handleLogout = () => {
    clearAuth()
    router.navigate({ to: '/login' })
  }

  return (
    <div className="flex h-screen w-64 flex-col bg-slate-900 text-white">
      {/* Logo */}
      <div className="flex h-16 items-center border-b border-slate-800 px-6">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-sky-500">
            <Server className="h-5 w-5" />
          </div>
          <div>
            <h1 className="text-lg font-bold">CrossLogic</h1>
            <p className="text-xs text-slate-400">GPU Platform</p>
          </div>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 space-y-1 px-3 py-4">
        {navigation.map((item) => {
          const Icon = item.icon
          return (
            <Link
              key={item.name}
              to={item.href}
              className={cn(
                'flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors',
                'hover:bg-slate-800 hover:text-white',
                'focus:outline-none focus:ring-2 focus:ring-sky-500'
              )}
              activeProps={{
                className: 'bg-slate-800 text-white',
              }}
            >
              <Icon className="h-5 w-5" />
              {item.name}
            </Link>
          )
        })}
      </nav>

      {/* User Section */}
      <div className="border-t border-slate-800 p-4">
        <button
          onClick={handleLogout}
          className="flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium text-slate-300 transition-colors hover:bg-slate-800 hover:text-white"
        >
          <LogOut className="h-5 w-5" />
          Sign Out
        </button>
      </div>
    </div>
  )
}
