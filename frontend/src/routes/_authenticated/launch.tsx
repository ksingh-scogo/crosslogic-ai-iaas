import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { Rocket, Cloud, Zap } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { ConfigDrawer } from '@/components/config-drawer'
import { fetchModels, launchInstance } from '@/lib/api'
import { toast } from 'sonner'

export const Route = createFileRoute('/_authenticated/launch')({
  component: LaunchPage,
})

function LaunchPage() {
  const navigate = useNavigate()
  const [config, setConfig] = useState({
    model_name: '',
    provider: 'azure',
    region: 'eastus',
    instance_type: 'Standard_NC4as_T4_v3',
    use_spot: true,
  })

  const { data: models = [], isLoading } = useQuery({
    queryKey: ['models'],
    queryFn: fetchModels,
  })

  const launchMutation = useMutation({
    mutationFn: launchInstance,
    onSuccess: () => {
      toast.success('Instance launch initiated')
      navigate({ to: '/nodes' })
    },
    onError: () => {
      toast.error('Failed to launch instance')
    },
  })

  const handleLaunch = () => {
    if (!config.model_name) {
      toast.error('Please select a model')
      return
    }
    launchMutation.mutate(config)
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
        <div className='mb-2 flex items-center justify-between space-y-2'>
          <div>
            <h1 className='text-2xl font-bold tracking-tight'>Launch GPU Instance</h1>
            <p className='text-muted-foreground'>Deploy an AI model on cloud GPUs</p>
          </div>
        </div>

        <div className='grid gap-6 lg:grid-cols-3'>
          <div className='lg:col-span-2 space-y-6'>
            <Card>
              <CardHeader>
                <CardTitle>Select Model</CardTitle>
                <CardDescription>Choose the AI model to deploy</CardDescription>
              </CardHeader>
              <CardContent>
                {isLoading ? (
                  <p className='text-muted-foreground'>Loading models...</p>
                ) : (
                  <div className='grid gap-3 sm:grid-cols-2'>
                    {models.slice(0, 6).map((model) => (
                      <div
                        key={model.id}
                        onClick={() => setConfig({ ...config, model_name: model.name })}
                        className={`cursor-pointer rounded-lg border-2 p-4 transition-all ${
                          config.model_name === model.name
                            ? 'border-primary bg-primary/10'
                            : 'border-border hover:border-primary/50'
                        }`}
                      >
                        <h3 className='font-medium'>{model.name}</h3>
                        <p className='text-xs text-muted-foreground mt-1'>
                          {model.family} • {model.size}
                        </p>
                        <p className='text-xs text-muted-foreground mt-2'>
                          VRAM: {model.vram_required_gb}GB
                        </p>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Configuration</CardTitle>
                <CardDescription>Select cloud provider and region</CardDescription>
              </CardHeader>
              <CardContent className='space-y-4'>
                <div className='grid gap-4 sm:grid-cols-3'>
                  {['azure', 'aws', 'gcp'].map((provider) => (
                    <div
                      key={provider}
                      onClick={() => setConfig({ ...config, provider })}
                      className={`cursor-pointer rounded-lg border-2 p-4 text-center transition-all ${
                        config.provider === provider
                          ? 'border-primary bg-primary/10'
                          : 'border-border hover:border-primary/50'
                      }`}
                    >
                      <Cloud className='mx-auto h-6 w-6 mb-2' />
                      <p className='font-medium capitalize'>{provider}</p>
                    </div>
                  ))}
                </div>

                <div className='space-y-2'>
                  <Label htmlFor='region'>Region</Label>
                  <Input
                    id='region'
                    value={config.region}
                    onChange={(e) => setConfig({ ...config, region: e.target.value })}
                  />
                </div>

                <div className='space-y-2'>
                  <Label htmlFor='instance'>Instance Type</Label>
                  <Input
                    id='instance'
                    value={config.instance_type}
                    onChange={(e) => setConfig({ ...config, instance_type: e.target.value })}
                  />
                </div>

                <div className='flex items-center justify-between rounded-lg border p-4'>
                  <div className='flex items-center gap-3'>
                    <Zap className='h-5 w-5' />
                    <div>
                      <p className='font-medium'>Use Spot Instance</p>
                      <p className='text-sm text-muted-foreground'>Save 70-90% on costs</p>
                    </div>
                  </div>
                  <Switch
                    checked={config.use_spot}
                    onCheckedChange={(checked) => setConfig({ ...config, use_spot: checked })}
                  />
                </div>
              </CardContent>
            </Card>
          </div>

          <div>
            <Card className='sticky top-6'>
              <CardHeader>
                <CardTitle>Launch Summary</CardTitle>
              </CardHeader>
              <CardContent className='space-y-4'>
                <div>
                  <p className='text-sm text-muted-foreground'>Model</p>
                  <p className='font-medium'>{config.model_name || 'Not selected'}</p>
                </div>
                <div>
                  <p className='text-sm text-muted-foreground'>Provider</p>
                  <p className='font-medium capitalize'>{config.provider}</p>
                </div>
                <div>
                  <p className='text-sm text-muted-foreground'>Region</p>
                  <p className='font-medium'>{config.region}</p>
                </div>
                <div>
                  <p className='text-sm text-muted-foreground'>Instance</p>
                  <p className='font-medium font-mono text-sm'>{config.instance_type}</p>
                </div>
                <div>
                  <p className='text-sm text-muted-foreground'>Pricing</p>
                  <p className='font-medium'>
                    {config.use_spot ? 'Spot (70-90% off)' : 'On-Demand'}
                  </p>
                </div>
                <Button
                  onClick={handleLaunch}
                  className='w-full'
                  disabled={!config.model_name || launchMutation.isPending}
                >
                  {launchMutation.isPending ? (
                    'Launching...'
                  ) : (
                    <>
                      <Rocket className='mr-2 h-4 w-4' />
                      Launch Instance
                    </>
                  )}
                </Button>
              </CardContent>
            </Card>
          </div>
        </div>
      </Main>
    </>
  )
}
