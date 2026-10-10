import { motion } from 'framer-motion'
import {
  Gamepad2,
  House,
  Layers,
  SlidersHorizontal,
  TrendingUp,
  User,
  type LucideIcon,
} from 'lucide-react'
import { Link, useLocation } from 'react-router'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { normalizePath, tabForPath, type TabId } from '@/lib/routes'

type Tab = {
  id: TabId
  to: string
  label: string
  /** Подпись для экранов уже 440px, где полная не помещается в пятую часть панели */
  short?: string
  icon: LucideIcon
}

const TABS: Tab[] = [
  { id: 'home', to: '/', label: t.nav.home, icon: House },
  { id: 'play', to: '/play', label: t.nav.play, icon: Gamepad2 },
  { id: 'coach', to: '/coach', label: t.nav.coach, icon: TrendingUp },
  { id: 'reviews', to: '/reviews', label: t.nav.reviews, icon: Layers },
  {
    id: 'tools',
    to: '/tools',
    label: t.nav.tools,
    short: t.nav.toolsShort,
    icon: SlidersHorizontal,
  },
  { id: 'profile', to: '/profile', label: t.nav.profile, icon: User },
]

/** Плавающая нижняя навигация: 1200px по центру, на узких экранах — с отступами 12px. */
export function BottomNav() {
  const { pathname } = useLocation()
  const active = tabForPath(pathname)

  return (
    <nav
      aria-label={t.nav.label}
      className="fixed inset-x-3 z-40 mx-auto h-[72px] max-w-[1200px] rounded-[20px] border border-line bg-surface/92 shadow-[0_8px_32px_rgba(0,0,0,0.45)] backdrop-blur-xl"
      style={{ bottom: 'calc(var(--nav-offset) + env(safe-area-inset-bottom))' }}
    >
      <ul className="grid h-full grid-cols-6 gap-0.5 p-1.5 sm:gap-1">
        {TABS.map((tab) => {
          const isActive = tab.id === active
          const Icon = tab.icon
          return (
            <li key={tab.id} className="min-w-0">
              <Link
                to={tab.to}
                aria-label={tab.label}
                aria-current={isActive ? 'page' : undefined}
                onClick={() => {
                  // повторный клик по открытой вкладке — наверх страницы
                  if (normalizePath(pathname) === tab.to)
                    window.scrollTo({ top: 0, behavior: 'smooth' })
                }}
                className={cn(
                  'relative isolate flex h-full flex-col items-center justify-center gap-1 rounded-[14px] font-medium transition-colors',
                  'text-[11px] leading-4 max-[399px]:text-[10px] tracking-[-0.01em] sm:text-[13px] sm:tracking-normal',
                  isActive ? 'text-fg' : 'text-muted hover:text-fg',
                )}
              >
                {isActive && (
                  <motion.span
                    layoutId="bottom-nav-active"
                    className="absolute inset-0 -z-10 rounded-[14px] bg-surface-2"
                    transition={{ type: 'spring', stiffness: 500, damping: 40 }}
                  />
                )}
                <Icon size={22} strokeWidth={isActive ? 2.1 : 1.8} aria-hidden />
                <span className={cn('max-w-full truncate', tab.short && 'max-[439px]:hidden')}>
                  {tab.label}
                </span>
                {tab.short && (
                  <span className="max-w-full truncate min-[440px]:hidden">{tab.short}</span>
                )}
              </Link>
            </li>
          )
        })}
      </ul>
    </nav>
  )
}
