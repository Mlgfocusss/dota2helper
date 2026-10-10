// Месяцы топа драфтеров: «2026-10» по московскому времени (SPEC 5.3).
import { t } from '@/i18n/ru'

/** Самый ранний месяц, который принимает сервер. */
export const FIRST_MONTH = '2025-01'

const MSK_OFFSET_MS = 3 * 60 * 60 * 1000

/** Текущий месяц по Москве (без перехода на летнее время, как на сервере). */
export function currentMonth(now: number = Date.now()): string {
  const d = new Date(now + MSK_OFFSET_MS)
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`
}

export function isMonth(value: string | null): value is string {
  return value !== null && /^\d{4}-(0[1-9]|1[0-2])$/.test(value)
}

/** Сдвиг месяца: shiftMonth('2026-01', -1) → '2025-12'. */
export function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split('-').map(Number)
  const index = y * 12 + (m - 1) + delta
  return `${Math.floor(index / 12)}-${String((index % 12) + 1).padStart(2, '0')}`
}

/** Допустимый месяц из адреса: между FIRST_MONTH и текущим, иначе текущий. */
export function normalizeMonth(raw: string | null, now: number = Date.now()): string {
  const current = currentMonth(now)
  if (!isMonth(raw) || raw < FIRST_MONTH || raw > current) return current
  return raw
}

/** «ОКТЯБРЬ 2026» — регистр капсом делает стиль .caps. */
export function monthLabel(month: string): string {
  const [y, m] = month.split('-').map(Number)
  return `${t.time.monthsFull[m - 1]} ${y}`
}
