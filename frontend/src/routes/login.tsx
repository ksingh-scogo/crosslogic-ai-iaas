import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { Server } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useAuthStore } from '@/stores/auth'
import { toast } from 'sonner'

export const Route = createFileRoute('/login')({
  component: LoginPage,
})

function LoginPage() {
  const navigate = useNavigate()
  const { login } = useAuthStore()
  const [token, setToken] = useState('')
  const [loading, setLoading] = useState(false)

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()

    if (!token) {
      toast.error('Please enter an admin token')
      return
    }

    setLoading(true)

    try {
      // For now, just store the token
      // In production, you'd validate it with an API call
      login(token, { id: 'admin', email: 'admin@crosslogic.ai', name: 'Admin' })
      toast.success('Successfully logged in')
      navigate({ to: '/' })
    } catch (error) {
      toast.error('Invalid token')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-900 via-blue-900 to-slate-900 p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1 text-center">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-sky-500">
            <Server className="h-7 w-7 text-white" />
          </div>
          <CardTitle className="text-2xl font-bold">CrossLogic Platform</CardTitle>
          <CardDescription>Enter your admin token to continue</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleLogin} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="token">Admin Token</Label>
              <Input
                id="token"
                type="password"
                placeholder="Enter your admin token"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                disabled={loading}
              />
            </div>
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? 'Signing in...' : 'Sign In'}
            </Button>
            <p className="text-center text-xs text-gray-500">
              Admin token can be found in your environment configuration
            </p>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
