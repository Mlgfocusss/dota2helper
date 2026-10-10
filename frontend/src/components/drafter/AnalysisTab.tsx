import { ArrowLeftRight, Replace, Trash2 } from 'lucide-react'
import { useCallback, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router'
import type { DraftAnalysis, Hero, Position } from '@/api/types'
import { HeroPicker } from '@/components/dota/HeroPicker'
import { HeroIcon } from '@/components/dota/HeroPortrait'
import { PositionIcon } from '@/components/dota/PositionIcon'
import { Button } from '@/components/ui/Button'
import { ErrorState } from '@/components/ui/ErrorState'
import { Menu } from '@/components/ui/Menu'
import { Sheet } from '@/components/ui/Sheet'
import { Skeleton } from '@/components/ui/Skeleton'
import { ApiError } from '@/api/client'
import { useDrafterMatrix } from '@/features/useDrafter'
import { useHeroes } from '@/features/useHeroes'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import {
  analyzeDraft,
  clearSlot,
  draftIsEmpty,
  MAX_BANS,
  moveHero,
  nextEmpty,
  parseDraftState,
  setSlot,
  slotsOf,
  suggestPicks,
  teamSize,
  usedHeroes,
  writeDraftState,
  type DraftMatrix,
  type DraftState,
  type TeamKey,
  type TeamState,
} from '@/lib/drafter'
import { formatSigned } from '@/lib/format'
import { POSITIONS, positionFull, positionShort } from '@/lib/positions'
import { PairView, PanelBlock } from './PairView'
import { BanFace, SlotFace } from './SlotFace'

type Target = { kind: 'slot'; team: TeamKey; index: number } | { kind: 'ban' } | null

const TEAM_NAME: Record<TeamKey, string> = { radiant: t.drafter.radiant, dire: t.drafter.dire }
const TEAM_COLOR: Record<TeamKey, string> = { radiant: 'var(--radiant)', dire: 'var(--dire)' }

/** «Анализ» (SPEC 4.2): две команды и баны слева, итоги и подсказки пика — в правой панели. */
export function AnalysisTab() {
  const [params, setParams] = useSearchParams()
  const state = useMemo(() => parseDraftState(params), [params])
  const matrix = useDrafterMatrix()
  const { data: heroes } = useHeroes()
  const [target, setTarget] = useState<Target>(null)
  const [moving, setMoving] = useState<{ team: TeamKey; index: number } | null>(null)

  // состояние живёт в URL: драфтом можно делиться ссылкой
  const commit = useCallback(
    (next: DraftState) =>
      setParams((p) => writeDraftState(p, next), { replace: true, preventScrollReset: true }),
    [setParams],
  )

  const analysis = useMemo(
    () => (matrix.data ? analyzeDraft(matrix.data, state.radiant, state.dire) : null),
    [matrix.data, state.radiant, state.dire],
  )
  const used = useMemo(() => [...usedHeroes(state)], [state])

  const setTeam = (team: TeamKey, next: TeamState) => commit({ ...state, [team]: next })

  const onPick = (hero: Hero) => {
    if (target?.kind === 'slot')
      setTeam(target.team, setSlot(state[target.team], target.index, hero.id))
    else if (target?.kind === 'ban' && state.bans.length < MAX_BANS)
      commit({ ...state, bans: [...state.bans, hero.id] })
    setTarget(null)
  }

  const pickerPosition = target?.kind === 'slot' ? ((target.index + 1) as Position) : null
  const pickerTitle =
    target?.kind === 'slot'
      ? t.drafter.pickFor(TEAM_NAME[target.team], positionFull((target.index + 1) as Position))
      : t.drafter.pickBan

  return (
    <>
      <div className="min-w-0 space-y-6">
        {draftIsEmpty(state) && (
          <p className="text-[15px] text-muted">
            <strong className="font-semibold text-fg">{t.drafter.hintLead}</strong>
            {t.drafter.hintRest}
          </p>
        )}

        {(['radiant', 'dire'] as const).map((key) => (
          <TeamSection
            key={key}
            teamKey={key}
            team={state[key]}
            scores={analysis?.[key]}
            heroes={heroes?.byId}
            showClear={key === 'radiant' && !draftIsEmpty(state)}
            onClear={() =>
              commit({
                radiant: [null, null, null, null, null],
                dire: [null, null, null, null, null],
                bans: [],
              })
            }
            onAdd={(index) => setTarget({ kind: 'slot', team: key, index })}
            onMove={(index) => setMoving({ team: key, index })}
            onRemove={(index) => setTeam(key, clearSlot(state[key], index))}
          />
        ))}

        <section>
          <h2 className="caps mb-3">{t.drafter.bans}</h2>
          <div className="grid grid-cols-5 gap-2 sm:grid-cols-10">
            {Array.from({ length: MAX_BANS }, (_, i) => {
              const id = state.bans[i]
              const hero = id === undefined ? null : heroes?.byId.get(id)
              return id === undefined ? (
                <button
                  key={i}
                  type="button"
                  aria-label={t.drafter.emptyBan}
                  onClick={() => setTarget({ kind: 'ban' })}
                  className="group block"
                >
                  <BanFace />
                </button>
              ) : (
                <button
                  key={i}
                  type="button"
                  aria-label={t.drafter.filledBan(hero?.localized_name ?? t.heroes.unknown)}
                  title={hero?.localized_name}
                  onClick={() => commit({ ...state, bans: state.bans.filter((b) => b !== id) })}
                  className="group block"
                >
                  <BanFace hero={hero} />
                </button>
              )
            })}
          </div>
        </section>
      </div>

      <aside className="min-w-0 xl:sticky xl:top-[88px] xl:self-start">
        <div className="rounded-[16px] border border-line bg-surface p-5 xl:max-h-[calc(100dvh-112px)] xl:overflow-y-auto">
          <AnalysisPanel
            matrix={matrix}
            analysis={analysis}
            state={state}
            onTake={(team, index, hero) => setTeam(team, setSlot(state[team], index, hero))}
          />
        </div>
      </aside>

      <HeroPicker
        open={target !== null}
        onClose={() => setTarget(null)}
        onPick={onPick}
        disabledIds={used}
        title={pickerTitle}
        positionFilter={
          matrix.data ? { playable: matrix.data.playable, initial: pickerPosition } : undefined
        }
      />

      <MoveSheet
        moving={moving}
        state={state}
        heroes={heroes?.byId}
        onClose={() => setMoving(null)}
        onMove={(to) => {
          if (moving) setTeam(moving.team, moveHero(state[moving.team], moving.index, to))
          setMoving(null)
        }}
      />
    </>
  )
}

// ----- команда -----------------------------------------------------------------

type TeamSectionProps = {
  teamKey: TeamKey
  team: TeamState
  scores?: DraftAnalysis['radiant']
  heroes?: Map<number, Hero>
  showClear: boolean
  onClear: () => void
  onAdd: (index: number) => void
  onMove: (index: number) => void
  onRemove: (index: number) => void
}

function TeamSection({
  teamKey,
  team,
  scores,
  heroes,
  showClear,
  onClear,
  onAdd,
  onMove,
  onRemove,
}: TeamSectionProps) {
  const scoreOf = new Map(scores?.heroes.map((h) => [h.hero_id, h.score]))
  return (
    <section>
      <div className="mb-3 flex items-center gap-3">
        <h2 className="caps flex items-center gap-2" style={{ color: TEAM_COLOR[teamKey] }}>
          <span aria-hidden>■</span>
          {TEAM_NAME[teamKey]}
        </h2>
        {showClear && (
          <Button variant="ghost" size="sm" className="-my-1.5 ml-auto" onClick={onClear}>
            {t.drafter.clearAll}
          </Button>
        )}
      </div>
      <div className="grid grid-cols-5 gap-2.5">
        {POSITIONS.map((position, index) => {
          const id = team[index]
          const posName = positionFull(position)
          if (id === null) {
            return (
              <button
                key={position}
                type="button"
                aria-label={t.drafter.emptySlot(TEAM_NAME[teamKey], posName)}
                onClick={() => onAdd(index)}
                className="group block"
              >
                <SlotFace position={position} tone={teamKey} />
              </button>
            )
          }
          const hero = heroes?.get(id)
          return (
            <Menu
              key={position}
              label={t.drafter.filledSlot(hero?.localized_name ?? t.heroes.unknown, posName)}
              triggerClassName="group block w-full"
              className="min-w-0"
              align="left"
              trigger={() => (
                <SlotFace hero={hero} position={position} tone={teamKey} score={scoreOf.get(id)} />
              )}
              items={[
                { label: t.drafter.replace, icon: Replace, onSelect: () => onAdd(index) },
                {
                  label: t.drafter.changePosition,
                  icon: ArrowLeftRight,
                  onSelect: () => onMove(index),
                },
                {
                  label: t.drafter.remove,
                  icon: Trash2,
                  onSelect: () => onRemove(index),
                  tone: 'danger',
                },
              ]}
            />
          )
        })}
      </div>
    </section>
  )
}

// ----- смена позиции -------------------------------------------------------------

function MoveSheet({
  moving,
  state,
  heroes,
  onClose,
  onMove,
}: {
  moving: { team: TeamKey; index: number } | null
  state: DraftState
  heroes?: Map<number, Hero>
  onClose: () => void
  onMove: (to: number) => void
}) {
  return (
    <Sheet open={moving !== null} onClose={onClose} title={t.drafter.changePosition} width={420}>
      {moving && (
        <div>
          <p className="mb-3 text-[14px] text-muted">{t.drafter.pickPosition}</p>
          <div className="space-y-2">
            {POSITIONS.map((p, i) => {
              const occupant = state[moving.team][i]
              const hero = occupant === null ? undefined : heroes?.get(occupant)
              const current = i === moving.index
              return (
                <button
                  key={p}
                  type="button"
                  disabled={current}
                  onClick={() => onMove(i)}
                  className={cn(
                    'flex w-full items-center gap-3 rounded-[12px] border px-4 py-3 text-left transition-colors',
                    current
                      ? 'border-accent-2 bg-accent-2/15'
                      : 'border-line bg-bg hover:bg-surface-2',
                  )}
                >
                  <PositionIcon position={p} size={22} />
                  <span className="flex-1 font-medium">{positionFull(p)}</span>
                  {hero && !current && <HeroIcon hero={hero} width={28} height={28} radius={8} />}
                </button>
              )
            })}
          </div>
        </div>
      )}
    </Sheet>
  )
}

// ----- правая панель --------------------------------------------------------------

type PanelProps = {
  matrix: ReturnType<typeof useDrafterMatrix>
  analysis: DraftAnalysis | null
  state: DraftState
  onTake: (team: TeamKey, index: number, hero: number) => void
}

function Total({ label, value, color }: { label: string; value: number | null; color: string }) {
  return (
    <div className="min-w-0 flex-1 text-center">
      <div className="caps" style={{ color }}>
        {label}
      </div>
      <div
        className={cn(
          'mt-1 font-mono text-[32px] font-semibold leading-10 tabular-nums',
          value === null
            ? 'text-dim'
            : value > 0
              ? 'text-win-strong'
              : value < 0
                ? 'text-loss-strong'
                : 'text-fg',
        )}
      >
        {value === null ? '—' : formatSigned(value)}
      </div>
    </div>
  )
}

function AnalysisPanel({ matrix, analysis, state, onTake }: PanelProps) {
  if (matrix.isPending) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-[72px]" />
        <Skeleton className="h-[56px]" />
        <Skeleton className="h-[56px]" />
      </div>
    )
  }
  if (matrix.isError || !matrix.data || !analysis) {
    const collecting = matrix.error instanceof ApiError && matrix.error.status === 503
    return (
      <ErrorState
        title={collecting ? t.drafter.panel.collecting : t.drafter.panel.loadError}
        description={collecting ? undefined : t.states.errorText}
        onRetry={() => matrix.refetch()}
        retrying={matrix.isRefetching}
        className="px-0 py-4"
      />
    )
  }

  const nobody = draftIsEmpty(state)
  const rCount = teamSize(state.radiant)
  const dCount = teamSize(state.dire)

  return (
    <div className="space-y-4">
      <div className="flex items-stretch gap-4" aria-label={t.drafter.panel.total}>
        <Total
          label={t.drafter.radiantShort}
          value={rCount ? analysis.radiant.total : null}
          color="var(--radiant)"
        />
        <div aria-hidden className="w-px self-stretch bg-line" />
        <Total
          label={t.drafter.direShort}
          value={dCount ? analysis.dire.total : null}
          color="var(--dire)"
        />
      </div>

      {nobody && <p className="text-[14px] text-muted">{t.drafter.panel.empty}</p>}

      {analysis.best_pair && rCount + dCount >= 2 && (
        <PanelBlock title={t.drafter.panel.strongestPair}>
          <PairView pair={analysis.best_pair} kind="with" />
        </PanelBlock>
      )}
      {analysis.best_matchup && (
        <PanelBlock title={t.drafter.panel.bestMatchup}>
          <PairView pair={analysis.best_matchup} kind="vs" />
        </PanelBlock>
      )}
      {analysis.worst_matchup && (
        <PanelBlock title={t.drafter.panel.worstMatchup}>
          <PairView pair={analysis.worst_matchup} kind="vs" />
        </PanelBlock>
      )}
      {!matrix.data.hasWith && <p className="text-[12px] text-dim">{t.drafter.panel.noSynergy}</p>}

      {(['radiant', 'dire'] as const).map(
        (key) =>
          teamSize(state[key]) < 5 && (
            <Suggestions
              key={key}
              teamKey={key}
              matrix={matrix.data}
              state={state}
              onTake={onTake}
            />
          ),
      )}
    </div>
  )
}

