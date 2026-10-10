import { api } from '../client'
import type { CoachSummary, MatchDetail, MatchesPage, ResolvedProfile } from '../types'

export const resolveProfile = (input: string) =>
  api<ResolvedProfile>('/profile/resolve', { method: 'POST', body: { input } })

export const setTrackedProfile = (accountId: number) =>
  api<{ account_id: number }>('/profile/tracked', {
    method: 'PUT',
    body: { account_id: accountId },
  })

export const getCoachSummary = (accountId: number, signal?: AbortSignal) =>
  api<CoachSummary>(`/coach/${accountId}/summary`, { signal })

export const getCoachMatches = (
  accountId: number,
  filter: string,
  cursor: string | null,
  signal?: AbortSignal,
) =>
  api<MatchesPage>(`/coach/${accountId}/matches`, {
    query: { filter, cursor: cursor ?? undefined },
    signal,
  })

export const refreshCoach = (accountId: number) =>
  api<void>(`/coach/${accountId}/refresh`, { method: 'POST' })

export const getMatch = (matchId: number, accountId: number | null, signal?: AbortSignal) =>
  api<MatchDetail>(`/matches/${matchId}`, { query: { account_id: accountId }, signal })
