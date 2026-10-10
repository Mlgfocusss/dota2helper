import { t } from '@/i18n/ru'

/** Настоящий минус (U+2212) для отрицательных чисел. */
export const MINUS = '\u2212'
const NBSP = '\u00A0'

type PluralForms = readonly [one: string, few: string, many: string]

/** Форма слова для числа: plural(5, ['драфт', 'драфта', 'драфтов']) → 'драфтов'. */
export function plural(n: number, forms: PluralForms): string {
  const abs = Math.abs(Math.trunc(n))
  const mod100 = abs % 100
  const mod10 = abs % 10
  if (mod100 >= 11 && mod100 <= 14) return forms[2]
  if (mod10 === 1) return forms[0]
  if (mod10 >= 2 && mod10 <= 4) return forms[1]
  return forms[2]
}

/** Число со словом: pluralize(32, t.plural.draft) → '32 драфта'. */
export function pluralize(n: number, forms: PluralForms): string {
  return `${formatNumber(n)} ${plural(n, forms)}`
}

/** Разделители разрядов неразрывным пробелом: 12345 → '12 345'. */
export function formatNumber(n: number): string {
  const sign = n < 0 ? MINUS : ''
  const digits = String(Math.round(Math.abs(n)))
  return sign + digits.replace(/\B(?=(\d{3})+(?!\d))/g, NBSP)
}

/** Доля в целых процентах: 0.523 → '52%'. */
export function formatPercent(fraction: number): string {
  return `${Math.round(fraction * 100)}%`
}

/** Оценка со знаком и одним знаком после точки: 2.66 → '+2.7', -6.1 → '−6.1'. */
export function formatSigned(value: number, digits = 1): string {
  const rounded = Number(value.toFixed(digits))
  if (rounded === 0) return (0).toFixed(digits)
  const abs = Math.abs(rounded).toFixed(digits)
  return rounded > 0 ? `+${abs}` : `${MINUS}${abs}`
}

/** Длительность матча: 2380 → '39:40', 3735 → '1:02:15'. */
export function formatDuration(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  const pad = (v: number) => String(v).padStart(2, '0')
  return h > 0 ? `${h}:${pad(m)}:${pad(sec)}` : `${m}:${pad(sec)}`
}

function toDate(value: Date | string | number): Date {
  return value instanceof Date ? value : new Date(value)
}

/** Короткая дата: '12 окт', для прошлых лет — '12 окт 2025'. */
export function formatShortDate(value: Date | string | number, now: Date = new Date()): string {
  const d = toDate(value)
  const base = `${d.getDate()} ${t.time.monthsShort[d.getMonth()]}`
  return d.getFullYear() === now.getFullYear() ? base : `${base} ${d.getFullYear()}`
}

/** Относительное время: «только что», «5 мин назад», «4 дн. назад», «2 нед. назад», потом дата. */
export function formatRelativeTime(value: Date | string | number, now: Date = new Date()): string {
  const d = toDate(value)
  const seconds = Math.max(0, Math.floor((now.getTime() - d.getTime()) / 1000))
  if (seconds < 60) return t.time.justNow
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return t.time.minutesAgo(minutes)
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return t.time.hoursAgo(hours)
  const days = Math.floor(hours / 24)
  if (days < 7) return t.time.daysAgo(days)
  if (days < 28) return t.time.weeksAgo(Math.floor(days / 7))
  return formatShortDate(d, now)
}

/** Сколько миллисекунд осталось до момента iso; 0 — уже наступил или момента нет. */
export function msUntil(iso: string | null | undefined, now: number = Date.now()): number {
  if (!iso) return 0
  return Math.max(0, new Date(iso).getTime() - now)
}
