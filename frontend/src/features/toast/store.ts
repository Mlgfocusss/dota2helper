import { create } from 'zustand'

export type ToastKind = 'success' | 'error' | 'info'

export type Toast = { id: number; kind: ToastKind; message: string }

type ToastState = {
  toasts: Toast[]
  push: (kind: ToastKind, message: string) => void
  dismiss: (id: number) => void
}

const VISIBLE_MS = 4000
const MAX_TOASTS = 3
let nextId = 1

export const useToastStore = create<ToastState>((set, get) => ({
  toasts: [],
  push: (kind, message) => {
    // одинаковые ошибки от нескольких запросов показываем один раз
    if (get().toasts.some((item) => item.message === message)) return
    const id = nextId++
    set((s) => ({ toasts: [...s.toasts, { id, kind, message }].slice(-MAX_TOASTS) }))
    window.setTimeout(() => get().dismiss(id), VISIBLE_MS)
  },
  dismiss: (id) => set((s) => ({ toasts: s.toasts.filter((item) => item.id !== id) })),
}))

export const toast = {
  success: (message: string) => useToastStore.getState().push('success', message),
  error: (message: string) => useToastStore.getState().push('error', message),
  info: (message: string) => useToastStore.getState().push('info', message),
}
