import { motion } from 'framer-motion'
import { useId, useRef, type KeyboardEvent } from 'react'
import { cn } from '@/lib/cn'

export type TabItem<T extends string> = { value: T; label: string; count?: number }

type TabsProps<T extends string> = {
  items: TabItem<T>[]
  value: T
  onChange: (value: T) => void
  'aria-label'?: string
  className?: string
}

/** Текстовые табы с подчёркиванием активного и необязательным счётчиком. */
export function Tabs<T extends string>({
  items,
  value,
  onChange,
  className,
  ...aria
}: TabsProps<T>) {
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
      className={cn('flex gap-6 border-b border-line', className)}
    >
      {items.map((item, i) => {
        const selected = item.value === value
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
              'relative -mb-px pb-3 pt-1 text-[15px] font-medium transition-colors',
              selected ? 'text-fg' : 'text-muted hover:text-fg',
            )}
          >
            {item.label}
            {item.count !== undefined && (
              <span className={cn('ml-1.5 tabular-nums', selected ? 'text-muted' : 'text-dim')}>
                {item.count}
              </span>
            )}
            {selected && (
              <motion.span
                layoutId={`tabs-${id}`}
                className="absolute inset-x-0 -bottom-px h-0.5 rounded-full bg-fg"
                transition={{ type: 'spring', stiffness: 500, damping: 40 }}
              />
            )}
          </button>
        )
      })}
    </div>
  )
}
