import { createFileRoute } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState, useCallback } from 'react'
import { Key, Plus, Trash2, Copy, Check, Shield, Clock, Search as SearchIcon, Sparkles, AlertTriangle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { ConfigDrawer } from '@/components/config-drawer'
import { StatusBadge } from '@/components/common/StatusBadge'
import { fetchApiKeys, createApiKey, revokeApiKey } from '@/lib/api'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

export const Route = createFileRoute('/_authenticated/api-keys')({
  component: ApiKeysPage,
})

const DEFAULT_TENANT = '00000000-0000-0000-0000-000000000000'

function getRelativeTime(date: Date): string {
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSecs = Math.floor(diffMs / 1000)
  const diffMins = Math.floor(diffSecs / 60)
  const diffHours = Math.floor(diffMins / 60)
  const diffDays = Math.floor(diffHours / 24)

  if (diffDays > 30) return date.toLocaleDateString()
  if (diffDays > 0) return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`
  if (diffHours > 0) return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`
  if (diffMins > 0) return `${diffMins} minute${diffMins > 1 ? 's' : ''} ago`
  return 'Just now'
}

function ApiKeysPage() {
  const queryClient = useQueryClient()
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [newKeyName, setNewKeyName] = useState('')
  const [createdKey, setCreatedKey] = useState('')
  const [deleteKeyId, setDeleteKeyId] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [copiedId, setCopiedId] = useState<string | null>(null)

  const { data: keys = [], isLoading } = useQuery({
    queryKey: ['api-keys', DEFAULT_TENANT],
    queryFn: () => fetchApiKeys(DEFAULT_TENANT),
  })

  const filteredKeys = keys.filter((key) =>
    key.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    key.prefix.toLowerCase().includes(searchQuery.toLowerCase())
  )

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
      setDeleteKeyId(null)
    },
  })

  const handleCreate = () => {
    if (!newKeyName.trim()) {
      toast.error('Please enter a key name')
      return
    }
    createMutation.mutate(newKeyName)
  }

  const copyToClipboard = useCallback((text: string, id?: string) => {
    navigator.clipboard.writeText(text)
    if (id) {
      setCopiedId(id)
      setTimeout(() => setCopiedId(null), 2000)
    }
    toast.success('Copied to clipboard')
  }, [])

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
            <h1 className='text-2xl font-bold tracking-tight'>API Keys</h1>
            <p className='text-muted-foreground'>Manage your API keys for secure authentication</p>
          </div>
          <Button
            onClick={() => setIsCreateOpen(true)}
            className='shadow-lg shadow-primary/20'
          >
            <Plus className='mr-2 h-4 w-4' />
            Create Key
          </Button>
        </div>

        <Card className='relative overflow-hidden'>
          <div className='absolute inset-0 bg-gradient-to-br from-primary/5 via-transparent to-transparent pointer-events-none' />
          <CardHeader className='pb-4 relative z-10'>
            <div className='flex items-center justify-between gap-4'>
              <div className='flex items-center gap-3'>
                <div className='p-2 rounded-lg bg-primary/10'>
                  <Shield className='h-5 w-5 text-primary' />
                </div>
                <div>
                  <CardTitle>Your API Keys</CardTitle>
                  <CardDescription className='mt-0.5'>
                    {keys.length} key{keys.length !== 1 ? 's' : ''} configured
                  </CardDescription>
                </div>
              </div>
              {keys.length > 0 && (
                <div className='relative w-64'>
                  <SearchIcon className='absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground' />
                  <Input
                    placeholder='Search keys...'
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    className='pl-9'
                  />
                </div>
              )}
            </div>
          </CardHeader>
          <CardContent className='relative z-10'>
            {isLoading ? (
              <div className='space-y-3'>
                {Array.from({ length: 3 }).map((_, i) => (
                  <div key={i} className='rounded-lg border p-4 animate-pulse'>
                    <div className='flex items-center gap-4'>
                      <div className='h-10 w-10 rounded-lg bg-muted' />
                      <div className='flex-1 space-y-2'>
                        <div className='h-4 w-32 rounded bg-muted' />
                        <div className='h-3 w-48 rounded bg-muted' />
                      </div>
                      <div className='h-6 w-16 rounded-full bg-muted' />
                    </div>
                  </div>
                ))}
              </div>
            ) : keys.length === 0 ? (
              <div className='text-center py-12'>
                <div className='mx-auto mb-4 rounded-full bg-muted/50 p-4 w-fit ring-8 ring-muted/30'>
                  <Key className='h-8 w-8 text-muted-foreground/80' />
                </div>
                <h3 className='text-lg font-semibold mb-2'>No API keys yet</h3>
                <p className='text-sm text-muted-foreground max-w-sm mx-auto mb-6'>
                  Create your first API key to authenticate requests to the CrossLogic API
                </p>
                <Button onClick={() => setIsCreateOpen(true)}>
                  <Sparkles className='mr-2 h-4 w-4' />
                  Create Your First Key
                </Button>
              </div>
            ) : filteredKeys.length === 0 ? (
              <div className='text-center py-8'>
                <SearchIcon className='mx-auto h-8 w-8 text-muted-foreground/50 mb-3' />
                <p className='text-sm text-muted-foreground'>No keys match your search</p>
              </div>
            ) : (
              <div className='space-y-3'>
                {filteredKeys.map((key, index) => (
                  <div
                    key={key.id}
                    className={cn(
                      'group relative flex items-center justify-between rounded-xl border p-4',
                      'bg-card/50 backdrop-blur-sm',
                      'hover:border-primary/30 hover:shadow-md hover:shadow-primary/5',
                      'transition-all duration-300'
                    )}
                    style={{ animationDelay: `${index * 50}ms` }}
                  >
                    <div className='flex items-center gap-4'>
                      <div className={cn(
                        'flex h-10 w-10 items-center justify-center rounded-lg',
                        key.status === 'active'
                          ? 'bg-gradient-to-br from-green-500/20 to-green-600/10'
                          : 'bg-muted'
                      )}>
                        <Key className={cn(
                          'h-5 w-5',
                          key.status === 'active' ? 'text-green-600 dark:text-green-400' : 'text-muted-foreground'
                        )} />
                      </div>
                      <div className='flex-1'>
                        <h3 className='font-semibold text-sm'>{key.name}</h3>
                        <div className='flex items-center gap-2 mt-1'>
                          <code className='text-xs font-mono bg-muted px-2 py-0.5 rounded text-muted-foreground'>
                            {key.prefix}...
                          </code>
                          <Button
                            variant='ghost'
                            size='sm'
                            className='h-6 w-6 p-0 opacity-0 group-hover:opacity-100 transition-opacity'
                            onClick={() => copyToClipboard(key.prefix, key.id)}
                          >
                            {copiedId === key.id ? (
                              <Check className='h-3 w-3 text-green-500' />
                            ) : (
                              <Copy className='h-3 w-3' />
                            )}
                          </Button>
                        </div>
                        <div className='flex items-center gap-1.5 mt-1.5 text-xs text-muted-foreground'>
                          <Clock className='h-3 w-3' />
                          <span>Created {getRelativeTime(new Date(key.created_at))}</span>
                        </div>
                      </div>
                    </div>
                    <div className='flex items-center gap-3'>
                      <StatusBadge
                        status={key.status === 'active' ? 'active' : 'inactive'}
                        showPulse={key.status === 'active'}
                      />
                      <Button
                        variant='ghost'
                        size='icon'
                        className='h-8 w-8 opacity-0 group-hover:opacity-100 transition-opacity hover:bg-destructive/10'
                        onClick={() => setDeleteKeyId(key.id)}
                      >
                        <Trash2 className='h-4 w-4 text-destructive' />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}

            {filteredKeys.length > 0 && (
              <div className='flex items-center justify-between mt-4 pt-4 border-t'>
                <p className='text-sm text-muted-foreground'>
                  Showing {filteredKeys.length} of {keys.length} keys
                </p>
              </div>
            )}
          </CardContent>
        </Card>

        <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
          <DialogContent className='sm:max-w-md'>
            <DialogHeader>
              <div className='flex items-center gap-3'>
                <div className='p-2 rounded-lg bg-primary/10'>
                  <Key className='h-5 w-5 text-primary' />
                </div>
                <div>
                  <DialogTitle>Create API Key</DialogTitle>
                  <DialogDescription className='mt-0.5'>
                    Give your new API key a descriptive name
                  </DialogDescription>
                </div>
              </div>
            </DialogHeader>
            {createdKey ? (
              <div className='space-y-4 pt-2'>
                <div className='rounded-xl bg-gradient-to-br from-amber-500/10 to-orange-500/10 border border-amber-500/20 p-4'>
                  <div className='flex items-start gap-3'>
                    <div className='mt-0.5'>
                      <AlertTriangle className='h-5 w-5 text-amber-500' />
                    </div>
                    <div>
                      <p className='text-sm font-semibold text-amber-600 dark:text-amber-400'>
                        Save this key securely
                      </p>
                      <p className='text-xs text-muted-foreground mt-1'>
                        This key will only be shown once. Store it somewhere safe.
                      </p>
                    </div>
                  </div>
                </div>
                <div className='space-y-2'>
                  <Label className='text-xs font-medium uppercase tracking-wider text-muted-foreground'>
                    Your API Key
                  </Label>
                  <div className='flex gap-2'>
                    <Input
                      value={createdKey}
                      readOnly
                      className='font-mono text-sm bg-muted/50'
                    />
                    <Button
                      onClick={() => copyToClipboard(createdKey, 'new-key')}
                      size='icon'
                      variant={copiedId === 'new-key' ? 'default' : 'outline'}
                      className={cn(
                        'shrink-0 transition-all duration-300',
                        copiedId === 'new-key' && 'bg-green-500 hover:bg-green-600 border-green-500'
                      )}
                    >
                      {copiedId === 'new-key' ? (
                        <Check className='h-4 w-4' />
                      ) : (
                        <Copy className='h-4 w-4' />
                      )}
                    </Button>
                  </div>
                </div>
                <DialogFooter className='pt-2'>
                  <Button
                    onClick={() => {
                      setCreatedKey('')
                      setIsCreateOpen(false)
                    }}
                    className='w-full sm:w-auto'
                  >
                    <Check className='mr-2 h-4 w-4' />
                    Done
                  </Button>
                </DialogFooter>
              </div>
            ) : (
              <>
                <div className='space-y-4 pt-2'>
                  <div className='space-y-2'>
                    <Label htmlFor='name' className='text-xs font-medium uppercase tracking-wider text-muted-foreground'>
                      Key Name
                    </Label>
                    <Input
                      id='name'
                      placeholder='e.g., Production API, Development Key'
                      value={newKeyName}
                      onChange={(e) => setNewKeyName(e.target.value)}
                      onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
                      className='bg-muted/50'
                    />
                    <p className='text-xs text-muted-foreground'>
                      Choose a name that helps you identify this key's purpose
                    </p>
                  </div>
                </div>
                <DialogFooter className='pt-2 gap-2 sm:gap-0'>
                  <Button variant='outline' onClick={() => setIsCreateOpen(false)}>
                    Cancel
                  </Button>
                  <Button
                    onClick={handleCreate}
                    disabled={createMutation.isPending || !newKeyName.trim()}
                    className='shadow-lg shadow-primary/20'
                  >
                    {createMutation.isPending ? (
                      <>
                        <div className='h-4 w-4 mr-2 animate-spin rounded-full border-2 border-current border-t-transparent' />
                        Creating...
                      </>
                    ) : (
                      <>
                        <Sparkles className='mr-2 h-4 w-4' />
                        Create Key
                      </>
                    )}
                  </Button>
                </DialogFooter>
              </>
            )}
          </DialogContent>
        </Dialog>

        <AlertDialog open={!!deleteKeyId} onOpenChange={() => setDeleteKeyId(null)}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <div className='flex items-center gap-3 mb-2'>
                <div className='p-2 rounded-lg bg-destructive/10'>
                  <Trash2 className='h-5 w-5 text-destructive' />
                </div>
                <AlertDialogTitle>Revoke API Key?</AlertDialogTitle>
              </div>
              <AlertDialogDescription className='text-sm'>
                This action cannot be undone. All applications using this key will immediately lose access to the API.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter className='gap-2 sm:gap-0'>
              <AlertDialogCancel>Keep Key</AlertDialogCancel>
              <AlertDialogAction
                onClick={() => deleteKeyId && revokeMutation.mutate(deleteKeyId)}
                className='bg-destructive hover:bg-destructive/90 shadow-lg shadow-destructive/20'
              >
                {revokeMutation.isPending ? (
                  <>
                    <div className='h-4 w-4 mr-2 animate-spin rounded-full border-2 border-current border-t-transparent' />
                    Revoking...
                  </>
                ) : (
                  'Yes, Revoke Key'
                )}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </Main>
    </>
  )
}
