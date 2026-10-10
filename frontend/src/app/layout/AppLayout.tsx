import { Outlet, ScrollRestoration, useNavigation } from 'react-router'
import { Toaster } from '@/components/ui/Toaster'
import { useAuthEffects } from '@/features/useAuthEffects'
import { t } from '@/i18n/ru'
import { AppHeader } from './AppHeader'
import { BottomNav } from './BottomNav'

export function AppLayout() {
  useAuthEffects()

  return (
    <div className="min-h-dvh">
      <a
        href="#main"
        className="fixed left-4 top-3 z-[70] -translate-y-20 rounded-[10px] bg-accent px-4 py-2 font-semibold text-on-accent transition-transform focus:translate-y-0"
      >
        {t.header.skipToContent}
      </a>
      <NavigationProgress />
      <AppHeader />
      {/* нижний отступ — под плавающую навигацию, чтобы она не перекрывала контент */}
      <main
        id="main"
        tabIndex={-1}
        className="outline-none"
        style={{ paddingBottom: 'calc(var(--content-bottom) + env(safe-area-inset-bottom))' }}
      >
        <Outlet />
      </main>
      <BottomNav />
      <Toaster />
      {/* наверх при переходе, восстановление позиции при «назад» */}
      <ScrollRestoration />
    </div>
  )
}

/** Тонкая полоса сверху, пока грузится код следующей страницы. */
function NavigationProgress() {
  const navigation = useNavigation()
  if (navigation.state === 'idle') return null
  return (
    <div className="pointer-events-none fixed inset-x-0 top-0 z-50 h-0.5">
      <div className="h-full animate-progress bg-accent-2" />
    </div>
  )
}
