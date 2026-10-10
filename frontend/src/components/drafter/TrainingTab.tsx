import { LayoutGrid, List, RotateCcw, Search, Share2 } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ApiError } from '@/api/client'
import type { Hero, TrainingResult } from '@/api/types'
import { SteamLoginButton } from '@/components/auth/SteamLoginButton'
import { HeroIcon, HeroPortrait } from '@/components/dota/HeroPortrait'
import { PositionIcon } from '@/components/dota/PositionIcon'
import { Button, IconButton } from '@/components/ui/Button'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Input } from '@/components/ui/Field'
import { Skeleton } from '@/components/ui/Skeleton'
import { toast } from '@/features/toast/store'
import { useDrafterMatrix, useNewTraining, useSubmitTraining } from '@/features/useDrafter'
import { useHeroes } from '@/features/useHeroes'
import { useMe } from '@/features/useMe'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import {
  emptyTeam,
  nextEmpty,
  setSlot,
  slotsOf,
  teamSize,
  clearSlot,
  type TeamState,
} from '@/lib/drafter'
import { clearTraining, loadTraining, saveTraining, type SavedTraining } from '@/lib/drafterStorage'
import { formatSigned } from '@/lib/format'
import { filterHeroes } from '@/lib/heroSearch'
import { POSITIONS, positionFull, type Position } from '@/lib/positions'
import { PairView, PanelBlock } from './PairView'
import { SlotFace } from './SlotFace'

const RETURN_PATH = '/tools/drafter?tab=training'

type FilterChoice = Position | null | undefined

