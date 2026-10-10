import { Crosshair } from 'lucide-react'
import { Link, useParams } from 'react-router'
import { ApiError } from '@/api/client'
import type { Hero, HeroPageData, HeroPageMatchup, ItemRef } from '@/api/types'
import { HeroIcon, HeroPortrait } from '@/components/dota/HeroPortrait'
import { PositionIcon } from '@/components/dota/PositionIcon'
import { ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Chip } from '@/components/ui/Chip'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Page } from '@/components/ui/Page'
import { Skeleton } from '@/components/ui/Skeleton'
import { Tooltip } from '@/components/ui/Tooltip'
import { useHeroPage } from '@/features/useDrafter'
import { useHeroes } from '@/features/useHeroes'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatNumber, formatSigned } from '@/lib/format'
import { itemImageUrl } from '@/lib/match'
import { positionColor, positionFull, positionShort } from '@/lib/positions'

const STAGES = ['start', 'early', 'mid', 'late'] as const

function Difficulty({ level }: { level: number }) {
  const label = t.heroPage.difficultyLevels[level as 1 | 2 | 3]
  return (
    <div className="flex items-center gap-2 text-[14px] text-muted" title={label}>
      <span>{t.heroPage.difficulty}</span>
      <span className="flex gap-1" role="img" aria-label={label}>
        {[1, 2, 3].map((i) => (
          <span
            key={i}
            aria-hidden
            className={cn('size-2.5 rounded-full', i <= level ? 'bg-accent-2' : 'bg-surface-2')}
          />
        ))}
      </span>
    </div>
  )
}

function HeroHeader({ hero, difficulty }: { hero: Hero; difficulty: number }) {
  return (
    <Card padding="none" className="overflow-hidden">
      <HeroPortrait
        hero={hero}
        width={500}
        height={281}
        fluid
        radius={0}
        className="block w-full"
      />
      <div className="space-y-3 p-5">
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-page-title">{hero.localized_name}</h1>
          <Chip tone="accent" size="sm">
            {t.heroes.attrs[hero.primary_attr]}
          </Chip>
        </div>
        {hero.roles.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {hero.roles.map((r) => (
              <Chip key={r} size="sm" tone="outline">
                {t.heroPage.roles[r] ?? r}
              </Chip>
            ))}
          </div>
        )}
        {difficulty > 0 && <Difficulty level={difficulty} />}
      </div>
    </Card>
  )
}

