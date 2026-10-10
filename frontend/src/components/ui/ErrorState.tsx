import { CircleAlert, RotateCw } from 'lucide-react'
import { t } from '@/i18n/ru'
import { Button } from './Button'
import { EmptyState } from './EmptyState'

type ErrorStateProps = {
  title?: string
  /** Текст ошибки; по умолчанию — общий совет проверить соединение */
  description?: string
  onRetry?: () => void
  retrying?: boolean
  className?: string
}

/** Ошибка загрузки с кнопкой «Повторить». */
export function ErrorState({
  title = t.states.errorTitle,
  description = t.states.errorText,
  onRetry,
  retrying,
  className,
}: ErrorStateProps) {
  return (
    <EmptyState
      icon={CircleAlert}
      title={title}
      description={description}
      className={className}
      action={
        onRetry && (
          <Button variant="surface" onClick={onRetry} loading={retrying}>
            {!retrying && <RotateCw size={18} aria-hidden />}
            {t.common.retry}
          </Button>
        )
      }
    />
  )
}
