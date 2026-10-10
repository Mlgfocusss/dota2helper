import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { cn } from '@/lib/cn'

type EmptyStateProps = {
  icon?: LucideIcon
  title: string
  description?: ReactNode
  action?: ReactNode
  className?: string
}

/** Пустое состояние: иконка, заголовок, пояснение и действие. */
export function EmptyState({ icon: Icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div className={cn('flex flex-col items-center px-6 py-10 text-center', className)}>
      {Icon && (
        <div className="mb-4 flex size-12 items-center justify-center rounded-[14px] bg-surface-2 text-muted">
          <Icon size={22} aria-hidden />
        </div>
      )}
      <h3 className="text-card-title">{title}</h3>
      {description && <p className="mt-1.5 max-w-[440px] text-[15px] text-muted">{description}</p>}
      {action && <div className="mt-5">{action}</div>}
    </div>
  )
}
