import { api } from '../client'

export const logout = () => api<void>('/auth/logout', { method: 'POST' })
