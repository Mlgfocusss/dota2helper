import { AnimatePresence, motion } from 'framer-motion'
import { ChevronDown, Minus, Plus } from 'lucide-react'
import { useId, useState, type ReactNode } from 'react'
import { cn } from '@/lib/cn'

type AccordionProps = {
  title: ReactNode
  children: ReactNode
  /** Плюс/минус (как «Уже включена?») или шеврон */
  icon?: 'plus' | 'chevron'
  defaultOpen?: boolean
  className?: string
  /** Классы кнопки‑заголовка */
  triggerClassName?: string
}

/** Раскрывающийся блок с плавной анимацией высоты. */
export function Accordion({
  title,
  children,
  icon = 'chevron',
  defaultOpen = false,
  className,
  triggerClassName,
}: AccordionProps) {
  const [open, setOpen] = useState(defaultOpen)
  const contentId = useId()

  return (
    <div className={className}>
      <button
        type="button"
        aria-expanded={open}
        aria-controls={contentId}
        onClick={() => setOpen((v) => !v)}
        className={cn(
          'flex w-full items-center gap-3 py-3 text-left font-medium transition-colors hover:text-fg',
          triggerClassName,
        )}
      >
        <span className="min-w-0 flex-1">{title}</span>
        {icon === 'plus' ? (
          open ? (
            <Minus size={18} className="text-muted" aria-hidden />
          ) : (
            <Plus size={18} className="text-muted" aria-hidden />
          )
        ) : (
          <ChevronDown
            size={18}
            className={cn('text-muted transition-transform duration-200', open && 'rotate-180')}
            aria-hidden
          />
        )}
      </button>
      <AnimatePresence initial={false}>
        {open && (
          <motion.div
            id={contentId}
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.2, ease: 'easeOut' }}
            className="overflow-hidden"
          >
            <div className="pb-3 text-[15px] text-muted">{children}</div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}
