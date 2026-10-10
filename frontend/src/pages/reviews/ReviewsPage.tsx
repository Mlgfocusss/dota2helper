import { ChevronRight, Dumbbell, Layers, Target } from 'lucide-react'
import { Link, useSearchParams } from 'react-router'
import type { AnalysisListItem } from '@/api/types'
import { AnalysisStatusIcon } from '@/components/analysis/AnalysisStatusIcon'
import { PlanChecklist } from '@/components/analysis/PlanChecklist'
import { SteamLoginButton } from '@/components/auth/SteamLoginButton'
import { HeroPortrait } from '@/components/dota/HeroPortrait'
import { ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Page, PageHeader } from '@/components/ui/Page'
import { Skeleton } from '@/components/ui/Skeleton'
import { Tabs } from '@/components/ui/Tabs'
import { useAnalysisList } from '@/features/useAnalysis'
import { useHero } from '@/features/useHeroes'
import { useMe } from '@/features/useMe'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatRelativeTime } from '@/lib/format'
import { formatKda } from '@/lib/match'

type ReviewsTab = 'all' | 'train'

/** Сколько последних готовых разборов попадает в «Что тренировать». */
const TRAIN_FROM = 3

const when = (it: AnalysisListItem) => formatRelativeTime(it.start_time ?? it.created_at)

function ReviewRow({ item }: { item: AnalysisListItem }) {
  const hero = useHero(item.hero_id)
  const failed = item.status === 'failed'
  const heroName = hero?.localized_name ?? t.heroes.unknown
  return (
    <li>
      <Link
        to={
          failed
            ? `/coach/deep?match_id=${item.match_id}`
            : `/coach/analysis/${encodeURIComponent(item.id)}`
        }
        className="flex items-center gap-4 rounded-[14px] border border-line bg-surface px-4 py-3 transition-colors hover:bg-surface-2"
      >
        <HeroPortrait hero={hero} width={56} height={32} radius={6} className="shrink-0" />
        <span className="min-w-0 flex-1">
          <span className="block truncate font-semibold">{heroName}</span>
          <span className="mt-0.5 block truncate text-[13px] text-muted">
            {item.win !== undefined && (
              <>
                <span className={item.win ? 'text-win' : 'text-loss'}>
                  {item.win ? t.coach.matches.victory : t.coach.matches.defeat}
                </span>
                {' · '}
              </>
            )}
            {when(item)}
            {failed && item.error ? ` · ${item.error}` : null}
          </span>
        </span>
        {item.kills !== undefined && (
          <span className="shrink-0 font-mono text-[14px] tabular-nums max-sm:hidden">
            {formatKda({ kills: item.kills, deaths: item.deaths ?? 0, assists: item.assists ?? 0 })}
          </span>
        )}
        {item.rating !== undefined && (
          <span className="shrink-0 rounded-full bg-surface-2 px-2.5 py-0.5 font-mono text-[13px] font-semibold">
            {t.reviews.rating(item.rating)}
          </span>
        )}
        <AnalysisStatusIcon status={item.status} />
        <span className="shrink-0 text-dim max-sm:hidden" aria-hidden>
          {failed ? t.reviews.retry : <ChevronRight size={18} />}
        </span>
      </Link>
    </li>
  )
}

function AllReviews({ items }: { items: AnalysisListItem[] }) {
  return (
    <ul className="space-y-2">
      {items.map((it) => (
        <ReviewRow key={it.id} item={it} />
      ))}
    </ul>
  )
}

function TrainGroup({ item, first }: { item: AnalysisListItem; first: boolean }) {
  const hero = useHero(item.hero_id)
  return (
    <div className={cn(!first && 'mt-6')}>
      <Link
        to={`/coach/analysis/${encodeURIComponent(item.id)}`}
        className="mb-3 flex items-center gap-2 text-[14px] text-muted transition-colors hover:text-fg"
      >
        <span className="truncate">
          {t.reviews.matchOn(hero?.localized_name ?? t.heroes.unknown)} · {when(item)}
        </span>
        <ChevronRight size={16} aria-hidden className="shrink-0" />
      </Link>
      <div className="grid gap-3 sm:grid-cols-2">
        {item.improvements?.map((im) => (
          <Card key={im.title} padding="none" className="flex flex-col p-5">
            <h3 className="text-card-title">{im.title}</h3>
            <p className="mt-2 text-[14px] text-muted">
              <span className="mr-1 inline-flex align-text-bottom">
                <Target size={14} aria-hidden />
              </span>
              {im.why}
            </p>
            <p className="mt-3 border-t border-line pt-3 text-[14px]">
              <span className="mr-1 inline-flex align-text-bottom text-win">
                <Dumbbell size={14} aria-hidden />
              </span>
              <span className="font-semibold">{t.analysis.result.drill}: </span>
              {im.drill}
            </p>
          </Card>
        ))}
      </div>
    </div>
  )
}