function Positions({ data }: { data: HeroPageData }) {
  return (
    <Card padding="none" className="p-5">
      <h2 className="caps mb-1">{t.heroPage.positions}</h2>
      {!data.by_position && (
        <p className="mb-3 text-[12px] text-dim">{t.heroPage.positionsOverall}</p>
      )}
      {data.positions.length === 0 ? (
        <p className="mt-3 text-[14px] text-muted">{t.heroPage.positionsEmpty}</p>
      ) : (
        <ul className="mt-3 space-y-3">
          {data.positions.map((p) => (
            <li key={p.position} className="flex items-center gap-3">
              <span className="flex w-[120px] shrink-0 items-center gap-2 text-[14px]">
                <PositionIcon position={p.position} size={20} title={positionFull(p.position)} />
                {positionShort(p.position)}
              </span>
              <div
                role="img"
                aria-label={`${positionShort(p.position)}: ${(p.winrate * 100).toFixed(1)}%`}
                className="relative h-2.5 flex-1 overflow-hidden rounded-full bg-surface-2"
              >
                <div
                  className="h-full rounded-full"
                  style={{
                    width: `${Math.min(100, p.winrate * 100)}%`,
                    backgroundColor: positionColor(p.position),
                  }}
                />
                {/* отметка 50% */}
                <span aria-hidden className="absolute inset-y-0 left-1/2 w-px bg-bg/70" />
              </div>
              <span className="w-[56px] text-right font-mono text-[14px] font-semibold tabular-nums">
                {(p.winrate * 100).toFixed(1)}%
              </span>
              <span className="hidden w-[108px] whitespace-nowrap text-right text-[12px] text-dim sm:block">
                {formatNumber(p.matches)} {t.heroPage.matches}
              </span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}

function MatchupList({
  title,
  rows,
  heroes,
}: {
  title: string
  rows: HeroPageMatchup[]
  heroes?: Map<number, Hero>
}) {
  return (
    <Card padding="none" className="p-5">
      <h2 className="caps mb-3">{title}</h2>
      {rows.length === 0 ? (
        <p className="text-[14px] text-muted">{t.heroPage.matchupsEmpty}</p>
      ) : (
        <ul className="divide-y divide-line/60">
          {rows.map((r) => {
            const hero = heroes?.get(r.hero_id)
            return (
              <li key={r.hero_id}>
                <Link
                  to={`/tools/heroes/${r.hero_id}`}
                  className="-mx-2 flex items-center gap-3 rounded-[10px] px-2 py-2 transition-colors hover:bg-surface-2"
                >
                  <HeroIcon hero={hero} width={32} height={32} radius={8} />
                  <span className="min-w-0 flex-1 truncate text-[15px]">
                    {hero?.localized_name ?? t.heroes.unknown}
                  </span>
                  <span
                    className={cn(
                      'font-mono text-[15px] font-semibold tabular-nums',
                      r.value >= 0 ? 'text-win-strong' : 'text-loss-strong',
                    )}
                    title={t.heroPage.advantage}
                  >
                    {formatSigned(r.value)}
                  </span>
                </Link>
              </li>
            )
          })}
        </ul>
      )}
    </Card>
  )
}

function ItemImg({ item }: { item: ItemRef }) {
  return (
    <Tooltip content={item.name}>
      <img
        src={itemImageUrl(item.key)}
        alt={item.name}
        width={55}
        height={40}
        loading="lazy"
        className="h-10 w-[55px] rounded-[6px] bg-surface-2 object-cover"
      />
    </Tooltip>
  )
}

function Items({ items }: { items: HeroPageData['items'] }) {
  return (
    <Card padding="none" className="p-5">
      <h2 className="caps mb-3">{t.heroPage.items}</h2>
      <div className="space-y-4">
        {STAGES.map((stage) => (
          <div key={stage}>
            <h3 className="mb-2 text-[13px] font-medium text-muted">{t.heroPage.stages[stage]}</h3>
            {items[stage].length === 0 ? (
              <p className="text-[13px] text-dim">{t.heroPage.itemsEmpty}</p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {items[stage].map((it) => (
                  <ItemImg key={it.key} item={it} />
                ))}
              </div>
            )}
          </div>
        ))}
      </div>
    </Card>
  )
}

// Страница героя (SPEC 4.3): шапка и позиции слева, матчапы, союзники и предметы справа.
export default function HeroPage() {
  const params = useParams()
  const heroId = Number(params.heroId)
  const { data: heroes, isPending: heroesPending } = useHeroes()
  const page = useHeroPage(Number.isInteger(heroId) ? heroId : 0)
  const hero = heroes?.byId.get(heroId)

  const notFound =
    !Number.isInteger(heroId) ||
    (page.error instanceof ApiError && page.error.status === 404) ||
    (heroes !== undefined && !hero)

  if (notFound) {
    return (
      <Page width={1000} title={t.heroPage.notFoundTitle}>
        <Card padding="none" className="mt-4">
          <EmptyState
            icon={Crosshair}
            title={t.heroPage.notFoundTitle}
            description={t.heroPage.notFoundText}
            action={
              <ButtonLink to="/tools/heroes" variant="accent2">
                {t.heroPage.toList}
              </ButtonLink>
            }
          />
        </Card>
      </Page>
    )
  }

  if (page.isError) {
    return (
      <Page width={1000} title={hero?.localized_name}>
        <Card padding="none">
          <ErrorState onRetry={() => page.refetch()} retrying={page.isRefetching} />
        </Card>
      </Page>
    )
  }

  if (heroesPending || page.isPending || !hero || !page.data) {
    return (
      <Page width={1000}>
        <div className="grid gap-6 xl:grid-cols-2">
          <div className="space-y-6">
            <Skeleton radius={16} className="h-[420px]" />
            <Skeleton radius={16} className="h-[220px]" />
          </div>
          <div className="space-y-6">
            <Skeleton radius={16} className="h-[300px]" />
            <Skeleton radius={16} className="h-[300px]" />
          </div>
        </div>
      </Page>
    )
  }

  const data = page.data
  // позиция для драфтера: самая сыгранная; без данных — первая
  const mainPosition = [...data.positions].sort((a, b) => b.matches - a.matches)[0]?.position ?? 1

  return (
    <Page width={1000} title={hero.localized_name}>
      <div className="grid items-start gap-6 xl:grid-cols-2">
        <div className="space-y-6">
          <HeroHeader hero={hero} difficulty={data.difficulty} />
          <Positions data={data} />
          <ButtonLink
            to={`/tools/drafter?tab=analysis&r=${mainPosition}:${hero.id}`}
            variant="accent"
            size="lg"
            fullWidth
          >
            <Crosshair size={20} aria-hidden />
            {t.heroPage.takeToDrafter}
          </ButtonLink>
        </div>
        <div className="space-y-6">
          <MatchupList
            title={t.heroPage.strongAgainst}
            rows={data.strong_against}
            heroes={heroes?.byId}
          />
          <MatchupList
            title={t.heroPage.weakAgainst}
            rows={data.weak_against}
            heroes={heroes?.byId}
          />
          <MatchupList title={t.heroPage.allies} rows={data.allies} heroes={heroes?.byId} />
          <Items items={data.items} />
        </div>
      </div>
    </Page>
  )
}
