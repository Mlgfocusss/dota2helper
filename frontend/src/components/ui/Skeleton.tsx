import type { CSSProperties } from 'react'
import { cn } from '@/lib/cn'

type SkeletonProps = {
  className?: string
  style?: CSSProperties
  /** Скругление, по умолчанию 10px; 'full' — круг */
  radius?: number | 'full'
}

/** Заглушка на время загрузки с бегущим бликом. */
export function Skeleton({ className, style, radius = 10 }: SkeletonProps) {
  return (
    <div
      aria-hidden
      className={cn(
        'relative overflow-hidden bg-surface-2',
        'before:absolute before:inset-0 before:-translate-x-full before:animate-shimmer',
        'before:bg-linear-to-r before:from-transparent before:via-white/5 before:to-transparent',
        className,
      )}
      style={{ borderRadius: radius === 'full' ? 9999 : radius, ...style }}
    />
  )
}
