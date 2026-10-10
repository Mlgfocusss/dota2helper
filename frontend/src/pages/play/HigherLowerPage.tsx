import { useQueryClient } from '@tanstack/react-query'
import { animate, AnimatePresence, motion, useReducedMotion } from 'framer-motion'
import { ArrowDown, ArrowUp, Skull, Swords, TrendingUp } from 'lucide-react'
import { forwardRef, useCallback, useEffect, useRef, useState } from 'react'
import { ApiError } from '@/api/client'
import { getHigherLowerSnapshot } from '@/api/endpoints/games'
import { keys } from '@/api/keys'
import type { Hero, HigherLowerMode } from '@/api/types'
import { HeroPortrait } from '@/components/dota/HeroPortrait'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Confetti } from '@/components/ui/Confetti'
import { Page } from '@/components/ui/Page'
import { useGameRecords, useSaveScore } from '@/features/useGames'
import { useHeroes } from '@/features/useHeroes'
import { useMe } from '@/features/useMe'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatNumber, plural } from '@/lib/format'
import {
  formatHeroValue,
  isCorrect,
  pickFirstPair,
  pickNext,
  pushRecent,
  type Guess,
  type HeroValue,
} from '@/lib/higherLower'
import { getLocalRecord, setLocalRecord } from '@/lib/quiz'

const MODES: { id: HigherLowerMode; icon: typeof TrendingUp }[] = [
  { id: 'popularity', icon: TrendingUp },
  { id: 'kills', icon: Swords },
  { id: 'deaths', icon: Skull },
]

/** Пауза после ответа: ~600 мс отсчёт значения, затем показываем итог. */
const REVEAL_CORRECT_MS = 1300
const REVEAL_WRONG_MS = 1600

type Screen =
  | { name: 'select' }
  | { name: 'play'; mode: HigherLowerMode; pool: HeroValue[] }
  | { name: 'over'; mode: HigherLowerMode; streak: number; best: number; newRecord: boolean }

// «Больше / Меньше» (SPEC 5.7): выбор режима, игра на клиенте, итог.
export default function HigherLowerPage() {
  const [screen, setScreen] = useState<Screen>({ name: 'select' })
  const queryClient = useQueryClient()
  const { data: me } = useMe()
  const loggedIn = Boolean(me?.user)
  const records = useGameRecords()
  const saveScore = useSaveScore()

  /** Рекорд режима: лучший из серверного (вошедший) и локального. */
  const recordOf = useCallback(
    (mode: HigherLowerMode) =>
      Math.max(getLocalRecord(mode), loggedIn ? (records.data?.higher_lower[mode] ?? 0) : 0),
    [loggedIn, records.data],
  )

  // снапшот грузится один раз на режим (SPEC 5.7), дальше берётся из кэша
  const loadPool = useCallback(
    (mode: HigherLowerMode): Promise<HeroValue[]> =>
      queryClient.fetchQuery({
        queryKey: keys.higherLower(mode),
        queryFn: ({ signal }) => getHigherLowerSnapshot(mode, signal),
        staleTime: 10 * 60_000,
      }),
    [queryClient],
  )

  const finish = (mode: HigherLowerMode, streak: number) => {
    const previous = recordOf(mode)
    const newRecord = streak > previous
    if (newRecord) setLocalRecord(mode, streak)
    if (loggedIn && streak > 0) saveScore.mutate({ mode, streak })
    setScreen({ name: 'over', mode, streak, best: Math.max(previous, streak), newRecord })
  }

  return (
    <Page width={1000} title={t.pages.higherLower.title}>
      {screen.name === 'select' && (
        <ModeSelect
          recordOf={recordOf}
          loadPool={loadPool}
          onStart={(mode, pool) => setScreen({ name: 'play', mode, pool })}
        />
      )}
      {screen.name === 'play' && (
        <Game
          key={screen.mode}
          mode={screen.mode}
          pool={screen.pool}
          best={recordOf(screen.mode)}
          onOver={(streak) => finish(screen.mode, streak)}
        />
      )}
      {screen.name === 'over' && (
        <GameOver
          result={screen}
          onAgain={async () => {
            setScreen({ name: 'play', mode: screen.mode, pool: await loadPool(screen.mode) })
          }}
          onChangeMode={() => setScreen({ name: 'select' })}
        />
      )}
    </Page>
  )
}

