import { ArrowRight } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, Navigate } from 'react-router'
import { ApiError } from '@/api/client'
import type { CoachSummary, FormHero } from '@/api/types'
import { NoHistory } from '@/components/coach/NoHistory'
import { HeroPortrait } from '@/components/dota/HeroPortrait'
import { RankMedal } from '@/components/dota/RankMedal'
import { WinrateRing } from '@/components/dota/WinrateRing'
import { Avatar } from '@/components/ui/Avatar'
import { ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { ErrorState } from '@/components/ui/ErrorState'
import { Page } from '@/components/ui/Page'
import { Skeleton } from '@/components/ui/Skeleton'
import { markSummarySeen, useCoachAccount, useCoachSummary } from '@/features/useCoach'
import { useHero } from '@/features/useHeroes'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatPercent, pluralize } from '@/lib/format'
import { parseRankTier } from '@/lib/ranks'

// Сводка профиля (SPEC 5.3): ранг, винрейт за 20 игр и полгода, герои в форме.
export default function CoachSummaryPage() {
  const { accountId, loading } = useCoachAccount()
  const summary = useCoachSummary(accountId)

  // сводку показали — после этого вход ведёт сразу в «Мои матчи»
  useEffect(() => markSummarySeen(), [])

  if (loading || (accountId != null && summary.isPending)) return <SummarySkeleton />
  if (accountId == null) return <Navigate to="/coach" replace />
  if (summary.isError) {
    const notFound = summary.error instanceof ApiError && summary.error.status === 404
    return (
      <Page title={t.pages.coachSummary.title}>
        <Card padding="none">
          <ErrorState
            title={notFound ? t.coach.connect.notFound : undefined}
            description={summary.error.message}
            onRetry={() => summary.refetch()}
            retrying={summary.isRefetching}
          />
        </Card>
      </Page>
    )
  }
  const data = summary.data
  if (!data) return <SummarySkeleton />
  if (!data.has_history) {
    return (
      <Page title={t.pages.coachSummary.title}>
        <NoHistory summary={data} />
      </Page>
    )
  }
  return (
    <Page title={t.pages.coachSummary.title}>
      <SummaryBody summary={data} />
    </Page>
  )
}

function SummarySkeleton() {
  return (
    <Page title={t.pages.coachSummary.title}>
      <Skeleton className="h-4 w-32" />
      <Skeleton className="mt-5 h-20" radius={16} />
      <Skeleton className="mt-4 h-44" radius={16} />
      <Skeleton className="mt-4 h-14" radius={16} />
    </Page>
  )
}

