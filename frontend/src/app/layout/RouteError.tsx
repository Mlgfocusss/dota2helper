import { RotateCw, TriangleAlert } from 'lucide-react'
import { useEffect } from 'react'
import { useRouteError } from 'react-router'
import { Button } from '@/components/ui/Button'
import { EmptyState } from '@/components/ui/EmptyState'
import { t } from '@/i18n/ru'

/** Ошибка рендера или загрузки кода страницы (например, после деплоя устарели чанки). */
export function RouteError() {
  const error = useRouteError()

  useEffect(() => {
    console.error(error)
  }, [error])

  return (
    <div className="flex min-h-[60dvh] items-center justify-center p-6">
      <EmptyState
        icon={TriangleAlert}
        title={t.states.crashTitle}
        description={t.states.crashText}
        action={
          <Button onClick={() => window.location.reload()}>
            <RotateCw size={18} aria-hidden />
            {t.states.reload}
          </Button>
        }
      />
    </div>
  )
}

/** Пока при первом открытии грузится код страницы. */
export function AppSplash() {
  return <div className="min-h-dvh bg-bg" />
}
