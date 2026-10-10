import { api } from '../client'
import type {
  CommentsPage,
  MetaPositions,
  NewsComment,
  NewsItem,
  NewsListItem,
  PatchNotes,
} from '../types'

export const getMetaPositions = (limit: number, signal?: AbortSignal) =>
  api<MetaPositions>('/meta/positions', { query: { limit }, signal })

export const getNews = (limit: number, signal?: AbortSignal) =>
  api<NewsListItem[]>('/news', { query: { limit }, signal })

export const getNewsItem = (gid: string, signal?: AbortSignal) =>
  api<NewsItem>(`/news/${encodeURIComponent(gid)}`, { signal })

export const getPatchNotes = (version: string, signal?: AbortSignal) =>
  api<PatchNotes>(`/patches/${encodeURIComponent(version)}`, { signal })

export const getComments = (gid: string, before: number, signal?: AbortSignal) =>
  api<CommentsPage>(`/news/${encodeURIComponent(gid)}/comments`, {
    query: { before: before || undefined },
    signal,
  })

export const postComment = (gid: string, text: string) =>
  api<NewsComment>(`/news/${encodeURIComponent(gid)}/comments`, { method: 'POST', body: { text } })

export const deleteComment = (gid: string, id: number) =>
  api<void>(`/news/${encodeURIComponent(gid)}/comments/${id}`, { method: 'DELETE' })

export const sendFeedback = (body: { text: string; contact?: string }) =>
  api<{ id: number }>('/feedback', { method: 'POST', body })
