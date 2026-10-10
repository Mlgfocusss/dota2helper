import { useQueryClient } from '@tanstack/react-query'
import { CircleAlert, Layers } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useNavigate, useParams } from 'react-router'
import { ApiError } from '@/api/client'
import { SteamLoginButton } from '@/components/auth/SteamLoginButton'
import { ProgressSteps } from '@/components/analysis/ProgressSteps'
import { ResultView } from '@/components/analysis/ResultView'
import { Button, ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Page, PageHeader } from '@/components/ui/Page'
import { Skeleton } from '@/components/ui/Skeleton'
import { toast } from '@/features/toast/store'
import { isAnalysisActive, useAnalysis, useCreateAnalysis } from '@/features/useAnalysis'
import { useMe } from '@/features/useMe'
import { t } from '@/i18n/ru'

// Страница разбора (SPEC 4.8): ход работы, ошибка или готовый результат; /coach/analysis/example — пример.
export default function AnalysisPage() {
  const { id = '' } = useParams()
  const isExample = id === 'example'
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { data: me, isPending: mePending } = useMe()
  const loggedIn = Boolean(me?.user)
  const analysis = useAnalysis(id, isExample || loggedIn)
  const create = useCreateAnalysis()

  const status = analysis.data?.status
  // статус разбора виден в списках матчей «Тренера»: обновляем их, когда он закончился
  const lastStatus = useRef(status)
  useEffect(() => {
    if (lastStatus.current !== status && !isAnalysisActive(status) && status !== undefined) {
      void queryClient.invalidateQueries({ queryKey: ['coach'] })
    }
    lastStatus.current = status
  }, [status, queryClient])

  const title = isExample ? t.analysis.exampleTitle : t.analysis.resultTitle
  const header = (
    <PageHeader title={title} icon={<Layers size={30} className="text-accent-2" aria-hidden />} />
  )

  if (!isExample && mePending) {
    return (
      <Page width={1000} title={title}>
        {header}
        <Skeleton radius={16} className="h-[260px]" />
      </Page>
    )
  }

  if (!isExample && !loggedIn) {
    return (
      <Page width={1000} title={title}>
        {header}
        <Card padding="none">
          <EmptyState
            icon={Layers}
            title={t.analysis.guestTitle}
            description={t.analysis.loginToView}
            action={<SteamLoginButton size="lg" />}
          />
        </Card>
      </Page>
    )
  }

  if (analysis.isPending) {
    return (
      <Page width={1000} title={title}>
        {header}
        <div className="space-y-4">
          <Skeleton radius={16} className="h-[180px]" />
          <Skeleton radius={16} className="h-[140px]" />
        </div>
      </Page>
    )
  }

  if (analysis.isError) {
    const missing = analysis.error instanceof ApiError && analysis.error.status === 404
    return (
      <Page width={1000} title={title}>
        {header}
        <Card padding="none">
          {missing ? (
            <EmptyState
              icon={CircleAlert}
              title={t.analysis.notFoundTitle}
              description={t.analysis.notFoundText}
              action={
                <ButtonLink to="/coach/deep" variant="accent2">
                  {t.analysis.toMatches}
                </ButtonLink>
              }
            />
          ) : (
            <ErrorState onRetry={() => analysis.refetch()} retrying={analysis.isRefetching} />
          )}
        </Card>
      </Page>
    )
  }

  const view = analysis.data

  if (view.status === 'done' && view.result) {
    return (
      <Page width={1000} title={title}>
        {header}
        <ResultView
          result={view.result}
          analysisId={view.id}
          matchId={view.match_id}
          isExample={isExample}
        />
      </Page>
    )
  }

  if (view.status === 'failed') {
    const retry = () => {
      if (view.match_id === undefined) return
      create.mutate(view.match_id, {
        onSuccess: (next) => void navigate(`/coach/analysis/${next.id}`, { replace: true }),
        onError: (err) => toast.error(err instanceof ApiError ? err.message : t.states.unknown),
      })
    }
    return (
      <Page width={760} title={title}>
        {header}
        <Card padding="none">
          <EmptyState
            icon={CircleAlert}
            title={t.analysis.failedTitle}
            description={
              <>
                {view.error}
                <span className="mt-1 block text-dim">{t.analysis.failedNote}</span>
              </>
            }
            action={
              <div className="flex flex-wrap justify-center gap-2">
                <Button variant="accent2" loading={create.isPending} onClick={retry}>
                  {t.analysis.retry}
                </Button>
                <ButtonLink to="/coach/deep" variant="surface">
                  {t.analysis.toMatches}
                </ButtonLink>
              </div>
            }
          />
        </Card>
      </Page>
    )
  }

  // в очереди или в работе
  return (
    <Page width={760} title={title}>
      {header}
      <ProgressSteps status={view.status} />
      <p className="mt-4 text-[14px] text-muted">{t.analysis.waitNote}</p>
    </Page>
  )
}
