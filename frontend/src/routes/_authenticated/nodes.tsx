import { createFileRoute } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Server, Trash2, Activity } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { StatusBadge } from '@/components/common/StatusBadge'
import { fetchNodeSummaries, terminateNode } from '@/lib/api'
import { toast } from 'sonner'

export const Route = createFileRoute('/_authenticated/nodes')({
  component: NodesPage,
})

function NodesPage() {
  const queryClient = useQueryClient()

  const { data: nodes = [], isLoading } = useQuery({
    queryKey: ['nodes'],
    queryFn: fetchNodeSummaries,
  })

  const terminateMutation = useMutation({
    mutationFn: terminateNode,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['nodes'] })
      toast.success('Node termination initiated')
    },
    onError: () => {
      toast.error('Failed to terminate node')
    },
  })

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">GPU Nodes</h1>
          <p className="text-gray-500 mt-1">Manage your active GPU instances</p>
        </div>
        <Button>Launch New Instance</Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Active Nodes</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-gray-500">Loading...</p>
          ) : nodes.length === 0 ? (
            <div className="text-center py-8">
              <Server className="mx-auto h-12 w-12 text-gray-400" />
              <p className="mt-4 text-gray-600">No nodes running</p>
              <p className="text-sm text-gray-500">Launch an instance to get started</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="border-b">
                  <tr className="text-left">
                    <th className="pb-3 font-medium text-gray-600">Cluster</th>
                    <th className="pb-3 font-medium text-gray-600">Provider</th>
                    <th className="pb-3 font-medium text-gray-600">Model</th>
                    <th className="pb-3 font-medium text-gray-600">Status</th>
                    <th className="pb-3 font-medium text-gray-600 text-right">Health</th>
                    <th className="pb-3 font-medium text-gray-600">Last Heartbeat</th>
                    <th className="pb-3 font-medium text-gray-600 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  {nodes.map((node) => (
                    <tr key={node.id} className="text-gray-900">
                      <td className="py-3">
                        <p className="font-medium">{node.clusterName || node.id}</p>
                        <p className="text-xs text-gray-500 font-mono">{node.instanceType}</p>
                      </td>
                      <td className="py-3 capitalize">{node.provider}</td>
                      <td className="py-3 font-mono text-sm">{node.model || 'N/A'}</td>
                      <td className="py-3">
                        <StatusBadge
                          status={node.status === 'active' ? 'active' : 'inactive'}
                        />
                      </td>
                      <td className="py-3 text-right">
                        <div className="flex items-center justify-end gap-2">
                          <Activity className="h-4 w-4 text-green-600" />
                          <span className="font-mono">{node.health}%</span>
                        </div>
                      </td>
                      <td className="py-3 text-sm text-gray-600">
                        {new Date(node.lastHeartbeat).toLocaleString()}
                      </td>
                      <td className="py-3 text-right">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => node.clusterName && terminateMutation.mutate(node.clusterName)}
                          disabled={!node.clusterName}
                        >
                          <Trash2 className="h-4 w-4 text-red-600" />
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
