import { createFileRoute } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { BarChart3 } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { fetchUsageHistory } from '@/lib/api'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'

export const Route = createFileRoute('/_authenticated/usage')({
  component: UsagePage,
})

const DEFAULT_TENANT = '00000000-0000-0000-0000-000000000000'

function UsagePage() {
  const { data: usage = [], isLoading } = useQuery({
    queryKey: ['usage', DEFAULT_TENANT],
    queryFn: () => fetchUsageHistory(DEFAULT_TENANT),
  })

  const totalTokens = usage.reduce((acc, point) => acc + point.totalTokens, 0)
  const totalCost = usage.reduce(
    (acc, point) => acc + parseFloat(point.totalCost.replace('$', '')),
    0
  )

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-gray-900">Usage & Billing</h1>
        <p className="text-gray-500 mt-1">Track your API usage and costs</p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Total Tokens</CardTitle>
            <CardDescription>Last 24 hours</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold">{totalTokens.toLocaleString()}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Total Cost</CardTitle>
            <CardDescription>Last 24 hours</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold text-green-600">${totalCost.toFixed(4)}</div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Usage Over Time</CardTitle>
          <CardDescription>Hourly token usage</CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="h-80 flex items-center justify-center">
              <p className="text-gray-500">Loading chart...</p>
            </div>
          ) : usage.length === 0 ? (
            <div className="h-80 flex items-center justify-center">
              <div className="text-center">
                <BarChart3 className="mx-auto h-12 w-12 text-gray-400" />
                <p className="mt-4 text-gray-600">No usage data</p>
              </div>
            </div>
          ) : (
            <ResponsiveContainer width="100%" height={320}>
              <LineChart data={usage}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-gray-200" />
                <XAxis
                  dataKey="timestamp"
                  tickFormatter={(value) => new Date(value).toLocaleTimeString()}
                  className="text-sm"
                />
                <YAxis className="text-sm" />
                <Tooltip
                  content={({ active, payload }) => {
                    if (active && payload && payload.length) {
                      return (
                        <div className="bg-white p-3 border rounded-lg shadow-lg">
                          <p className="text-sm font-medium">
                            {new Date(payload[0].payload.timestamp).toLocaleString()}
                          </p>
                          <p className="text-sm text-gray-600 mt-1">
                            Tokens: {payload[0].value?.toLocaleString()}
                          </p>
                          <p className="text-sm text-green-600">
                            Cost: {payload[0].payload.totalCost}
                          </p>
                        </div>
                      )
                    }
                    return null
                  }}
                />
                <Line
                  type="monotone"
                  dataKey="totalTokens"
                  stroke="#0ea5e9"
                  strokeWidth={2}
                  dot={false}
                />
              </LineChart>
            </ResponsiveContainer>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Detailed Usage</CardTitle>
          <CardDescription>Hourly breakdown</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b">
                <tr className="text-left">
                  <th className="pb-3 font-medium text-gray-600">Timestamp</th>
                  <th className="pb-3 font-medium text-gray-600 text-right">Tokens</th>
                  <th className="pb-3 font-medium text-gray-600 text-right">Requests</th>
                  <th className="pb-3 font-medium text-gray-600 text-right">Cost</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {usage.map((point) => (
                  <tr key={point.timestamp} className="text-gray-900">
                    <td className="py-3">{new Date(point.timestamp).toLocaleString()}</td>
                    <td className="py-3 text-right font-mono">{point.totalTokens.toLocaleString()}</td>
                    <td className="py-3 text-right font-mono">{point.requests || 0}</td>
                    <td className="py-3 text-right font-mono text-green-600">{point.totalCost}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
