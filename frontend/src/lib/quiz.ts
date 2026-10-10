// Общая логика квизов, без React — чтобы проверять тестами.

/** Сколько вариантов можно выбрать в вопросах подбора героя (SPEC 5.9). */
export const MAX_CHOICES = 2

/** Выбор варианта: повторный клик снимает выбор; третий выбрать нельзя. */
export function toggleChoice(selected: readonly string[], id: string): string[] {
  if (selected.includes(id)) return selected.filter((x) => x !== id)
  if (selected.length >= MAX_CHOICES) return [...selected]
  return [...selected, id]
}

/** Ответ клавишей 1–9 → индекс варианта; null, если клавиша не подходит. */
export function optionIndexFromKey(key: string, optionsCount: number): number | null {
  if (!/^[1-9]$/.test(key)) return null
  const index = Number(key) - 1
  return index < optionsCount ? index : null
}

const LAST_POSITION_KEY = 'last_position'

/** Последняя позиция из квиза у гостя (хранится в localStorage). */
export function getLastPosition(): 1 | 2 | 3 | 4 | 5 | null {
  try {
    const n = Number(localStorage.getItem(LAST_POSITION_KEY))
    return n >= 1 && n <= 5 && Number.isInteger(n) ? (n as 1 | 2 | 3 | 4 | 5) : null
  } catch {
    return null
  }
}

export function setLastPosition(position: number) {
  try {
    localStorage.setItem(LAST_POSITION_KEY, String(position))
  } catch {
    // нет доступа к хранилищу — результат просто не запомнится
  }
}

const RECORD_KEY = (mode: string) => `hl_best_${mode}`

/** Рекорд гостя в «Больше / Меньше» (localStorage). */
export function getLocalRecord(mode: string): number {
  try {
    const n = Number(localStorage.getItem(RECORD_KEY(mode)))
    return Number.isInteger(n) && n > 0 ? n : 0
  } catch {
    return 0
  }
}

export function setLocalRecord(mode: string, value: number) {
  try {
    localStorage.setItem(RECORD_KEY(mode), String(value))
  } catch {
    // нет доступа к хранилищу — рекорд не запомнится
  }
}

/** «контроль, сейвы и лечение» — перечисление через запятую и «и». */
export function joinLabels(labels: readonly string[]): string {
  if (labels.length <= 1) return labels.join('')
  return `${labels.slice(0, -1).join(', ')} и ${labels[labels.length - 1]}`
}
