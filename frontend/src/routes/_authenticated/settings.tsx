import { createFileRoute } from '@tanstack/react-router'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

export const Route = createFileRoute('/_authenticated/settings')({
  component: SettingsPage,
})

function SettingsPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-gray-900">Settings</h1>
        <p className="text-gray-500 mt-1">Manage your account and preferences</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>API Configuration</CardTitle>
          <CardDescription>Configure your API endpoint and settings</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="api-url">API Base URL</Label>
            <Input
              id="api-url"
              defaultValue="http://localhost:8080"
              placeholder="https://api.crosslogic.ai"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="tenant-id">Tenant ID</Label>
            <Input
              id="tenant-id"
              defaultValue="00000000-0000-0000-0000-000000000000"
              readOnly
              className="font-mono text-sm"
            />
          </div>
          <Button>Save Changes</Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Account Information</CardTitle>
          <CardDescription>View your account details</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label>Email</Label>
            <p className="text-sm text-gray-600">admin@crosslogic.ai</p>
          </div>
          <div className="space-y-2">
            <Label>Role</Label>
            <p className="text-sm text-gray-600">Administrator</p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Appearance</CardTitle>
          <CardDescription>Customize your dashboard appearance</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-gray-500">Dark mode and theme customization coming soon</p>
        </CardContent>
      </Card>
    </div>
  )
}
