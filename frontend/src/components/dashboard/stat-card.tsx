import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { ArrowUpIcon, ArrowDownIcon, LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

interface StatCardProps {
  title: string
  value: string | number
  description?: string
  icon: LucideIcon
  trend?: {
    value: number
    label: string
    isPositive?: boolean
  }
  variant?: 'default' | 'primary' | 'success' | 'warning' | 'info'
}

const variantStyles = {
  default: {
    gradient: 'from-muted/50 via-transparent to-transparent',
    iconBg: 'bg-muted',
    iconColor: 'text-muted-foreground',
  },
  primary: {
    gradient: 'from-primary/10 via-primary/5 to-transparent',
    iconBg: 'bg-primary/10',
    iconColor: 'text-primary',
  },
  success: {
    gradient: 'from-green-500/10 via-green-500/5 to-transparent',
    iconBg: 'bg-green-500/10',
    iconColor: 'text-green-600 dark:text-green-400',
  },
  warning: {
    gradient: 'from-orange-500/10 via-orange-500/5 to-transparent',
    iconBg: 'bg-orange-500/10',
    iconColor: 'text-orange-600 dark:text-orange-400',
  },
  info: {
    gradient: 'from-blue-500/10 via-blue-500/5 to-transparent',
    iconBg: 'bg-blue-500/10',
    iconColor: 'text-blue-600 dark:text-blue-400',
  },
}

export function StatCard({
  title,
  value,
  description,
  icon: Icon,
  trend,
  variant = 'default',
}: StatCardProps) {
  const styles = variantStyles[variant]

  return (
    <Card className={cn(
      'relative overflow-hidden group',
      'transition-all duration-300 ease-out',
      'hover:shadow-lg hover:-translate-y-0.5',
      'border-border/50 hover:border-primary/30'
    )}>
      {/* Gradient Background */}
      <div
        className={cn(
          'absolute inset-0 bg-gradient-to-br opacity-70 group-hover:opacity-100 transition-opacity duration-300',
          styles.gradient
        )}
      />

      {/* Shimmer Effect on Hover */}
      <div className="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-500 overflow-hidden">
        <div className="absolute inset-0 -translate-x-full group-hover:translate-x-full transition-transform duration-1000 bg-gradient-to-r from-transparent via-white/5 to-transparent" />
      </div>

      <CardHeader className="relative flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">{title}</CardTitle>
        <div className={cn(
          'p-2 rounded-lg transition-all duration-300',
          'group-hover:scale-110',
          styles.iconBg
        )}>
          <Icon className={cn('h-4 w-4', styles.iconColor)} />
        </div>
      </CardHeader>

      <CardContent className="relative">
        <div className="flex items-end justify-between gap-2">
          <div className="space-y-1">
            <div className="text-2xl font-bold tracking-tight">{value}</div>
            {description && (
              <p className="text-xs text-muted-foreground">{description}</p>
            )}
          </div>

          {trend && (
            <Badge
              variant="secondary"
              className={cn(
                'flex items-center gap-1 font-mono text-xs shrink-0',
                'transition-colors duration-200',
                trend.isPositive
                  ? 'bg-green-500/10 text-green-700 dark:text-green-400 hover:bg-green-500/20'
                  : 'bg-red-500/10 text-red-700 dark:text-red-400 hover:bg-red-500/20'
              )}
            >
              {trend.isPositive ? (
                <ArrowUpIcon className="h-3 w-3" />
              ) : (
                <ArrowDownIcon className="h-3 w-3" />
              )}
              {Math.abs(trend.value)}%
            </Badge>
          )}
        </div>

        {trend?.label && (
          <p className="text-xs text-muted-foreground mt-1.5">{trend.label}</p>
        )}
      </CardContent>
    </Card>
  )
}
