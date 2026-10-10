import { api } from '../client'
import type { ProfileStats } from '../types'

export const getProfileStats = (signal?: AbortSignal) =>
  api<ProfileStats>('/profile/stats', { signal })
