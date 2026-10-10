import { Circle, CircleCheck, Layers } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { ApiError } from '@/api/client'
import type { MatchRow } from '@/api/types'
import { AnalysisStatusIcon } from '@/components/analysis/AnalysisStatusIcon'
import { SteamLoginButton } from '@/components/auth/SteamLoginButton'
import { HeroPortrait } from '@/components/dota/HeroPortrait'
import { Button, ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Page, PageHeader } from '@/components/ui/Page'
import { Skeleton } from '@/components/ui/Skeleton'
import { toast } from '@/features/toast/store'
import { useAnalysisQuota, useCreateAnalysis } from '@/features/useAnalysis'
import { useCoachAccount, useCoachMatches } from '@/features/useCoach'
import { useHero } from '@/features/useHeroes'
import { useAppConfig, useMe } from '@/features/useMe'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatDuration, formatRelativeTime, msUntil } from '@/lib/format'
import { formatKda } from '@/lib/match'
import { formatRemaining } from '@/lib/party'

/** Уже есть действующий разбор (не неудавшийся) — заказывать заново не нужно. */
const hasAnalysis = (m: MatchRow) => m.analysis !== undefined && m.analysis.status !== 'failed'

function MatchChoice({
  match,
  selected,
  onSelect,
}: {
  match: MatchRow
  selected: boolean
  onSelect: () => void
}) {
  const hero = useHero(match.hero_id)
  return (
    <li>
      <button
        type="button"
        role="radio"
        aria-checked={selected}
        onClick={onSelect}
        className={cn(
          'flex w-full items-center gap-4 rounded-[14px] border px-4 py-3 text-left transition-colors',
          selected ? 'border-accent-2 bg-accent-2/10' : 'border-line bg-surface hover:bg-surface-2',
        )}
      >
        <span aria-hidden className="shrink-0 text-accent-2">
          {selected ? <CircleCheck size={20} /> : <Circle size={20} className="text-dim" />}
        </span>
        <HeroPortrait hero={hero} width={56} height={32} radius={6} className="shrink-0" />
        <span className="min-w-0 flex-1">
          <span className="block truncate font-semibold">
            {hero?.localized_name ?? t.heroes.unknown}
          </span>
          <span className="mt-0.5 block text-[13px] text-muted">
            <span className={match.win ? 'text-win' : 'text-loss'}>
              {match.win ? t.coach.matches.victory : t.coach.matches.defeat}
            </span>
            {' · '}
            {formatRelativeTime(match.start_time)} · {formatDuration(match.duration)}
          </span>
        </span>
        <span className="shrink-0 font-mono text-[14px] tabular-nums max-sm:hidden">
          {formatKda(match)}
        </span>
        <AnalysisStatusIcon status={match.analysis?.status} />
      </button>
    </li>
  )
}