function SummaryBody({ summary }: { summary: CoachSummary }) {
  const { player, last20, six_months: six } = summary
  const rank = player.rank_tier ? parseRankTier(player.rank_tier, player.leaderboard_rank) : null
  const total = last20.wins + last20.losses
  const winrate = total > 0 ? last20.wins / total : 0

  return (
    <>
      <div className="flex items-center">
        <h1 className="caps flex-1">{t.coach.summary.yourProfile}</h1>
        <Link
          to="/coach/connect"
          className="text-[14px] font-medium text-muted transition-colors hover:text-fg"
        >
          {t.coach.summary.change} ↗
        </Link>
      </div>

      <Card padding="none" className="mt-3 p-5">
        <div className="flex items-center gap-4">
          <Avatar src={player.avatar} name={player.persona_name} size={64} radius={14} />
          <div className="min-w-0 flex-1">
            <div className="truncate text-[22px] font-semibold leading-7">
              {player.persona_name || t.profile.noName}
            </div>
            <div className="text-[15px] text-muted">{rank?.label ?? t.coach.summary.unrated}</div>
          </div>
          <RankMedal
            rankTier={player.rank_tier}
            leaderboardRank={player.leaderboard_rank}
            size={48}
          />
        </div>

        <hr className="my-5 border-line" />

        <h2 className="caps">{t.coach.summary.last20}</h2>
        <div className="mt-4 flex items-center gap-8 max-sm:gap-5">
          <WinrateRing value={winrate} aria-label={formatPercent(winrate)}>
            <span className="text-[34px] font-bold leading-none tabular-nums">
              {formatPercent(winrate)}
            </span>
            <span className="mt-1 text-[13px] text-muted">{t.coach.summary.winrateWord}</span>
          </WinrateRing>
          <dl className="space-y-4">
            <div>
              <dt className="text-[14px] text-muted">{t.coach.summary.wins}</dt>
              <dd className="text-[32px] font-bold leading-9 tabular-nums">{last20.wins}</dd>
            </div>
            <div>
              <dt className="text-[14px] text-loss">{t.coach.summary.losses}</dt>
              <dd className="text-[32px] font-bold leading-9 tabular-nums">{last20.losses}</dd>
            </div>
          </dl>
        </div>

        <div className="mt-5 flex items-center justify-between rounded-[12px] bg-surface-2 px-4 py-3">
          {six.matches > 0 ? (
            <>
              <span className="text-[15px]">
                <b className="font-bold">{formatPercent(six.winrate)}</b>{' '}
                {t.coach.summary.sixMonths}
              </span>
              <span className="text-[14px] text-muted">
                {pluralize(six.matches, t.plural.match)}
              </span>
            </>
          ) : (
            <span className="text-[14px] text-muted">{t.coach.summary.noSixMonths}</span>
          )}
        </div>
      </Card>

      <FormHeroes summary={summary} />

      <ButtonLink to="/coach" size="lg" fullWidth className="mt-6">
        {t.coach.summary.openMatches}
        <ArrowRight size={18} aria-hidden />
      </ButtonLink>
    </>
  )
}

function FormHeroes({ summary }: { summary: CoachSummary }) {
  const periods = summary.heroes_in_form
  const [selected, setSelected] = useState(periods[0]?.period ?? 'all')
  const current = periods.find((p) => p.period === selected) ?? periods[0]

  return (
    <section className="mt-8">
      <div className="flex items-center gap-3">
        <h2 className="text-section-title flex-1">{t.coach.summary.form}</h2>
        {periods.length > 1 && (
          <div role="tablist" aria-label={t.coach.summary.form} className="flex gap-4">
            {periods.map((p) => (
              <button
                key={p.period}
                type="button"
                role="tab"
                aria-selected={p.period === current?.period}
                onClick={() => setSelected(p.period)}
                className={cn(
                  'text-[14px] font-medium transition-colors',
                  p.period === current?.period ? 'text-fg' : 'text-muted hover:text-fg',
                )}
              >
                {p.period === 'all' ? t.coach.summary.allVersions : summary.patch.label}
              </button>
            ))}
          </div>
        )}
      </div>

      {current && current.heroes.length > 0 ? (
        <ul className="mt-4 grid grid-cols-3 gap-3 max-sm:gap-2">
          {current.heroes.map((h) => (
            <li key={h.hero_id}>
              <FormHeroCard hero={h} />
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-4 rounded-[16px] border border-line bg-surface p-5 text-[15px] text-muted">
          {t.coach.summary.noForm}
        </p>
      )}
    </section>
  )
}

function FormHeroCard({ hero: stat }: { hero: FormHero }) {
  const hero = useHero(stat.hero_id)
  return (
    <Link
      to={`/tools/heroes/${stat.hero_id}`}
      className="group block overflow-hidden rounded-[16px] border border-line bg-surface transition-colors hover:bg-surface-2"
    >
      <HeroPortrait
        hero={hero}
        width={256}
        height={144}
        radius={0}
        fluid
        className="transition-transform duration-200 group-hover:scale-[1.03]"
      />
      <div className="p-3">
        <div className="truncate text-[14px] font-medium">
          {hero?.localized_name ?? t.heroes.unknown}
        </div>
        <div
          className={cn(
            'mt-0.5 text-[24px] font-bold leading-8 tabular-nums',
            stat.winrate >= 0.5 ? 'text-fg' : 'text-loss',
          )}
        >
          {formatPercent(stat.winrate)}
        </div>
        <div className="text-[13px] text-muted">{pluralize(stat.matches, t.plural.match)}</div>
      </div>
    </Link>
  )
}
