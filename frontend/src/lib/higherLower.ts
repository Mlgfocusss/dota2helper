// Логика «Больше / Меньше» (SPEC 5.7): подбор пар и проверка ответов. Без React — чтобы проверять тестами.

export type HeroValue = { hero_id: number; value: number }
export type Guess = 'higher' | 'lower'
export type Rng = () => number

/** Сколько последних показанных героев не повторять. */
export const RECENT_LIMIT = 10
/** С этой серии пары подбираются так, чтобы значения отличались не больше чем на MAX_GAP. */
export const HARD_FROM_STREAK = 5
export const MAX_GAP = 0.25
/** Пауза после верного ответа до следующей пары. */
export const NEXT_DELAY_MS = 700

function randomItem<T>(list: readonly T[], rng: Rng): T {
  return list[Math.floor(rng() * list.length)]
}

/** Разница значений относительно большего: 0 — равны, 1 — одно из них нулевое. */
export function relativeGap(a: number, b: number): number {
  const top = Math.max(Math.abs(a), Math.abs(b))
  return top === 0 ? 0 : Math.abs(a - b) / top
}

/** Верен ли ответ. При равных значениях верен любой. */
export function isCorrect(guess: Guess, left: HeroValue, right: HeroValue): boolean {
  if (left.value === right.value) return true
  return guess === 'higher' ? right.value > left.value : right.value < left.value
}

/** Добавляет героев в список недавних, оставляя последние RECENT_LIMIT. */
export function pushRecent(recent: readonly number[], ...ids: number[]): number[] {
  return [...recent, ...ids].slice(-RECENT_LIMIT)
}

/**
 * Следующий герой для правой карточки. Не повторяет недавних и левого героя;
 * с серии HARD_FROM_STREAK берёт близкие по значению (если таких нет — любого).
 */
export function pickNext(
  pool: readonly HeroValue[],
  left: HeroValue,
  recent: readonly number[],
  streak: number,
  rng: Rng = Math.random,
): HeroValue {
  const fresh = pool.filter((h) => h.hero_id !== left.hero_id && !recent.includes(h.hero_id))
  // если пул мал и недавние исключили всех — разрешаем всех, кроме левого
  const candidates = fresh.length > 0 ? fresh : pool.filter((h) => h.hero_id !== left.hero_id)
  if (candidates.length === 0) return left
  if (streak >= HARD_FROM_STREAK) {
    const close = candidates.filter((h) => relativeGap(h.value, left.value) <= MAX_GAP)
    if (close.length > 0) return randomItem(close, rng)
  }
  return randomItem(candidates, rng)
}

/** Первая пара партии. */
export function pickFirstPair(
  pool: readonly HeroValue[],
  rng: Rng = Math.random,
): { left: HeroValue; right: HeroValue } {
  const left = randomItem(pool, rng)
  return { left, right: pickNext(pool, left, [left.hero_id], 0, rng) }
}

/** Значение режима для показа: популярность — целое, остальное — одна цифра после запятой. */
export function formatHeroValue(mode: 'popularity' | 'kills' | 'deaths', value: number): string {
  if (mode === 'popularity') return String(Math.round(value))
  return value.toFixed(1).replace('.', ',')
}
