import { Bar, BarChart, ResponsiveContainer, XAxis, YAxis } from 'recharts'
import type { UsagePoint } from '@/types'

interface UsageChartProps {
  data: UsagePoint[]
}

export function UsageChart({ data }: UsageChartProps) {
  const chartData = data.slice(0, 12).map((point) => ({
    name: new Date(point.timestamp).toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
    }),
    total: point.totalTokens,
  }))

  return (
    <ResponsiveContainer width='100%' height={350}>
      <BarChart data={chartData}>
        <XAxis
          dataKey='name'
          stroke='#888888'
          fontSize={12}
          tickLine={false}
          axisLine={false}
        />
        <YAxis
          direction='ltr'
          stroke='#888888'
          fontSize={12}
          tickLine={false}
          axisLine={false}
          tickFormatter={(value) => `${(value / 1000).toFixed(0)}K`}
        />
        <Bar
          dataKey='total'
          fill='currentColor'
          radius={[4, 4, 0, 0]}
          className='fill-primary'
        />
      </BarChart>
    </ResponsiveContainer>
  )
}
