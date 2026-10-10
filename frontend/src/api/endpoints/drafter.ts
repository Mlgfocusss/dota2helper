import { api } from '../client'
import type {
  DraftMatrixPayload,
  DraftSlot,
  HeroPageData,
  LeaderboardData,
  TrainingHistoryPage,
  TrainingResult,
  TrainingStart,
} from '../types'

export const getDrafterMatrix = (signal?: AbortSignal) =>
  api<DraftMatrixPayload>('/drafter/matrix', { signal })

export const postTrainingNew = () => api<TrainingStart>('/drafter/training/new', { method: 'POST' })

export const postTrainingSubmit = (trainingId: string, team: DraftSlot[]) =>
  api<TrainingResult>(`/drafter/training/${trainingId}/submit`, { method: 'POST', body: { team } })

export const getDrafterHistory = (cursor: string | null, signal?: AbortSignal) =>
  api<TrainingHistoryPage>('/drafter/history', { query: { cursor }, signal })

export const getLeaderboard = (month: string | null, signal?: AbortSignal) =>
  api<LeaderboardData>('/drafter/leaderboard', { query: { month }, signal })

export const getHeroPage = (heroId: number, signal?: AbortSignal) =>
  api<HeroPageData>(`/heroes/${heroId}/page`, { signal })
