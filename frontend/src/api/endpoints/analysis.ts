import { api } from '../client'
import type { AnalysisListItem, AnalysisQuota, AnalysisView } from '../types'

export const createAnalysis = (matchId: number) =>
  api<AnalysisView>('/analyses', { method: 'POST', body: { match_id: matchId } })

export const getAnalysis = (id: string, signal?: AbortSignal) =>
  api<AnalysisView>(`/analyses/${id}`, { signal })

export const getAnalysisExample = (signal?: AbortSignal) =>
  api<AnalysisView>('/analyses/example', { signal })

export const getAnalysisQuota = (signal?: AbortSignal) =>
  api<AnalysisQuota>('/analyses/quota', { signal })

export const getAnalysisList = (signal?: AbortSignal) =>
  api<{ items: AnalysisListItem[] }>('/analyses', { signal })
