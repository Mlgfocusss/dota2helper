import { Search } from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import type { Hero, Position } from '@/api/types'
import { Chip } from '@/components/ui/Chip'
import { EmptyState } from '@/components/ui/EmptyState'
import { Input } from '@/components/ui/Field'
import { Sheet } from '@/components/ui/Sheet'
import { Skeleton } from '@/components/ui/Skeleton'
import { useHeroes } from '@/features/useHeroes'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { filterHeroes, moveHighlight, type AttrFilter } from '@/lib/heroSearch'
import { useIsDesktop } from '@/lib/hooks'
import { POSITIONS, positionFull } from '@/lib/positions'
import { HeroPortrait } from './HeroPortrait'
import { PositionIcon } from './PositionIcon'

type HeroPickerProps = {
  open: boolean
  onClose: () => void
  onPick: (hero: Hero) => void
  /** Герои, которых выбрать нельзя (уже выбраны, забанены) */
  disabledIds?: readonly number[]
  title?: string
  /**
   * Фильтр по позиции: playable[i] — герои, которых реально играют на позиции i + 1.
   * initial — позиция, выбранная при открытии.
   */
  positionFilter?: { playable: readonly (readonly number[])[]; initial?: Position | null }
}

const ATTRS: Exclude<AttrFilter, null>[] = ['str', 'agi', 'int', 'all']

/**
 * Выбор героя (SPEC 2.5): модалка 880×640 на десктопе, bottom‑sheet на мобиле.
 * Фокус сразу в поиске; стрелки двигают подсветку, Enter выбирает, Esc закрывает.
 */
export function HeroPicker(props: HeroPickerProps) {
  const { open, onClose } = props
  // состояние поиска живёт внутри и сбрасывается при каждом открытии
  return (
    <Sheet
      open={open}
      onClose={onClose}
      title={props.title ?? t.heroes.pick}
      width={880}
      height={640}
    >
      {open && <PickerBody {...props} />}
    </Sheet>
  )
}

function PickerBody({ onPick, disabledIds = [], positionFilter }: HeroPickerProps) {
  const { data, isPending, isError } = useHeroes()
  const isDesktop = useIsDesktop()
  const cols = isDesktop ? 8 : 4
  const [query, setQuery] = useState('')
  const [attr, setAttr] = useState<AttrFilter>(null)
  const [pos, setPos] = useState<Position | null>(positionFilter?.initial ?? null)
  const [highlight, setHighlight] = useState(0)
  const gridRef = useRef<HTMLDivElement>(null)

  const playableIds = useMemo(
    () => (pos && positionFilter ? new Set(positionFilter.playable[pos - 1] ?? []) : null),
    [pos, positionFilter],
  )
  const visible = useMemo(() => {
    const found = filterHeroes(data?.list ?? [], query, attr)
    // поиск по названию — по всем героям, фильтр позиции применяется только к полному списку
    return playableIds && !query.trim() ? found.filter((h) => playableIds.has(h.id)) : found
  }, [data?.list, query, attr, playableIds])
  const disabled = useMemo(() => new Set(disabledIds), [disabledIds])
  const active = Math.min(highlight, Math.max(0, visible.length - 1))

  useEffect(() => {
    gridRef.current
      ?.querySelector<HTMLElement>(`[data-index="${active}"]`)
      ?.scrollIntoView({ block: 'nearest' })
  }, [active])

  const pick = (hero: Hero) => {
    if (!disabled.has(hero.id)) onPick(hero)
  }

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter') {
      const hero = visible[active]
      if (hero) {
        e.preventDefault()
        pick(hero)
      }
      return
    }
    // в пустом поиске стрелки влево/вправо двигают подсветку, иначе — каретку
    if ((e.key === 'ArrowLeft' || e.key === 'ArrowRight') && query) return
    const next = moveHighlight(active, e.key, visible.length, cols)
    if (next !== null) {
      e.preventDefault()
      setHighlight(next)
    } else if (e.key.startsWith('Arrow') && e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') {
      e.preventDefault()
    }
  }

  return (
    <div onKeyDown={onKeyDown} className="flex h-full min-h-[420px] flex-col gap-3">
      <div className="relative">
        <Search
          size={18}
          aria-hidden
          className="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-dim"
        />
        <Input
          type="search"
          value={query}
          onChange={(e) => {
            setQuery(e.target.value)
            setHighlight(0)
          }}
          placeholder={t.heroes.searchPlaceholder}
          aria-label={t.heroes.searchLabel}
          autoComplete="off"
          className="pl-11"
        />
      </div>

      <div className="flex gap-2 overflow-x-auto pb-1" role="group" aria-label={t.heroes.attrLabel}>
        <Chip
          selected={attr === null}
          onClick={() => {
            setAttr(null)
            setHighlight(0)
          }}
        >
          {t.heroes.allAttrs}
        </Chip>
        {ATTRS.map((a) => (
          <Chip
            key={a}
            selected={attr === a}
            onClick={() => {
              setAttr(attr === a ? null : a)
              setHighlight(0)
            }}
          >
            {t.heroes.attrs[a]}
          </Chip>
        ))}
        {positionFilter && (
          <div
            className="ml-auto flex shrink-0 gap-1"
            role="group"
            aria-label={t.drafter.training.positionFilter}
          >
            {POSITIONS.map((p) => (
              <button
                key={p}
                type="button"
                aria-pressed={pos === p}
                aria-label={positionFull(p)}
                title={positionFull(p)}
                onClick={() => {
                  setPos(pos === p ? null : p)
                  setHighlight(0)
                }}
                className={cn(
                  'flex size-8 items-center justify-center rounded-[10px] border transition-colors',
                  pos === p
                    ? 'border-accent-2 bg-accent-2/15'
                    : 'border-line bg-bg hover:bg-surface-2',
                )}
              >
                <PositionIcon position={p} size={18} />
              </button>
            ))}
          </div>
        )}
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto pr-1" ref={gridRef}>
        {isPending ? (
          <div className="grid grid-cols-4 gap-2 md:grid-cols-8">
            {Array.from({ length: 24 }, (_, i) => (
              <Skeleton key={i} className="aspect-[16/11]" />
            ))}
          </div>
        ) : isError ? (
          <EmptyState title={t.states.errorTitle} description={t.states.errorText} />
        ) : visible.length === 0 ? (
          <EmptyState icon={Search} title={t.heroes.nothingFound} />
        ) : (
          <div
            role="listbox"
            aria-label={t.heroes.pick}
            className="grid grid-cols-4 gap-2 md:grid-cols-8"
          >
            {visible.map((hero, i) => {
              const off = disabled.has(hero.id)
              return (
                <button
                  key={hero.id}
                  type="button"
                  role="option"
                  aria-selected={i === active}
                  aria-disabled={off || undefined}
                  data-index={i}
                  tabIndex={-1}
                  onClick={() => pick(hero)}
                  onMouseMove={() => i !== active && setHighlight(i)}
                  className={cn(
                    'group rounded-[10px] p-1 text-left outline-none transition-colors',
                    i === active && 'bg-surface-2 ring-1 ring-accent-2',
                    off && 'cursor-not-allowed opacity-35',
                  )}
                >
                  <HeroPortrait
                    hero={hero}
                    width={104}
                    height={58}
                    fluid
                    radius={8}
                    grayscale={off}
                    className="block w-full"
                  />
                  <span className="mt-1 block truncate text-[11px] leading-4 text-muted group-hover:text-fg">
                    {hero.localized_name}
                  </span>
                </button>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
