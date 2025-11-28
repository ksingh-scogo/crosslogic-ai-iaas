import { createFileRoute, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Activity, CreditCard, DollarSign, Server, Rocket, Key, BarChart3 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { TopNav } from '@/components/layout/top-nav'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { ConfigDrawer } from '@/components/config-drawer'
import { fetchUsageHistory, fetchNodeSummaries } from '@/lib/api'
import { UsageChart } from '@/components/dashboard/usage-chart'
import { RecentActivity } from '@/components/dashboard/recent-activity'
import { StatCard } from '@/components/dashboard/stat-card'
import { QuickActionCard } from '@/components/dashboard/quick-action-card'
import { EmptyStateCard } from '@/components/dashboard/empty-state'

export const Route = createFileRoute('/_authenticated/')({
  component: DashboardPage,
})

const DEFAULT_TENANT = '00000000-0000-0000-0000-000000000000'

const topNav = [
  {
    title: 'Overview',
    href: '/',
    isActive: true,
    disabled: false,
  },
  {
    title: 'Analytics',
    href: '/analytics',
    isActive: false,
    disabled: true,
  },
]

function DashboardPage() {
  const { data: usage = [] } = useQuery({
    queryKey: ['usage', DEFAULT_TENANT],
    queryFn: () => fetchUsageHistory(DEFAULT_TENANT),
  })

  const { data: nodes = [] } = useQuery({
    queryKey: ['nodes'],
    queryFn: fetchNodeSummaries,
  })

  const totalTokens = usage.reduce((acc, point) => acc + point.totalTokens, 0)
  const totalRequests = usage.reduce((acc, point) => acc + (point.requests || 0), 0)
  const activeNodes = nodes.filter((node) => node.status === 'active').length
  const totalCost = usage.reduce(
    (acc, point) => acc + parseFloat(point.totalCost.replace('$', '')),
    0
  )

  return (
    <>
      {/* ===== Top Heading ===== */}
      <Header>
        <TopNav links={topNav} />
        <div className='ms-auto flex items-center space-x-4'>
          <Search />
          <ThemeSwitch />
          <ConfigDrawer />
          <ProfileDropdown />
        </div>
      </Header>

      {/* ===== Main ===== */}
      <Main>
        <div className='mb-2 flex items-center justify-between space-y-2'>
          <h1 className='text-2xl font-bold tracking-tight'>Dashboard</h1>
          <div className='flex items-center space-x-2'>
            <Button asChild>
              <Link to='/launch'>
                <Rocket className='mr-2 h-4 w-4' />
                Launch Instance
              </Link>
            </Button>
          </div>
        </div>

        <div className='space-y-6'>
          {/* Stats Cards */}
          <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
            <StatCard
              title='Total Tokens'
              value={totalTokens.toLocaleString()}
              description='Last 24 hours'
              icon={Activity}
              variant='primary'
            />
            <StatCard
              title='Total Requests'
              value={totalRequests.toLocaleString()}
              description='API calls processed'
              icon={CreditCard}
              variant='info'
            />
            <StatCard
              title='Active Nodes'
              value={activeNodes}
              description={`${nodes.length} total nodes`}
              icon={Server}
              variant='success'
            />
            <StatCard
              title='Total Cost'
              value={`$${totalCost.toFixed(2)}`}
              description='Last 24 hours'
              icon={DollarSign}
              variant='warning'
            />
          </div>

          {/* Quick Actions */}
          <Card className='relative overflow-hidden'>
            <div className='absolute inset-0 bg-gradient-to-br from-primary/5 via-transparent to-transparent pointer-events-none' />
            <CardHeader className='pb-3 relative z-10'>
              <CardTitle className='text-lg'>Quick Actions</CardTitle>
              <CardDescription>Common tasks to get you started</CardDescription>
            </CardHeader>
            <CardContent className='relative z-10'>
              <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
                <QuickActionCard
                  title='Launch Instance'
                  description='Deploy a model'
                  icon={Rocket}
                  href='/launch'
                  variant='primary'
                />
                <QuickActionCard
                  title='Create API Key'
                  description='Generate credentials'
                  icon={Key}
                  href='/api-keys'
                  variant='warning'
                />
                <QuickActionCard
                  title='View Nodes'
                  description='Manage instances'
                  icon={Server}
                  href='/nodes'
                  variant='success'
                />
                <QuickActionCard
                  title='View Usage'
                  description='Check analytics'
                  icon={BarChart3}
                  href='/usage'
                  variant='info'
                />
              </div>
            </CardContent>
          </Card>

          {/* Charts */}
          <div className='grid grid-cols-1 gap-4 lg:grid-cols-7'>
            <Card className='col-span-1 lg:col-span-4'>
              <CardHeader>
                <CardTitle>Usage Overview</CardTitle>
              </CardHeader>
              <CardContent className='ps-2'>
                <UsageChart data={usage} />
              </CardContent>
            </Card>
            <Card className='col-span-1 lg:col-span-3'>
              <CardHeader>
                <CardTitle>Recent Activity</CardTitle>
                <CardDescription>
                  Latest usage data points
                </CardDescription>
              </CardHeader>
              <CardContent>
                <RecentActivity usage={usage.slice(0, 5)} />
              </CardContent>
            </Card>
          </div>

          {/* Getting Started (show when no active nodes) */}
          {nodes.length === 0 && <EmptyStateCard />}
        </div>
      </Main>
    </>
  )
}
