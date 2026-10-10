import { describe, expect, it } from 'vitest'
import type { DraftMatrixPayload } from '@/api/types'
import {
  analyzeDraft,
  buildMatrix,
  clearSlot,
  draftIsEmpty,
  emptyDraft,
  moveHero,
  nextEmpty,
  parseDraftState,
  round1,
  setSlot,
  slotsOf,
  suggestPicks,
  teamSize,
  usedHeroes,
  writeDraftState,
  type TeamState,
} from './drafter'

// Та же синтетическая матрица, что в backend/internal/drafter тестах:
// герои 1..12, vs(a,b) = a − b, with(a,b) = 1 для соседей по id.
function matrix(withSynergy = true) {
  const ids = Array.from({ length: 12 }, (_, i) => i + 1)
  const payload: DraftMatrixPayload = {
    ids,
    vs: ids.map((a) => ids.map((b) => a - b)),
    with: withSynergy ? ids.map((a) => ids.map((b) => (Math.abs(a - b) === 1 ? 1 : 0))) : null,
    playable: [ids.slice(0, 6), ids.slice(0, 6), ids.slice(0, 6), ids.slice(0, 6), ids.slice(0, 6)],
    has_with: withSynergy,
    updated_at: '2026-10-09T00:00:00Z',
  }
  return buildMatrix(payload)
}

const team = (...heroes: number[]): TeamState =>
  [...heroes, null, null, null, null, null].slice(0, 5)

describe('analyzeDraft', () => {
  it('считает оценки героев, итоги и пары так же, как сервер', () => {
    const a = analyzeDraft(matrix(), team(10, 11), team(1, 2))
    expect(a.radiant.heroes).toEqual([
      { hero_id: 10, score: 17.5 },
      { hero_id: 11, score: 19.5 },
    ])
    expect(a.radiant.total).toBe(37)
    expect(a.dire.total).toBe(-35)
    expect(a.best_pair).toEqual({ a: 10, b: 11, value: 1 })
    expect(a.best_matchup).toEqual({ a: 11, b: 1, value: 10 })
    expect(a.worst_matchup).toEqual({ a: 1, b: 11, value: -10 })
  })

  it('без синергий «сильнейшая пара» скрыта, а оценка — только матчапы', () => {
    const a = analyzeDraft(matrix(false), team(10, 11), team(1))
    expect(a.best_pair).toBeNull()
    expect(a.radiant.heroes[0].score).toBe(9)
  })

  it('пустой драфт и драфт без врагов', () => {
    const empty = analyzeDraft(matrix(), team(), team())
    expect(empty.radiant.total).toBe(0)
    expect(empty.best_matchup).toBeNull()
    expect(empty.worst_matchup).toBeNull()
    const solo = analyzeDraft(matrix(), team(5, 6), team())
    expect(solo.radiant.heroes[0].score).toBe(0.5)
    expect(solo.best_matchup).toBeNull()
  })

  it('герои считаются по слотам позиций', () => {
    const t: TeamState = [null, 7, null, 9, null]
    expect(slotsOf(t)).toEqual([
      { hero_id: 7, position: 2 },
      { hero_id: 9, position: 4 },
    ])
  })

  it('неизвестный герой даёт нулевой вклад', () => {
    const a = analyzeDraft(matrix(), team(10), team(99))
    expect(a.radiant.heroes[0].score).toBe(0)
  })
})

describe('round1', () => {
  it('округляет до десятых от нуля и не даёт «−0»', () => {
    expect(round1(2.66)).toBe(2.7)
    expect(round1(-6.14)).toBe(-6.1)
    expect(round1(-0.15)).toBe(-0.2)
    expect(round1(0.15)).toBe(0.2)
    expect(Object.is(round1(-0.04), 0)).toBe(true)
  })
})

