import type { LucideIcon } from 'lucide-react'
import { useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { cn } from '@/lib/cn'

export type MenuItem = {
  label: string
  icon?: LucideIcon
  /** Внешняя ссылка (открывается в новой вкладке) вместо действия */
  href?: string
  onSelect?: () => void
  tone?: 'danger'
}

type MenuProps = {
  items: MenuItem[]
  /** Кнопка‑триггер; получает признак открытости */
  trigger: (open: boolean) => ReactNode
  /** Подпись триггера для скринридера */
  label: string
  align?: 'left' | 'right'
  className?: string
  triggerClassName?: string
}

const itemClass =
  'flex w-full items-center gap-3 rounded-[10px] px-3 py-2.5 text-left text-[15px] transition-colors ' +
  'hover:bg-surface-2 focus-visible:bg-surface-2 focus-visible:outline-none'

/** Выпадающее меню действий: закрывается по Esc, клику снаружи и выбору пункта. */
export function Menu({
  items,
  trigger,
  label,
  align = 'right',
  className,
  triggerClassName,
}: MenuProps) {
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const id = useId()

  useEffect(() => {
    if (!open) return
    const onPointerDown = (e: PointerEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpen(false)
        rootRef.current?.querySelector<HTMLElement>('button[aria-haspopup]')?.focus()
      }
    }
    document.addEventListener('pointerdown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    // фокус на первый пункт, чтобы меню сразу управлялось с клавиатуры
    listRef.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus()
    return () => {
      document.removeEventListener('pointerdown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [open])

  const onListKeyDown = (e: React.KeyboardEvent) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
    e.preventDefault()
    const nodes = Array.from(
      listRef.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [],
    )
    const at = nodes.indexOf(document.activeElement as HTMLElement)
    const next = (at + (e.key === 'ArrowDown' ? 1 : -1) + nodes.length) % nodes.length
    nodes[next]?.focus()
  }

  return (
    <div ref={rootRef} className={cn('relative', className)}>
      <button
        type="button"
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? id : undefined}
        onClick={() => setOpen((v) => !v)}
        className={triggerClassName}
      >
        {trigger(open)}
      </button>
      {open && (
        <div
          ref={listRef}
          id={id}
          role="menu"
          aria-label={label}
          onKeyDown={onListKeyDown}
          className={cn(
            'absolute top-full z-30 mt-2 min-w-[220px] rounded-[14px] border border-line bg-surface p-1.5 shadow-2xl',
            align === 'right' ? 'right-0' : 'left-0',
          )}
        >
          {items.map((item) => {
            const Icon = item.icon
            const content = (
              <>
                {Icon && <Icon size={18} className="shrink-0" aria-hidden />}
                {item.label}
              </>
            )
            const cls = cn(itemClass, item.tone === 'danger' ? 'text-loss' : 'text-fg')
            return item.href ? (
              <a
                key={item.label}
                role="menuitem"
                href={item.href}
                target="_blank"
                rel="noreferrer"
                className={cls}
                onClick={() => setOpen(false)}
              >
                {content}
              </a>
            ) : (
              <button
                key={item.label}
                type="button"
                role="menuitem"
                className={cls}
                onClick={() => {
                  setOpen(false)
                  item.onSelect?.()
                }}
              >
                {content}
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}