// Глубокий разбор (SPEC 4.8): выбор матча из истории профиля и запуск бесплатного разбора.
export default function DeepAnalysisPage() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const { data: me, isPending: mePending } = useMe()
  const { data: config } = useAppConfig()
  const loggedIn = Boolean(me?.user)
  const { accountId } = useCoachAccount()
  const matches = useCoachMatches(loggedIn ? accountId : null, 'all')
  const quota = useAnalysisQuota(loggedIn)
  const create = useCreateAnalysis()

  const preselected = Number(params.get('match_id')) || null
  const [picked, setPicked] = useState<number | null>(preselected)

  const items = useMemo(() => matches.data?.pages.flatMap((p) => p.items) ?? [], [matches.data])
  const selected = items.find((m) => m.match_id === picked) ?? null
  // выбранный матч мог быть далеко в истории — подгружаем страницы, пока не найдём
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = matches
  useEffect(() => {
    if (
      preselected &&
      !items.some((m) => m.match_id === preselected) &&
      hasNextPage &&
      !isFetchingNextPage
    ) {
      void fetchNextPage()
    }
  }, [preselected, items, hasNextPage, isFetchingNextPage, fetchNextPage])

  const header = (
    <PageHeader
      title={t.analysis.title}
      subtitle={t.analysis.subtitle}
      icon={<Layers size={30} className="text-accent-2" aria-hidden />}
    />
  )

  if (mePending) {
    return (
      <Page title={t.analysis.title}>
        {header}
        <Skeleton radius={16} className="h-[200px]" />
      </Page>
    )
  }

  if (!loggedIn) {
    return (
      <Page title={t.analysis.title}>
        {header}
        <Card padding="none">
          <EmptyState
            icon={Layers}
            title={t.analysis.guestTitle}
            description={t.analysis.guestText}
            action={<SteamLoginButton size="lg" />}
          />
        </Card>
        <p className="mt-4 text-center">
          <Link to="/coach/analysis/example" className="text-accent-2 hover:underline">
            {t.analysis.exampleLink}
          </Link>
        </p>
      </Page>
    )
  }

  if (config && !config.features.deep_analysis) {
    return (
      <Page title={t.analysis.title}>
        {header}
        <Card padding="none">
          <EmptyState
            icon={Layers}
            title={t.analysis.unavailableTitle}
            description={t.analysis.unavailableText}
            action={
              <ButtonLink to="/coach/analysis/example" variant="accent2">
                {t.analysis.exampleLink}
              </ButtonLink>
            }
          />
        </Card>
      </Page>
    )
  }

  const left = quota.data?.left
  const resetIn = msUntil(quota.data?.reset_at)
  const noQuota = left === 0

  const start = () => {
    if (!selected) return
    create.mutate(selected.match_id, {
      onSuccess: (view) => {
        toast.success(t.analysis.started)
        void navigate(`/coach/analysis/${view.id}`)
      },
      onError: (err) => toast.error(err instanceof ApiError ? err.message : t.states.unknown),
    })
  }

  return (
    <Page title={t.analysis.title}>
      {header}

      <Card padding="none" className="flex flex-wrap items-center gap-3 p-4">
        <span className="min-w-0 flex-1 basis-[220px] font-medium">
          {quota.isPending ? (
            <Skeleton className="h-5 w-48" />
          ) : quota.data ? (
            t.analysis.quotaLeft(quota.data.left, quota.data.per_day)
          ) : (
            t.analysis.pickMatch
          )}
        </span>
        <Link to="/coach/analysis/example" className="text-[14px] text-accent-2 hover:underline">
          {t.analysis.exampleLink}
        </Link>
        {noQuota && resetIn > 0 && (
          <p className="basis-full text-[13px] text-muted">
            {t.analysis.quotaReset(formatRemaining(resetIn))}
          </p>
        )}
      </Card>

      <h2 className="caps mb-3 mt-6">{t.analysis.pickMatch}</h2>
      <p className="mb-3 text-[14px] text-muted">{t.analysis.pickHint}</p>

      {matches.isPending ? (
        <ul className="space-y-2">
          {Array.from({ length: 6 }, (_, i) => (
            <Skeleton key={i} radius={14} className="h-[64px]" />
          ))}
        </ul>
      ) : matches.isError ? (
        <Card padding="none">
          <ErrorState
            title={t.analysis.loadError}
            onRetry={() => matches.refetch()}
            retrying={matches.isRefetching}
          />
        </Card>
      ) : items.length === 0 ? (
        <Card padding="none">
          <EmptyState
            icon={Layers}
            title={t.analysis.noMatchesTitle}
            description={t.analysis.noMatchesText}
            action={
              <ButtonLink to="/coach" variant="accent2">
                {t.analysis.toCoach}
              </ButtonLink>
            }
          />
        </Card>
      ) : (
        <>
          <ul role="radiogroup" aria-label={t.analysis.pickMatch} className="space-y-2">
            {items.map((m) => (
              <MatchChoice
                key={m.match_id}
                match={m}
                selected={m.match_id === picked}
                onSelect={() => setPicked(m.match_id)}
              />
            ))}
          </ul>
          {hasNextPage && (
            <div className="mt-3 flex justify-center">
              <Button
                variant="surface"
                loading={isFetchingNextPage}
                onClick={() => void fetchNextPage()}
              >
                {t.common.more}
              </Button>
            </div>
          )}

          <div className="sticky bottom-[calc(var(--nav-h)+var(--nav-offset)+12px+env(safe-area-inset-bottom))] z-10 mt-5">
            {selected && hasAnalysis(selected) && selected.analysis ? (
              <ButtonLink
                to={`/coach/analysis/${selected.analysis.id}`}
                variant="accent2"
                size="lg"
                fullWidth
                className="shadow-2xl"
              >
                {selected.analysis.status === 'done' ? t.analysis.open : t.analysis.inProgress}
              </ButtonLink>
            ) : (
              <Button
                variant="accent"
                size="lg"
                fullWidth
                className="shadow-2xl"
                disabled={!selected || noQuota}
                loading={create.isPending}
                onClick={start}
              >
                {t.analysis.start}
              </Button>
            )}
          </div>
        </>
      )}
    </Page>
  )
}
