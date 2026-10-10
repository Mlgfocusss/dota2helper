import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import {
  deleteComment,
  getComments,
  getMetaPositions,
  getNews,
  getNewsItem,
  getPatchNotes,
  postComment,
  sendFeedback,
} from '@/api/endpoints/content'
import { keys, STALE } from '@/api/keys'

/** Сколько новостей грузим: одна и та же выборка нужна главной и списку. */
export const NEWS_LIMIT = 20

/** Топ героев по позициям (карусель меты на главной). */
export function useMetaPositions(limit = 5) {
  return useQuery({
    queryKey: keys.metaPositions(limit),
    queryFn: ({ signal }) => getMetaPositions(limit, signal),
    staleTime: STALE.meta,
    // 503 «мета ещё собирается» — повторяем, пока бэкенд не соберёт данные
    retry: (count, error) =>
      error instanceof ApiError && error.status === 503 ? count < 5 : count < 2,
    retryDelay: (attempt) => Math.min(30_000, 5_000 * 2 ** attempt),
  })
}

export function useNews() {
  return useQuery({
    queryKey: keys.news(NEWS_LIMIT),
    queryFn: ({ signal }) => getNews(NEWS_LIMIT, signal),
    staleTime: 5 * 60_000,
  })
}

export function useNewsItem(gid: string) {
  return useQuery({
    queryKey: keys.newsItem(gid),
    queryFn: ({ signal }) => getNewsItem(gid, signal),
    staleTime: 5 * 60_000,
  })
}

/** Изменения патча: грузятся, только если новость о патче (version задан). */
export function usePatchNotes(version: string | undefined) {
  return useQuery({
    queryKey: keys.patchNotes(version ?? ''),
    queryFn: ({ signal }) => getPatchNotes(version ?? '', signal),
    enabled: Boolean(version),
    staleTime: 30 * 60_000,
  })
}

/** Комментарии к новости: новые сверху, дальше — «Показать ещё». */
export function useComments(gid: string) {
  return useInfiniteQuery({
    queryKey: keys.newsComments(gid),
    queryFn: ({ pageParam, signal }) => getComments(gid, pageParam, signal),
    initialPageParam: 0,
    getNextPageParam: (last) => last.next_before ?? undefined,
    staleTime: 30_000,
  })
}

export function useAddComment(gid: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (text: string) => postComment(gid, text),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.newsComments(gid) }),
  })
}

export function useDeleteComment(gid: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => deleteComment(gid, id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.newsComments(gid) }),
  })
}

export function useSendFeedback() {
  return useMutation({ mutationFn: sendFeedback })
}
