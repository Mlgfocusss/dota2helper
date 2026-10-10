import { AnimatePresence, motion } from 'framer-motion'
import { CircleAlert, CircleCheck, Info, X } from 'lucide-react'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { useToastStore, type ToastKind } from '@/features/toast/store'

const icons: Record<ToastKind, typeof Info> = {
  success: CircleCheck,
  error: CircleAlert,
  info: Info,
}

const iconColors: Record<ToastKind, string> = {
  success: 'text-win-strong',
  error: 'text-loss-strong',
  info: 'text-accent-2',
}

/** Тосты над нижней навигацией. */
export function Toaster() {
  const toasts = useToastStore((s) => s.toasts)
  const dismiss = useToastStore((s) => s.dismiss)

  return (
    <div
      role="status"
      aria-live="polite"
      className="pointer-events-none fixed inset-x-0 z-[60] flex flex-col items-center gap-2 px-4"
      style={{
        bottom: 'calc(var(--nav-offset) + var(--nav-h) + 12px + env(safe-area-inset-bottom))',
      }}
    >
      <AnimatePresence initial={false}>
        {toasts.map((item) => {
          const Icon = icons[item.kind]
          return (
            <motion.div
              key={item.id}
              layout
              initial={{ opacity: 0, y: 12, scale: 0.98 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: 8, scale: 0.98 }}
              transition={{ duration: 0.18 }}
              className="pointer-events-auto flex w-full max-w-[440px] items-center gap-3 rounded-[14px] border border-line bg-surface-2 py-3 pl-4 pr-2 shadow-2xl"
            >
              <Icon size={20} className={cn('shrink-0', iconColors[item.kind])} aria-hidden />
              <p className="min-w-0 flex-1 text-[15px]">{item.message}</p>
              <button
                type="button"
                aria-label={t.common.close}
                onClick={() => dismiss(item.id)}
                className="flex size-8 shrink-0 items-center justify-center rounded-[10px] text-muted transition-colors hover:bg-white/5 hover:text-fg"
              >
                <X size={16} aria-hidden />
              </button>
            </motion.div>
          )
        })}
      </AnimatePresence>
    </div>
  )
}