/** «Тренировка» (SPEC 4.2): враги от сервера, сборка своей пятёрки, оценка на сервере. */
export function TrainingTab() {
  const { data: me } = useMe()
  const loggedIn = Boolean(me?.user)
  const matrix = useDrafterMatrix()
  const { data: heroes } = useHeroes()
  const newTraining = useNewTraining()
  const submit = useSubmitTraining()

  const [session, setSession] = useState<SavedTraining | null>(() => loadTraining())
  const [result, setResult] = useState<TrainingResult | null>(null)
  const [active, setActive] = useState(0)
  const [filter, setFilter] = useState<FilterChoice>(undefined)
  const [query, setQuery] = useState('')
  const [view, setView] = useState<'grid' | 'list'>('grid')

  const start = useCallback(() => {
    newTraining.mutate(undefined, {
      onSuccess: (res) => {
        setSession({
          trainingId: res.training_id,
          enemies: res.enemies,
          team: emptyTeam(),
          submitAfterLogin: false,
        })
        setResult(null)
        setActive(0)
        setFilter(undefined)
        setQuery('')
      },
    })
    // newTraining.mutate стабилен между рендерами
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // первая тренировка: если сохранённой нет — просим сервер выбрать врагов
  const started = useRef(false)
  useEffect(() => {
    if (started.current || session) return
    started.current = true
    start()
  }, [session, start])

  // открытая тренировка переживает перезагрузку и вход через Steam
  useEffect(() => {
    if (session && !result) saveTraining(session)
  }, [session, result])

  const team = session?.team ?? emptyTeam()
  const enemyIds = useMemo(
    () => new Set(session?.enemies.map((e) => e.hero_id)),
    [session?.enemies],
  )
  const chosenIds = useMemo(() => new Set(team.filter((h): h is number => h !== null)), [team])
  const complete = teamSize(team) === 5
  const locked = result !== null

  const setTeam = (next: TeamState) => setSession((s) => (s ? { ...s, team: next } : s))

  const activate = (index: number) => {
    if (locked) return
    if (index === active && team[index] !== null) setTeam(clearSlot(team, index))
    setActive(index)
    setFilter(undefined)
  }

  const place = (hero: Hero) => {
    if (locked || enemyIds.has(hero.id) || chosenIds.has(hero.id)) return
    const next = setSlot(team, active, hero.id)
    setTeam(next)
    setActive(nextEmpty(next, active) ?? active)
    setFilter(undefined)
  }

  const evaluate = useCallback(() => {
    if (!session) return
    submit.mutate(
      { trainingId: session.trainingId, team: slotsOf(session.team) },
      {
        onSuccess: (res) => {
          setResult(res)
          clearTraining()
        },
        onError: (err) => {
          if (
            err instanceof ApiError &&
            (err.code === 'training_closed' || err.code === 'training_not_found')
          ) {
            toast.info(t.drafter.training.closedToast)
            clearTraining()
            start()
          } else {
            toast.error(err instanceof ApiError ? err.message : t.states.unknown)
          }
        },
      },
    )
  }, [session, submit, start])

  // гость нажал «Оценить», вошёл через Steam и вернулся — оцениваем сам
  const autoSubmitted = useRef(false)
  useEffect(() => {
    if (!loggedIn || !session?.submitAfterLogin || !complete || result || autoSubmitted.current)
      return
    autoSubmitted.current = true
    setSession((s) => (s ? { ...s, submitAfterLogin: false } : s))
    evaluate()
  }, [loggedIn, session?.submitAfterLogin, complete, result, evaluate])

  // портреты по позиции: пока матрица не загружена, фильтр по позиции недоступен
  const activePosition = (active + 1) as Position
  const position = filter === undefined ? activePosition : filter
  const playable = useMemo(
    () => (position && matrix.data ? new Set(matrix.data.playable[position - 1]) : null),
    [position, matrix.data],
  )
  const shown = useMemo(() => {
    const all = heroes?.list ?? []
    if (query.trim()) return filterHeroes(all, query, null) // поиск — по всем героям
    return playable ? all.filter((h) => playable.has(h.id)) : all
  }, [heroes?.list, query, playable])

  if (newTraining.isError && !session) {
    return (
      <>
        <div className="min-w-0">
          <ErrorState
            title={t.drafter.training.startError}
            description={
              newTraining.error instanceof ApiError ? newTraining.error.message : undefined
            }
            onRetry={start}
            retrying={newTraining.isPending}
          />
        </div>
        <aside />
      </>
    )
  }

  return (
    <>
      <div className="min-w-0 space-y-6">
        <section>
          <div className="mb-3 flex items-center gap-3">
            <h2 className="caps flex items-center gap-2" style={{ color: 'var(--dire)' }}>
              <span aria-hidden>■</span>
              {t.drafter.training.enemies}
            </h2>
            <Button
              variant="ghost"
              size="sm"
              className="-my-1.5 ml-auto"
              loading={newTraining.isPending}
              onClick={() => {
                clearTraining()
                start()
              }}
              aria-label={t.drafter.training.newEnemiesLabel}
            >
              {!newTraining.isPending && <RotateCcw size={16} aria-hidden />}
              {t.drafter.training.newEnemies}
            </Button>
          </div>
          <div className="grid grid-cols-5 gap-2.5">
            {POSITIONS.map((p, i) => {
              const enemy = session?.enemies.find((e) => e.position === p) ?? session?.enemies[i]
              const hero = enemy ? heroes?.byId.get(enemy.hero_id) : null
              return session ? (
                <span key={p} title={hero?.localized_name} className="block">
                  <SlotFace hero={hero} position={enemy?.position ?? p} tone="dire" passive />
                </span>
              ) : (
                <Skeleton key={p} radius={14} className="aspect-square w-full" />
              )
            })}
          </div>
        </section>

        <section>
          <h2 className="caps mb-3 flex items-center gap-2" style={{ color: 'var(--accent-2)' }}>
            <span aria-hidden>■</span>
            {t.drafter.training.myTeam}
          </h2>
          <div className="grid grid-cols-5 gap-2.5">
            {POSITIONS.map((p, i) => {
              const id = team[i]
              const hero = id === null ? null : heroes?.byId.get(id)
              const heroBreakdown = result?.breakdown.heroes.find((h) => h.position === p)
              const score = heroBreakdown
                ? heroBreakdown.vs + heroBreakdown.synergy + heroBreakdown.meta
                : undefined
              return (
                <button
                  key={p}
                  type="button"
                  disabled={locked}
                  aria-pressed={i === active && !locked}
                  aria-label={t.drafter.training.slot(positionFull(p), hero?.localized_name)}
                  onClick={() => activate(i)}
                  className="group block disabled:cursor-default"
                >
                  <SlotFace
                    hero={hero}
                    position={p}
                    tone="mine"
                    active={i === active && !locked}
                    passive={locked}
                    score={score}
                  />
                </button>
              )
            })}
          </div>
        </section>

        {!locked && (
          <section aria-label={t.drafter.training.searchLabel}>
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
                  placeholder={t.drafter.training.searchPlaceholder}
                  aria-label={t.drafter.training.searchLabel}
                  autoComplete="off"
                  className="h-11 pl-11"
                />
              </div>
              <div
                className="flex shrink-0 gap-1"
                role="group"
                aria-label={t.drafter.training.positionFilter}
              >
                {POSITIONS.map((p) => (
                  <button
                    key={p}
                    type="button"
                    aria-pressed={position === p}
                    aria-label={positionFull(p)}
                    title={positionFull(p)}
                    onClick={() => setFilter(position === p ? null : p)}
                    className={cn(
                      'flex size-11 items-center justify-center rounded-[12px] border transition-colors',
                      position === p
                        ? 'border-accent-2 bg-accent-2/15'
                        : 'border-line bg-bg hover:bg-surface-2',
                    )}
                  >
                    <PositionIcon position={p} size={22} />
                  </button>
                ))}
              </div>
              <IconButton
                variant="surface"
                aria-label={
                  view === 'grid' ? t.drafter.training.viewList : t.drafter.training.viewGrid
                }
                onClick={() => setView(view === 'grid' ? 'list' : 'grid')}
              >
                {view === 'grid' ? <List size={20} /> : <LayoutGrid size={20} />}
              </IconButton>
            </div>

            <HeroChoices
              heroes={shown}
              loading={!heroes}
              view={view}
              disabled={(h) => enemyIds.has(h.id) || chosenIds.has(h.id)}
              onPick={place}
            />
          </section>
        )}
      </div>

      <aside className="min-w-0 xl:sticky xl:top-[88px] xl:self-start">
        <div className="rounded-[16px] border border-line bg-surface p-5 xl:max-h-[calc(100dvh-112px)] xl:overflow-y-auto">
          {result ? (
            <ResultView
              result={result}
              onNew={() => {
                clearTraining()
                start()
              }}
              starting={newTraining.isPending}
            />
          ) : (
            <EvaluatePanel
              count={teamSize(team)}
              complete={complete}
              loggedIn={loggedIn}
              ready={Boolean(session)}
              submitting={submit.isPending}
              onEvaluate={evaluate}
              onLoginStart={() => session && saveTraining({ ...session, submitAfterLogin: true })}
            />
          )}
        </div>
      </aside>
    </>
  )
}

// ----- сетка и список героев -------------------------------------------------------

function HeroChoices({
  heroes,
  loading,
  view,
  disabled,
  onPick,
}: {
  heroes: Hero[]
  loading: boolean
  view: 'grid' | 'list'
  disabled: (h: Hero) => boolean
  onPick: (h: Hero) => void
}) {
  if (loading) {
    return (
      <div className="mt-4 grid grid-cols-4 gap-2 md:grid-cols-6">
        {Array.from({ length: 12 }, (_, i) => (
          <Skeleton key={i} className="aspect-[16/11]" />
        ))}
      </div>
    )
  }
  if (heroes.length === 0) {
    return <EmptyState icon={Search} title={t.drafter.training.nothingFound} className="py-8" />
  }
  if (view === 'list') {
    return (
      <ul className="mt-4 divide-y divide-line rounded-[14px] border border-line">
        {heroes.map((h) => {
          const off = disabled(h)
          return (
            <li key={h.id}>
              <button
                type="button"
                disabled={off}
                onClick={() => onPick(h)}
                className="flex w-full items-center gap-3 px-3 py-2 text-left transition-colors enabled:hover:bg-surface-2 disabled:opacity-35"
              >
                <HeroIcon hero={h} width={32} height={32} radius={8} grayscale={off} />
                <span className="flex-1 truncate text-[15px]">{h.localized_name}</span>
                <span className="text-[12px] text-dim">{t.heroes.attrs[h.primary_attr]}</span>
              </button>
            </li>
          )
        })}
      </ul>
    )
  }
  return (
    <div className="mt-4 grid grid-cols-4 gap-2 md:grid-cols-6">
      {heroes.map((h) => {
        const off = disabled(h)
        return (
          <button
            key={h.id}
            type="button"
            disabled={off}
            title={h.localized_name}
            onClick={() => onPick(h)}
            className="group rounded-[10px] text-left disabled:cursor-not-allowed disabled:opacity-35"
          >
            <span className="relative block overflow-hidden rounded-[8px] border border-transparent transition-[transform,border-color] group-enabled:group-hover:scale-[1.03] group-enabled:group-hover:border-accent-2">
              <HeroPortrait
                hero={h}
                width={120}
                height={68}
                fluid
                radius={0}
                grayscale={off}
                className="block w-full"
              />
            </span>
            <span className="mt-1 block truncate text-[11px] leading-4 text-muted group-enabled:group-hover:text-fg">
              {h.localized_name}
            </span>
          </button>
        )
      })}
    </div>
  )
}

// ----- правая панель ---------------------------------------------------------------

function EvaluatePanel({
  count,
  complete,
  loggedIn,
  ready,
  submitting,
  onEvaluate,
  onLoginStart,
}: {
  count: number
  complete: boolean
  loggedIn: boolean
  ready: boolean
  submitting: boolean
  onEvaluate: () => void
  onLoginStart: () => void
}) {
  return (
    <div className="space-y-4">
      {!complete ? (
        <div className="text-center">
          <p className="text-[15px] text-muted">{t.drafter.training.panelHint}</p>
          <div className="mt-3 font-mono text-[40px] font-semibold tabular-nums">
            {t.drafter.training.counter(count)}
          </div>
        </div>
      ) : loggedIn ? (
        <Button
          variant="accent"
          size="lg"
          fullWidth
          loading={submitting}
          disabled={!ready}
          onClick={onEvaluate}
        >
          {t.drafter.training.evaluate}
        </Button>
      ) : (
        <div className="space-y-3">
          <SteamLoginButton size="lg" fullWidth returnTo={RETURN_PATH} onStart={onLoginStart}>
            {t.drafter.training.loginAndEvaluate}
          </SteamLoginButton>
          <p className="text-center text-[13px] text-muted">{t.drafter.training.loginHint}</p>
        </div>
      )}
    </div>
  )
}

function scoreTone(score: number): string {
  if (score >= 70) return 'text-win-strong'
  if (score < 40) return 'text-loss-strong'
  return 'text-fg'
}

function ResultView({
  result,
  onNew,
  starting,
}: {
  result: TrainingResult
  onNew: () => void
  starting: boolean
}) {
  const { breakdown } = result
  const rows: { key: keyof typeof t.drafter.training.contrib; value: number }[] = [
    { key: 'vs', value: breakdown.contrib.vs },
    { key: 'synergy', value: breakdown.contrib.synergy },
    { key: 'meta', value: breakdown.contrib.meta },
    { key: 'penalty', value: -breakdown.contrib.penalty },
  ]

  const share = async () => {
    const text = t.drafter.training.shareText(
      result.score.toFixed(1),
      `${location.origin}${RETURN_PATH}`,
    )
    try {
      if (navigator.share) await navigator.share({ text })
      else {
        await navigator.clipboard.writeText(text)
        toast.success(t.drafter.training.shareCopied)
      }
    } catch {
      // пользователь закрыл окно «Поделиться» — это не ошибка
    }
  }

  return (
    <div className="space-y-4">
      <div className="text-center">
        <div className="caps">{t.drafter.training.yourScore}</div>
        <div
          className={cn(
            'mt-1 font-mono text-[56px] font-semibold leading-[64px] tabular-nums',
            scoreTone(result.score),
          )}
        >
          {result.score.toFixed(1)}
        </div>
        <div className="text-[13px] text-dim">{t.drafter.training.outOf}</div>
        {result.month_rank !== null && (
          <div className="mt-2 inline-block rounded-full bg-accent-2/15 px-3 py-1 text-[14px] font-medium text-accent-2">
            {t.drafter.training.monthRank(result.month_rank)}
          </div>
        )}
      </div>

      <PanelBlock title={t.drafter.training.breakdown}>
        <dl className="space-y-2 text-[14px]">
          {rows.map((r) => (
            <div key={r.key} className="flex items-center justify-between gap-3">
              <dt className="text-muted">{t.drafter.training.contrib[r.key]}</dt>
              <dd
                className={cn(
                  'font-mono font-semibold tabular-nums',
                  r.value > 0 ? 'text-win-strong' : r.value < 0 ? 'text-loss-strong' : 'text-muted',
                )}
              >
                {formatSigned(r.value)}
              </dd>
            </div>
          ))}
        </dl>
        {breakdown.off_position > 0 && (
          <p className="mt-2 text-[12px] text-dim">
            {t.drafter.training.offPosition(breakdown.off_position)}
          </p>
        )}
      </PanelBlock>

      {breakdown.best_matchup && (
        <PanelBlock title={t.drafter.training.bestMatchup}>
          <PairView pair={breakdown.best_matchup} kind="vs" />
        </PanelBlock>
      )}
      {breakdown.worst_matchup && (
        <PanelBlock title={t.drafter.training.worstMatchup}>
          <PairView pair={breakdown.worst_matchup} kind="vs" />
        </PanelBlock>
      )}

      <div className="grid gap-2 border-t border-line pt-4">
        <Button variant="accent2" fullWidth loading={starting} onClick={onNew}>
          {t.drafter.training.newTraining}
        </Button>
        <Button variant="surface" fullWidth onClick={() => void share()}>
          <Share2 size={18} aria-hidden />
          {t.drafter.training.share}
        </Button>
      </div>
    </div>
  )
}
