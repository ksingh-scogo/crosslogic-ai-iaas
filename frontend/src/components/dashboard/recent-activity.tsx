import { Activity } from 'lucide-react'
import type { UsagePoint } from '@/types'

interface RecentActivityProps {
  usage: UsagePoint[]
}

export function RecentActivity({ usage }: RecentActivityProps) {
  return (
    <div className='space-y-8'>
      {usage.map((point) => (
        <div key={point.timestamp} className='flex items-center gap-4'>
          <div className='flex h-9 w-9 items-center justify-center rounded-full bg-primary/10'>
            <Activity className='h-4 w-4 text-primary' />
          </div>
          <div className='flex flex-1 flex-wrap items-center justify-between'>
            <div className='space-y-1'>
              <p className='text-sm leading-none font-medium'>
                {new Date(point.timestamp).toLocaleTimeString('en-US', {
                  hour: '2-digit',
                  minute: '2-digit',
                })}
              </p>
              <p className='text-muted-foreground text-sm'>
                {point.requests || 0} requests
              </p>
            </div>
            <div className='space-y-1 text-right'>
              <div className='font-medium'>{point.totalTokens.toLocaleString()} tokens</div>
              <p className='text-muted-foreground text-sm'>{point.totalCost}</p>
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}
