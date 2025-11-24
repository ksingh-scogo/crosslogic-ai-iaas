import { createFileRoute } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Activity, CreditCard, DollarSign, Server } from 'lucide-react'
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
            <Button>Launch Instance</Button>
          </div>
        </div>

        <div className='space-y-4'>
          {/* Stats Cards */}
          <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
            <Card>
              <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                <CardTitle className='text-sm font-medium'>
                  Total Tokens
                </CardTitle>
                <Activity className='text-muted-foreground h-4 w-4' />
              </CardHeader>
              <CardContent>
                <div className='text-2xl font-bold'>{totalTokens.toLocaleString()}</div>
                <p className='text-muted-foreground text-xs'>
                  Last 24 hours
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                <CardTitle className='text-sm font-medium'>
                  Total Requests
                </CardTitle>
                <CreditCard className='text-muted-foreground h-4 w-4' />
              </CardHeader>
              <CardContent>
                <div className='text-2xl font-bold'>{totalRequests.toLocaleString()}</div>
                <p className='text-muted-foreground text-xs'>
                  API calls processed
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                <CardTitle className='text-sm font-medium'>Active Nodes</CardTitle>
                <Server className='text-muted-foreground h-4 w-4' />
              </CardHeader>
              <CardContent>
                <div className='text-2xl font-bold'>{activeNodes}</div>
                <p className='text-muted-foreground text-xs'>
                  {nodes.length} total nodes
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                <CardTitle className='text-sm font-medium'>
                  Total Cost
                </CardTitle>
                <DollarSign className='text-muted-foreground h-4 w-4' />
              </CardHeader>
              <CardContent>
                <div className='text-2xl font-bold'>${totalCost.toFixed(2)}</div>
                <p className='text-muted-foreground text-xs'>
                  Last 24 hours
                </p>
              </CardContent>
            </Card>
          </div>

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
        </div>
      </Main>
    </>
  )
}
