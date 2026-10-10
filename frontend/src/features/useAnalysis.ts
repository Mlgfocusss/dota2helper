import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import {
  createAnalysis,
  getAnalysis,
  getAnalysisList,
  getAnalysisExample,
  getAnalysisQuota,
} from '@/api/endpoints/analysis'
import { keys } from '@/api/keys'
import type { AnalysisView } from '@/api/types'

const noRetryOnClientError = (count: number, error: Error) =>
  error instanceof ApiError && error.status >= 400 && error.status < 500 ? false : count < 2

/** Как часто спрашивать статус, пока разбор не завершился. */
export const POLL_MS = 4000

export const isAnalysisActive = (status: AnalysisView['status'] | undefined) =>
  status === 'queued' || status === 'parsing' || status === 'analyzing'

/** Сколько бесплатных разборов осталось сегодня. Только для вошедших. */
export function useAnalysisQuota(enabled: boolean) {
  return useQuery({
    queryKey: keys.analysisQuota,
    queryFn: ({ signal }) => getAnalysisQuota(signal),
    enabled,
    staleTime: 30_000,
    retry: noRetryOnClientError,
  })
}

/** Все разборы пользователя; пока какой-то идёт, список обновляется сам. */
export function useAnalysisList(enabled: boolean) {
  return useQuery({
    queryKey: keys.analysisList,
    enabled,
    queryFn: ({ signal }) => getAnalysisList(signal),
    select: (data) => data.items,
    refetchInterval: (query) =>
      query.state.data?.items.some((it) => isAnalysisActive(it.status)) ? POLL_MS : false,
    retry: noRetryOnClientError,
  })
}

/** Разбор по id; пока он в работе, статус обновляется сам. id = "example" — пример для всех. */
export function useAnalysis(id: string, enabled = true) {
  const isExample = id === 'example'
  return useQuery({
    queryKey: keys.analysis(id),
    enabled,
    queryFn: ({ signal }) => (isExample ? getAnalysisExample(signal) : getAnalysis(id, signal)),
    staleTime: isExample ? 60 * 60_000 : 0,
    refetchInterval: (query) => (isAnalysisActive(query.state.data?.status) ? POLL_MS : false),
    retry: noRetryOnClientError,
  })
}

/** Заказ разбора. Ошибки (лимит, чужой матч) показывает страница сама. */
export function useCreateAnalysis() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: createAnalysis,
    onSuccess: (view) => {
      queryClient.setQueryData(keys.analysis(view.id), view)
      void queryClient.invalidateQueries({ queryKey: keys.analysisQuota })
      void queryClient.invalidateQueries({ queryKey: keys.analysisList })
      // статус разбора виден в списках матчей «Тренера»
      void queryClient.invalidateQueries({ queryKey: ['coach'] })
    },
    meta: { silent: true },
  })
}
