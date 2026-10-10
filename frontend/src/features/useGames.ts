import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import {
  getGameRecords,
  getHeroQuiz,
  getLastPosition,
  getPositionQuiz,
  postHeroResult,
  postHigherLowerScore,
  postPositionResult,
} from '@/api/endpoints/games'
import { keys } from '@/api/keys'

// вопросы квиза не меняются в течение сессии
const QUIZ_STALE = 60 * 60_000

const noRetryOnClientError = (count: number, error: Error) =>
  error instanceof ApiError && error.status >= 400 && error.status < 500 ? false : count < 1

/** Рекорды вошедшего пользователя (у гостя сервер отдаёт нули — рекорды берём из localStorage). */
export function useGameRecords() {
  return useQuery({
    queryKey: keys.gameRecords,
    queryFn: ({ signal }) => getGameRecords(signal),
    staleTime: 30_000,
  })
}

export function useSaveScore() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ mode, streak }: { mode: string; streak: number }) =>
      postHigherLowerScore(mode, streak),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.gameRecords }),
    // рекорд в localStorage уже записан — тост об ошибке сохранения не нужен
    meta: { silent: true },
  })
}

export function usePositionQuiz() {
  return useQuery({
    queryKey: keys.quiz('position'),
    queryFn: ({ signal }) => getPositionQuiz(signal),
    staleTime: QUIZ_STALE,
    retry: noRetryOnClientError,
  })
}

export function useHeroQuiz(position: number | null) {
  return useQuery({
    queryKey: keys.quiz(`hero/${position}`),
    queryFn: ({ signal }) => getHeroQuiz(position ?? 0, signal),
    enabled: position != null,
    staleTime: QUIZ_STALE,
    retry: noRetryOnClientError,
  })
}

/** Последняя позиция из квиза у вошедшего пользователя. */
export function useLastPosition(enabled: boolean) {
  return useQuery({
    queryKey: keys.lastPosition,
    queryFn: ({ signal }) => getLastPosition(signal),
    enabled,
    staleTime: 60_000,
  })
}

export function usePositionResult() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: postPositionResult,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.lastPosition }),
  })
}

export function useHeroResult() {
  return useMutation({
    mutationFn: ({ position, answers }: { position: number; answers: Record<string, string[]> }) =>
      postHeroResult(position, answers),
  })
}
