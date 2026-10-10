import type { Hero } from '@/api/types'

export type AttrFilter = Hero['primary_attr'] | null

function normalize(s: string): string {
  return s
    .toLowerCase()
    .replace(/ё/g, 'е')
    .replace(/['’`]/g, '')
    .replace(/[^\p{L}\p{N}]+/gu, ' ')
    .trim()
}

/** Герои, подходящие под поиск по названию и фильтр атрибута; порядок исходного списка сохраняется. */
export function filterHeroes(heroes: readonly Hero[], query: string, attr: AttrFilter): Hero[] {
  const q = normalize(query)
  return heroes.filter((h) => {
    if (attr && h.primary_attr !== attr) return false
    if (!q) return true
    return normalize(h.localized_name).includes(q) || normalize(h.short_name).includes(q)
  })
}

/** Следующая позиция подсветки в сетке с cols колонками; null — движение невозможно. */
export function moveHighlight(
  current: number,
  key: string,
  total: number,
  cols: number,
): number | null {
  const step: Record<string, number> = {
    ArrowRight: 1,
    ArrowLeft: -1,
    ArrowDown: cols,
    ArrowUp: -cols,
  }
  if (total === 0 || !(key in step)) return null
  const next = current + step[key]
  return next < 0 || next >= total ? null : next
}

/**
 * Делит название на «до / совпадение / после» для подсветки найденного.
 * Если запрос не входит в название буквально (нашли по внутреннему имени) — без подсветки.
 */
export function splitMatch(
  name: string,
  query: string,
): [before: string, match: string, after: string] {
  const q = query.trim().toLowerCase()
  const at = q ? name.toLowerCase().indexOf(q) : -1
  if (at < 0) return [name, '', '']
  return [name.slice(0, at), name.slice(at, at + q.length), name.slice(at + q.length)]
}