describe('suggestPicks', () => {
  it('предлагает лучших против врагов, без занятых героев', () => {
    const m = matrix()
    const enemies = slotsOf(team(1, 2))
    const picks = suggestPicks(m, 3, [], enemies, new Set([6]), 3)
    // playable[2] = 1..6; шестой занят → лучшие 5, 4, 3 (vs(h,1)+vs(h,2) растёт с id)
    expect(picks.map((p) => p.hero_id)).toEqual([5, 4, 3])
    expect(picks[0].score).toBe(7)
  })

  it('учитывает синергию с союзниками и сортирует стабильно', () => {
    const m = matrix()
    const allies = slotsOf(team(8))
    const picks = suggestPicks(m, 1, allies, [], new Set(), 6)
    // без врагов оценка — только 0.5×с союзниками; сосед героя 8 — герой 7 не из пула, остальные 0
    expect(picks.every((p) => p.score === 0)).toBe(true)
    expect(picks.map((p) => p.hero_id)).toEqual([1, 2, 3, 4, 5, 6])
  })
})

describe('редактирование команды', () => {
  it('ставит, очищает и меняет местами', () => {
    let t = team()
    t = setSlot(t, 2, 9)
    expect(t[2]).toBe(9)
    t = setSlot(t, 0, 4)
    expect(moveHero(t, 0, 2)).toEqual([9, null, 4, null, null])
    expect(moveHero(t, 0, 3)).toEqual([null, null, 9, 4, null])
    expect(clearSlot(t, 2)[2]).toBeNull()
    expect(t[2]).toBe(9) // исходный массив не меняется
    expect(moveHero(t, 1, 1)).toBe(t)
  })

  it('ищет следующий пустой слот по кругу', () => {
    expect(nextEmpty(team(1, 2, 3, 4, 5))).toBeNull()
    expect(nextEmpty([1, null, 3, null, 5], 0)).toBe(1)
    expect(nextEmpty([1, null, 3, null, 5], 1)).toBe(3)
    expect(nextEmpty([null, 2, 3, 4, 5], 3)).toBe(0)
    expect(teamSize([1, null, 3, null, 5])).toBe(3)
  })
})

describe('состояние драфта в URL', () => {
  it('разбирает и собирает без потерь', () => {
    const params = new URLSearchParams('tab=analysis&r=1:12,2:45&d=5:3&b=7,8')
    const state = parseDraftState(params)
    expect(state.radiant).toEqual([12, 45, null, null, null])
    expect(state.dire).toEqual([null, null, null, null, 3])
    expect(state.bans).toEqual([7, 8])
    const back = writeDraftState(params, state)
    expect(back.get('tab')).toBe('analysis')
    expect(parseDraftState(back)).toEqual(state)
    expect(back.get('r')).toBe('1:12,2:45')
  })

  it('отбрасывает мусор, повторы героев и лишние баны', () => {
    const state = parseDraftState(
      new URLSearchParams(
        'r=1:5,1:6,9:7,2:x,3:5&d=4:5,4:8&b=8,9,9,x,' +
          Array.from({ length: 12 }, (_, i) => 20 + i).join(','),
      ),
    )
    expect(state.radiant).toEqual([5, null, null, null, null]) // повтор позиции и героя пропущены
    expect(state.dire).toEqual([null, null, null, 8, null])
    expect(state.bans.slice(0, 2)).toEqual([9, 20]) // 8 уже занят в дире
    expect(state.bans.length).toBeLessThanOrEqual(10)
    expect(new Set(state.bans).size).toBe(state.bans.length)
  })

  it('пустой драфт не добавляет параметров', () => {
    const params = writeDraftState(new URLSearchParams('tab=analysis'), emptyDraft())
    expect(params.toString()).toBe('tab=analysis')
    expect(draftIsEmpty(parseDraftState(params))).toBe(true)
  })

  it('usedHeroes объединяет команды и баны', () => {
    const state = parseDraftState(new URLSearchParams('r=1:1&d=2:2&b=3'))
    expect([...usedHeroes(state)].sort()).toEqual([1, 2, 3])
  })
})
