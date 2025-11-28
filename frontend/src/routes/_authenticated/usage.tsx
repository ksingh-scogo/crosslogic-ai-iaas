import { createFileRoute } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { BarChart3, Activity, DollarSign, TrendingUp, Calendar, Download, Zap } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { ConfigDrawer } from '@/components/config-drawer'
import { StatCard } from '@/components/dashboard/stat-card'
import { fetchUsageHistory } from '@/lib/api'
import { XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Area, AreaChart } from 'recharts'
import { cn } from '@/lib/utils'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

export const Route = createFileRoute('/_authenticated/usage')({
  component: UsagePage,
})

const DEFAULT_TENANT = '00000000-0000-0000-0000-000000000000'

const TIME_RANGES = [
  { label: '24h', value: '24h', hours: 24 },
  { label: '7d', value: '7d', hours: 168 },
  { label: '30d', value: '30d', hours: 720 },
  { label: '90d', value: '90d', hours: 2160 },
] as const

function UsagePage() {
  const [selectedRange, setSelectedRange] = useState<string>('24h')

  const { data: usage = [], isLoading } = useQuery({
    queryKey: ['usage', DEFAULT_TENANT, selectedRange],
    queryFn: () => fetchUsageHistory(DEFAULT_TENANT),
  })

  const totalTokens = usage.reduce((acc, point) => acc + point.totalTokens, 0)
  const totalRequests = usage.reduce((acc, point) => acc + (point.requests || 0), 0)
  const totalCost = usage.reduce(
    (acc, point) => acc + parseFloat(point.totalCost.replace('$', '')),
    0
  )
  const avgTokensPerRequest = totalRequests > 0 ? Math.round(totalTokens / totalRequests) : 0

  return (
    <>
      <Header>
        <div className='ms-auto flex items-center space-x-4'>
          <Search />
          <ThemeSwitch />
          <ConfigDrawer />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='mb-6 flex items-center justify-between'>
          <div>
            <h1 className='text-2xl font-bold tracking-tight'>Usage & Billing</h1>
            <p className='text-muted-foreground'>Track your API usage and costs over time</p>
          </div>
          <div className='flex items-center gap-3'>
            {/* Time Range Selector */}
            <div className='flex items-center rounded-lg border bg-muted/50 p-1'>
              {TIME_RANGES.map((range) => (
                <button
                  key={range.value}
                  onClick={() => setSelectedRange(range.value)}
                  className={cn(
                    'px-3 py-1.5 text-sm font-medium rounded-md transition-all duration-200',
                    selectedRange === range.value
                      ? 'bg-background text-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground'
                  )}
                >
                  {range.label}
                </button>
              ))}
            </div>
            <Button variant='outline' size='sm'>
              <Download className='mr-2 h-4 w-4' />
              Export
            </Button>
          </div>
        </div>

        {/* Stats Grid */}
        <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4 mb-6'>
          <StatCard
            title='Total Tokens'
            value={totalTokens.toLocaleString()}
            description={`Last ${selectedRange}`}
            icon={Activity}
            variant='primary'
          />
          <StatCard
            title='Total Requests'
            value={totalRequests.toLocaleString()}
            description='API calls processed'
            icon={Zap}
            variant='info'
          />
          <StatCard
            title='Avg Tokens/Request'
            value={avgTokensPerRequest.toLocaleString()}
            description='Average efficiency'
            icon={TrendingUp}
            variant='success'
          />
          <StatCard
            title='Total Cost'
            value={`$${totalCost.toFixed(2)}`}
            description={`Last ${selectedRange}`}
            icon={DollarSign}
            variant='warning'
          />
        </div>

        {/* Chart */}
        <Card className='relative overflow-hidden mb-6'>
          <div className='absolute inset-0 bg-gradient-to-br from-primary/5 via-transparent to-transparent pointer-events-none' />
          <CardHeader className='relative z-10'>
            <div className='flex items-center justify-between'>
              <div className='flex items-center gap-3'>
                <div className='p-2 rounded-lg bg-primary/10'>
                  <BarChart3 className='h-5 w-5 text-primary' />
                </div>
                <div>
                  <CardTitle>Usage Over Time</CardTitle>
                  <CardDescription>Token consumption trend</CardDescription>
                </div>
              </div>
            </div>
          </CardHeader>
          <CardContent className='relative z-10'>
            {isLoading ? (
              <div className='h-80 flex items-center justify-center'>
                <div className='flex flex-col items-center gap-3'>
                  <div className='h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent' />
                  <p className='text-sm text-muted-foreground'>Loading chart data...</p>
                </div>
              </div>
            ) : usage.length === 0 ? (
              <div className='h-80 flex items-center justify-center'>
                <div className='text-center'>
                  <div className='mx-auto mb-4 rounded-full bg-muted/50 p-4 w-fit ring-8 ring-muted/30'>
                    <BarChart3 className='h-8 w-8 text-muted-foreground/80' />
                  </div>
                  <h3 className='text-lg font-semibold mb-2'>No usage data</h3>
                  <p className='text-sm text-muted-foreground max-w-sm mx-auto'>
                    Start making API calls to see your usage analytics here
                  </p>
                </div>
              </div>
            ) : (
              <ResponsiveContainer width='100%' height={320}>
                <AreaChart data={usage}>
                  <defs>
                    <linearGradient id='tokenGradient' x1='0' y1='0' x2='0' y2='1'>
                      <stop offset='5%' stopColor='hsl(var(--primary))' stopOpacity={0.3} />
                      <stop offset='95%' stopColor='hsl(var(--primary))' stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray='3 3' className='stroke-muted' />
                  <XAxis
                    dataKey='timestamp'
                    tickFormatter={(value) => new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    className='text-xs'
                    stroke='hsl(var(--muted-foreground))'
                    fontSize={12}
                  />
                  <YAxis
                    className='text-xs'
                    stroke='hsl(var(--muted-foreground))'
                    fontSize={12}
                    tickFormatter={(value) => value >= 1000 ? `${(value / 1000).toFixed(0)}k` : value}
                  />
                  <Tooltip
                    content={({ active, payload }) => {
                      if (active && payload && payload.length) {
                        return (
                          <div className='bg-popover border border-border p-3 rounded-lg shadow-xl'>
                            <p className='text-sm font-semibold text-foreground'>
                              {new Date(payload[0].payload.timestamp).toLocaleString()}
                            </p>
                            <div className='mt-2 space-y-1'>
                              <p className='text-sm text-muted-foreground flex items-center gap-2'>
                                <span className='h-2 w-2 rounded-full bg-primary' />
                                Tokens: <span className='font-mono font-medium text-foreground'>{payload[0].value?.toLocaleString()}</span>
                              </p>
                              <p className='text-sm text-muted-foreground flex items-center gap-2'>
                                <span className='h-2 w-2 rounded-full bg-green-500' />
                                Cost: <span className='font-mono font-medium text-green-600 dark:text-green-400'>{payload[0].payload.totalCost}</span>
                              </p>
                            </div>
                          </div>
                        )
                      }
                      return null
                    }}
                  />
                  <Area
                    type='monotone'
                    dataKey='totalTokens'
                    stroke='hsl(var(--primary))'
                    strokeWidth={2}
                    fill='url(#tokenGradient)'
                  />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        {/* Detailed Usage Table */}
        <Card className='relative overflow-hidden'>
          <div className='absolute inset-0 bg-gradient-to-br from-muted/30 via-transparent to-transparent pointer-events-none' />
          <CardHeader className='relative z-10'>
            <div className='flex items-center gap-3'>
              <div className='p-2 rounded-lg bg-muted'>
                <Calendar className='h-5 w-5 text-muted-foreground' />
              </div>
              <div>
                <CardTitle>Detailed Usage</CardTitle>
                <CardDescription>Hourly breakdown of your API usage</CardDescription>
              </div>
            </div>
          </CardHeader>
          <CardContent className='relative z-10'>
            {isLoading ? (
              <div className='space-y-3'>
                {Array.from({ length: 5 }).map((_, i) => (
                  <div key={i} className='h-12 rounded-lg bg-muted animate-pulse' />
                ))}
              </div>
            ) : usage.length === 0 ? (
              <div className='text-center py-8'>
                <p className='text-sm text-muted-foreground'>No usage data available</p>
              </div>
            ) : (
              <div className='rounded-lg border'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Timestamp</TableHead>
                      <TableHead className='text-right'>Tokens</TableHead>
                      <TableHead className='text-right'>Requests</TableHead>
                      <TableHead className='text-right'>Cost</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {usage.map((point) => (
                      <TableRow key={point.timestamp}>
                        <TableCell>
                          <time className='text-sm'>
                            {new Date(point.timestamp).toLocaleString()}
                          </time>
                        </TableCell>
                        <TableCell className='text-right'>
                          <code className='text-xs font-mono bg-muted px-2 py-0.5 rounded'>
                            {point.totalTokens.toLocaleString()}
                          </code>
                        </TableCell>
                        <TableCell className='text-right'>
                          <code className='text-xs font-mono bg-muted px-2 py-0.5 rounded'>
                            {point.requests || 0}
                          </code>
                        </TableCell>
                        <TableCell className='text-right'>
                          <span className='text-sm font-mono font-medium text-green-600 dark:text-green-400'>
                            {point.totalCost}
                          </span>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>
      </Main>
    </>
  )
}
