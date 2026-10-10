import type { ReactNode } from 'react'
import { cn } from '@/lib/cn'

type ChipTone = 'default' | 'accent' | 'win' | 'loss' | 'outline'

const tones: Record<ChipTone, string> = {
  default: 'bg-surface-2 text-fg',
  accent: 'bg-accent-2/15 text-accent-2',
  win: 'bg-win-strong/12 text-win',
  loss: 'bg-loss-strong/12 text-loss',
  outline: 'border border-line text-muted',
}

const sizes = {
  sm: 'h-6 px-2.5 text-[12px]',
  md: 'h-8 px-3.5 text-[14px]',
}

type ChipProps = {
  children: ReactNode
  tone?: ChipTone
  size?: keyof typeof sizes
  icon?: ReactNode
  /** Для чипов‑переключателей: выбранный подсвечивается рамкой --accent-2 */
  selected?: boolean
  onClick?: () => void
  className?: string
}

/** Чип‑пилюля. С onClick становится кнопкой‑переключателем. */
export function Chip({
  children,
  tone = 'default',
  size = 'md',
  icon,
  selected,
  onClick,
  className,
}: ChipProps) {
  const classes = cn(
    'inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full font-medium transition-colors',
    sizes[size],
    tones[tone],
    selected && 'bg-accent-2/15 text-fg ring-1 ring-accent-2',
    className,
  )
  if (!onClick) {
    return (
      <span className={classes}>
        {icon}
        {children}
      </span>
    )
  }
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={onClick}
      className={cn(classes, !selected && 'hover:text-fg')}
    >
      {icon}
      {children}
    </button>
  )
}
