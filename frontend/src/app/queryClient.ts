import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import { toast } from '@/features/toast/store'
import { t } from '@/i18n/ru'

declare module '@tanstack/react-query' {
  interface Register {
    queryMeta: { silent?: boolean }
    mutationMeta: { silent?: boolean }
  }
}

function isServerOrNetwork(error: unknown): error is ApiError {
  return error instanceof ApiError && (error.isNetwork || error.status >= 500)
}

export const queryClient = new QueryClient({
  queryCache: new QueryCache({
    // экраны сами показывают error state; тост — только про сеть и падения сервера
    onError: (error, query) => {
      if (!query.meta?.silent && isServerOrNetwork(error)) toast.error(error.message)
    },
  }),
  mutationCache: new MutationCache({
    onError: (error, _vars, _ctx, mutation) => {
      if (mutation.meta?.silent) return
      toast.error(error instanceof ApiError ? error.message : t.states.unknown)
    },
  }),
  defaultOptions: {
    queries: {
      // 4xx повторять бессмысленно; сеть и 5xx — до двух повторов
      retry: (count, error) =>
        error instanceof ApiError && error.status >= 400 && error.status < 500 ? false : count < 2,
      refetchOnWindowFocus: false,
    },
  },
})
