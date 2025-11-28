import { Link } from '@tanstack/react-router'
import { ArrowRight, LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

interface QuickActionCardProps {
  title: string
  description: string
  icon: LucideIcon
  href: string
  variant?: 'primary' | 'success' | 'warning' | 'info'
}

const variantStyles = {
  primary: {
    gradient: 'from-primary to-primary/80',
    hoverBg: 'hover:from-primary/5 hover:to-primary/10',
    hoverBorder: 'hover:border-primary/50',
  },
  success: {
    gradient: 'from-green-500 to-emerald-500',
    hoverBg: 'hover:from-green-500/5 hover:to-emerald-500/10',
    hoverBorder: 'hover:border-green-500/50',
  },
  warning: {
    gradient: 'from-orange-500 to-amber-500',
    hoverBg: 'hover:from-orange-500/5 hover:to-amber-500/10',
    hoverBorder: 'hover:border-orange-500/50',
  },
  info: {
    gradient: 'from-blue-500 to-cyan-500',
    hoverBg: 'hover:from-blue-500/5 hover:to-cyan-500/10',
    hoverBorder: 'hover:border-blue-500/50',
  },
}

export function QuickActionCard({
  title,
  description,
  icon: Icon,
  href,
  variant = 'primary',
}: QuickActionCardProps) {
  const styles = variantStyles[variant]

  return (
    <Link
      to={href}
      className={cn(
        'group relative flex items-center gap-4 rounded-xl border p-4',
        'bg-gradient-to-br from-background to-muted/30',
        'transition-all duration-300 ease-out',
        'hover:shadow-md hover:-translate-y-0.5',
        styles.hoverBg,
        styles.hoverBorder
      )}
    >
      {/* Icon Container */}
      <div className={cn(
        'p-3 rounded-xl bg-gradient-to-br transition-all duration-300',
        'group-hover:scale-110 group-hover:shadow-lg',
        styles.gradient
      )}>
        <Icon className="h-5 w-5 text-white" />
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold mb-0.5">{title}</p>
        <p className="text-xs text-muted-foreground">{description}</p>
      </div>

      {/* Arrow */}
      <ArrowRight className={cn(
        'h-4 w-4 text-muted-foreground',
        'opacity-0 -translate-x-2',
        'group-hover:opacity-100 group-hover:translate-x-0',
        'transition-all duration-300'
      )} />
    </Link>
  )
}