/** «Что тренировать»: план из последнего разбора и слабые места из нескольких последних. */
function TrainView({ items }: { items: AnalysisListItem[] }) {
  const done = items
    .filter((it) => it.status === 'done' && (it.improvements?.length ?? 0) > 0)
    .slice(0, TRAIN_FROM)
  const latest = done[0]

  if (!latest) {
    return (
      <Card padding="none">
        <EmptyState
          icon={Dumbbell}
          title={t.reviews.trainEmptyTitle}
          description={t.reviews.trainEmptyText}
          action={
            <ButtonLink to="/coach/deep" variant="accent2">
              {t.reviews.new}
            </ButtonLink>
          }
        />
      </Card>
    )
  }

  return (
    <div className="space-y-8">
      {latest.next_games_plan && latest.next_games_plan.length > 0 && (
        <div>
          <p className="mb-3 text-[13px] text-dim">{t.reviews.planFrom(when(latest))}</p>
          <PlanChecklist analysisId={latest.id} items={latest.next_games_plan} />
        </div>
      )}
      <section aria-labelledby="train-title">
        <h2 id="train-title" className="caps mb-1">
          {t.reviews.trainTitle}
        </h2>
        <p className="mb-4 text-[13px] text-dim">{t.reviews.trainFrom}</p>
        {done.map((it, i) => (
          <TrainGroup key={it.id} item={it} first={i === 0} />
        ))}
      </section>
    </div>
  )
}

function ListSkeleton() {
  return (
    <div className="space-y-2">
      {Array.from({ length: 5 }, (_, i) => (
        <Skeleton key={i} radius={14} className="h-[64px]" />
      ))}
    </div>
  )
}

// Вкладка «Разборы»: все разборы пользователя и отдельный экран «Что тренировать».
export default function ReviewsPage() {
  const [params, setParams] = useSearchParams()
  const tab: ReviewsTab = params.get('tab') === 'train' ? 'train' : 'all'
  const { data: me, isPending: mePending } = useMe()
  const loggedIn = Boolean(me?.user)
  const list = useAnalysisList(loggedIn)

  const header = (
    <PageHeader
      title={t.reviews.title}
      subtitle={t.reviews.subtitle}
      icon={<Layers size={30} className="text-accent-2" aria-hidden />}
      actions={
        loggedIn ? (
          <ButtonLink to="/coach/deep" variant="accent2" size="sm" className="max-sm:hidden">
            {t.reviews.new}
          </ButtonLink>
        ) : undefined
      }
    />
  )

  if (mePending) {
    return (
      <Page title={t.reviews.title}>
        {header}
        <ListSkeleton />
      </Page>
    )
  }

  if (!loggedIn) {
    return (
      <Page title={t.reviews.title}>
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

  const items = list.data ?? []

  return (
    <Page title={t.reviews.title}>
      {header}

      <ButtonLink to="/coach/deep" variant="accent2" fullWidth className="mb-4 sm:hidden">
        {t.reviews.new}
      </ButtonLink>

      <Tabs
        aria-label={t.reviews.title}
        value={tab}
        onChange={(value) => setParams(value === 'all' ? {} : { tab: value }, { replace: true })}
        items={[
          { value: 'all', label: t.reviews.tabs.all, count: list.data ? items.length : undefined },
          { value: 'train', label: t.reviews.tabs.train },
        ]}
      />

      <div className="mt-5">
        {list.isPending ? (
          <ListSkeleton />
        ) : list.isError ? (
          <Card padding="none">
            <ErrorState
              title={t.reviews.listError}
              onRetry={() => list.refetch()}
              retrying={list.isRefetching}
            />
          </Card>
        ) : tab === 'train' ? (
          <TrainView items={items} />
        ) : items.length === 0 ? (
          <Card padding="none">
            <EmptyState
              icon={Layers}
              title={t.reviews.emptyTitle}
              description={t.reviews.emptyText}
              action={
                <ButtonLink to="/coach/deep" variant="accent2">
                  {t.reviews.new}
                </ButtonLink>
              }
            />
          </Card>
        ) : (
          <AllReviews items={items} />
        )}
      </div>
    </Page>
  )
}
