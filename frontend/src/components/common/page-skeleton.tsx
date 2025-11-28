import { cn } from '@/lib/utils'

interface SkeletonProps extends React.HTMLAttributes<HTMLDivElement> {
  className?: string
}

export function Skeleton({ className, style, ...props }: SkeletonProps) {
  return (
    <div
      className={cn(
        'animate-pulse rounded-md bg-muted',
        className
      )}
      style={style}
      {...props}
    />
  )
}

export function CardSkeleton() {
  return (
    <div className='rounded-xl border bg-card p-6 space-y-4'>
      <div className='flex items-center gap-4'>
        <Skeleton className='h-12 w-12 rounded-lg' />
        <div className='space-y-2 flex-1'>
          <Skeleton className='h-4 w-32' />
          <Skeleton className='h-3 w-24' />
        </div>
      </div>
      <Skeleton className='h-8 w-28' />
    </div>
  )
}

export function TableSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <div className='rounded-xl border'>
      <div className='border-b p-4'>
        <div className='flex gap-4'>
          <Skeleton className='h-4 w-32' />
          <Skeleton className='h-4 w-24' />
          <Skeleton className='h-4 w-20' />
          <Skeleton className='h-4 w-16 ml-auto' />
        </div>
      </div>
      <div className='divide-y'>
        {Array.from({ length: rows }).map((_, i) => (
          <div key={i} className='p-4 flex items-center gap-4'>
            <Skeleton className='h-10 w-10 rounded-lg' />
            <div className='space-y-2 flex-1'>
              <Skeleton className='h-4 w-48' />
              <Skeleton className='h-3 w-32' />
            </div>
            <Skeleton className='h-6 w-16 rounded-full' />
            <Skeleton className='h-8 w-8 rounded' />
          </div>
        ))}
      </div>
    </div>
  )
}

export function StatCardSkeleton() {
  return (
    <div className='rounded-xl border bg-card p-6'>
      <div className='flex items-center justify-between'>
        <div className='space-y-2'>
          <Skeleton className='h-3 w-20' />
          <Skeleton className='h-8 w-24' />
          <Skeleton className='h-3 w-16' />
        </div>
        <Skeleton className='h-12 w-12 rounded-lg' />
      </div>
    </div>
  )
}

export function ChartSkeleton() {
  return (
    <div className='rounded-xl border bg-card p-6'>
      <div className='flex items-center gap-3 mb-6'>
        <Skeleton className='h-10 w-10 rounded-lg' />
        <div className='space-y-2'>
          <Skeleton className='h-4 w-32' />
          <Skeleton className='h-3 w-24' />
        </div>
      </div>
      <div className='h-64 flex items-end gap-2 px-4'>
        {Array.from({ length: 12 }).map((_, i) => (
          <Skeleton
            key={i}
            className='flex-1 rounded-t'
            style={{ height: `${Math.random() * 80 + 20}%` }}
          />
        ))}
      </div>
    </div>
  )
}

export function DashboardSkeleton() {
  return (
    <div className='space-y-6'>
      <div className='flex items-center justify-between'>
        <div className='space-y-2'>
          <Skeleton className='h-8 w-32' />
          <Skeleton className='h-4 w-48' />
        </div>
        <Skeleton className='h-10 w-36' />
      </div>
      <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
        {Array.from({ length: 4 }).map((_, i) => (
          <StatCardSkeleton key={i} />
        ))}
      </div>
      <div className='grid grid-cols-1 gap-4 lg:grid-cols-7'>
        <div className='lg:col-span-4'>
          <ChartSkeleton />
        </div>
        <div className='lg:col-span-3'>
          <TableSkeleton rows={4} />
        </div>
      </div>
    </div>
  )
}

export function FormSkeleton() {
  return (
    <div className='space-y-6'>
      <div className='space-y-2'>
        <Skeleton className='h-4 w-20' />
        <Skeleton className='h-10 w-full' />
      </div>
      <div className='space-y-2'>
        <Skeleton className='h-4 w-24' />
        <Skeleton className='h-10 w-full' />
      </div>
      <div className='space-y-2'>
        <Skeleton className='h-4 w-16' />
        <Skeleton className='h-24 w-full' />
      </div>
      <Skeleton className='h-10 w-28' />
    </div>
  )
}
