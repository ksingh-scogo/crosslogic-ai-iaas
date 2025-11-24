import {
  LayoutDashboard,
  Rocket,
  Key,
  BarChart3,
  Server,
  Settings,
  Zap,
} from 'lucide-react'
import { type SidebarData } from '../types'

export const sidebarData: SidebarData = {
  user: {
    name: 'Admin User',
    email: 'admin@crosslogic.ai',
    avatar: '/avatars/shadcn.jpg',
  },
  teams: [
    {
      name: 'CrossLogic AI',
      logo: Zap,
      plan: 'GPU IaaS Platform',
    },
  ],
  navGroups: [
    {
      title: 'Main',
      items: [
        {
          title: 'Dashboard',
          url: '/',
          icon: LayoutDashboard,
        },
        {
          title: 'Launch',
          url: '/launch',
          icon: Rocket,
        },
        {
          title: 'API Keys',
          url: '/api-keys',
          icon: Key,
        },
        {
          title: 'Usage',
          url: '/usage',
          icon: BarChart3,
        },
        {
          title: 'Nodes',
          url: '/nodes',
          icon: Server,
        },
        {
          title: 'Settings',
          url: '/settings',
          icon: Settings,
        },
      ],
    },
  ],
}
