import { describe, expect, it } from 'vitest'
import { t } from '@/i18n/ru'
import {
  MINUS,
  formatDuration,
  formatNumber,
  formatPercent,
  formatRelativeTime,
  formatShortDate,
  formatSigned,
  plural,
  pluralize,
} from './format'

describe('plural', () => {
  it.each([
    [1, 'драфт'],
    [2, 'драфта'],
    [4, 'драфта'],
    [5, 'драфтов'],
    [11, 'драфтов'],
    [12, 'драфтов'],
    [14, 'драфтов'],
    [21, 'драфт'],
    [22, 'драфта'],
    [32, 'драфта'],
    [100, 'драфтов'],
    [111, 'драфтов'],
    [0, 'драфтов'],
  ])('%i → %s', (n, word) => {
    expect(plural(n, t.plural.draft)).toBe(word)
  })

  it('добавляет число', () => {
    expect(pluralize(32, t.plural.draft)).toBe('32 драфта')
    expect(pluralize(1, t.plural.match)).toBe('1 матч')
  })
})

describe('formatNumber', () => {
  it('разделяет разряды неразрывным пробелом', () => {
    expect(formatNumber(12345)).toBe('12\u00A0345')
    expect(formatNumber(1234567)).toBe('1\u00A0234\u00A0567')
    expect(formatNumber(999)).toBe('999')
    expect(formatNumber(-1500)).toBe(`${MINUS}1\u00A0500`)
  })
})

describe('formatPercent', () => {
  it('округляет до целого', () => {
    expect(formatPercent(0.7)).toBe('70%')
    expect(formatPercent(0.5234)).toBe('52%')
    expect(formatPercent(0.555)).toBe('56%')
  })
})

describe('formatSigned', () => {
  it('ставит знак и настоящий минус', () => {
    expect(formatSigned(2.66)).toBe('+2.7')
    expect(formatSigned(-6.14)).toBe(`${MINUS}6.1`)
    expect(formatSigned(20)).toBe('+20.0')
    expect(formatSigned(0)).toBe('0.0')
    expect(formatSigned(-0.04)).toBe('0.0')
  })
})

describe('formatDuration', () => {
  it('минуты и секунды', () => {
    expect(formatDuration(2380)).toBe('39:40')
    expect(formatDuration(65)).toBe('1:05')
    expect(formatDuration(0)).toBe('0:00')
  })
  it('больше часа', () => {
    expect(formatDuration(3735)).toBe('1:02:15')
  })
})

describe('formatRelativeTime', () => {
  const now = new Date(2026, 9, 9, 12, 0, 0)
  const ago = (ms: number) => new Date(now.getTime() - ms)
  const MIN = 60_000
  const HOUR = 60 * MIN
  const DAY = 24 * HOUR

  it.each([
    [10_000, 'только что'],
    [5 * MIN, '5 мин назад'],
    [3 * HOUR, '3 ч назад'],
    [4 * DAY, '4 дн. назад'],
    [15 * DAY, '2 нед. назад'],
  ])('%i мс назад → %s', (ms, text) => {
    expect(formatRelativeTime(ago(ms), now)).toBe(text)
  })

  it('дальше 4 недель — дата', () => {
    expect(formatRelativeTime(new Date(2026, 7, 12, 10, 0), now)).toBe('12 авг')
  })

  it('время в будущем считается «только что»', () => {
    expect(formatRelativeTime(new Date(now.getTime() + 5 * MIN), now)).toBe('только что')
  })
})

describe('formatShortDate', () => {
  it('добавляет год для прошлых лет', () => {
    const now = new Date(2026, 9, 9)
    expect(formatShortDate(new Date(2026, 9, 12), now)).toBe('12 окт')
    expect(formatShortDate(new Date(2025, 4, 1), now)).toBe('1 мая 2025')
  })
})
