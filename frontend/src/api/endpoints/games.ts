import { api } from '../client'
import type {
  GameRecords,
  HeroResult,
  HeroValue,
  LastPosition,
  PositionQuizData,
  PositionResult,
  HeroQuizData,
} from '../types'

export const getHigherLowerSnapshot = (mode: string, signal?: AbortSignal) =>
  api<HeroValue[]>(`/games/higher-lower/${mode}`, { signal })

export const postHigherLowerScore = (mode: string, streak: number) =>
  api<{ best: number }>(`/games/higher-lower/${mode}/score`, { method: 'POST', body: { streak } })

export const getGameRecords = (signal?: AbortSignal) =>
  api<GameRecords>('/games/records', { signal })

export const getPositionQuiz = (signal?: AbortSignal) =>
  api<PositionQuizData>('/quiz/position', { signal })

export const getHeroQuiz = (position: number, signal?: AbortSignal) =>
  api<HeroQuizData>(`/quiz/hero/${position}`, { signal })

export const getLastPosition = (signal?: AbortSignal) =>
  api<LastPosition>('/quiz/position/last', { signal })

export const postPositionResult = (answers: Record<string, string>) =>
  api<PositionResult>('/quiz/position/result', { method: 'POST', body: { answers } })

export const postHeroResult = (position: number, answers: Record<string, string[]>) =>
  api<HeroResult>('/quiz/hero/result', { method: 'POST', body: { position, answers } })
