export type TabId = 'home' | 'play' | 'coach' | 'reviews' | 'tools' | 'profile'

const TAB_ROOTS: Record<string, TabId> = {
  '/': 'home',
  '/play': 'play',
  '/coach': 'coach',
  '/reviews': 'reviews',
  '/tools': 'tools',
  '/profile': 'profile',
}

const TAB_BY_SEGMENT: Record<string, TabId> = {
  play: 'play',
  coach: 'coach',
  reviews: 'reviews',
  tools: 'tools',
  profile: 'profile',
}

// Родители там, где «отрезать последний сегмент» даёт несуществующий маршрут
const PARENT_OVERRIDES: [RegExp, string][] = [
  [/^\/play\/quiz\/[^/]+$/, '/play'],
  [/^\/coach\/matches\/[^/]+$/, '/coach'],
  [/^\/coach\/(deep|analysis\/[^/]+)$/, '/reviews'],
  [/^\/dev\/[^/]+$/, '/'],
]

export function normalizePath(pathname: string): string {
  const trimmed = pathname.replace(/\/+$/, '')
  return trimmed === '' ? '/' : trimmed
}

/** Вкладка нижней навигации, к которой относится маршрут (/coach/deep → «Тренер»). */
export function tabForPath(pathname: string): TabId {
  const path = normalizePath(pathname)
  // запуск и результат разбора живут во вкладке «Разборы»
  if (/^\/coach\/(deep|analysis\/)/.test(path)) return 'reviews'
  const segment = path.split('/')[1] ?? ''
  return TAB_BY_SEGMENT[segment] ?? 'home'
}

/** Корневые страницы вкладок — на них в шапке нет кнопки «назад». */
export function isTabRoot(pathname: string): boolean {
  return normalizePath(pathname) in TAB_ROOTS
}

/** Куда ведёт «назад», если в истории браузера нет предыдущей страницы. */
export function parentPath(pathname: string): string {
  const path = normalizePath(pathname)
  for (const [re, parent] of PARENT_OVERRIDES) {
    if (re.test(path)) return parent
  }
  const idx = path.lastIndexOf('/')
  return idx <= 0 ? '/' : path.slice(0, idx)
}
