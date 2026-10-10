import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import {
  getCoachMatches,
  getCoachSummary,
  getMatch,
  refreshCoach,
  resolveProfile,
  setTrackedProfile,
} from '@/api/endpoints/coach'
import { keys, STALE } from '@/api/keys'
import type { Me } from '@/api/types'
import { getTrackedAccountId, setTrackedAccountId } from '@/features/tracked'
import { useMe } from '@/features/useMe'

/** Чей профиль смотрим в «Тренере»: у вошедшего — выбранный или свой, у гостя — из localStorage. */
export function useCoachAccount() {
  const { data: me, isPending } = useMe()
  const loggedIn = Boolean(me?.user)
  const accountId = me?.user
    ? (me.tracked_account_id ?? me.user.account_id)
    : isPending
      ? null
      : getTrackedAccountId()
  return { accountId, loggedIn, loading: isPending }
}

// 404 («игрок не найден») повторять бессмысленно
const retryUnlessClientError = (count: number, error: Error) =>
  error instanceof ApiError && error.status >= 400 && error.status < 500 ? false : count < 1

export function useCoachSummary(accountId: number | null) {
  return useQuery({
    queryKey: keys.coachSummary(accountId ?? 0),
    queryFn: ({ signal }) => getCoachSummary(accountId ?? 0, signal),
    enabled: accountId != null,
    staleTime: STALE.summary,
    retry: retryUnlessClientError,
  })
}

export type MatchFilter = 'all' | 'win' | 'loss'

export function useCoachMatches(accountId: number | null, filter: MatchFilter) {
  return useInfiniteQuery({
    queryKey: keys.coachMatches(accountId ?? 0, filter),
    queryFn: ({ pageParam, signal }) => getCoachMatches(accountId ?? 0, filter, pageParam, signal),
    initialPageParam: null as string | null,
    getNextPageParam: (last) => last.next_cursor,
    enabled: accountId != null,
    staleTime: STALE.summary,
    retry: retryUnlessClientError,
  })
}

/** «Проверить снова ↻»: OpenDota перезагружает историю, затем перечитываем данные. */
export function useRefreshCoach(accountId: number | null) {
  const queryClient = useQueryClient()
  return useMutation({
    // кнопка крутится, пока данные не перечитаны заново
    mutationFn: async () => {
      await refreshCoach(accountId ?? 0)
      if (accountId != null)
        await queryClient.invalidateQueries({ queryKey: keys.coach(accountId) })
    },
  })
}

export function useMatchDetail(matchId: number, accountId: number | null) {
  return useQuery({
    queryKey: keys.match(matchId, accountId),
    queryFn: ({ signal }) => getMatch(matchId, accountId, signal),
    enabled: Number.isSafeInteger(matchId) && matchId > 0,
    staleTime: 5 * 60_000,
    retry: retryUnlessClientError,
  })
}

/** Поиск профиля по ссылке; ошибки показывает сама форма, а не тост. */
export function useResolveProfile() {
  return useMutation({ mutationFn: resolveProfile, meta: { silent: true } })
}

function resetCoachData(queryClient: QueryClient) {
  queryClient.removeQueries({ queryKey: ['coach'] })
  queryClient.removeQueries({ queryKey: ['match'] })
}

/** «Это я»: вошедший сохраняет выбор на сервере, гость — в localStorage. */
export function useSelectProfile() {
  const queryClient = useQueryClient()
  const { data: me } = useMe()
  return useMutation({
    mutationFn: async (accountId: number) => {
      if (me?.user) await setTrackedProfile(accountId)
      else setTrackedAccountId(accountId)
      return accountId
    },
    onSuccess: (accountId) => {
      resetCoachData(queryClient)
      if (me?.user) {
        queryClient.setQueryData<Me>(keys.me, { ...me, tracked_account_id: accountId })
      }
    },
  })
}

const SEEN_KEY = 'coach_summary_seen'

/** Видел ли пользователь сводку после первого входа (SPEC 5.3). */
export function hasSeenSummary(): boolean {
  try {
    return localStorage.getItem(SEEN_KEY) === '1'
  } catch {
    return true // без localStorage не гоняем на сводку при каждом заходе
  }
}

export function markSummarySeen() {
  try {
    localStorage.setItem(SEEN_KEY, '1')
  } catch {
    // нет доступа к хранилищу — сводка просто не запомнится
  }
}
