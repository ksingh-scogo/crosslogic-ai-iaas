import { createFileRoute } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { BarChart3, Cpu, DollarSign, Server } from 'lucide-react'
import { StatCard } from '@/components/common/StatCard'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { fetchUsageHistory, fetchNodeSummaries } from '@/lib/api'

export const Route = createFileRoute('/_authenticated/')({
  component: DashboardPage,
})

const DEFAULT_TENANT = '00000000-0000-0000-0000-000000000000'

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
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Dashboard</h1>
          <p className="text-gray-500 mt-1">Monitor your GPU infrastructure and usage</p>
        </div>
        <div className="flex gap-3">
          <Button variant="outline">View Documentation</Button>
          <Button>Launch Instance</Button>
        </div>
      </div>

      {/* Stats Grid */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Total Tokens (24h)"
          value={totalTokens.toLocaleString()}
          subtitle="Across all models"
          icon={BarChart3}
        />
        <StatCard
          title="Total Requests"
          value={totalRequests.toLocaleString()}
          subtitle="API calls processed"
          icon={Cpu}
        />
        <StatCard
          title="Active Nodes"
          value={`${activeNodes}/${nodes.length}`}
          subtitle="GPU instances running"
          icon={Server}
        />
        <StatCard
          title="Total Cost"
          value={`$${totalCost.toFixed(2)}`}
          subtitle="Last 24 hours"
          icon={DollarSign}
        />
      </div>

      {/* Quick Start */}
      <Card className="border-sky-200 bg-gradient-to-br from-sky-50 to-blue-50">
        <CardHeader>
          <CardTitle className="text-xl">Quick Start Guide</CardTitle>
          <CardDescription>Get started with CrossLogic GPU Platform</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-start gap-3">
            <Badge className="mt-0.5">1</Badge>
            <div>
              <p className="font-medium">Generate an API Key</p>
              <p className="text-sm text-gray-600">Create a new API key from the API Keys page</p>
            </div>
          </div>
          <div className="flex items-start gap-3">
            <Badge className="mt-0.5">2</Badge>
            <div>
              <p className="font-medium">Launch a GPU Instance</p>
              <p className="text-sm text-gray-600">Deploy your AI model on cloud GPUs</p>
            </div>
          </div>
          <div className="flex items-start gap-3">
            <Badge className="mt-0.5">3</Badge>
            <div>
              <p className="font-medium">Make Your First Request</p>
              <p className="text-sm text-gray-600">Use the OpenAI-compatible API to run inference</p>
            </div>
          </div>
          <div className="mt-4 rounded-lg bg-slate-900 p-4 font-mono text-sm text-green-400">
            <code>
              {`curl https://api.crosslogic.ai/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -d '{"model": "mixtral-8x7b", "messages": [...]}'`}
            </code>
          </div>
        </CardContent>
      </Card>

      {/* Recent Usage */}
      <Card>
        <CardHeader>
          <CardTitle>Recent Usage</CardTitle>
          <CardDescription>Last 10 hourly data points</CardDescription>
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
                {usage.slice(0, 10).map((point) => (
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
