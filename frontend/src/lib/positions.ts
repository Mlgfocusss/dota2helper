import { t } from '@/i18n/ru'

export type Position = 1 | 2 | 3 | 4 | 5

export const POSITIONS: readonly Position[] = [1, 2, 3, 4, 5]

/** «Керри», «Мид», «Оффлейн», «Четвёрка», «Пятёрка». */
export function positionShort(p: Position): string {
  return t.positions.short[p]
}

/** «Позиция 1 — Керри». */
export function positionFull(p: Position): string {
  return t.positions.full[p]
}

/** «Керри», «Мидер», «Хардлейнер», «Роумер», «Фулл‑саппорт». */
export function positionRole(p: Position): string {
  return t.positions.role[p]
}

/** CSS‑цвет позиции из токенов --pos1..--pos5. */
export function positionColor(p: Position): string {
  return `var(--pos${p})`
}

export function isPosition(v: unknown): v is Position {
  return v === 1 || v === 2 || v === 3 || v === 4 || v === 5
}
