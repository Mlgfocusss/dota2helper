import { AnimatePresence, motion } from 'framer-motion'
import { useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { cn } from '@/lib/cn'

type TooltipProps = {
  content: ReactNode
  children: ReactNode
  side?: 'top' | 'bottom'
  className?: string
}

const DELAY_MS = 300

/** Подсказка по наведению или фокусу с задержкой 300 мс. */
export function Tooltip({ content, children, side = 'top', className }: TooltipProps) {
  const [open, setOpen] = useState(false)
  const timer = useRef<number | undefined>(undefined)
  const id = useId()

  useEffect(() => () => window.clearTimeout(timer.current), [])

  const show = () => {
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setOpen(true), DELAY_MS)
  }
  const hide = () => {
    window.clearTimeout(timer.current)
    setOpen(false)
  }

  return (
    <span
      className={cn('relative inline-flex', className)}
      onMouseEnter={show}
      onMouseLeave={hide}
      onFocus={show}
      onBlur={hide}
      aria-describedby={open ? id : undefined}
    >
      {children}
      <AnimatePresence>
        {open && (
          <motion.span
            id={id}
            role="tooltip"
            initial={{ opacity: 0, y: side === 'top' ? 4 : -4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.12 }}
            className={cn(
              'pointer-events-none absolute left-1/2 z-50 -translate-x-1/2 whitespace-nowrap rounded-[10px] border border-line bg-surface-2 px-2.5 py-1.5 text-[13px] font-medium text-fg shadow-lg',
              side === 'top' ? 'bottom-full mb-2' : 'top-full mt-2',
            )}
          >
            {content}
          </motion.span>
        )}
      </AnimatePresence>
    </span>
  )
}
