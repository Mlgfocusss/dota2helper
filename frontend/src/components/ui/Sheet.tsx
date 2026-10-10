import { AnimatePresence, motion } from 'framer-motion'
import { X } from 'lucide-react'
import { useEffect, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { useIsDesktop } from '@/lib/hooks'
import { IconButton } from './Button'

type SheetProps = {
  open: boolean
  onClose: () => void
  title?: string
  children: ReactNode
  /** Размер модалки на десктопе; на мобиле — bottom‑sheet на 90% высоты */
  width?: number
  height?: number
  className?: string
}

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

/** Модальное окно по центру на десктопе и bottom‑sheet на мобиле. Esc закрывает. */
export function Sheet({
  open,
  onClose,
  title,
  children,
  width = 560,
  height,
  className,
}: SheetProps) {
  const isDesktop = useIsDesktop()
  const panelRef = useRef<HTMLDivElement>(null)
  const onCloseRef = useRef(onClose)

  useEffect(() => {
    onCloseRef.current = onClose
  })

  useEffect(() => {
    if (!open) return
    const previouslyFocused = document.activeElement as HTMLElement | null
    const prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    // фокус внутрь: первое поле ввода (например, поиск в HeroPicker), иначе сама панель
    const focusTimer = window.setTimeout(() => {
      const panel = panelRef.current
      if (!panel) return
      const target = panel.querySelector<HTMLElement>('input, textarea') ?? panel
      target.focus()
    }, 0)

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        onCloseRef.current()
        return
      }
      if (e.key !== 'Tab' || !panelRef.current) return
      // фокус не уходит за пределы окна
      const items = Array.from(panelRef.current.querySelectorAll<HTMLElement>(FOCUSABLE))
      if (items.length === 0) return
      const first = items[0]
      const last = items[items.length - 1]
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', onKeyDown)

    return () => {
      window.clearTimeout(focusTimer)
      document.removeEventListener('keydown', onKeyDown)
      document.body.style.overflow = prevOverflow
      previouslyFocused?.focus?.()
    }
  }, [open])

  return createPortal(
    <AnimatePresence>
      {open && (
        <div className="fixed inset-0 z-50">
          <motion.div
            className="absolute inset-0 bg-black/60 backdrop-blur-[2px]"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15 }}
            onClick={onClose}
          />
          <motion.div
            ref={panelRef}
            role="dialog"
            aria-modal="true"
            aria-label={title}
            tabIndex={-1}
            className={cn(
              'absolute flex flex-col border-line bg-surface shadow-2xl outline-none',
              isDesktop
                ? 'left-1/2 top-1/2 max-h-[calc(100dvh-48px)] rounded-[20px] border'
                : 'inset-x-0 bottom-0 h-[90dvh] rounded-t-[20px] border-t pb-[env(safe-area-inset-bottom)]',
              className,
            )}
            style={
              isDesktop
                ? {
                    width: `min(${width}px, calc(100vw - 48px))`,
                    height: height ? `min(${height}px, calc(100dvh - 48px))` : undefined,
                    x: '-50%',
                    y: '-50%',
                  }
                : undefined
            }
            initial={isDesktop ? { opacity: 0, scale: 0.97 } : { y: '100%' }}
            animate={isDesktop ? { opacity: 1, scale: 1 } : { y: 0 }}
            exit={isDesktop ? { opacity: 0, scale: 0.97 } : { y: '100%' }}
            transition={
              isDesktop ? { duration: 0.16 } : { type: 'spring', stiffness: 420, damping: 40 }
            }
          >
            {!isDesktop && (
              <div aria-hidden className="mx-auto mt-2 h-1 w-10 shrink-0 rounded-full bg-line" />
            )}
            {title && (
              <header className="flex shrink-0 items-center gap-3 px-5 pb-2 pt-4">
                <h2 className="text-card-title min-w-0 flex-1 truncate">{title}</h2>
                <IconButton aria-label={t.common.close} size="icon-sm" onClick={onClose}>
                  <X size={20} />
                </IconButton>
              </header>
            )}
            <div className="min-h-0 flex-1 overflow-y-auto px-5 pb-5 pt-2">{children}</div>
          </motion.div>
        </div>
      )}
    </AnimatePresence>,
    document.body,
  )
}
