import type { CSSProperties, ReactNode } from 'react'
import { cn } from '@/lib/cn'
import { useDocumentTitle } from '@/lib/hooks'

type PageProps = {
  /** Ширина колонки контента в px (SPEC 2.5): 760, 1000, 1160, 1200 */
  width?: number
  /** Заголовок вкладки браузера; без него — только название приложения */
  title?: string
  className?: string
  children: ReactNode
}

/** Контейнер страницы фиксированной ширины по центру. */
export function Page({ width = 760, title, className, children }: PageProps) {
  useDocumentTitle(title)
  return (
    <div className={cn('page', className)} style={{ '--page-w': `${width}px` } as CSSProperties}>
      {children}
    </div>
  )
}

type PageHeaderProps = {
  title: string
  subtitle?: ReactNode
  /** Иконка слева от заголовка */
  icon?: ReactNode
  /** Кнопки справа */
  actions?: ReactNode
  className?: string
}

export function PageHeader({ title, subtitle, icon, actions, className }: PageHeaderProps) {
  return (
    <div className={cn('mb-6 flex items-start gap-4', className)}>
      {icon && <div className="mt-0.5 shrink-0">{icon}</div>}
      <div className="min-w-0 flex-1">
        <h1 className="text-page-title">{title}</h1>
        {subtitle && <p className="mt-1.5 text-muted">{subtitle}</p>}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </div>
  )
}
