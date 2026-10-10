import { describe, expect, it } from 'vitest'
import { currentMonth, isMonth, monthLabel, normalizeMonth, shiftMonth } from './months'

describe('months', () => {
  it('текущий месяц считается по Москве', () => {
    // 30 сентября 22:00 UTC — в Москве уже 1 октября
    expect(currentMonth(Date.UTC(2026, 8, 30, 22, 0))).toBe('2026-10')
    expect(currentMonth(Date.UTC(2026, 8, 30, 20, 59))).toBe('2026-09')
    expect(currentMonth(Date.UTC(2026, 11, 31, 21, 0))).toBe('2027-01')
  })

  it('сдвигает месяцы через границу года', () => {
    expect(shiftMonth('2026-01', -1)).toBe('2025-12')
    expect(shiftMonth('2025-12', 1)).toBe('2026-01')
    expect(shiftMonth('2026-10', -10)).toBe('2025-12')
    expect(shiftMonth('2026-10', 0)).toBe('2026-10')
  })

  it('проверяет формат и границы', () => {
    expect(isMonth('2026-10')).toBe(true)
    expect(isMonth('2026-13')).toBe(false)
    expect(isMonth('2026-1')).toBe(false)
    expect(isMonth(null)).toBe(false)
    const now = Date.UTC(2026, 9, 9)
    expect(normalizeMonth('2026-08', now)).toBe('2026-08')
    expect(normalizeMonth('2026-11', now)).toBe('2026-10') // будущее
    expect(normalizeMonth('2024-12', now)).toBe('2026-10') // до запуска
    expect(normalizeMonth('мусор', now)).toBe('2026-10')
    expect(normalizeMonth(null, now)).toBe('2026-10')
  })

  it('подписывает месяц', () => {
    expect(monthLabel('2026-10')).toBe('Октябрь 2026')
    expect(monthLabel('2025-01')).toBe('Январь 2025')
  })
})
