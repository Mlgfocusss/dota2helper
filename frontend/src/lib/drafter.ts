// Расчёты драфтера без React (SPEC 5): оценка героев в «Анализе» по матрице матчапов,
// подсказки пика и состояние драфта в URL. Формулы совпадают с серверными (internal/drafter).
import type {
  DraftAnalysis,
  DraftMatrixPayload,
  DraftPair,
  DraftSlot,
  Position,
  TeamAnalysis,
} from '@/api/types'
import { isPosition, POSITIONS } from '@/lib/positions'

/** Вес синергии с союзниками в оценке героя (SPEC 5.2). */
export const SYNERGY_WEIGHT = 0.5
export const TEAM_SIZE = 5
export const MAX_BANS = 10

/** Команда: id героя в слоте позиции i + 1 или null. */
export type TeamState = (number | null)[]

export type DraftState = { radiant: TeamState; dire: TeamState; bans: number[] }

export type TeamKey = 'radiant' | 'dire'

export const emptyTeam = (): TeamState => Array.from({ length: TEAM_SIZE }, () => null)

export const emptyDraft = (): DraftState => ({ radiant: emptyTeam(), dire: emptyTeam(), bans: [] })

// ----- матрица ----------------------------------------------------------------

export type DraftMatrix = {
  ids: number[]
  vs: number[][]
  with: number[][] | null
  hasWith: boolean
  /** [позиция − 1] → герои, которых реально играют на позиции */
  playable: number[][]
  updatedAt: string
  index: Map<number, number>
}

export function buildMatrix(p: DraftMatrixPayload): DraftMatrix {
  return {
    ids: p.ids,
    vs: p.vs,
    with: p.with,
    hasWith: p.has_with && p.with !== null,
    playable: p.playable,
    updatedAt: p.updated_at,
    index: new Map(p.ids.map((id, i) => [id, i])),
  }
}

/** Преимущество героя a над b в п. п. (0, если данных нет). */
export function vs(m: DraftMatrix, a: number, b: number): number {
  const i = m.index.get(a)
  const j = m.index.get(b)
  return i === undefined || j === undefined ? 0 : (m.vs[i]?.[j] ?? 0)
}

/** Синергия a и b в одной команде (0, если данных нет). */
export function synergy(m: DraftMatrix, a: number, b: number): number {
  const i = m.index.get(a)
  const j = m.index.get(b)
  if (!m.with || i === undefined || j === undefined) return 0
  return m.with[i]?.[j] ?? 0
}

/** Округление до 0.1 «от нуля», как math.Round на сервере; без «−0». */
export function round1(v: number): number {
  const r = (Math.sign(v) * Math.round(Math.abs(v) * 10)) / 10
  return r === 0 ? 0 : r
}

// ----- оценка -----------------------------------------------------------------

/** Заполненные слоты команды по порядку позиций. */
export function slotsOf(team: TeamState): DraftSlot[] {
  const out: DraftSlot[] = []
  team.forEach((hero, i) => {
    if (hero !== null) out.push({ hero_id: hero, position: (i + 1) as Position })
  })
  return out
}

/** score(h) = Σ_{e∈E} vs(h,e) + 0.5 × Σ_{a∈T, a≠h} with(h,a), округление до 0.1. */
export function heroScore(m: DraftMatrix, hero: number, team: DraftSlot[], enemies: DraftSlot[]) {
  let sum = 0
  for (const e of enemies) sum += vs(m, hero, e.hero_id)
  for (const a of team) if (a.hero_id !== hero) sum += SYNERGY_WEIGHT * synergy(m, hero, a.hero_id)
  return round1(sum)
}

function analyzeTeam(m: DraftMatrix, team: DraftSlot[], enemies: DraftSlot[]): TeamAnalysis {
  const heroes = team.map((s) => ({
    hero_id: s.hero_id,
    score: heroScore(m, s.hero_id, team, enemies),
  }))
  return { total: round1(heroes.reduce((acc, h) => acc + h.score, 0)), heroes }
}

function bestPair(m: DraftMatrix, teams: DraftSlot[][]): DraftPair | null {
  let best: DraftPair | null = null
  for (const team of teams) {
    for (let i = 0; i < team.length; i++) {
      for (let j = i + 1; j < team.length; j++) {
        const a = team[i].hero_id
        const b = team[j].hero_id
        const value = round1((synergy(m, a, b) + synergy(m, b, a)) / 2)
        if (best === null || value > best.value) best = { a, b, value }
      }
    }
  }
  return best
}

function extremeMatchups(m: DraftMatrix, x: DraftSlot[], y: DraftSlot[]) {
  let best: DraftPair | null = null
  let worst: DraftPair | null = null
  const consider = (from: DraftSlot[], against: DraftSlot[]) => {
    for (const a of from) {
      for (const b of against) {
        const value = round1(vs(m, a.hero_id, b.hero_id))
        if (best === null || value > best.value) best = { a: a.hero_id, b: b.hero_id, value }
        if (worst === null || value < worst.value) worst = { a: a.hero_id, b: b.hero_id, value }
      }
    }
  }
  consider(x, y)
  consider(y, x)
  return { best: best as DraftPair | null, worst: worst as DraftPair | null }
}

