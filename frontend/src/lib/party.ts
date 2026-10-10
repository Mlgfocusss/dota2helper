// Логика поиска пати без React: фильтры в URL, проверка формы анкеты, контакты.
import type { PartySearchParams } from '@/api/endpoints/party'
import type {
  PartyCard,
  PartyCardInput,
  PartyGoal,
  PartyLooking,
  PartyMode,
  Position,
} from '@/api/types'
import { isPosition, POSITIONS } from '@/lib/positions'

export const PARTY_MODES: readonly PartyMode[] = ['ranked', 'normal', 'turbo']
export const PARTY_GOALS: readonly PartyGoal[] = ['rating', 'fun', 'stack', 'scrim']

export const ABOUT_MAX = 300
export const HOURS_MAX = 50000
export const FAVORITES_MAX = 3
export const MAX_RANK_MEDAL = 8

/** Как на сервере (SPEC 4.6): 5–32 символа, латиница, цифры, «_». */
export const TELEGRAM_RE = /^@?[a-zA-Z0-9_]{5,32}$/
export const DISCORD_RE = /^[\p{L}\p{N}_.\-#]{2,40}$/u

const BUMP_COOLDOWN_MS = 24 * 60 * 60 * 1000

// ----- вкладки поиска -----------------------------------------------------------

/**
 * «Найти команду» показывает тех, кто набирает игроков (looking_for = players),
 * «Найти игрока» — тех, кто ищет команду (looking_for = team).
 */
export type SearchTab = 'team' | 'players'

export function lookingForTab(tab: SearchTab): PartyLooking {
  return tab === 'team' ? 'players' : 'team'
}

export function parseTab(raw: string | null): SearchTab {
  return raw === 'players' ? 'players' : 'team'
}

// ----- фильтры в URL ------------------------------------------------------------

export type PartyFilters = {
  rankMin: number
  rankMax: number
  positions: Position[]
  modes: PartyMode[]
  goals: PartyGoal[]
  mic: boolean
  discord: boolean
}

export const DEFAULT_FILTERS: PartyFilters = {
  rankMin: 0,
  rankMax: MAX_RANK_MEDAL,
  positions: [],
  modes: [],
  goals: [],
  mic: false,
  discord: false,
}

function parseRank(raw: string | null, fallback: number): number {
  if (raw === null || raw === '') return fallback
  const n = Number(raw)
  return Number.isInteger(n) && n >= 0 && n <= MAX_RANK_MEDAL ? n : fallback
}

function parseList<T extends string>(raw: string | null, allowed: readonly T[]): T[] {
  if (!raw) return []
  const wanted = new Set(raw.split(','))
  return allowed.filter((v) => wanted.has(v))
}

export function parseFilters(params: URLSearchParams): PartyFilters {
  let rankMin = parseRank(params.get('rank_min'), 0)
  let rankMax = parseRank(params.get('rank_max'), MAX_RANK_MEDAL)
  if (rankMin > rankMax) [rankMin, rankMax] = [rankMax, rankMin]
  const positions = (params.get('positions') ?? '').split(',').map(Number).filter(isPosition)
  return {
    rankMin,
    rankMax,
    positions: POSITIONS.filter((p) => positions.includes(p)),
    modes: parseList(params.get('modes'), PARTY_MODES),
    goals: parseList(params.get('goals'), PARTY_GOALS),
    mic: params.get('mic') === '1',
    discord: params.get('discord') === '1',
  }
}

/** Записывает фильтры в query; значения по умолчанию в адрес не попадают. */
export function applyFilters(params: URLSearchParams, f: PartyFilters): URLSearchParams {
  const next = new URLSearchParams(params)
  const set = (key: string, value: string | null) => {
    if (value) next.set(key, value)
    else next.delete(key)
  }
  set('rank_min', f.rankMin > 0 ? String(f.rankMin) : null)
  set('rank_max', f.rankMax < MAX_RANK_MEDAL ? String(f.rankMax) : null)
  set('positions', f.positions.join(',') || null)
  set('modes', f.modes.join(',') || null)
  set('goals', f.goals.join(',') || null)
  set('mic', f.mic ? '1' : null)
  set('discord', f.discord ? '1' : null)
  return next
}

/** Сколько фильтров включено (для бейджа на мобильной кнопке «Фильтры»). */
export function activeFilterCount(f: PartyFilters): number {
  return (
    Number(f.rankMin > 0 || f.rankMax < MAX_RANK_MEDAL) +
    Number(f.positions.length > 0) +
    Number(f.modes.length > 0) +
    Number(f.goals.length > 0) +
    Number(f.mic) +
    Number(f.discord)
  )
}

export function toSearchParams(tab: SearchTab, f: PartyFilters): PartySearchParams {
  return {
    looking_for: lookingForTab(tab),
    rank_min: f.rankMin > 0 ? f.rankMin : undefined,
    rank_max: f.rankMax < MAX_RANK_MEDAL ? f.rankMax : undefined,
    positions: f.positions.join(',') || undefined,
    modes: f.modes.join(',') || undefined,
    goals: f.goals.join(',') || undefined,
    mic: f.mic ? 1 : undefined,
    discord: f.discord ? 1 : undefined,
  }
}

/** Переключает значение в списке, сохраняя порядок исходного набора. */
export function toggleIn<T>(list: readonly T[], value: T, order: readonly T[]): T[] {
  const has = list.includes(value)
  return order.filter((v) => (v === value ? !has : list.includes(v)))
}

// ----- форма анкеты -------------------------------------------------------------

export type FormErrors = Partial<Record<keyof PartyCardInput, string>>

export function emptyForm(suggestedRank: number | null): PartyCardInput {
  return {
    rank_medal: suggestedRank ?? 0,
    hours: 0,
    positions: [],
    modes: ['ranked'],
    goal: 'rating',
    looking_for: 'team',
    mic: false,
    discord: false,
    discord_tag: '',
    telegram: '',
    fav_heroes: [],
    about: '',
  }
}

export function formFromCard(card: PartyCard): PartyCardInput {
  return {
    rank_medal: card.rank_medal,
    hours: card.hours,
    positions: card.positions,
    modes: card.modes,
    goal: card.goal,
    looking_for: card.looking_for,
    mic: card.mic,
    discord: card.discord,
    discord_tag: card.discord_tag ?? '',
    telegram: card.telegram ?? '',
    fav_heroes: card.fav_heroes,
    about: card.about,
  }
}

/** Проверка до отправки; сервер проверяет то же самое (и стоп-лист) заново. */
export function validateForm(
  f: PartyCardInput,
  messages: Record<'positions' | 'modes' | 'hours' | 'about' | 'discord' | 'telegram', string>,
): FormErrors {
  const errors: FormErrors = {}
  if (f.positions.length === 0) errors.positions = messages.positions
  if (f.modes.length === 0) errors.modes = messages.modes
  if (!Number.isInteger(f.hours) || f.hours < 0 || f.hours > HOURS_MAX)
    errors.hours = messages.hours
  if ([...f.about].length > ABOUT_MAX) errors.about = messages.about
  if (f.discord && !DISCORD_RE.test(f.discord_tag.trim())) errors.discord_tag = messages.discord
  if (f.telegram.trim() && !TELEGRAM_RE.test(f.telegram.trim())) errors.telegram = messages.telegram
  return errors
}

// ----- контакты и подъём --------------------------------------------------------

export function telegramUrl(tag: string): string {
  return `https://t.me/${tag.replace(/^@/, '')}`
}

/** Сколько мс осталось до следующего подъёма; 0 — можно поднимать. */
export function bumpRemainingMs(
  card: Pick<PartyCard, 'bumped_at' | 'next_bump_at'>,
  now = Date.now(),
) {
  const next = card.next_bump_at
    ? new Date(card.next_bump_at).getTime()
    : new Date(card.bumped_at).getTime() + BUMP_COOLDOWN_MS
  return Math.max(0, next - now)
}

/** «5 ч 20 мин», «40 мин», «меньше минуты». */
export function formatRemaining(ms: number): string {
  const totalMin = Math.ceil(ms / 60_000)
  if (totalMin <= 1) return 'меньше минуты'
  const h = Math.floor(totalMin / 60)
  const m = totalMin % 60
  if (h === 0) return `${m} мин`
  return m === 0 ? `${h} ч` : `${h} ч ${m} мин`
}
