import { describe, expect, it } from 'vitest'
import {
  activeFilterCount,
  applyFilters,
  bumpRemainingMs,
  DEFAULT_FILTERS,
  emptyForm,
  formatRemaining,
  lookingForTab,
  parseFilters,
  toggleIn,
  toSearchParams,
  validateForm,
} from './party'

const messages = {
  positions: 'p',
  modes: 'm',
  hours: 'h',
  about: 'a',
  discord: 'd',
  telegram: 't',
}

describe('фильтры в URL', () => {
  it('разбирает и собирает без потерь', () => {
    const params = new URLSearchParams(
      'rank_min=2&rank_max=6&positions=3,1&modes=turbo,ranked&goals=fun&mic=1',
    )
    const f = parseFilters(params)
    expect(f).toEqual({
      rankMin: 2,
      rankMax: 6,
      positions: [1, 3],
      modes: ['ranked', 'turbo'],
      goals: ['fun'],
      mic: true,
      discord: false,
    })
    const back = parseFilters(applyFilters(new URLSearchParams(), f))
    expect(back).toEqual(f)
  })

  it('игнорирует мусор и меняет местами перевёрнутый диапазон', () => {
    const f = parseFilters(new URLSearchParams('rank_min=7&rank_max=x&positions=9,2&modes=hax'))
    expect(f.rankMin).toBe(7)
    expect(f.rankMax).toBe(8)
    expect(f.positions).toEqual([2])
    expect(f.modes).toEqual([])
    const swapped = parseFilters(new URLSearchParams('rank_min=6&rank_max=2'))
    expect([swapped.rankMin, swapped.rankMax]).toEqual([2, 6])
  })

  it('значения по умолчанию не попадают в адрес и не считаются фильтрами', () => {
    const params = applyFilters(new URLSearchParams('tab=players'), DEFAULT_FILTERS)
    expect(params.toString()).toBe('tab=players')
    expect(activeFilterCount(DEFAULT_FILTERS)).toBe(0)
    expect(activeFilterCount({ ...DEFAULT_FILTERS, rankMin: 3, mic: true, positions: [1] })).toBe(3)
  })

  it('вкладка «Найти команду» ищет тех, кто набирает игроков', () => {
    expect(lookingForTab('team')).toBe('players')
    expect(lookingForTab('players')).toBe('team')
    const api = toSearchParams('team', { ...DEFAULT_FILTERS, positions: [2, 4], mic: true })
    expect(api).toMatchObject({ looking_for: 'players', positions: '2,4', mic: 1 })
    expect(api.rank_min).toBeUndefined()
  })

  it('toggleIn сохраняет порядок набора', () => {
    expect(toggleIn(['turbo'], 'ranked', ['ranked', 'normal', 'turbo'])).toEqual([
      'ranked',
      'turbo',
    ])
    expect(toggleIn(['ranked', 'turbo'], 'ranked', ['ranked', 'normal', 'turbo'])).toEqual([
      'turbo',
    ])
  })
})

describe('форма анкеты', () => {
  const ok = { ...emptyForm(5), positions: [1 as const] }

  it('подставляет подсказку ранга', () => {
    expect(emptyForm(5).rank_medal).toBe(5)
    expect(emptyForm(null).rank_medal).toBe(0)
  })

  it('пустая форма требует позицию', () => {
    expect(validateForm(emptyForm(null), messages)).toEqual({ positions: 'p' })
    expect(validateForm(ok, messages)).toEqual({})
  })

  it('проверяет режимы, часы, текст и контакты', () => {
    const errors = validateForm(
      {
        ...ok,
        modes: [],
        hours: 50001,
        about: 'я'.repeat(301),
        discord: true,
        discord_tag: ' ',
        telegram: 'ab',
      },
      messages,
    )
    expect(Object.keys(errors).sort()).toEqual([
      'about',
      'discord_tag',
      'hours',
      'modes',
      'telegram',
    ])
    expect(
      validateForm(
        { ...ok, telegram: '@dota_fan', discord: true, discord_tag: 'nick.name' },
        messages,
      ),
    ).toEqual({})
  })
})

describe('подъём анкеты', () => {
  const now = Date.parse('2026-10-09T12:00:00Z')

  it('считает остаток до следующего подъёма', () => {
    const card = { bumped_at: '2026-10-09T07:00:00Z' }
    expect(bumpRemainingMs(card, now)).toBe(19 * 3600_000)
    expect(bumpRemainingMs({ bumped_at: '2026-10-07T07:00:00Z' }, now)).toBe(0)
    expect(
      bumpRemainingMs(
        { bumped_at: '2026-10-09T07:00:00Z', next_bump_at: '2026-10-09T13:30:00Z' },
        now,
      ),
    ).toBe(90 * 60_000)
  })

  it('форматирует остаток', () => {
    expect(formatRemaining(30_000)).toBe('меньше минуты')
    expect(formatRemaining(40 * 60_000)).toBe('40 мин')
    expect(formatRemaining(5 * 3600_000 + 20 * 60_000)).toBe('5 ч 20 мин')
    expect(formatRemaining(2 * 3600_000)).toBe('2 ч')
  })
})
