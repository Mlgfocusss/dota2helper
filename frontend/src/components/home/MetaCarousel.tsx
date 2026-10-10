import { AnimatePresence, motion, type PanInfo } from 'framer-motion'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useRef, useState, type KeyboardEvent, type MouseEvent } from 'react'
import { Link } from 'react-router'
import type { MetaPositions } from '@/api/types'
import { HeroPortrait } from '@/components/dota/HeroPortrait'
import { PositionIcon } from '@/components/dota/PositionIcon'
import { IconButton } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { ErrorState } from '@/components/ui/ErrorState'
import { Skeleton } from '@/components/ui/Skeleton'
import { useMetaPositions } from '@/features/useContent'
import { useHeroes } from '@/features/useHeroes'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatPercent } from '@/lib/format'
import { POSITIONS, positionFull, positionShort } from '@/lib/positions'

const SLIDES = POSITIONS.length
const SWIPE_PX = 50

const slideVariants = {
  enter: (dir: number) => ({ x: dir >= 0 ? 48 : -48, opacity: 0 }),
  center: { x: 0, opacity: 1 },
  exit: (dir: number) => ({ x: dir >= 0 ? -48 : 48, opacity: 0 }),
}

/** «Мета патча»: по слайду на позицию, топ‑5 героев по винрейту (SPEC 5.1). */
export function MetaCarousel() {
  const meta = useMetaPositions(5)
  // [номер слайда, направление анимации]
  const [[index, direction], setSlide] = useState<[number, number]>([0, 0])
  const go = (delta: number) => setSlide(([i]) => [(i + delta + SLIDES) % SLIDES, delta])
  const goTo = (target: number) => setSlide(([i]) => [target, target >= i ? 1 : -1])

  return (
    <Card padding="none" className="p-5">
      <div className="flex items-center gap-2.5">
        <h2 className="caps">{t.home.metaTitle}</h2>
        {meta.data && (
          <span className="font-mono text-[12px] font-semibold text-muted">
            {meta.data.patch.label}
          </span>
        )}
        <div className="-mr-1.5 ml-auto flex">
          <IconButton
            aria-label={t.home.metaPrev}
            size="icon-sm"
            disabled={!meta.data}
            onClick={() => go(-1)}
          >
            <ChevronLeft size={20} />
          </IconButton>
          <IconButton
            aria-label={t.home.metaNext}
            size="icon-sm"
            disabled={!meta.data}
            onClick={() => go(1)}
          >
            <ChevronRight size={20} />
          </IconButton>
        </div>
      </div>

      {meta.isPending && <CarouselSkeleton />}
      {meta.isError && (
        <ErrorState
          className="py-6"
          description={meta.error.message}
          onRetry={() => meta.refetch()}
          retrying={meta.isRefetching}
        />
      )}
      {meta.data && (
        <>
          <Slides data={meta.data} index={index} direction={direction} onSwipe={go} />
          <div className="mt-4 flex items-center justify-center gap-1.5">
            {POSITIONS.map((pos, i) => (
              <button
                key={pos}
                type="button"
                aria-label={positionFull(pos)}
                aria-current={i === index || undefined}
                onClick={() => goTo(i)}
                className="flex h-4 items-center"
              >
                <span
                  className={cn(
                    'block h-1.5 rounded-full transition-all duration-300',
                    i === index ? 'w-5 bg-accent-2' : 'w-1.5 bg-line hover:bg-muted',
                  )}
                />
              </button>
            ))}
          </div>
        </>
      )}
    </Card>
  )
}

type SlidesProps = {
  data: MetaPositions
  index: number
  direction: number
  onSwipe: (delta: number) => void
}

function Slides({ data, index, direction, onSwipe }: SlidesProps) {
  const heroes = useHeroes()
  const dragged = useRef(false)
  const slide = data.positions[index]
  const position = slide?.position ?? POSITIONS[index]

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'ArrowLeft') onSwipe(-1)
    if (e.key === 'ArrowRight') onSwipe(1)
  }
  const onDragEnd = (_: unknown, info: PanInfo) => {
    if (info.offset.x < -SWIPE_PX) onSwipe(1)
    else if (info.offset.x > SWIPE_PX) onSwipe(-1)
    // клик, которым закончился свайп, не должен открывать героя
    window.setTimeout(() => {
      dragged.current = false
    }, 0)
  }
  const blockClickAfterDrag = (e: MouseEvent) => {
    if (dragged.current) e.preventDefault()
  }

  return (
    <div
      role="group"
      aria-roledescription="карусель"
      aria-label={t.home.metaTitle}
      tabIndex={0}
      onKeyDown={onKeyDown}
      className="relative mt-4 overflow-hidden rounded-[10px] outline-offset-4"
    >
      <AnimatePresence initial={false} custom={direction} mode="popLayout">
        <motion.div
          key={index}
          custom={direction}
          variants={slideVariants}
          initial="enter"
          animate="center"
          exit="exit"
          transition={{ duration: 0.24, ease: 'easeOut' }}
          drag="x"
          dragConstraints={{ left: 0, right: 0 }}
          dragElastic={0.18}
          onDragStart={() => {
            dragged.current = true
          }}
          onDragEnd={onDragEnd}
          aria-label={t.home.metaSlide(position)}
          className="touch-pan-y"
        >
          <div className="flex items-center gap-2">
            <PositionIcon position={position} size={22} />
            <span className="text-card-title">{positionShort(position)}</span>
            {!data.by_position && (
              <span className="ml-auto text-[12px] text-dim">{t.home.metaOverall}</span>
            )}
          </div>

          {slide && slide.heroes.length > 0 ? (
            <ul className="mt-3 grid grid-cols-5 gap-2">
              {slide.heroes.map((stat) => {
                const hero = heroes.data?.byId.get(stat.hero_id)
                return (
                  <li key={stat.hero_id}>
                    <Link
                      to={`/tools/heroes/${stat.hero_id}`}
                      draggable={false}
                      onClick={blockClickAfterDrag}
                      className="group block rounded-[10px]"
                      title={hero?.localized_name}
                    >
                      <HeroPortrait
                        hero={hero}
                        width={132}
                        height={132}
                        fluid
                        className="ring-accent-2 transition-transform duration-200 group-hover:scale-[1.03] group-hover:ring-2"
                      />
                      <span
                        className={cn(
                          'mt-1.5 block text-center text-[15px] font-semibold tabular-nums',
                          stat.winrate >= 0.5 ? 'text-win-strong' : 'text-loss',
                        )}
                      >
                        {formatPercent(stat.winrate)}
                      </span>
                    </Link>
                  </li>
                )
              })}
            </ul>
          ) : (
            <p className="mt-3 flex aspect-[692/132] items-center justify-center text-[15px] text-muted">
              {t.home.metaEmpty}
            </p>
          )}
        </motion.div>
      </AnimatePresence>
    </div>
  )
}

function CarouselSkeleton() {
  return (
    <div className="mt-4" aria-hidden>
      <Skeleton className="h-6 w-28" />
      <div className="mt-3 grid grid-cols-5 gap-2">
        {POSITIONS.map((p) => (
          <div key={p}>
            <Skeleton className="aspect-square w-full" />
            <Skeleton className="mx-auto mt-2 h-4 w-10" />
          </div>
        ))}
      </div>
      <div className="mt-4 flex justify-center">
        <Skeleton className="h-1.5 w-16" radius="full" />
      </div>
    </div>
  )
}
