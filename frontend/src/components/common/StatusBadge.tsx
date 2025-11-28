import { cn } from '@/lib/utils'

type Status = 'active' | 'inactive' | 'error' | 'pending' | 'degraded' | 'provisioning' | 'terminating'

interface StatusBadgeProps {
  status: Status
  className?: string
  showPulse?: boolean
}

const statusConfig = {
  active: {
    label: 'Active',
    bgClass: 'bg-green-500/10 dark:bg-green-500/20',
    textClass: 'text-green-700 dark:text-green-400',
    borderClass: 'border-green-500/30',
    dotClass: 'bg-green-500',
    pulse: true,
  },
  inactive: {
    label: 'Inactive',
    bgClass: 'bg-gray-500/10 dark:bg-gray-500/20',
    textClass: 'text-gray-700 dark:text-gray-400',
    borderClass: 'border-gray-500/30',
    dotClass: 'bg-gray-500',
    pulse: false,
  },
  error: {
    label: 'Error',
    bgClass: 'bg-red-500/10 dark:bg-red-500/20',
    textClass: 'text-red-700 dark:text-red-400',
    borderClass: 'border-red-500/30',
    dotClass: 'bg-red-500',
    pulse: true,
  },
  pending: {
    label: 'Pending',
    bgClass: 'bg-yellow-500/10 dark:bg-yellow-500/20',
    textClass: 'text-yellow-700 dark:text-yellow-400',
    borderClass: 'border-yellow-500/30',
    dotClass: 'bg-yellow-500',
    pulse: true,
  },
  degraded: {
    label: 'Degraded',
    bgClass: 'bg-orange-500/10 dark:bg-orange-500/20',
    textClass: 'text-orange-700 dark:text-orange-400',
    borderClass: 'border-orange-500/30',
    dotClass: 'bg-orange-500',
    pulse: true,
  },
  provisioning: {
    label: 'Provisioning',
    bgClass: 'bg-blue-500/10 dark:bg-blue-500/20',
    textClass: 'text-blue-700 dark:text-blue-400',
    borderClass: 'border-blue-500/30',
    dotClass: 'bg-blue-500',
    pulse: true,
  },
  terminating: {
    label: 'Terminating',
    bgClass: 'bg-red-500/10 dark:bg-red-500/20',
    textClass: 'text-red-700 dark:text-red-400',
    borderClass: 'border-red-500/30',
    dotClass: 'bg-red-500',
    pulse: true,
  },
}

export function StatusBadge({ status, className, showPulse = true }: StatusBadgeProps) {
  const config = statusConfig[status]
  const shouldPulse = showPulse && config.pulse

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full',
        'text-xs font-semibold border',
        'transition-all duration-200',
        config.bgClass,
        config.textClass,
        config.borderClass,
        className
      )}
    >
      <span className="relative flex h-2 w-2">
        {shouldPulse && (
          <span
            className={cn(
              'absolute inline-flex h-full w-full rounded-full opacity-75 animate-ping',
              config.dotClass
            )}
            style={{ animationDuration: '1.5s' }}
          />
        )}
        <span className={cn('relative inline-flex rounded-full h-2 w-2', config.dotClass)} />
      </span>
      {config.label}
    </span>
  )
}
