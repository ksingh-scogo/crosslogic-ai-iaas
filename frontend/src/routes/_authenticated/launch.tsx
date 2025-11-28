import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState, useEffect } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { Rocket, Zap, MapPin, Cpu, HardDrive, Brain, CheckCircle2, Sparkles } from 'lucide-react'

// Cloud provider icons
import awsIcon from '@/assets/images/aws_icon.png'
import azureIcon from '@/assets/images/azure_icon.png'
import gcpIcon from '@/assets/images/gcp_icon.png'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { ConfigDrawer } from '@/components/config-drawer'
import { fetchModels, launchInstance, fetchRegions, fetchInstanceTypes } from '@/lib/api'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/_authenticated/launch')({
  component: LaunchPage,
})

function LaunchPage() {
  const navigate = useNavigate()
  const [config, setConfig] = useState({
    model_name: '',
    provider: 'azure',
    region: '',
    instance_type: '',
    use_spot: true,
  })

  const { data: models = [], isLoading } = useQuery({
    queryKey: ['models'],
    queryFn: fetchModels,
  })

  // Fetch regions when provider changes
  const { data: regions = [], isLoading: regionsLoading } = useQuery({
    queryKey: ['regions', config.provider],
    queryFn: () => fetchRegions(config.provider),
    enabled: !!config.provider,
  })

  // Fetch instance types when provider and region change
  const { data: instanceTypes = [], isLoading: instanceTypesLoading } = useQuery({
    queryKey: ['instanceTypes', config.provider, config.region],
    queryFn: () => fetchInstanceTypes(config.provider, config.region),
    enabled: !!config.provider && !!config.region,
  })

  // Reset region and instance_type when provider changes
  useEffect(() => {
    setConfig((prev) => ({ ...prev, region: '', instance_type: '' }))
  }, [config.provider])

  // Reset instance_type when region changes
  useEffect(() => {
    setConfig((prev) => ({ ...prev, instance_type: '' }))
  }, [config.region])

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
    if (!config.region) {
      toast.error('Please select a region')
      return
    }
    if (!config.instance_type) {
      toast.error('Please select an instance type')
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
                  <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
                    {Array.from({ length: 6 }).map((_, i) => (
                      <div key={i} className='rounded-xl border-2 p-5 animate-pulse'>
                        <div className='flex items-start gap-3'>
                          <div className='h-10 w-10 rounded-lg bg-muted' />
                          <div className='flex-1 space-y-2'>
                            <div className='h-4 w-24 rounded bg-muted' />
                            <div className='h-3 w-16 rounded bg-muted' />
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
                    {models.slice(0, 6).map((model) => (
                      <div
                        key={model.id}
                        onClick={() => setConfig({ ...config, model_name: model.name })}
                        className={cn(
                          'group cursor-pointer rounded-xl border-2 p-5 transition-all duration-300',
                          'hover:shadow-lg hover:-translate-y-1',
                          config.model_name === model.name
                            ? 'border-primary bg-primary/5 shadow-lg shadow-primary/10'
                            : 'border-border hover:border-primary/40 hover:bg-accent/30'
                        )}
                      >
                        <div className='flex items-start justify-between mb-3'>
                          <div className='flex items-start gap-3'>
                            <div className={cn(
                              'p-2.5 rounded-lg transition-all duration-300',
                              config.model_name === model.name
                                ? 'bg-primary/10'
                                : 'bg-muted group-hover:bg-primary/10'
                            )}>
                              <Brain className={cn(
                                'h-5 w-5 transition-colors duration-300',
                                config.model_name === model.name
                                  ? 'text-primary'
                                  : 'text-muted-foreground group-hover:text-primary'
                              )} />
                            </div>
                            <div>
                              <h3 className='font-semibold text-base'>{model.name}</h3>
                              <div className='flex items-center gap-2 mt-1'>
                                <Badge variant='secondary' className='text-xs'>
                                  {model.family}
                                </Badge>
                                <span className='text-xs text-muted-foreground'>•</span>
                                <span className='text-xs text-muted-foreground'>{model.size}</span>
                              </div>
                            </div>
                          </div>
                          {config.model_name === model.name && (
                            <div className='rounded-full bg-primary p-1 animate-scale-in'>
                              <CheckCircle2 className='h-4 w-4 text-primary-foreground' />
                            </div>
                          )}
                        </div>
                        <div className='flex items-center gap-2 pt-3 border-t border-border/50'>
                          <HardDrive className='h-3.5 w-3.5 text-muted-foreground' />
                          <span className='text-xs font-medium'>{model.vram_required_gb}GB VRAM required</span>
                        </div>
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
              <CardContent className='space-y-6'>
                <div className='grid gap-4 sm:grid-cols-3'>
                  {[
                    { id: 'azure', name: 'Azure', icon: azureIcon, bgColor: 'bg-blue-50 dark:bg-blue-950/30' },
                    { id: 'aws', name: 'AWS', icon: awsIcon, bgColor: 'bg-orange-50 dark:bg-orange-950/30' },
                    { id: 'gcp', name: 'GCP', icon: gcpIcon, bgColor: 'bg-emerald-50 dark:bg-emerald-950/30' },
                  ].map((provider) => (
                    <div
                      key={provider.id}
                      onClick={() => setConfig({ ...config, provider: provider.id })}
                      className={cn(
                        'group relative cursor-pointer rounded-xl border-2 p-5 text-center transition-all duration-300',
                        'hover:shadow-lg hover:-translate-y-1',
                        config.provider === provider.id
                          ? 'border-primary bg-primary/5 shadow-lg shadow-primary/10'
                          : 'border-border hover:border-primary/40 hover:bg-accent/30'
                      )}
                    >
                      {config.provider === provider.id && (
                        <div className='absolute top-3 right-3 rounded-full bg-primary p-1 animate-scale-in'>
                          <CheckCircle2 className='h-4 w-4 text-primary-foreground' />
                        </div>
                      )}
                      <div className={cn(
                        'mx-auto h-16 w-16 rounded-xl flex items-center justify-center mb-3 transition-all duration-300',
                        config.provider === provider.id ? provider.bgColor : 'bg-muted/50 group-hover:bg-muted'
                      )}>
                        <img
                          src={provider.icon}
                          alt={`${provider.name} logo`}
                          className='h-10 w-10 object-contain'
                        />
                      </div>
                      <p className='font-semibold text-base'>{provider.name}</p>
                    </div>
                  ))}
                </div>

                <div className='space-y-2'>
                  <Label htmlFor='region' className='flex items-center gap-2'>
                    <MapPin className='h-4 w-4' />
                    Region
                  </Label>
                  <Select
                    value={config.region}
                    onValueChange={(value) => setConfig({ ...config, region: value })}
                    disabled={regionsLoading || !regions.length}
                  >
                    <SelectTrigger id='region' className='w-full'>
                      <SelectValue placeholder={regionsLoading ? 'Loading regions...' : 'Select region'}>
                        {config.region && regions.length > 0 && (
                          <div className='flex items-center gap-2'>
                            <MapPin className='h-4 w-4 text-muted-foreground' />
                            <span>
                              {regions.find(r => r.region_code === config.region)?.region_name || config.region}
                            </span>
                          </div>
                        )}
                      </SelectValue>
                    </SelectTrigger>
                    <SelectContent className='max-h-[300px]'>
                      {regions.map((region) => (
                        <SelectItem
                          key={region.id}
                          value={region.region_code}
                          className='py-3 cursor-pointer'
                        >
                          <div className='flex items-start gap-3'>
                            <MapPin className='h-4 w-4 mt-0.5 text-muted-foreground flex-shrink-0' />
                            <div className='flex flex-col gap-1.5'>
                              <div className='font-medium text-sm'>{region.region_name}</div>
                              <div className='text-xs text-muted-foreground leading-relaxed'>{region.location}</div>
                            </div>
                          </div>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className='space-y-2'>
                  <Label htmlFor='instance' className='flex items-center gap-2'>
                    <Cpu className='h-4 w-4' />
                    Instance Type
                  </Label>
                  <Select
                    value={config.instance_type}
                    onValueChange={(value) => setConfig({ ...config, instance_type: value })}
                    disabled={instanceTypesLoading || !instanceTypes.length || !config.region}
                  >
                    <SelectTrigger id='instance' className='w-full'>
                      <SelectValue
                        placeholder={
                          !config.region
                            ? 'Select region first'
                            : instanceTypesLoading
                              ? 'Loading instance types...'
                              : 'Select instance type'
                        }
                      >
                        {config.instance_type && instanceTypes.length > 0 && (
                          <div className='flex items-center gap-2'>
                            <Cpu className='h-4 w-4 text-muted-foreground' />
                            <span className='font-mono text-sm'>
                              {instanceTypes.find(i => i.instance_type === config.instance_type)?.instance_type || config.instance_type}
                            </span>
                          </div>
                        )}
                      </SelectValue>
                    </SelectTrigger>
                    <SelectContent className='max-h-[400px]'>
                      {instanceTypes.map((instance) => (
                        <SelectItem
                          key={instance.id}
                          value={instance.instance_type}
                          className='py-3 cursor-pointer'
                        >
                          <div className='flex flex-col gap-2'>
                            <div className='flex items-center justify-between gap-2'>
                              <div className='font-medium text-sm font-mono'>{instance.instance_type}</div>
                              {instance.price_per_hour && (
                                <Badge variant='secondary' className='text-xs font-normal'>
                                  ${config.use_spot && instance.spot_price_per_hour
                                    ? instance.spot_price_per_hour.toFixed(2)
                                    : instance.price_per_hour.toFixed(2)}/hr
                                </Badge>
                              )}
                            </div>
                            <div className='flex flex-wrap gap-3 text-xs text-muted-foreground'>
                              <div className='flex items-center gap-1'>
                                <Cpu className='h-3 w-3' />
                                <span>{instance.vcpu_count} vCPU</span>
                              </div>
                              <div className='flex items-center gap-1'>
                                <HardDrive className='h-3 w-3' />
                                <span>{instance.memory_gb} GB RAM</span>
                              </div>
                              <div className='flex items-center gap-1 font-medium text-primary'>
                                <Zap className='h-3 w-3' />
                                <span>{instance.gpu_count}x {instance.gpu_model}</span>
                              </div>
                              <span className='text-muted-foreground/60'>({instance.gpu_memory_gb} GB VRAM)</span>
                            </div>
                          </div>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className='flex items-center justify-between rounded-lg border p-4 bg-muted/30 hover:bg-muted/50 transition-colors'>
                  <div className='flex items-center gap-3'>
                    <div className='p-2 rounded-full bg-primary/10'>
                      <Zap className='h-5 w-5 text-primary' />
                    </div>
                    <div>
                      <div className='flex items-center gap-2'>
                        <p className='font-medium'>Use Spot Instance</p>
                        <Badge variant='secondary' className='text-xs'>
                          Recommended
                        </Badge>
                      </div>
                      <p className='text-sm text-muted-foreground mt-0.5'>Save 70-90% on compute costs</p>
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
            <Card className={cn(
              'sticky top-6 relative overflow-hidden',
              'border-primary/20 shadow-lg',
              'bg-gradient-to-br from-primary/5 via-background to-background'
            )}>
              <div className='absolute top-0 right-0 w-32 h-32 bg-primary/5 rounded-full blur-3xl -z-10' />
              <CardHeader>
                <CardTitle className='flex items-center gap-3'>
                  <div className='rounded-lg bg-primary/10 p-2'>
                    <Rocket className='h-5 w-5 text-primary' />
                  </div>
                  Launch Summary
                </CardTitle>
              </CardHeader>
              <CardContent className='space-y-4'>
                <div className='space-y-3'>
                  <div className='flex items-center justify-between py-2 border-b'>
                    <p className='text-sm text-muted-foreground'>Model</p>
                    <p className='font-medium text-sm'>{config.model_name || <span className='text-muted-foreground'>Not selected</span>}</p>
                  </div>
                  <div className='flex items-center justify-between py-2 border-b'>
                    <p className='text-sm text-muted-foreground'>Provider</p>
                    <div className='flex items-center gap-2'>
                      <img
                        src={config.provider === 'azure' ? azureIcon : config.provider === 'aws' ? awsIcon : gcpIcon}
                        alt={`${config.provider} logo`}
                        className='h-5 w-5 object-contain'
                      />
                      <p className='font-medium text-sm capitalize'>{config.provider}</p>
                    </div>
                  </div>
                  <div className='flex items-center justify-between py-2 border-b'>
                    <p className='text-sm text-muted-foreground'>Region</p>
                    <div className='flex items-center gap-2'>
                      <MapPin className='h-4 w-4 text-muted-foreground' />
                      <p className='font-medium text-sm'>{config.region || <span className='text-muted-foreground'>-</span>}</p>
                    </div>
                  </div>
                  <div className='flex items-center justify-between py-2 border-b'>
                    <p className='text-sm text-muted-foreground'>Instance</p>
                    <p className='font-medium font-mono text-xs'>{config.instance_type || <span className='text-muted-foreground'>-</span>}</p>
                  </div>
                  {config.instance_type && instanceTypes.length > 0 && (
                    <div className='flex items-center justify-between py-2 border-b'>
                      <p className='text-sm text-muted-foreground'>Cost</p>
                      <div className='text-right'>
                        {(() => {
                          const selectedInstance = instanceTypes.find(i => i.instance_type === config.instance_type)
                          if (!selectedInstance) return <span className='text-muted-foreground'>-</span>
                          const price = config.use_spot
                            ? selectedInstance.spot_price_per_hour
                            : selectedInstance.price_per_hour
                          const savingsPercent = selectedInstance.price_per_hour && selectedInstance.spot_price_per_hour
                            ? Math.round((1 - selectedInstance.spot_price_per_hour / selectedInstance.price_per_hour) * 100)
                            : 0
                          return (
                            <div className='flex flex-col items-end gap-1'>
                              <p className='font-bold text-lg'>${price?.toFixed(2) || '-'}/hr</p>
                              {config.use_spot && savingsPercent > 0 && (
                                <Badge variant='secondary' className='text-xs'>
                                  {savingsPercent}% savings
                                </Badge>
                              )}
                            </div>
                          )
                        })()}
                      </div>
                    </div>
                  )}
                  <div className='flex items-center justify-between py-2'>
                    <p className='text-sm text-muted-foreground'>Type</p>
                    <Badge variant={config.use_spot ? 'default' : 'secondary'}>
                      {config.use_spot ? (
                        <><Zap className='h-3 w-3 mr-1' /> Spot</>
                      ) : (
                        'On-Demand'
                      )}
                    </Badge>
                  </div>
                </div>
                <Button
                  onClick={handleLaunch}
                  className={cn(
                    'w-full relative overflow-hidden',
                    'bg-gradient-to-r from-primary to-primary/80',
                    'shadow-lg shadow-primary/20 hover:shadow-xl hover:shadow-primary/30',
                    'transition-all duration-300'
                  )}
                  size='lg'
                  disabled={!config.model_name || !config.region || !config.instance_type || launchMutation.isPending}
                >
                  {launchMutation.isPending ? (
                    <>
                      <div className='h-4 w-4 mr-2 animate-spin rounded-full border-2 border-current border-t-transparent' />
                      Launching...
                    </>
                  ) : (
                    <>
                      <Sparkles className='mr-2 h-4 w-4' />
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
