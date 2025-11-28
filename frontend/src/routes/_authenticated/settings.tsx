import { createFileRoute } from '@tanstack/react-router'
import { User, Palette, Globe } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { ConfigDrawer } from '@/components/config-drawer'

export const Route = createFileRoute('/_authenticated/settings')({
  component: SettingsPage,
})

function SettingsPage() {
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
        <div className='mb-6'>
          <h1 className='text-2xl font-bold tracking-tight'>Settings</h1>
          <p className='text-muted-foreground'>Manage your account and preferences</p>
        </div>

        <div className='space-y-6'>
          <Card className='relative overflow-hidden'>
            <div className='absolute inset-0 bg-gradient-to-br from-primary/5 via-transparent to-transparent pointer-events-none' />
            <CardHeader className='relative z-10'>
              <div className='flex items-center gap-3'>
                <div className='p-2 rounded-lg bg-primary/10'>
                  <Globe className='h-5 w-5 text-primary' />
                </div>
                <div>
                  <CardTitle>API Configuration</CardTitle>
                  <CardDescription>Configure your API endpoint and settings</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className='space-y-4 relative z-10'>
              <div className='space-y-2'>
                <Label htmlFor='api-url' className='text-xs font-medium uppercase tracking-wider text-muted-foreground'>
                  API Base URL
                </Label>
                <Input
                  id='api-url'
                  defaultValue='http://localhost:8080'
                  placeholder='https://api.crosslogic.ai'
                  className='bg-muted/50'
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='tenant-id' className='text-xs font-medium uppercase tracking-wider text-muted-foreground'>
                  Tenant ID
                </Label>
                <Input
                  id='tenant-id'
                  defaultValue='00000000-0000-0000-0000-000000000000'
                  readOnly
                  className='font-mono text-sm bg-muted/50'
                />
              </div>
              <Button className='shadow-lg shadow-primary/20'>Save Changes</Button>
            </CardContent>
          </Card>

          <Card className='relative overflow-hidden'>
            <div className='absolute inset-0 bg-gradient-to-br from-muted/30 via-transparent to-transparent pointer-events-none' />
            <CardHeader className='relative z-10'>
              <div className='flex items-center gap-3'>
                <div className='p-2 rounded-lg bg-muted'>
                  <User className='h-5 w-5 text-muted-foreground' />
                </div>
                <div>
                  <CardTitle>Account Information</CardTitle>
                  <CardDescription>View your account details</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className='space-y-4 relative z-10'>
              <div className='space-y-1'>
                <Label className='text-xs font-medium uppercase tracking-wider text-muted-foreground'>Email</Label>
                <p className='text-sm font-medium'>admin@crosslogic.ai</p>
              </div>
              <div className='space-y-1'>
                <Label className='text-xs font-medium uppercase tracking-wider text-muted-foreground'>Role</Label>
                <p className='text-sm font-medium'>Administrator</p>
              </div>
            </CardContent>
          </Card>

          <Card className='relative overflow-hidden'>
            <div className='absolute inset-0 bg-gradient-to-br from-muted/30 via-transparent to-transparent pointer-events-none' />
            <CardHeader className='relative z-10'>
              <div className='flex items-center gap-3'>
                <div className='p-2 rounded-lg bg-muted'>
                  <Palette className='h-5 w-5 text-muted-foreground' />
                </div>
                <div>
                  <CardTitle>Appearance</CardTitle>
                  <CardDescription>Customize your dashboard appearance</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className='relative z-10'>
              <p className='text-sm text-muted-foreground'>
                Use the theme settings button in the header to customize your dashboard appearance including dark mode and layout options.
              </p>
            </CardContent>
          </Card>
        </div>
      </Main>
    </>
  )
}
