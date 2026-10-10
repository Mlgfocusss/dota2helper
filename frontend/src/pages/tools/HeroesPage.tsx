import { LayoutGrid, Search, Shield, X } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link } from 'react-router'
import type { Hero, MetaPositions, Position } from '@/api/types'
import { ApiError } from '@/api/client'
import { HeroPortrait } from '@/components/dota/HeroPortrait'
import { PositionIcon } from '@/components/dota/PositionIcon'
import { Button, IconButton } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Chip } from '@/components/ui/Chip'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Input } from '@/components/ui/Field'
import { Page, PageHeader } from '@/components/ui/Page'
import { Skeleton } from '@/components/ui/Skeleton'
import { useMetaPositions } from '@/features/useContent'
import { useDrafterMatrix } from '@/features/useDrafter'
import { useHeroes } from '@/features/useHeroes'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { filterHeroes, splitMatch, type AttrFilter } from '@/lib/heroSearch'
import { POSITIONS, positionFull, positionShort } from '@/lib/positions'

const ATTRS: Exclude<AttrFilter, null>[] = ['str', 'agi', 'int', 'all']
const PREVIEW = 2
const EXPANDED = 10
/** Порог матчей на позицию: тот же, что на бэкенде (META_MIN_MATCHES по умолчанию). */
const MIN_MATCHES = 200

function Highlighted({ name, query }: { name: string; query: string }) {
  const [before, match, after] = splitMatch(name, query)
  return (
    <>
      {before}
      {match && <mark className="rounded-[3px] bg-accent-2/30 px-0.5 text-fg">{match}</mark>}
      {after}
    </>
  )
}

function HeroTile({ hero, query }: { hero: Hero; query: string }) {
  return (
    <Link
      to={`/tools/heroes/${hero.id}`}
      className="group rounded-[10px] text-left outline-none focus-visible:ring-2 focus-visible:ring-accent-2"
    >
      <span className="block overflow-hidden rounded-[8px] border border-transparent transition-[transform,border-color] group-hover:scale-[1.03] group-hover:border-accent-2">
        <HeroPortrait
          hero={hero}
          width={120}
          height={68}
          fluid
          radius={0}
          className="block w-full"
        />
      </span>
      <span className="mt-1 block truncate text-[12px] leading-4 text-muted group-hover:text-fg">
        <Highlighted name={hero.localized_name} query={query} />
      </span>
    </Link>
  )
}

// Герои (SPEC 4.3): поиск, полная сетка с фильтрами и «Сильные сейчас» по позициям.
export default function HeroesPage() {
  const { data: heroes, isPending, isError, refetch } = useHeroes()
  const [query, setQuery] = useState('')
  const [grid, setGrid] = useState(false)
  const [attr, setAttr] = useState<AttrFilter>(null)
  const [position, setPosition] = useState<Position | null>(null)

  const searching = query.trim().length > 0
  const showAll = grid || searching
  // составы позиций нужны только для фильтра в сетке — грузим матрицу, когда он открыт
  const matrix = useDrafterMatrix(grid)

  const found = useMemo(() => {
    const list = filterHeroes(heroes?.list ?? [], query, attr)
    if (!position || !matrix.data) return list
    const playable = new Set(matrix.data.playable[position - 1])
    return list.filter((h) => playable.has(h.id))
  }, [heroes?.list, query, attr, position, matrix.data])

  return (
    <Page title={t.heroesPage.title}>
      <PageHeader
        title={t.heroesPage.title}
        icon={<Shield size={30} className="text-accent-2" aria-hidden />}
      />

      <div className="flex items-center gap-2">
        <div className="relative min-w-0 flex-1">
          <Search
            size={18}
            aria-hidden
            className="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-dim"
          />
          <Input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t.heroesPage.searchLabel(heroes?.list.length ?? 0)}
            aria-label={t.heroesPage.searchAria}
            autoComplete="off"
            className="pl-11 pr-11"
          />
          {searching && (
            <button
              type="button"
              aria-label={t.heroesPage.clearSearch}
              onClick={() => setQuery('')}
              className="absolute right-3 top-1/2 flex size-7 -translate-y-1/2 items-center justify-center rounded-full text-muted transition-colors hover:text-fg"
            >
              <X size={16} aria-hidden />
            </button>
          )}
        </div>
        <IconButton
          variant={grid ? 'accent2' : 'surface'}
          aria-label={grid ? t.heroesPage.gridClose : t.heroesPage.gridToggle}
          aria-pressed={grid}
          title={grid ? t.heroesPage.gridClose : t.heroesPage.gridToggle}
          className="size-12"
          onClick={() => setGrid((v) => !v)}
        >
          <LayoutGrid size={20} />
        </IconButton>
      </div>

      {showAll ? (
        <section className="mt-5">
          {grid && (
            <div className="mb-4 space-y-3">
              <div className="flex flex-wrap gap-2" role="group" aria-label={t.heroes.attrLabel}>
                <Chip selected={attr === null} onClick={() => setAttr(null)}>
                  {t.heroes.allAttrs}
                </Chip>
                {ATTRS.map((a) => (
                  <Chip
                    key={a}
                    selected={attr === a}
                    onClick={() => setAttr(attr === a ? null : a)}
                  >
                    {t.heroes.attrs[a]}
                  </Chip>
                ))}
              </div>
              <div
                className="flex flex-wrap gap-2"
                role="group"
                aria-label={t.heroesPage.positionFilter}
              >
                <Chip selected={position === null} onClick={() => setPosition(null)}>
                  {t.heroesPage.allPositions}
                </Chip>
                {POSITIONS.map((p) => (
                  <Chip
                    key={p}
                    selected={position === p}
                    icon={<PositionIcon position={p} size={16} />}
                    onClick={() => setPosition(position === p ? null : p)}
                  >
                    {positionShort(p)}
                  </Chip>
                ))}
              </div>
            </div>
          )}

          {isPending ? (
            <div className="grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-5">
              {Array.from({ length: 15 }, (_, i) => (
                <Skeleton key={i} className="aspect-[16/11]" />
              ))}
            </div>
          ) : isError ? (
            <Card padding="none">
              <ErrorState title={t.heroesPage.loadError} onRetry={() => refetch()} />
            </Card>
          ) : found.length === 0 ? (
            <Card padding="none">
              <EmptyState icon={Search} title={t.heroesPage.nothingFound} />
            </Card>
          ) : (
            <>
              <p className="mb-3 text-[13px] text-dim" aria-live="polite">
                {t.heroesPage.resultsCount(found.length)}
              </p>
              <div className="grid grid-cols-3 gap-x-3 gap-y-4 sm:grid-cols-4 md:grid-cols-5">
                {found.map((h) => (
                  <HeroTile key={h.id} hero={h} query={query} />
                ))}
              </div>
            </>
          )}
        </section>
      ) : (
        <StrongNow />
      )}
    </Page>
  )
}

