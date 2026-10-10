import { api } from '../client'
import type {
  PartyCard,
  PartyCardInput,
  PartyLooking,
  PartyMe,
  PartyPage,
  PartyReportReason,
} from '../types'

export const getPartyMe = (signal?: AbortSignal) => api<PartyMe>('/party/me', { signal })

export const putPartyMe = (input: PartyCardInput) =>
  api<PartyCard>('/party/me', { method: 'PUT', body: input })

export const setPartyActive = (isActive: boolean) =>
  api<PartyCard>('/party/me', { method: 'PATCH', body: { is_active: isActive } })

export const deletePartyMe = () => api<void>('/party/me', { method: 'DELETE' })

export const bumpPartyMe = () => api<PartyCard>('/party/me/bump', { method: 'POST' })

export type PartySearchParams = {
  looking_for: PartyLooking
  rank_min?: number
  rank_max?: number
  /** «1,3,5» */
  positions?: string
  modes?: string
  goals?: string
  mic?: 1
  discord?: 1
  cursor?: string | null
}

export const searchParty = (params: PartySearchParams, signal?: AbortSignal) =>
  api<PartyPage>('/party', { query: params, signal })

export const reportParty = (userId: number, reason: PartyReportReason) =>
  api<void>(`/party/${userId}/report`, { method: 'POST', body: { reason } })
