// Открытая тренировка хранится в localStorage: пережить перезагрузку и вход через Steam
// (SPEC 4.2: гость собирает драфт, входит и возвращается к нему).
import type { DraftSlot } from '@/api/types'
import type { TeamState } from './drafter'

export type SavedTraining = {
  trainingId: string
  enemies: DraftSlot[]
  team: TeamState
  /** Гость нажал «Оценить» и ушёл входить: после возврата оцениваем сам */
  submitAfterLogin: boolean
}

const KEY = 'd2h_training'

function isSlot(v: unknown): v is DraftSlot {
  const s = v as DraftSlot
  return (
    typeof s === 'object' &&
    s !== null &&
    Number.isInteger(s.hero_id) &&
    Number.isInteger(s.position) &&
    s.position >= 1 &&
    s.position <= 5
  )
}

export function loadTraining(): SavedTraining | null {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return null
    const v = JSON.parse(raw) as Partial<SavedTraining>
    const team = v.team
    if (
      typeof v.trainingId !== 'string' ||
      !Array.isArray(v.enemies) ||
      v.enemies.length !== 5 ||
      !v.enemies.every(isSlot) ||
      !Array.isArray(team) ||
      team.length !== 5 ||
      !team.every((h) => h === null || Number.isInteger(h))
    ) {
      return null
    }
    return {
      trainingId: v.trainingId,
      enemies: v.enemies,
      team: team as TeamState,
      submitAfterLogin: v.submitAfterLogin === true,
    }
  } catch {
    return null
  }
}

export function saveTraining(training: SavedTraining) {
  try {
    localStorage.setItem(KEY, JSON.stringify(training))
  } catch {
    // нет доступа к хранилищу — тренировка просто не переживёт перезагрузку
  }
}

export function clearTraining() {
  try {
    localStorage.removeItem(KEY)
  } catch {
    // ничего страшного
  }
}
