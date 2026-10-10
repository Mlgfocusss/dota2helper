import { api } from '../client'
import type { Hero, Patch } from '../types'

export const getHeroes = (signal?: AbortSignal) => api<Hero[]>('/heroes', { signal })

export const getPatch = (signal?: AbortSignal) => api<Patch>('/patch', { signal })
