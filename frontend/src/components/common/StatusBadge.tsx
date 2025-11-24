import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

type Status = 'active' | 'inactive' | 'error' | 'pending' | 'degraded'

interface StatusBadgeProps {
  status: Status
  className?: string
}

const statusConfig = {
  active: {
    label: 'Active',
    className: 'bg-green-100 text-green-800 hover:bg-green-100',
  },
  inactive: {
    label: 'Inactive',
    className: 'bg-gray-100 text-gray-800 hover:bg-gray-100',
  },
  error: {
    label: 'Error',
    className: 'bg-red-100 text-red-800 hover:bg-red-100',
  },
  pending: {
    label: 'Pending',
    className: 'bg-yellow-100 text-yellow-800 hover:bg-yellow-100',
  },
  degraded: {
    label: 'Degraded',
    className: 'bg-orange-100 text-orange-800 hover:bg-orange-100',
  },
}

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const config = statusConfig[status]

  return (
    <Badge className={cn(config.className, className)}>
      <span className="mr-1.5 inline-block h-1.5 w-1.5 rounded-full bg-current" />
      {config.label}
    </Badge>
  )
}
