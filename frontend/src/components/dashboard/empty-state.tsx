import { Link } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Rocket, Key, CheckCircle2, Zap } from 'lucide-react'
import { cn } from '@/lib/utils'

export function EmptyStateCard() {
  return (
    <Card className={cn(
      'border-dashed border-2 relative overflow-hidden',
      'hover:border-primary/50 transition-colors duration-300'
    )}>
      {/* Background Effects */}
      <div className="absolute top-0 right-0 w-64 h-64 bg-primary/5 rounded-full blur-3xl -z-10" />
      <div className="absolute bottom-0 left-0 w-48 h-48 bg-blue-500/5 rounded-full blur-3xl -z-10" />

      <CardHeader className="text-center pb-2 relative z-10">
        {/* Animated Icon */}
        <div className="mx-auto mb-4 relative w-20 h-20">
          <div className="absolute inset-0 rounded-full bg-primary/20 animate-ping" style={{ animationDuration: '2s' }} />
          <div className="absolute inset-2 rounded-full bg-primary/10 animate-pulse" />
          <div className="relative flex items-center justify-center w-full h-full rounded-full bg-gradient-to-br from-primary to-primary/60 shadow-lg shadow-primary/20">
            <Zap className="h-8 w-8 text-white animate-float" />
          </div>
        </div>

        <CardTitle className="text-xl font-semibold">Get Started with CrossLogic AI</CardTitle>
        <CardDescription className="max-w-md mx-auto mt-2 text-base">
          Deploy your first AI model in minutes. Our platform handles infrastructure so you can focus on building.
        </CardDescription>
      </CardHeader>

      <CardContent className="pt-6">
        <div className="mx-auto max-w-lg space-y-3">
          {[
            {
              icon: Key,
              title: '1. Create an API Key',
              description: 'Generate credentials to authenticate your requests',
              delay: '100ms',
            },
            {
              icon: Rocket,
              title: '2. Launch a GPU Instance',
              description: 'Select a model and deploy to cloud GPUs',
              delay: '200ms',
            },
            {
              icon: CheckCircle2,
              title: '3. Start Making Requests',
              description: 'Use our OpenAI-compatible API to generate completions',
              delay: '300ms',
            },
          ].map((step, index) => (
            <div
              key={index}
              className={cn(
                'flex items-start gap-4 p-4 rounded-xl border',
                'bg-gradient-to-r from-muted/30 to-transparent',
                'hover:from-muted/50 hover:to-muted/20',
                'transition-all duration-200',
                'animate-fade-in-up'
              )}
              style={{ animationDelay: step.delay }}
            >
              <div className="p-2.5 rounded-lg bg-primary/10">
                <step.icon className="h-5 w-5 text-primary" />
              </div>
              <div className="flex-1">
                <p className="font-semibold text-sm">{step.title}</p>
                <p className="text-xs text-muted-foreground mt-0.5">{step.description}</p>
              </div>
            </div>
          ))}
        </div>

        <div className="mt-8 flex flex-col sm:flex-row justify-center gap-3">
          <Button asChild variant="outline" size="lg">
            <Link to="/api-keys">
              <Key className="mr-2 h-4 w-4" />
              Create API Key
            </Link>
          </Button>
          <Button asChild size="lg" className="shadow-lg shadow-primary/20">
            <Link to="/launch">
              <Rocket className="mr-2 h-4 w-4" />
              Launch Instance
            </Link>
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
