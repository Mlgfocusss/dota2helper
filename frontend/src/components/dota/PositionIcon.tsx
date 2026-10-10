import type { ReactNode } from 'react'
import { cn } from '@/lib/cn'
import { positionColor, positionFull, type Position } from '@/lib/positions'

// Собственные иконки позиций (24×24, обводка 1.75)
const HAND = (
  <>
    <path d="M2.5 15.5 6 13.2c.9-.6 2-.7 3-.4l4.2 1.3a1.4 1.4 0 0 1-.8 2.7l-3-.8" />
    <path d="M5.5 20.5 8 18.6c.5-.4 1.1-.6 1.8-.6h4.6c.9 0 1.7-.3 2.3-.9l3.9-3.6a1.45 1.45 0 0 0-2-2.1l-3.3 3" />
  </>
)

const PATHS: Record<Position, ReactNode> = {
  // меч
  1: (
    <>
      <path d="M7 15 16.5 5.5 20 4l-1.5 3.5L9 17z" fill="currentColor" fillOpacity={0.22} />
      <path d="m5 13 6 6" />
      <path d="M8 16 4 20" />
    </>
  ),
  // прицел
  2: (
    <>
      <circle cx="12" cy="12" r="7.5" fill="currentColor" fillOpacity={0.12} />
      <path d="M12 2.5v5M12 16.5v5M2.5 12h5M16.5 12h5" />
      <circle cx="12" cy="12" r="1.6" fill="currentColor" stroke="none" />
    </>
  ),
  // щит с молнией
  3: (
    <>
      <path
        d="M12 3 19 5.8v5.5c0 4.6-2.9 8-7 9.7-4.1-1.7-7-5.1-7-9.7V5.8z"
        fill="currentColor"
        fillOpacity={0.14}
      />
      <path d="M13 7 9.5 12.5h3L11 17l3.5-5.5h-3z" fill="currentColor" stroke="none" />
    </>
  ),
  // ладонь с огоньком
  4: (
    <>
      <path
        d="M12 2.5c1.7 1.9 3 3.3 3 5.1a3 3 0 0 1-6 0c0-1.1.5-2 1.3-2.9.2.9.7 1.5 1.4 1.7 0-1.4-.1-2.6.3-3.9z"
        fill="currentColor"
        fillOpacity={0.25}
      />
      {HAND}
    </>
  ),
  // ладонь с искрами
  5: (
    <>
      <path
        d="M12 2.6 12.9 5l2.4.9-2.4.9-.9 2.4-.9-2.4-2.4-.9 2.4-.9z"
        fill="currentColor"
        stroke="none"
      />
      <path d="m17.6 2.2.4 1 1 .4-1 .4-.4 1-.4-1-1-.4 1-.4z" fill="currentColor" stroke="none" />
      <path
        d="m6.6 8.4.35.85.85.35-.85.35-.35.85-.35-.85-.85-.35.85-.35z"
        fill="currentColor"
        stroke="none"
      />
      {HAND}
    </>
  ),
}

type PositionIconProps = {
  position: Position
  size?: number
  /** false — цвет наследуется от текста (например, белая иконка на цветном фоне) */
  colored?: boolean
  /** Подпись для скринридера; без неё иконка декоративная */
  title?: string
  className?: string
}

export function PositionIcon({
  position,
  size = 24,
  colored = true,
  title,
  className,
}: PositionIconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      role={title ? 'img' : undefined}
      aria-label={title}
      aria-hidden={title ? undefined : true}
      className={cn('shrink-0', className)}
      style={colored ? { color: positionColor(position) } : undefined}
    >
      {title && <title>{title}</title>}
      {PATHS[position]}
    </svg>
  )
}

/** Иконка с полным названием позиции в подсказке скринридера. */
export function PositionIconLabeled(props: Omit<PositionIconProps, 'title'>) {
  return <PositionIcon {...props} title={positionFull(props.position)} />
}
