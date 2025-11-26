import { createFileRoute, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Activity, CreditCard, DollarSign, Server, Rocket, Key, BarChart3, ArrowRight, Zap, CheckCircle2 } from 'lucide-react'
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
            <Button asChild>
              <Link to='/launch'>
                <Rocket className='mr-2 h-4 w-4' />
                Launch Instance
              </Link>
            </Button>
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

          {/* Quick Actions */}
          <Card>
            <CardHeader className='pb-3'>
              <CardTitle className='text-lg'>Quick Actions</CardTitle>
              <CardDescription>Common tasks to get you started</CardDescription>
            </CardHeader>
            <CardContent>
              <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
                <Link
                  to='/launch'
                  className='group flex items-center gap-3 rounded-lg border p-3 transition-colors hover:bg-muted/50'
                >
                  <div className='rounded-md bg-primary/10 p-2'>
                    <Rocket className='h-4 w-4 text-primary' />
                  </div>
                  <div className='flex-1'>
                    <p className='text-sm font-medium'>Launch Instance</p>
                    <p className='text-xs text-muted-foreground'>Deploy a model</p>
                  </div>
                  <ArrowRight className='h-4 w-4 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100' />
                </Link>
                <Link
                  to='/api-keys'
                  className='group flex items-center gap-3 rounded-lg border p-3 transition-colors hover:bg-muted/50'
                >
                  <div className='rounded-md bg-orange-500/10 p-2'>
                    <Key className='h-4 w-4 text-orange-500' />
                  </div>
                  <div className='flex-1'>
                    <p className='text-sm font-medium'>Create API Key</p>
                    <p className='text-xs text-muted-foreground'>Generate credentials</p>
                  </div>
                  <ArrowRight className='h-4 w-4 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100' />
                </Link>
                <Link
                  to='/nodes'
                  className='group flex items-center gap-3 rounded-lg border p-3 transition-colors hover:bg-muted/50'
                >
                  <div className='rounded-md bg-green-500/10 p-2'>
                    <Server className='h-4 w-4 text-green-500' />
                  </div>
                  <div className='flex-1'>
                    <p className='text-sm font-medium'>View Nodes</p>
                    <p className='text-xs text-muted-foreground'>Manage instances</p>
                  </div>
                  <ArrowRight className='h-4 w-4 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100' />
                </Link>
                <Link
                  to='/usage'
                  className='group flex items-center gap-3 rounded-lg border p-3 transition-colors hover:bg-muted/50'
                >
                  <div className='rounded-md bg-blue-500/10 p-2'>
                    <BarChart3 className='h-4 w-4 text-blue-500' />
                  </div>
                  <div className='flex-1'>
                    <p className='text-sm font-medium'>View Usage</p>
                    <p className='text-xs text-muted-foreground'>Check analytics</p>
                  </div>
                  <ArrowRight className='h-4 w-4 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100' />
                </Link>
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
          {nodes.length === 0 && (
            <Card className='border-dashed'>
              <CardHeader className='text-center pb-2'>
                <div className='mx-auto mb-2 rounded-full bg-primary/10 p-3 w-fit'>
                  <Zap className='h-6 w-6 text-primary' />
                </div>
                <CardTitle>Get Started with CrossLogic AI</CardTitle>
                <CardDescription className='max-w-md mx-auto'>
                  Deploy your first AI model in minutes. Our platform handles infrastructure so you can focus on building.
                </CardDescription>
              </CardHeader>
              <CardContent className='pt-4'>
                <div className='mx-auto max-w-lg space-y-3'>
                  <div className='flex items-start gap-3'>
                    <CheckCircle2 className='h-5 w-5 text-muted-foreground mt-0.5' />
                    <div>
                      <p className='font-medium text-sm'>1. Create an API Key</p>
                      <p className='text-xs text-muted-foreground'>Generate credentials to authenticate your requests</p>
                    </div>
                  </div>
                  <div className='flex items-start gap-3'>
                    <CheckCircle2 className='h-5 w-5 text-muted-foreground mt-0.5' />
                    <div>
                      <p className='font-medium text-sm'>2. Launch a GPU Instance</p>
                      <p className='text-xs text-muted-foreground'>Select a model and deploy to cloud GPUs</p>
                    </div>
                  </div>
                  <div className='flex items-start gap-3'>
                    <CheckCircle2 className='h-5 w-5 text-muted-foreground mt-0.5' />
                    <div>
                      <p className='font-medium text-sm'>3. Start Making Requests</p>
                      <p className='text-xs text-muted-foreground'>Use our OpenAI-compatible API to generate completions</p>
                    </div>
                  </div>
                </div>
                <div className='mt-6 flex justify-center gap-3'>
                  <Button asChild variant='outline'>
                    <Link to='/api-keys'>Create API Key</Link>
                  </Button>
                  <Button asChild>
                    <Link to='/launch'>
                      <Rocket className='mr-2 h-4 w-4' />
                      Launch Instance
                    </Link>
                  </Button>
                </div>
              </CardContent>
            </Card>
          )}
        </div>
      </Main>
    </>
  )
}
