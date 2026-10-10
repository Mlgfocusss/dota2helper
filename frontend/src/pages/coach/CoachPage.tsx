import { ArrowUpRight, Layers, RotateCw, UserCog } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { Link, Navigate } from 'react-router'
import { steamLoginUrl } from '@/api/endpoints/system'
import { CoachLanding } from '@/components/coach/CoachLanding'
import { LastMatchesStrip } from '@/components/coach/LastMatchesStrip'
import { MatchListRow } from '@/components/coach/MatchListRow'
import { NoHistory } from '@/components/coach/NoHistory'
import { AnalysisStatusIcon } from '@/components/analysis/AnalysisStatusIcon'
import { HeroPortrait } from '@/components/dota/HeroPortrait'
import { RankMedal } from '@/components/dota/RankMedal'
import { Avatar } from '@/components/ui/Avatar'
import { Button, ButtonLink, IconButton } from '@/components/ui/Button'
import { buttonClass } from '@/components/ui/button-styles'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Page } from '@/components/ui/Page'
import { Skeleton } from '@/components/ui/Skeleton'
import { Tabs } from '@/components/ui/Tabs'
import { toast } from '@/features/toast/store'
import {
  hasSeenSummary,
  useCoachAccount,
  useCoachMatches,
  useCoachSummary,
  useRefreshCoach,
  type MatchFilter,
} from '@/features/useCoach'
import { useHero } from '@/features/useHeroes'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatPercent, formatRelativeTime } from '@/lib/format'
import { formatKda } from '@/lib/match'
import { parseRankTier } from '@/lib/ranks'
import { ApiError } from '@/api/client'
import type { CoachSummary } from '@/api/types'

// «Тренер»: гость — лендинг со входом, иначе «Мои матчи» выбранного игрока (SPEC 5.2, 5.4).
export default function CoachPage() {
  const { accountId, loggedIn, loading } = useCoachAccount()

  if (loading) {
    return (
      <Page title={t.pages.coach.title}>
        <Skeleton className="h-12 w-2/3" />
        <Skeleton className="mt-6 h-40" radius={16} />
      </Page>
    )
  }
  if (accountId == null) return <CoachLanding />
  // после первого входа сначала показываем сводку (SPEC 5.3)
  if (loggedIn && !hasSeenSummary()) return <Navigate to="/coach/summary" replace />
  return <MyMatches accountId={accountId} loggedIn={loggedIn} />
}

function MyMatches({ accountId, loggedIn }: { accountId: number; loggedIn: boolean }) {
  const summary = useCoachSummary(accountId)

  if (summary.isPending) return <MatchesSkeleton />
  if (summary.isError) {
    const notFound = summary.error instanceof ApiError && summary.error.status === 404
    return (
      <Page title={t.pages.coach.title}>
        <Card padding="none">
          <ErrorState
            title={notFound ? t.coach.connect.notFound : undefined}
            description={summary.error.message}
            onRetry={() => summary.refetch()}
            retrying={summary.isRefetching}
          />
          <div className="pb-8 text-center">
            <ButtonLink to="/coach/connect" variant="outline">
              {t.coach.matches.switchAccount}
            </ButtonLink>
          </div>
        </Card>
      </Page>
    )
  }
  if (!summary.data.has_history) {
    return (
      <Page title={t.pages.coach.title}>
        <NoHistory summary={summary.data} />
      </Page>
    )
  }
  return <MatchesView accountId={accountId} loggedIn={loggedIn} summary={summary.data} />
}

function MatchesSkeleton() {
  return (
    <Page title={t.pages.coach.title}>
      <div className="flex items-center gap-3">
        <Skeleton className="size-12" radius={12} />
        <Skeleton className="h-6 w-40" />
      </div>
      <Skeleton className="mt-6 h-24" radius={16} />
      <Skeleton className="mt-4 h-28" radius={16} />
      <Skeleton className="mt-4 h-24" radius={16} />
    </Page>
  )
}

