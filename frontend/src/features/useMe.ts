import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { logout } from '@/api/endpoints/auth'
import { getAppConfig, getMe } from '@/api/endpoints/system'
import { keys, STALE } from '@/api/keys'
import type { Me } from '@/api/types'

const GUEST: Me = { user: null, tracked_account_id: null }

/** Текущий пользователь (user = null для гостя). */
export function useMe() {
  return useQuery({
    queryKey: keys.me,
    queryFn: ({ signal }) => getMe(signal),
    staleTime: STALE.me,
  })
}

/** Выход: сессия удаляется на сервере, интерфейс сразу переходит в гостевой режим. */
export function useLogout() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: logout,
    onSuccess: () => {
      queryClient.setQueryData(keys.me, GUEST)
      // данные, привязанные к аккаунту, больше не нужны
      queryClient.removeQueries({ queryKey: keys.profileStats })
      queryClient.removeQueries({ queryKey: keys.partyMe })
      queryClient.removeQueries({ queryKey: keys.analyses })
      queryClient.removeQueries({ queryKey: keys.drafterHistory })
    },
  })
}

/** Публичные настройки сайта: промо‑баннер, ссылки поддержки, флаги фич. */
export function useAppConfig() {
  return useQuery({
    queryKey: keys.config,
    queryFn: ({ signal }) => getAppConfig(signal),
    staleTime: STALE.constants,
  })
}
