import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import {
  bumpPartyMe,
  deletePartyMe,
  getPartyMe,
  putPartyMe,
  reportParty,
  searchParty,
  setPartyActive,
} from '@/api/endpoints/party'
import { getProfileStats } from '@/api/endpoints/profile'
import { keys } from '@/api/keys'
import type { PartyCard, PartyMe, PartyReportReason } from '@/api/types'
import { toSearchParams, type PartyFilters, type SearchTab } from '@/lib/party'

const noRetryOnClientError = (count: number, error: Error) =>
  error instanceof ApiError && error.status >= 400 && error.status < 500 ? false : count < 1

/** Моя анкета и подсказка ранга для формы. Только для вошедших. */
export function usePartyMe(enabled = true) {
  return useQuery({
    queryKey: keys.partyMe,
    queryFn: ({ signal }) => getPartyMe(signal),
    enabled,
    staleTime: 30_000,
    retry: noRetryOnClientError,
  })
}

/** Кладёт свежую анкету в кэш: после сохранения, подъёма и скрытия. */
function useStoreCard() {
  const queryClient = useQueryClient()
  return (card: PartyCard) => {
    queryClient.setQueryData<PartyMe>(keys.partyMe, (old) => ({
      suggested_rank: old?.suggested_rank ?? null,
      card,
    }))
    // порядок и состав выдачи изменились
    void queryClient.invalidateQueries({ queryKey: keys.partySearchAll })
  }
}

export function useSaveCard() {
  const store = useStoreCard()
  return useMutation({
    mutationFn: putPartyMe,
    onSuccess: store,
    // ошибки по полям форма показывает сама
    meta: { silent: true },
  })
}

export function useBumpCard() {
  const store = useStoreCard()
  return useMutation({ mutationFn: bumpPartyMe, onSuccess: store })
}

export function useSetCardActive() {
  const store = useStoreCard()
  return useMutation({ mutationFn: setPartyActive, onSuccess: store })
}

export function useDeleteCard() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: deletePartyMe,
    onSuccess: () => {
      queryClient.setQueryData<PartyMe>(keys.partyMe, (old) => ({
        suggested_rank: old?.suggested_rank ?? null,
        card: null,
      }))
      void queryClient.invalidateQueries({ queryKey: keys.partySearchAll })
    },
  })
}

export function useReportCard() {
  return useMutation({
    mutationFn: ({ userId, reason }: { userId: number; reason: PartyReportReason }) =>
      reportParty(userId, reason),
    // «уже жаловался» и другие понятные ошибки показывает окно жалобы
    meta: { silent: true },
  })
}

/** Выдача поиска с подгрузкой по курсору. */
export function usePartySearch(tab: SearchTab, filters: PartyFilters, enabled: boolean) {
  const params = toSearchParams(tab, filters)
  return useInfiniteQuery({
    queryKey: keys.partySearch(JSON.stringify(params)),
    queryFn: ({ pageParam, signal }) => searchParty({ ...params, cursor: pageParam }, signal),
    initialPageParam: null as string | null,
    getNextPageParam: (last) => last.next_cursor,
    enabled,
    staleTime: 30_000,
    retry: noRetryOnClientError,
  })
}

/** Рекорды, квизы и статистика драфтера для профиля. */
export function useProfileStats(enabled: boolean) {
  return useQuery({
    queryKey: keys.profileStats,
    queryFn: ({ signal }) => getProfileStats(signal),
    enabled,
    staleTime: 30_000,
    retry: noRetryOnClientError,
  })
}
