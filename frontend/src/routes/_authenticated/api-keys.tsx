import { createFileRoute } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Key, Plus, Trash2, Copy } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { fetchApiKeys, createApiKey, revokeApiKey } from '@/lib/api'
import { toast } from 'sonner'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

export const Route = createFileRoute('/_authenticated/api-keys')({
  component: ApiKeysPage,
})

const DEFAULT_TENANT = '00000000-0000-0000-0000-000000000000'

function ApiKeysPage() {
  const queryClient = useQueryClient()
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [newKeyName, setNewKeyName] = useState('')
  const [createdKey, setCreatedKey] = useState('')

  const { data: keys = [], isLoading } = useQuery({
    queryKey: ['api-keys', DEFAULT_TENANT],
    queryFn: () => fetchApiKeys(DEFAULT_TENANT),
  })

  const createMutation = useMutation({
    mutationFn: (name: string) => createApiKey({ tenant_id: DEFAULT_TENANT, name }),
    onSuccess: (data) => {
      setCreatedKey(data.key)
      setNewKeyName('')
      queryClient.invalidateQueries({ queryKey: ['api-keys'] })
      toast.success('API key created successfully')
    },
    onError: () => {
      toast.error('Failed to create API key')
    },
  })

  const revokeMutation = useMutation({
    mutationFn: revokeApiKey,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['api-keys'] })
      toast.success('API key revoked')
    },
  })

  const handleCreate = () => {
    if (!newKeyName.trim()) {
      toast.error('Please enter a key name')
      return
    }
    createMutation.mutate(newKeyName)
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    toast.success('Copied to clipboard')
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">API Keys</h1>
          <p className="text-gray-500 mt-1">Manage your API keys for authentication</p>
        </div>
        <Button onClick={() => setIsCreateOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          Create Key
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Your API Keys</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-gray-500">Loading...</p>
          ) : keys.length === 0 ? (
            <div className="text-center py-8">
              <Key className="mx-auto h-12 w-12 text-gray-400" />
              <p className="mt-4 text-gray-600">No API keys yet</p>
              <p className="text-sm text-gray-500">Create your first API key to get started</p>
            </div>
          ) : (
            <div className="space-y-3">
              {keys.map((key) => (
                <div
                  key={key.id}
                  className="flex items-center justify-between rounded-lg border p-4"
                >
                  <div className="flex-1">
                    <h3 className="font-medium">{key.name}</h3>
                    <p className="text-sm text-gray-500 font-mono">{key.prefix}...</p>
                    <p className="text-xs text-gray-400 mt-1">
                      Created: {new Date(key.created_at).toLocaleDateString()}
                    </p>
                  </div>
                  <div className="flex items-center gap-3">
                    <Badge
                      className={
                        key.status === 'active'
                          ? 'bg-green-100 text-green-800'
                          : 'bg-gray-100 text-gray-800'
                      }
                    >
                      {key.status}
                    </Badge>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => revokeMutation.mutate(key.id)}
                    >
                      <Trash2 className="h-4 w-4 text-red-600" />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create API Key</DialogTitle>
            <DialogDescription>Give your new API key a descriptive name</DialogDescription>
          </DialogHeader>
          {createdKey ? (
            <div className="space-y-4">
              <div className="rounded-lg bg-yellow-50 border border-yellow-200 p-4">
                <p className="text-sm font-medium text-yellow-900">⚠️ Save this key securely</p>
                <p className="text-xs text-yellow-700 mt-1">
                  This key will only be shown once. Store it in a safe place.
                </p>
              </div>
              <div className="space-y-2">
                <Label>Your API Key</Label>
                <div className="flex gap-2">
                  <Input value={createdKey} readOnly className="font-mono text-sm" />
                  <Button onClick={() => copyToClipboard(createdKey)}>
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
              </div>
              <DialogFooter>
                <Button
                  onClick={() => {
                    setCreatedKey('')
                    setIsCreateOpen(false)
                  }}
                >
                  Done
                </Button>
              </DialogFooter>
            </div>
          ) : (
            <>
              <div className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="name">Key Name</Label>
                  <Input
                    id="name"
                    placeholder="e.g., Production API"
                    value={newKeyName}
                    onChange={(e) => setNewKeyName(e.target.value)}
                  />
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsCreateOpen(false)}>
                  Cancel
                </Button>
                <Button onClick={handleCreate} disabled={createMutation.isPending}>
                  {createMutation.isPending ? 'Creating...' : 'Create Key'}
                </Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