function MatchesView({
  accountId,
  loggedIn,
  summary,
}: {
  accountId: number
  loggedIn: boolean
  summary: CoachSummary
}) {
  const [filter, setFilter] = useState<MatchFilter>('all')
  const first = useCoachMatches(accountId, 'all')
  const list = useCoachMatches(accountId, filter)
  const refresh = useRefreshCoach(accountId)

  const strip = first.data?.pages[0]?.items.slice(0, 20) ?? []
  const counts = first.data?.pages[0]?.counts
  const items = list.data?.pages.flatMap((p) => p.items) ?? []
  const total = summary.last20.wins + summary.last20.losses
  const winrate = total > 0 ? summary.last20.wins / total : 0

  const sentinel = useRef<HTMLDivElement>(null)
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = list
  useEffect(() => {
    const el = sentinel.current
    if (!el || !hasNextPage) return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting && !isFetchingNextPage) void fetchNextPage()
      },
      { rootMargin: '300px' },
    )
    observer.observe(el)
    return () => observer.disconnect()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage])

  return (
    <Page title={t.pages.coach.title}>
      <PlayerHeader summary={summary} />

      <Card padding="none" className="mt-5 p-5">
        <div className="flex items-baseline gap-3">
          <h2 className="caps flex-1">{t.coach.matches.last20}</h2>
          <span className="text-[22px] font-bold tabular-nums">
            {t.coach.matches.winrate(formatPercent(winrate))}
          </span>
        </div>
        <div className="mt-3">
          {strip.length > 0 ? (
            <LastMatchesStrip matches={strip} />
          ) : (
            <Skeleton className="h-[14px] w-full" radius={4} />
          )}
        </div>
      </Card>

      {summary.last_match && <LastMatchCard summary={summary} />}

      <DeepCta loggedIn={loggedIn} />

      <div className="mt-8 flex items-start gap-3">
        <div className="min-w-0 flex-1">
          <h2 className="text-section-title">{t.coach.matches.title}</h2>
          <p className="mt-1 text-[14px] text-muted">{t.coach.matches.subtitle}</p>
        </div>
        <IconButton
          aria-label={t.coach.matches.refresh}
          variant="surface"
          disabled={refresh.isPending}
          onClick={() =>
            refresh.mutate(undefined, { onSuccess: () => toast.success(t.coach.matches.refreshed) })
          }
        >
          <RotateCw size={20} className={cn(refresh.isPending && 'animate-spin')} />
        </IconButton>
      </div>

      <Tabs
        className="mt-4"
        aria-label={t.coach.matches.title}
        value={filter}
        onChange={setFilter}
        items={[
          { value: 'all', label: t.coach.matches.all, count: counts?.all },
          { value: 'win', label: t.coach.matches.win, count: counts?.win },
          { value: 'loss', label: t.coach.matches.loss, count: counts?.loss },
        ]}
      />

      <div className="mt-2">
        {list.isPending && (
          <div className="space-y-3 pt-3">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-14" radius={12} />
            ))}
          </div>
        )}
        {list.isError && <ErrorState onRetry={() => list.refetch()} retrying={list.isRefetching} />}
        {list.data && items.length === 0 && (
          <EmptyState
            title={t.coach.matches.emptyFilter}
            description={t.coach.matches.emptyFilterText}
          />
        )}
        {items.length > 0 && (
          <ul className="divide-y divide-line">
            {items.map((m) => (
              <li key={m.match_id}>
                <MatchListRow match={m} />
              </li>
            ))}
          </ul>
        )}
        {hasNextPage && (
          <div ref={sentinel} className="py-4 text-center">
            <Button variant="outline" loading={isFetchingNextPage} onClick={() => fetchNextPage()}>
              {t.coach.matches.more}
            </Button>
          </div>
        )}
      </div>
    </Page>
  )
}

/** Компактная шапка игрока: аватар, ник, ранг, «Сводка» и смена аккаунта. */
function PlayerHeader({ summary }: { summary: CoachSummary }) {
  const { persona_name, avatar, rank_tier, leaderboard_rank } = summary.player
  const rank = rank_tier ? parseRankTier(rank_tier, leaderboard_rank).label : null
  return (
    <div className="flex items-center gap-3">
      <Avatar src={avatar} name={persona_name} size={48} radius={12} />
      <div className="min-w-0 flex-1">
        <h1 className="truncate text-[20px] font-semibold leading-6">
          {persona_name || t.profile.noName}
        </h1>
        <div className="mt-0.5 flex items-center gap-1.5 text-[12px] font-semibold uppercase tracking-[0.06em] text-muted">
          <RankMedal
            rankTier={rank_tier}
            leaderboardRank={leaderboard_rank}
            size={16}
            showStars={false}
          />
          {rank ?? t.coach.summary.unrated}
        </div>
      </div>
      <Link
        to="/coach/summary"
        className="rounded-[10px] px-2 py-1.5 text-[15px] font-medium text-muted transition-colors hover:text-fg"
      >
        {t.coach.matches.summaryButton}
      </Link>
      <Link
        to="/coach/connect"
        aria-label={t.coach.matches.switchAccount}
        className={buttonClass({ variant: 'ghost', size: 'icon' })}
      >
        <UserCog size={22} />
      </Link>
    </div>
  )
}

function LastMatchCard({ summary }: { summary: CoachSummary }) {
  const match = summary.last_match
  const hero = useHero(match?.hero_id)
  if (!match) return null
  return (
    <Link
      to={`/coach/matches/${match.match_id}`}
      className="relative mt-4 flex gap-4 rounded-[16px] border border-line bg-surface p-4 transition-colors hover:bg-surface-2"
    >
      <HeroPortrait hero={hero} width={160} height={90} radius={10} className="max-sm:w-[120px]" />
      <div className="min-w-0 flex-1">
        <div className="text-[12px] text-muted">
          {t.coach.matches.lastMatch} · {formatRelativeTime(match.start_time)}
        </div>
        <div className="mt-1 truncate text-[20px] font-semibold">
          {hero?.localized_name ?? t.heroes.unknown}
        </div>
        <div className={cn('mt-1 text-[15px] font-medium', match.win ? 'text-win' : 'text-loss')}>
          {match.win ? t.coach.matches.victory : t.coach.matches.defeat} · {formatKda(match)}
        </div>
      </div>
      <AnalysisStatusIcon status={match.analysis?.status} className="absolute bottom-3 right-3" />
    </Link>
  )
}

function DeepCta({ loggedIn }: { loggedIn: boolean }) {
  const className =
    'mt-4 flex items-center gap-4 rounded-[16px] bg-accent p-5 text-on-accent transition-opacity hover:opacity-90'
  const body = (
    <>
      <Layers size={28} aria-hidden />
      <div className="min-w-0 flex-1">
        <div className="text-[18px] font-semibold">{t.coach.matches.deepTitle}</div>
        <div className="text-[14px] opacity-75">{t.coach.matches.deepText}</div>
      </div>
      <ArrowUpRight size={22} aria-hidden />
    </>
  )
  // гость идёт на вход и возвращается к выбору матча для разбора
  return loggedIn ? (
    <Link to="/coach/deep" className={className}>
      {body}
    </Link>
  ) : (
    <a href={steamLoginUrl('/coach/deep')} className={className}>
      {body}
    </a>
  )
}
