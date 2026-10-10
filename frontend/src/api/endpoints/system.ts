import { API_BASE } from '@/lib/config'
import { api } from '../client'
import type { AppConfig, Me } from '../types'

export const getMe = (signal?: AbortSignal) => api<Me>('/me', { signal })

export const getAppConfig = (signal?: AbortSignal) => api<AppConfig>('/config', { signal })

/** Адрес входа через Steam: переход полный, в той же вкладке. */
export function steamLoginUrl(returnTo: string): string {
  return `${API_BASE}/auth/steam/login?return_to=${encodeURIComponent(returnTo)}`
}