function ModeSelect({
  recordOf,
  loadPool,
  onStart,
}: {
  recordOf: (mode: HigherLowerMode) => number
  loadPool: (mode: HigherLowerMode) => Promise<HeroValue[]>
  onStart: (mode: HigherLowerMode, pool: HeroValue[]) => void
}) {
  const heroes = useHeroes()
  const [mode, setMode] = useState<HigherLowerMode>('popularity')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const start = async () => {
    setLoading(true)
    setError(null)
    try {
      const pool = await loadPool(mode)
      if (pool.length >= 2) onStart(mode, pool)
      else setError(t.higherLower.unavailable)
    } catch (err) {
      setError(
        err instanceof ApiError && err.status === 503
          ? t.higherLower.unavailable
          : t.higherLower.loadError,
      )
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      <h1 className="text-page-title">{t.pages.higherLower.title}</h1>
      <p className="mt-1.5 text-muted">{t.higherLower.chooseMode}</p>

      <div
        role="radiogroup"
        aria-label={t.higherLower.chooseMode}
        className="mt-6 grid gap-3 md:grid-cols-3"
      >
        {MODES.map(({ id, icon: Icon }) => {
          const info = t.higherLower.modes[id]
          const selected = id === mode
          return (
            <button
              key={id}
              type="button"
              role="radio"
              aria-checked={selected}
              onClick={() => {
                setMode(id)
                setError(null)
              }}
              className={cn(
                'rounded-[16px] border bg-surface p-5 text-left transition-colors hover:bg-surface-2',
                selected ? 'border-accent-2 ring-1 ring-accent-2' : 'border-line',
              )}
            >
              <span className="flex size-11 items-center justify-center rounded-[12px] bg-surface-2 text-accent-2">
                <Icon size={22} aria-hidden />
              </span>
              <span className="text-card-title mt-4 block">{info.title}</span>
              <span className="mt-1 block text-[14px] text-muted">{info.text}</span>
              <span className="mt-4 block font-mono text-[13px] text-muted">
                {t.higherLower.record(recordOf(id))}
              </span>
            </button>
          )
        })}
      </div>

      {error && (
        <p
          role="alert"
          className="mt-4 rounded-[12px] border border-line bg-surface px-4 py-3 text-[15px] text-muted"
        >
          {error}
        </p>
      )}
      <Button
        size="lg"
        fullWidth
        className="mt-6 md:mx-auto md:w-[320px] md:flex"
        loading={loading}
        disabled={heroes.isPending}
        onClick={start}
      >
        {t.higherLower.start}
      </Button>
    </>
  )
}

type Reveal = { guess: Guess; correct: boolean }

function Game({
  mode,
  pool,
  best,
  onOver,
}: {
  mode: HigherLowerMode
  pool: HeroValue[]
  best: number
  onOver: (streak: number) => void
}) {
  const heroes = useHeroes()
  const [pair, setPair] = useState(() => pickFirstPair(pool))
  const [streak, setStreak] = useState(0)
  const [reveal, setReveal] = useState<Reveal | null>(null)
  const recent = useRef(pushRecent([], pair.left.hero_id, pair.right.hero_id))
  const timer = useRef<number | undefined>(undefined)
  const { left, right } = pair

  useEffect(() => () => window.clearTimeout(timer.current), [])

  const answer = useCallback(
    (guess: Guess) => {
      if (reveal) return
      const correct = isCorrect(guess, left, right)
      setReveal({ guess, correct })
      timer.current = window.setTimeout(
        () => {
          if (!correct) {
            onOver(streak)
            return
          }
          const nextStreak = streak + 1
          const next = pickNext(pool, right, recent.current, nextStreak)
          recent.current = pushRecent(recent.current, next.hero_id)
          setStreak(nextStreak)
          setPair({ left: right, right: next })
          setReveal(null)
        },
        correct ? REVEAL_CORRECT_MS : REVEAL_WRONG_MS,
      )
    },
    [reveal, left, right, streak, pool, onOver],
  )

  // ↑ — «Больше», ↓ — «Меньше»
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        answer('higher')
      } else if (e.key === 'ArrowDown') {
        e.preventDefault()
        answer('lower')
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [answer])

  const heroOf = (v: HeroValue) => heroes.data?.byId.get(v.hero_id)
  const leftName = heroOf(left)?.localized_name ?? t.heroes.unknown

  return (
    <div>
      <div className="mb-4 flex items-baseline justify-between gap-4">
        <span className="text-[22px] font-bold tabular-nums">{t.higherLower.streak(streak)}</span>
        <span className="font-mono text-[14px] text-muted">
          {t.higherLower.modes[mode].title} · {t.higherLower.best(Math.max(best, streak))}
        </span>
      </div>

      <div className="relative grid gap-4 md:grid-cols-2 md:gap-6">
        <AnimatePresence mode="popLayout" initial={false}>
          <HeroCard
            key={left.hero_id}
            slot="left"
            mode={mode}
            hero={heroOf(left)}
            value={left}
            reveal={null}
          />
          <HeroCard
            key={right.hero_id}
            slot="right"
            mode={mode}
            hero={heroOf(right)}
            value={right}
            reveal={reveal}
            leftName={leftName}
            onAnswer={answer}
          />
        </AnimatePresence>
        <span
          aria-hidden
          className="pointer-events-none absolute left-1/2 top-1/2 z-10 flex size-14 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border border-line bg-bg text-[15px] font-bold shadow-lg md:top-[130px]"
        >
          {t.higherLower.vs}
        </span>
      </div>
      <p className="mt-4 hidden text-center text-[13px] text-dim md:block">{t.higherLower.hint}</p>
    </div>
  )
}

