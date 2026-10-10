import { ChevronDown, History } from 'lucide-react'
import { useState } from 'react'
import type { TrainingHistoryItem } from '@/api/types'
import { SteamLoginButton } from '@/components/auth/SteamLoginButton'
import { HeroIcon } from '@/components/dota/HeroPortrait'
import { PairView, PanelBlock } from '@/components/drafter/PairView'
import { Button, ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Page, PageHeader } from '@/components/ui/Page'
import { Skeleton } from '@/components/ui/Skeleton'
import { useDrafterHistory } from '@/features/useDrafter'
import { useHeroes } from '@/features/useHeroes'
import { useMe } from '@/features/useMe'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatShortDate, formatSigned } from '@/lib/format'
import { positionShort } from '@/lib/positions'

function scoreTone(score: number): string {
  if (score >= 70) return 'text-win-strong'
  if (score < 40) return 'text-loss-strong'
  return 'text-fg'
}

function IconRow({ ids, label }: { ids: number[]; label: string }) {
  const { data } = useHeroes()
  return (
    <div className="flex gap-1" role="list" aria-label={label}>
      {ids.map((id) => {
        const hero = data?.byId.get(id)
        return (
          <span key={id} role="listitem" title={hero?.localized_name}>
            <HeroIcon hero={hero} width={32} height={32} radius={8} />
          </span>
        )
      })}
    </div>
  )
}

function HistoryRow({ item }: { item: TrainingHistoryItem }) {
  const [open, setOpen] = useState(false)
  const { data } = useHeroes()
  const b = item.breakdown
  const teamIds = [...item.team].sort((a, c) => a.position - c.position).map((s) => s.hero_id)
  const enemyIds = [...item.enemies].sort((a, c) => a.position - c.position).map((s) => s.hero_id)

  return (
    <li className="border-b border-line last:border-0">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        className="flex w-full flex-wrap items-center gap-x-4 gap-y-2 px-5 py-4 text-left transition-colors hover:bg-surface-2"
      >
        <span className="w-[72px] shrink-0 text-[14px] text-muted">
          {formatShortDate(item.submitted_at)}
        </span>
        <span
          className={cn(
            'w-[64px] shrink-0 font-mono text-[22px] font-semibold tabular-nums',
            scoreTone(item.score),
          )}
        >
          {item.score.toFixed(1)}
        </span>
        <IconRow ids={teamIds} label={t.drafter.history.you} />
        <span className="text-[12px] text-dim">{t.drafter.history.vs}</span>
        <IconRow ids={enemyIds} label={t.drafter.history.enemies} />
        <ChevronDown
          size={18}
          aria-hidden
          className={cn('ml-auto shrink-0 text-muted transition-transform', open && 'rotate-180')}
        />
      </button>
      {open && b && (
        <div className="grid gap-5 px-5 pb-5 pt-1 md:grid-cols-2">
          <PanelBlock title={t.drafter.training.breakdown} className="border-t-0 pt-0">
            <dl className="space-y-2 text-[14px]">
              {(['vs', 'synergy', 'meta', 'penalty'] as const).map((key) => {
                const value = key === 'penalty' ? -b.contrib.penalty : b.contrib[key]
                return (
                  <div key={key} className="flex items-center justify-between gap-3">
                    <dt className="text-muted">{t.drafter.training.contrib[key]}</dt>
                    <dd
                      className={cn(
                        'font-mono font-semibold tabular-nums',
                        value > 0
                          ? 'text-win-strong'
                          : value < 0
                            ? 'text-loss-strong'
                            : 'text-muted',
                      )}
                    >
                      {formatSigned(value)}
                    </dd>
                  </div>
                )
              })}
            </dl>
            <ul className="mt-3 space-y-1 text-[13px] text-muted">
              {b.heroes.map((h) => (
                <li key={h.position} className="flex items-center gap-2">
                  <span className="w-[72px] shrink-0">{positionShort(h.position)}</span>
                  <span className="truncate">
                    {data?.byId.get(h.hero_id)?.localized_name ?? t.heroes.unknown}
                  </span>
                  <span className="ml-auto font-mono tabular-nums">
                    {formatSigned(h.vs + h.synergy + h.meta)}
                  </span>
                </li>
              ))}
            </ul>
          </PanelBlock>
          <div className="space-y-4">
            {b.best_matchup && (
              <PanelBlock title={t.drafter.training.bestMatchup} className="border-t-0 pt-0">
                <PairView pair={b.best_matchup} kind="vs" />
              </PanelBlock>
            )}
            {b.worst_matchup && (
              <PanelBlock
                title={t.drafter.training.worstMatchup}
                className={b.best_matchup ? undefined : 'border-t-0 pt-0'}
              >
                <PairView pair={b.worst_matchup} kind="vs" />
              </PanelBlock>
            )}
          </div>
        </div>
      )}
    </li>
  )
}

// История тренировок (SPEC 4.2): дата, счёт, пятёрки; клик раскрывает разбор.
export default function DrafterHistoryPage() {
  const { data: me, isPending: mePending } = useMe()
  const loggedIn = Boolean(me?.user)
  const history = useDrafterHistory(loggedIn)

  const header = (
    <PageHeader
      title={t.drafter.history.title}
      subtitle={t.drafter.history.subtitle}
      icon={<History size={30} className="text-accent-2" aria-hidden />}
    />
  )

  if (mePending || (loggedIn && history.isPending)) {
    return (
      <Page width={1000} title={t.drafter.history.title}>
        {header}
        <Card padding="none" className="divide-y divide-line">
          {Array.from({ length: 4 }, (_, i) => (
            <div key={i} className="px-5 py-4">
              <Skeleton className="h-8" />
            </div>
          ))}
        </Card>
      </Page>
    )
  }

  if (!loggedIn) {
    return (
      <Page width={1000} title={t.drafter.history.title}>
        {header}
        <Card padding="none">
          <EmptyState
            icon={History}
            title={t.drafter.history.guestTitle}
            description={t.drafter.history.guestText}
            action={<SteamLoginButton size="lg" />}
          />
        </Card>
      </Page>
    )
  }

  if (history.isError) {
    return (
      <Page width={1000} title={t.drafter.history.title}>
        {header}
        <Card padding="none">
          <ErrorState onRetry={() => history.refetch()} retrying={history.isRefetching} />
        </Card>
      </Page>
    )
  }

  const items = history.data?.pages.flatMap((p) => p.items) ?? []
  return (
    <Page width={1000} title={t.drafter.history.title}>
      {header}
      {items.length === 0 ? (
        <Card padding="none">
          <EmptyState
            icon={History}
            title={t.drafter.history.emptyTitle}
            description={t.drafter.history.emptyText}
            action={
              <ButtonLink to="/tools/drafter?tab=training" variant="accent2">
                {t.drafter.history.start}
              </ButtonLink>
            }
          />
        </Card>
      ) : (
        <>
          <Card padding="none">
            <ul>
              {items.map((item) => (
                <HistoryRow key={item.id} item={item} />
              ))}
            </ul>
          </Card>
          {history.hasNextPage && (
            <div className="mt-4 flex justify-center">
              <Button
                variant="surface"
                loading={history.isFetchingNextPage}
                onClick={() => void history.fetchNextPage()}
              >
                {t.drafter.history.loadMore}
              </Button>
            </div>
          )}
        </>
      )}
    </Page>
  )
}
