import { createFileRoute, Link } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Server, Trash2, MoreHorizontal, ExternalLink, RefreshCw, Search as SearchIcon, Rocket, Copy } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Badge } from '@/components/ui/badge'
import { StatusBadge } from '@/components/common/StatusBadge'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { ConfigDrawer } from '@/components/config-drawer'
import { fetchNodeSummaries, terminateNode } from '@/lib/api'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/_authenticated/nodes')({
  component: NodesPage,
})

function NodesPage() {
  const queryClient = useQueryClient()
  const [searchQuery, setSearchQuery] = useState('')

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

  const filteredNodes = nodes.filter((node) =>
    (node.clusterName || node.id).toLowerCase().includes(searchQuery.toLowerCase()) ||
    node.provider.toLowerCase().includes(searchQuery.toLowerCase()) ||
    (node.model || '').toLowerCase().includes(searchQuery.toLowerCase())
  )

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    toast.success('Copied to clipboard')
  }

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
            <h1 className='text-2xl font-bold tracking-tight'>GPU Nodes</h1>
            <p className='text-muted-foreground'>Manage your active GPU instances</p>
          </div>
          <Button asChild className='shadow-lg shadow-primary/20'>
            <Link to='/launch'>
              <Rocket className='mr-2 h-4 w-4' />
              Launch Instance
            </Link>
          </Button>
        </div>

        <Card>
          <CardHeader className='pb-4'>
            <div className='flex items-center justify-between gap-4'>
              <div>
                <CardTitle>Active Nodes</CardTitle>
                <CardDescription className='mt-1'>
                  {nodes.length} node{nodes.length !== 1 ? 's' : ''} deployed
                </CardDescription>
              </div>
              <div className='relative w-64'>
                <SearchIcon className='absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground' />
                <Input
                  placeholder='Search nodes...'
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className='pl-9'
                />
              </div>
            </div>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className='space-y-3'>
                {Array.from({ length: 3 }).map((_, i) => (
                  <div key={i} className='rounded-lg border p-4 animate-pulse'>
                    <div className='flex items-center gap-4'>
                      <div className='h-10 w-10 rounded-lg bg-muted' />
                      <div className='flex-1 space-y-2'>
                        <div className='h-4 w-32 rounded bg-muted' />
                        <div className='h-3 w-24 rounded bg-muted' />
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            ) : nodes.length === 0 ? (
              <div className='text-center py-12'>
                <div className='mx-auto mb-4 rounded-full bg-muted/50 p-4 w-fit ring-8 ring-muted/30'>
                  <Server className='h-8 w-8 text-muted-foreground/80' />
                </div>
                <h3 className='text-lg font-semibold mb-2'>No nodes running</h3>
                <p className='text-sm text-muted-foreground max-w-sm mx-auto mb-6'>
                  Launch an instance to get started with AI model deployment
                </p>
                <Button asChild>
                  <Link to='/launch'>
                    <Rocket className='mr-2 h-4 w-4' />
                    Launch Instance
                  </Link>
                </Button>
              </div>
            ) : (
              <div className='rounded-lg border'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Cluster</TableHead>
                      <TableHead>Provider</TableHead>
                      <TableHead>Model</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead className='text-right'>Health</TableHead>
                      <TableHead>Last Heartbeat</TableHead>
                      <TableHead className='text-right'>Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filteredNodes.map((node) => (
                      <TableRow key={node.id}>
                        <TableCell>
                          <div className='space-y-1'>
                            <p className='font-semibold text-sm'>{node.clusterName || node.id}</p>
                            <code className='text-xs text-muted-foreground font-mono bg-muted/50 px-1.5 py-0.5 rounded'>
                              {node.instanceType}
                            </code>
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant='outline' className='capitalize font-medium'>
                            {node.provider}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <code className='text-xs font-mono bg-muted px-2 py-1 rounded'>
                            {node.model || 'N/A'}
                          </code>
                        </TableCell>
                        <TableCell>
                          <StatusBadge
                            status={node.status === 'active' ? 'active' : 'inactive'}
                          />
                        </TableCell>
                        <TableCell className='text-right'>
                          <div className='inline-flex items-center gap-2'>
                            <div className='w-16 h-1.5 bg-muted rounded-full overflow-hidden'>
                              <div
                                className={cn(
                                  'h-full transition-all duration-500',
                                  node.health >= 80 ? 'bg-green-500' : node.health >= 50 ? 'bg-yellow-500' : 'bg-red-500'
                                )}
                                style={{ width: `${node.health}%` }}
                              />
                            </div>
                            <span className='font-mono text-xs font-semibold w-10 text-right'>
                              {node.health}%
                            </span>
                          </div>
                        </TableCell>
                        <TableCell>
                          <time className='text-sm text-muted-foreground'>
                            {new Date(node.lastHeartbeat).toLocaleString()}
                          </time>
                        </TableCell>
                        <TableCell className='text-right'>
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button variant='ghost' className='h-8 w-8 p-0'>
                                <span className='sr-only'>Open menu</span>
                                <MoreHorizontal className='h-4 w-4' />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align='end' className='w-48'>
                              <DropdownMenuLabel>Actions</DropdownMenuLabel>
                              <DropdownMenuItem onClick={() => copyToClipboard(node.id)}>
                                <Copy className='mr-2 h-4 w-4' />
                                Copy node ID
                              </DropdownMenuItem>
                              <DropdownMenuSeparator />
                              <DropdownMenuItem>
                                <RefreshCw className='mr-2 h-4 w-4' />
                                Refresh status
                              </DropdownMenuItem>
                              <DropdownMenuItem>
                                <ExternalLink className='mr-2 h-4 w-4' />
                                View logs
                              </DropdownMenuItem>
                              <DropdownMenuSeparator />
                              <DropdownMenuItem
                                onClick={() => node.clusterName && terminateMutation.mutate(node.clusterName)}
                                disabled={!node.clusterName}
                                className='text-destructive focus:text-destructive'
                              >
                                <Trash2 className='mr-2 h-4 w-4' />
                                Terminate node
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}

            {filteredNodes.length > 0 && (
              <div className='flex items-center justify-between mt-4 pt-4 border-t'>
                <p className='text-sm text-muted-foreground'>
                  Showing {filteredNodes.length} of {nodes.length} nodes
                </p>
              </div>
            )}
          </CardContent>
        </Card>
      </Main>
    </>
  )
}