type HeroCardProps = {
  slot: 'left' | 'right'
  mode: HigherLowerMode
  hero: Hero | undefined
  value: HeroValue
  reveal: Reveal | null
  leftName?: string
  onAnswer?: (guess: Guess) => void
}

// forwardRef: AnimatePresence в режиме popLayout измеряет карточку через ref
const HeroCard = forwardRef<HTMLDivElement, HeroCardProps>(function HeroCard(
  { slot, mode, hero, value, reveal, leftName, onAnswer },
  ref,
) {
  const reduce = useReducedMotion()
  return (
    <motion.div
      ref={ref}
      layout
      initial={{ opacity: 0, x: reduce ? 0 : 48 }}
      animate={{ opacity: 1, x: 0 }}
      exit={{ opacity: 0, x: reduce ? 0 : -48 }}
      transition={{ duration: reduce ? 0 : 0.28, ease: 'easeOut' }}
      className={cn(
        'overflow-hidden rounded-[16px] border bg-surface transition-colors duration-200',
        reveal ? (reveal.correct ? 'border-win-strong' : 'border-loss-strong') : 'border-line',
      )}
    >
      <HeroPortrait
        hero={hero}
        width={512}
        height={288}
        radius={0}
        fluid
        className="aspect-[5/2] w-full max-w-none md:aspect-video"
      />
      <div className="p-5">
        <h2 className="truncate text-[22px] font-semibold leading-7">
          {hero?.localized_name ?? t.heroes.unknown}
        </h2>
        {slot === 'left' || reveal ? (
          <div className="mt-3">
            <ValueDisplay mode={mode} value={value.value} animateIn={slot === 'right'} />
          </div>
        ) : (
          <div className="mt-3">
            <p className="mb-3 text-[14px] text-muted">{t.higherLower.question(leftName ?? '')}</p>
            <div className="grid grid-cols-2 gap-3">
              <Button variant="accent2" size="lg" onClick={() => onAnswer?.('higher')}>
                {t.higherLower.higher} <ArrowUp size={18} aria-hidden />
              </Button>
              <Button variant="surface" size="lg" onClick={() => onAnswer?.('lower')}>
                {t.higherLower.lower} <ArrowDown size={18} aria-hidden />
              </Button>
            </div>
          </div>
        )}
      </div>
    </motion.div>
  )
})

/** Значение героя крупно; у правого героя «отсчитывается» от нуля (~600 мс). */
function ValueDisplay({
  mode,
  value,
  animateIn,
}: {
  mode: HigherLowerMode
  value: number
  animateIn: boolean
}) {
  const reduce = useReducedMotion()
  const [shown, setShown] = useState(animateIn && !reduce ? 0 : value)

  useEffect(() => {
    if (!animateIn || reduce) return
    const controls = animate(0, value, { duration: 0.6, ease: 'easeOut', onUpdate: setShown })
    return () => controls.stop()
  }, [animateIn, reduce, value])

  const text =
    mode === 'popularity' ? formatNumber(Math.round(shown)) : formatHeroValue(mode, shown)
  const unit =
    mode === 'popularity'
      ? plural(Math.round(value), t.plural.match)
      : t.higherLower.modes[mode].unit
  return (
    <div>
      <div className="text-[40px] font-bold leading-[48px] tabular-nums" aria-live="polite">
        {text}
      </div>
      <div className="text-[14px] text-muted">{unit}</div>
    </div>
  )
}

function GameOver({
  result,
  onAgain,
  onChangeMode,
}: {
  result: { mode: HigherLowerMode; streak: number; best: number; newRecord: boolean }
  onAgain: () => Promise<void>
  onChangeMode: () => void
}) {
  const [loading, setLoading] = useState(false)
  return (
    <Card
      padding="none"
      className="relative mx-auto mt-6 max-w-[560px] overflow-visible p-8 text-center"
    >
      {result.newRecord && <Confetti />}
      <p className="caps">{t.higherLower.gameOver}</p>
      <div className="mt-3 text-[56px] font-bold leading-none tabular-nums">{result.streak}</div>
      <p className="mt-2 text-[18px] text-muted">{t.higherLower.streak(result.streak)}</p>
      <p className="mt-1 font-mono text-[15px] text-muted">{t.higherLower.best(result.best)}</p>
      {result.newRecord && (
        <p className="mt-4 text-[20px] font-bold text-win-strong">{t.higherLower.newRecord}</p>
      )}
      <div className="mt-8 grid grid-cols-2 gap-3 max-sm:grid-cols-1">
        <Button
          size="lg"
          loading={loading}
          onClick={async () => {
            setLoading(true)
            try {
              await onAgain()
            } finally {
              setLoading(false)
            }
          }}
        >
          {t.higherLower.again}
        </Button>
        <Button size="lg" variant="outline" onClick={onChangeMode}>
          {t.higherLower.changeMode}
        </Button>
      </div>
    </Card>
  )
}
