import { motion } from 'framer-motion'
import type { LucideIcon } from 'lucide-react'
import { useId, useRef, type KeyboardEvent } from 'react'
import { cn } from '@/lib/cn'

export type SegmentedItem<T extends string> = { value: T; label: string; icon?: LucideIcon }

type SegmentedProps<T extends string> = {
  items: SegmentedItem<T>[]
  value: T
  onChange: (value: T) => void
  'aria-label'?: string
  className?: string
}

/** Переключатель режимов на всю ширину (например, «Анализ / Тренировка»). */
export function Segmented<T extends string>({
  items,
  value,
  onChange,
  className,
  ...aria
}: SegmentedProps<T>) {
  const id = useId()
  const refs = useRef<(HTMLButtonElement | null)[]>([])

  const onKeyDown = (e: KeyboardEvent, index: number) => {
    const step = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0
    if (!step) return
    e.preventDefault()
    const next = (index + step + items.length) % items.length
    onChange(items[next].value)
    refs.current[next]?.focus()
  }

  return (
    <div
      role="tablist"
      aria-label={aria['aria-label']}
      className={cn(
        'grid auto-cols-fr grid-flow-col gap-1 rounded-[14px] border border-line bg-surface p-1',
        className,
      )}
    >
      {items.map((item, i) => {
        const selected = item.value === value
        const Icon = item.icon
        return (
          <button
            key={item.value}
            ref={(el) => {
              refs.current[i] = el
            }}
            type="button"
            role="tab"
            aria-selected={selected}
            tabIndex={selected ? 0 : -1}
            onClick={() => onChange(item.value)}
            onKeyDown={(e) => onKeyDown(e, i)}
            className={cn(
              'relative isolate flex h-10 items-center justify-center gap-2 rounded-[10px] text-[15px] font-medium transition-colors',
              selected ? 'text-fg' : 'text-muted hover:text-fg',
            )}
          >
            {selected && (
              <motion.span
                layoutId={`segmented-${id}`}
                className="absolute inset-0 -z-10 rounded-[10px] border border-line bg-surface-2"
                transition={{ type: 'spring', stiffness: 500, damping: 40 }}
              />
            )}
            {Icon && <Icon size={18} aria-hidden />}
            {item.label}
          </button>
        )
      })}
    </div>
  )
}
