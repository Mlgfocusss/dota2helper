import { useCallback, useEffect, useSyncExternalStore } from 'react'
import { APP_NAME } from './config'

/** Заголовок вкладки: «Тренер — D2Helper»; без title — только название. */
export function useDocumentTitle(title?: string) {
  useEffect(() => {
    document.title = title ? `${title} — ${APP_NAME}` : APP_NAME
  }, [title])
}

export function useMediaQuery(query: string): boolean {
  const subscribe = useCallback(
    (onChange: () => void) => {
      const mql = window.matchMedia(query)
      mql.addEventListener('change', onChange)
      return () => mql.removeEventListener('change', onChange)
    },
    [query],
  )
  return useSyncExternalStore(
    subscribe,
    () => window.matchMedia(query).matches,
    () => false,
  )
}

/** Десктоп‑раскладка (≥ 768px). */
export function useIsDesktop(): boolean {
  return useMediaQuery('(min-width: 768px)')
}

function subscribeScroll(onChange: () => void) {
  window.addEventListener('scroll', onChange, { passive: true })
  return () => window.removeEventListener('scroll', onChange)
}

/** true, если страница прокручена дальше threshold пикселей. */
export function useScrolled(threshold = 0): boolean {
  return useSyncExternalStore(
    subscribeScroll,
    () => window.scrollY > threshold,
    () => false,
  )
}
