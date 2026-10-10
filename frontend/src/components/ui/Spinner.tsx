import { LoaderCircle } from 'lucide-react'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'

export function Spinner({ size = 24, className }: { size?: number; className?: string }) {
  return (
    <LoaderCircle
      size={size}
      role="status"
      aria-label={t.common.loading}
      className={cn('animate-spin text-muted', className)}
    />
  )
}