/** Оценка обеих команд. Баны на оценку не влияют. */
export function analyzeDraft(m: DraftMatrix, radiant: TeamState, dire: TeamState): DraftAnalysis {
  const r = slotsOf(radiant)
  const d = slotsOf(dire)
  const { best, worst } = extremeMatchups(m, r, d)
  return {
    radiant: analyzeTeam(m, r, d),
    dire: analyzeTeam(m, d, r),
    best_pair: m.hasWith ? bestPair(m, [r, d]) : null,
    best_matchup: best,
    worst_matchup: worst,
  }
}

export type PickSuggestion = { hero_id: number; score: number }

/**
 * Кого взять на позицию: герои, которых играют на ней, по убыванию оценки
 * против текущих врагов (и с учётом синергии с уже выбранными союзниками).
 */
export function suggestPicks(
  m: DraftMatrix,
  position: Position,
  team: DraftSlot[],
  enemies: DraftSlot[],
  unavailable: ReadonlySet<number>,
  limit = 5,
): PickSuggestion[] {
  const out: PickSuggestion[] = []
  for (const hero of m.playable[position - 1] ?? []) {
    if (unavailable.has(hero)) continue
    out.push({ hero_id: hero, score: heroScore(m, hero, team, enemies) })
  }
  out.sort((a, b) => b.score - a.score || a.hero_id - b.hero_id)
  return out.slice(0, limit)
}

// ----- редактирование команды --------------------------------------------------

export function setSlot(team: TeamState, index: number, hero: number): TeamState {
  const next = [...team]
  next[index] = hero
  return next
}

export function clearSlot(team: TeamState, index: number): TeamState {
  const next = [...team]
  next[index] = null
  return next
}

/** Перенос героя на другую позицию; если она занята — герои меняются местами. */
export function moveHero(team: TeamState, from: number, to: number): TeamState {
  if (from === to) return team
  const next = [...team]
  ;[next[from], next[to]] = [next[to], next[from]]
  return next
}

/** Ближайший пустой слот после index (по кругу); null, если команда заполнена. */
export function nextEmpty(team: TeamState, after = -1): number | null {
  for (let step = 1; step <= team.length; step++) {
    const i = (((after + step) % team.length) + team.length) % team.length
    if (team[i] === null) return i
  }
  return null
}

export function teamSize(team: TeamState): number {
  return team.filter((h) => h !== null).length
}

/** Все герои, которых нельзя выбрать повторно: обе команды и баны. */
export function usedHeroes(state: DraftState): Set<number> {
  return new Set<number>([...state.radiant, ...state.dire, ...state.bans].filter((h) => h !== null))
}

export function draftIsEmpty(state: DraftState): boolean {
  return teamSize(state.radiant) === 0 && teamSize(state.dire) === 0 && state.bans.length === 0
}

// ----- состояние в URL ------------------------------------------------------------

/** «1:12,2:45» → команда; повторы героев и мусор отбрасываются. */
function parseTeam(raw: string | null, used: Set<number>): TeamState {
  const team = emptyTeam()
  for (const part of (raw ?? '').split(',')) {
    const [pos, hero] = part.split(':').map(Number)
    if (!isPosition(pos) || !Number.isInteger(hero) || hero < 1 || used.has(hero)) continue
    if (team[pos - 1] !== null) continue
    team[pos - 1] = hero
    used.add(hero)
  }
  return team
}

export function parseDraftState(params: URLSearchParams): DraftState {
  const used = new Set<number>()
  const radiant = parseTeam(params.get('r'), used)
  const dire = parseTeam(params.get('d'), used)
  const bans: number[] = []
  for (const part of (params.get('b') ?? '').split(',')) {
    const hero = Number(part)
    if (part && Number.isInteger(hero) && hero > 0 && !used.has(hero) && bans.length < MAX_BANS) {
      bans.push(hero)
      used.add(hero)
    }
  }
  return { radiant, dire, bans }
}

function encodeTeam(team: TeamState): string {
  return POSITIONS.filter((p) => team[p - 1] !== null)
    .map((p) => `${p}:${team[p - 1]}`)
    .join(',')
}

/** Записывает драфт в query, сохраняя остальные параметры (например, ?tab=). */
export function writeDraftState(params: URLSearchParams, state: DraftState): URLSearchParams {
  const next = new URLSearchParams(params)
  const set = (key: string, value: string) => {
    if (value) next.set(key, value)
    else next.delete(key)
  }
  set('r', encodeTeam(state.radiant))
  set('d', encodeTeam(state.dire))
  set('b', state.bans.join(','))
  return next
}