// ----- «Сильные сейчас» --------------------------------------------------------------

function StrongNow() {
  const meta = useMetaPositions(EXPANDED)
  const { data: heroes } = useHeroes()
  const [expanded, setExpanded] = useState<Set<Position>>(new Set())

  const toggle = (p: Position) =>
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(p)) next.delete(p)
      else next.add(p)
      return next
    })

  return (
    <section className="mt-8">
      <div className="mb-3 flex items-center gap-3">
        <h2 className="caps">{t.heroesPage.strongNow}</h2>
        {meta.data && (
          <span className="ml-auto font-mono text-[12px] font-semibold uppercase text-muted">
            {t.heroesPage.patch} {meta.data.patch.label}
          </span>
        )}
      </div>

      {meta.isPending ? (
        <Card padding="none" className="divide-y divide-line">
          {Array.from({ length: 5 }, (_, i) => (
            <div key={i} className="space-y-3 p-5">
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-8" />
              <Skeleton className="h-8" />
            </div>
          ))}
        </Card>
      ) : meta.isError ? (
        <Card padding="none">
          <ErrorState
            title={
              meta.error instanceof ApiError && meta.error.status === 503
                ? t.heroesPage.metaCollecting
                : t.heroesPage.loadError
            }
            description={
              meta.error instanceof ApiError && meta.error.status === 503
                ? undefined
                : t.states.errorText
            }
            onRetry={() => meta.refetch()}
            retrying={meta.isRefetching}
          />
        </Card>
      ) : (
        <>
          <Card padding="none" className="divide-y divide-line">
            {meta.data.positions.map((group) => (
              <PositionGroup
                key={group.position}
                group={group}
                heroes={heroes?.byId}
                expanded={expanded.has(group.position)}
                onToggle={() => toggle(group.position)}
              />
            ))}
          </Card>
          <p className="mt-3 text-[13px] text-dim">
            {t.heroesPage.footnote(MIN_MATCHES)}
            {!meta.data.by_position && <> · {t.heroesPage.overallNote}</>}
          </p>
        </>
      )}
    </section>
  )
}

function PositionGroup({
  group,
  heroes,
  expanded,
  onToggle,
}: {
  group: MetaPositions['positions'][number]
  heroes?: Map<number, Hero>
  expanded: boolean
  onToggle: () => void
}) {
  const list = group.heroes.slice(0, expanded ? EXPANDED : PREVIEW)
  const canExpand = group.heroes.length > PREVIEW
  return (
    <div className="p-5">
      <div className="mb-2 flex items-center gap-2">
        <PositionIcon position={group.position} size={18} title={positionFull(group.position)} />
        <h3 className="caps">{positionShort(group.position)}</h3>
        {canExpand && (
          <Button
            variant="ghost"
            size="sm"
            className="-my-1.5 ml-auto"
            aria-expanded={expanded}
            onClick={onToggle}
          >
            {expanded ? t.heroesPage.less : t.heroesPage.more}
          </Button>
        )}
      </div>
      <ul className="divide-y divide-line/60">
        {list.map((h) => {
          const hero = heroes?.get(h.hero_id)
          return (
            <li key={h.hero_id}>
              <Link
                to={`/tools/heroes/${h.hero_id}`}
                className="-mx-2 flex items-center gap-3 rounded-[10px] px-2 py-2 transition-colors hover:bg-surface-2"
              >
                <HeroPortrait hero={hero} width={48} height={27} radius={6} />
                <span className="min-w-0 flex-1 truncate text-[15px] font-medium">
                  {hero?.localized_name ?? t.heroes.unknown}
                </span>
                <span
                  className={cn('font-mono text-[15px] font-semibold tabular-nums text-win-strong')}
                >
                  {(h.winrate * 100).toFixed(1)}%
                </span>
              </Link>
            </li>
          )
        })}
      </ul>
    </div>
  )
}
