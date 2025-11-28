import { cn } from '@/lib/utils'

interface LoadingSpinnerProps {
  size?: 'sm' | 'md' | 'lg'
  className?: string
  label?: string
}

const sizeClasses = {
  sm: 'h-4 w-4 border-2',
  md: 'h-8 w-8 border-2',
  lg: 'h-12 w-12 border-3',
}

export function LoadingSpinner({ size = 'md', className, label }: LoadingSpinnerProps) {
  return (
    <div className='flex flex-col items-center justify-center gap-3'>
      <div
        className={cn(
          'animate-spin rounded-full border-primary border-t-transparent',
          sizeClasses[size],
          className
        )}
        role='status'
        aria-label={label || 'Loading'}
      />
      {label && (
        <p className='text-sm text-muted-foreground animate-pulse'>{label}</p>
      )}
    </div>
  )
}

interface LoadingOverlayProps {
  label?: string
}

export function LoadingOverlay({ label = 'Loading...' }: LoadingOverlayProps) {
  return (
    <div className='absolute inset-0 flex items-center justify-center bg-background/80 backdrop-blur-sm z-50'>
      <LoadingSpinner size='lg' label={label} />
    </div>
  )
}

interface PageLoadingProps {
  title?: string
}

export function PageLoading({ title = 'Loading' }: PageLoadingProps) {
  return (
    <div className='flex h-[50vh] flex-col items-center justify-center gap-4'>
      <div className='relative'>
        <div className='h-16 w-16 animate-spin rounded-full border-4 border-primary/20 border-t-primary' />
        <div className='absolute inset-0 flex items-center justify-center'>
          <div className='h-8 w-8 rounded-full bg-gradient-to-br from-primary/20 to-primary/5 animate-pulse' />
        </div>
      </div>
      <div className='text-center'>
        <h3 className='text-lg font-semibold text-foreground'>{title}</h3>
        <p className='text-sm text-muted-foreground mt-1'>Please wait a moment...</p>
      </div>
    </div>
  )
}
