import { motion } from 'framer-motion'
import { cn } from '@/lib/cn'

type ProgressBarProps = {
  /** 0..1 */
  value: number
  label?: string
  className?: string
  /** Цвет заполнения, по умолчанию --accent-2 */
  color?: string
}

export function ProgressBar({ value, label, className, color }: ProgressBarProps) {
  const pct = Math.min(1, Math.max(0, value)) * 100
  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(pct)}
      className={cn('h-1.5 overflow-hidden rounded-full bg-surface-2', className)}
    >
      <motion.div
        className="h-full rounded-full bg-accent-2"
        style={color ? { backgroundColor: color } : undefined}
        initial={false}
        animate={{ width: `${pct}%` }}
        transition={{ type: 'spring', stiffness: 200, damping: 30 }}
      />
    </div>
  )
}
