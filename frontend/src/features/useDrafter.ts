import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import {
  getDrafterHistory,
  getDrafterMatrix,
  getHeroPage,
  getLeaderboard,
  postTrainingNew,
  postTrainingSubmit,
} from '@/api/endpoints/drafter'
import { keys } from '@/api/keys'
import type { DraftSlot } from '@/api/types'
import { buildMatrix } from '@/lib/drafter'

const noRetryOnClientError = (count: number, error: Error) =>
  error instanceof ApiError && error.status >= 400 && error.status < 500 ? false : count < 1

/** 503 «матрицу ещё собирают» — ждём и повторяем, пока бэкенд не соберёт данные. */
const retryWhileCollecting = (count: number, error: Error) =>
  error instanceof ApiError && error.status === 503 ? count < 5 : count < 2

/** Матрица матчапов: грузится один раз за сессию, дальше ревалидируется по ETag. */
export function useDrafterMatrix(enabled = true) {
  return useQuery({
    queryKey: keys.drafterMatrix,
    enabled,
    queryFn: ({ signal }) => getDrafterMatrix(signal),
    staleTime: 60 * 60_000,
    gcTime: Infinity,
    retry: retryWhileCollecting,
    retryDelay: (attempt) => Math.min(30_000, 5_000 * 2 ** attempt),
    select: buildMatrix,
  })
}

/** Начать тренировку: сервер выбирает врагов. */
export function useNewTraining() {
  return useMutation({ mutationFn: postTrainingNew })
}

export function useSubmitTraining() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ trainingId, team }: { trainingId: string; team: DraftSlot[] }) =>
      postTrainingSubmit(trainingId, team),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: keys.drafterHistory })
      void queryClient.invalidateQueries({ queryKey: ['drafter', 'leaderboard'] })
      void queryClient.invalidateQueries({ queryKey: keys.profileStats })
    },
    // «тренировка устарела», «слишком много» — понятные тексты показывает сама страница
    meta: { silent: true },
  })
}

/** История тренировок с подгрузкой по курсору. Только для вошедших. */
export function useDrafterHistory(enabled: boolean) {
  return useInfiniteQuery({
    queryKey: keys.drafterHistory,
    queryFn: ({ pageParam, signal }) => getDrafterHistory(pageParam, signal),
    initialPageParam: null as string | null,
    getNextPageParam: (last) => last.next_cursor,
    enabled,
    staleTime: 30_000,
    retry: noRetryOnClientError,
  })
}

/** Топ драфтеров за месяц («2026-10»); null — текущий. */
export function useLeaderboard(month: string | null) {
  return useQuery({
    queryKey: keys.drafterLeaderboard(month ?? 'current'),
    queryFn: ({ signal }) => getLeaderboard(month, signal),
    staleTime: 60_000,
    retry: noRetryOnClientError,
  })
}

/** Данные страницы героя. */
export function useHeroPage(heroId: number) {
  return useQuery({
    queryKey: keys.heroPage(heroId),
    queryFn: ({ signal }) => getHeroPage(heroId, signal),
    enabled: heroId > 0,
    staleTime: 10 * 60_000,
    retry: noRetryOnClientError,
  })
}
