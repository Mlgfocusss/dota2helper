import { motion, useReducedMotion } from 'framer-motion'
import type { ReactNode } from 'react'

type WinrateRingProps = {
  /** Доля побед, 0–1 */
  value: number
  size?: number
  stroke?: number
  /** Содержимое в центре кольца */
  children?: ReactNode
  'aria-label'?: string
}

/** Кольцо побед: дуга --win на подложке, заполняется при появлении. */
export function WinrateRing({
  value,
  size = 140,
  stroke = 12,
  children,
  ...aria
}: WinrateRingProps) {
  const reduce = useReducedMotion()
  const radius = (size - stroke) / 2
  const fraction = Math.min(1, Math.max(0, value))

  return (
    <div
      className="relative inline-flex shrink-0 items-center justify-center"
      style={{ width: size, height: size }}
      role="img"
      aria-label={aria['aria-label']}
    >
      <svg width={size} height={size} className="-rotate-90" aria-hidden>
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="var(--surface-2)"
          strokeWidth={stroke}
        />
        <motion.circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="var(--win)"
          strokeWidth={stroke}
          strokeLinecap="round"
          initial={{ pathLength: reduce ? fraction : 0 }}
          animate={{ pathLength: fraction }}
          transition={{ duration: reduce ? 0 : 0.9, ease: 'easeOut' }}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">{children}</div>
    </div>
  )
}