/** «Кого взять Свету на позицию N»: топ‑5 героев против текущих врагов. */
function Suggestions({
  teamKey,
  matrix,
  state,
  onTake,
}: {
  teamKey: TeamKey
  matrix: DraftMatrix
  state: DraftState
  onTake: (team: TeamKey, index: number, hero: number) => void
}) {
  const { data: heroes } = useHeroes()
  const [chosen, setChosen] = useState<number | null>(null)
  const team = state[teamKey]
  const enemyKey: TeamKey = teamKey === 'radiant' ? 'dire' : 'radiant'
  const empties = team.flatMap((h, i) => (h === null ? [i] : []))
  const index = chosen !== null && team[chosen] === null ? chosen : (nextEmpty(team) ?? 0)
  const position = (index + 1) as Position
  const enemies = slotsOf(state[enemyKey])
  const picks =
    enemies.length === 0
      ? []
      : suggestPicks(matrix, position, slotsOf(team), enemies, new Set(usedHeroes(state)), 5)

  return (
    <PanelBlock title={t.drafter.panel.suggest(teamKey, `позицию ${position}`)}>
      {empties.length > 1 && enemies.length > 0 && (
        <div className="mb-3 flex gap-1" role="group" aria-label={t.drafter.pickPosition}>
          {empties.map((i) => (
            <button
              key={i}
              type="button"
              aria-pressed={i === index}
              aria-label={positionFull((i + 1) as Position)}
              title={positionShort((i + 1) as Position)}
              onClick={() => setChosen(i)}
              className={cn(
                'flex size-8 items-center justify-center rounded-[10px] border transition-colors',
                i === index
                  ? 'border-accent-2 bg-accent-2/15'
                  : 'border-line bg-bg hover:bg-surface-2',
              )}
            >
              <PositionIcon position={(i + 1) as Position} size={18} />
            </button>
          ))}
        </div>
      )}
      {enemies.length === 0 ? (
        <p className="text-[13px] text-muted">{t.drafter.panel.suggestNeedEnemies}</p>
      ) : (
        <ul className="space-y-1">
          {picks.map((p) => {
            const hero = heroes?.byId.get(p.hero_id)
            return (
              <li key={p.hero_id}>
                <button
                  type="button"
                  aria-label={t.drafter.panel.suggestTake(hero?.localized_name ?? t.heroes.unknown)}
                  onClick={() => onTake(teamKey, index, p.hero_id)}
                  className="flex w-full items-center gap-3 rounded-[10px] px-2 py-1.5 text-left transition-colors hover:bg-surface-2"
                >
                  <HeroIcon hero={hero} width={32} height={32} radius={8} />
                  <span className="min-w-0 flex-1 truncate text-[14px]">
                    {hero?.localized_name ?? t.heroes.unknown}
                  </span>
                  <span
                    className={cn(
                      'rounded-[8px] px-1.5 py-0.5 font-mono text-[12px] font-semibold tabular-nums',
                      p.score >= 0 ? 'bg-win-strong/20 text-win' : 'bg-loss-strong/20 text-loss',
                    )}
                  >
                    {formatSigned(p.score)}
                  </span>
                </button>
              </li>
            )
          })}
        </ul>
      )}
    </PanelBlock>
  )
}
