import { describe, expect, it } from 'vitest'
import {
  formatHeroValue,
  isCorrect,
  MAX_GAP,
  pickFirstPair,
  pickNext,
  pushRecent,
  RECENT_LIMIT,
  relativeGap,
  type HeroValue,
} from './higherLower'

const hero = (id: number, value: number): HeroValue => ({ hero_id: id, value })

// детерминированный «случайный» ряд
function seeded(seed = 1) {
  let s = seed
  return () => {
    s = (s * 16807) % 2147483647
    return s / 2147483647
  }
}

describe('isCorrect', () => {
  it('сравнивает значения правого и левого героев', () => {
    expect(isCorrect('higher', hero(1, 10), hero(2, 20))).toBe(true)
    expect(isCorrect('lower', hero(1, 10), hero(2, 20))).toBe(false)
    expect(isCorrect('lower', hero(1, 20), hero(2, 10))).toBe(true)
    expect(isCorrect('higher', hero(1, 20), hero(2, 10))).toBe(false)
  })

  it('при равных значениях засчитывает любой ответ', () => {
    expect(isCorrect('higher', hero(1, 7), hero(2, 7))).toBe(true)
    expect(isCorrect('lower', hero(1, 7), hero(2, 7))).toBe(true)
  })
})

describe('relativeGap', () => {
  it('считает разницу относительно большего значения', () => {
    expect(relativeGap(100, 75)).toBe(0.25)
    expect(relativeGap(75, 100)).toBe(0.25)
    expect(relativeGap(5, 5)).toBe(0)
    expect(relativeGap(0, 0)).toBe(0)
  })
})

describe('pushRecent', () => {
  it('хранит только последние RECENT_LIMIT героев', () => {
    let recent: number[] = []
    for (let i = 1; i <= 15; i++) recent = pushRecent(recent, i)
    expect(recent).toHaveLength(RECENT_LIMIT)
    expect(recent[0]).toBe(6)
    expect(recent.at(-1)).toBe(15)
  })
})

describe('pickNext', () => {
  const pool = Array.from({ length: 40 }, (_, i) => hero(i + 1, (i + 1) * 100))

  it('не выбирает левого и недавних героев', () => {
    const rng = seeded(7)
    const left = pool[0]
    const recent = pool.slice(1, 11).map((h) => h.hero_id)
    for (let i = 0; i < 200; i++) {
      const next = pickNext(pool, left, recent, 0, rng)
      expect(next.hero_id).not.toBe(left.hero_id)
      expect(recent).not.toContain(next.hero_id)
    }
  })

  it('с серии 5 подбирает близкие значения (разница не больше 25%)', () => {
    const rng = seeded(3)
    const left = hero(100, 2000)
    for (let i = 0; i < 200; i++) {
      const next = pickNext(pool, left, [], 5, rng)
      expect(relativeGap(next.value, left.value)).toBeLessThanOrEqual(MAX_GAP)
    }
  })

  it('до серии 5 разброс не ограничен', () => {
    const rng = seeded(5)
    const left = hero(100, 100)
    const gaps = Array.from({ length: 200 }, () =>
      relativeGap(pickNext(pool, left, [], 2, rng).value, 100),
    )
    expect(Math.max(...gaps)).toBeGreaterThan(MAX_GAP)
  })

  it('если близких нет — берёт любого, а не зависает', () => {
    const left = hero(100, 1_000_000)
    const next = pickNext(pool, left, [], 9, seeded(2))
    expect(pool).toContainEqual(next)
  })

  it('если недавние исключили всех — разрешает их, но не левого', () => {
    const small = [hero(1, 1), hero(2, 2), hero(3, 3)]
    const next = pickNext(small, small[0], [1, 2, 3], 0, seeded(4))
    expect(next.hero_id).not.toBe(1)
  })
})

describe('pickFirstPair', () => {
  it('даёт двух разных героев', () => {
    const pool = [hero(1, 5), hero(2, 6), hero(3, 7)]
    for (let seed = 1; seed < 30; seed++) {
      const { left, right } = pickFirstPair(pool, seeded(seed))
      expect(left.hero_id).not.toBe(right.hero_id)
    }
  })
})

describe('formatHeroValue', () => {
  it('популярность — целое, убийства и смерти — одна цифра после запятой', () => {
    expect(formatHeroValue('popularity', 1234.4)).toBe('1234')
    expect(formatHeroValue('kills', 7.456)).toBe('7,5')
    expect(formatHeroValue('deaths', 6)).toBe('6,0')
  })
})
