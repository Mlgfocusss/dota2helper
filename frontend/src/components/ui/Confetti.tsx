import { motion, useReducedMotion } from 'framer-motion'
import { useState } from 'react'

const COLORS = [
  'var(--accent)',
  'var(--accent-2)',
  'var(--win)',
  'var(--loss)',
  '#E8A030',
  '#EDEDEF',
]
const COUNT = 46

type Piece = {
  x: number
  drift: number
  delay: number
  size: number
  color: string
  spin: number
  fall: number
}

function makePieces(): Piece[] {
  return Array.from({ length: COUNT }, (_, i) => ({
    x: Math.random() * 100,
    drift: (Math.random() - 0.5) * 160,
    delay: Math.random() * 0.35,
    size: 6 + Math.random() * 7,
    color: COLORS[i % COLORS.length],
    spin: (Math.random() - 0.5) * 900,
    fall: 520 + Math.random() * 280,
  }))
}

/** Конфетти для «Новый рекорд!»: падает один раз; при prefers-reduced-motion не показывается. */
export function Confetti() {
  const reduce = useReducedMotion()
  // случайные значения считаем один раз при появлении
  const [pieces] = useState(makePieces)
  if (reduce) return null

  return (
    <div
      aria-hidden
      className="pointer-events-none absolute inset-x-0 top-0 z-20 h-0 overflow-visible"
    >
      {pieces.map((p, i) => (
        <motion.span
          key={i}
          className="absolute block rounded-[2px]"
          style={{
            left: `${p.x}%`,
            top: -12,
            width: p.size,
            height: p.size * 0.6,
            background: p.color,
          }}
          initial={{ y: 0, x: 0, opacity: 1, rotate: 0 }}
          animate={{ y: p.fall, x: p.drift, opacity: [1, 1, 0], rotate: p.spin }}
          transition={{ duration: 1.9, delay: p.delay, ease: 'easeIn' }}
        />
      ))}
    </div>
  )
}
