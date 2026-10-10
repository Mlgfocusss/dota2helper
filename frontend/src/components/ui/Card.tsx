import type { HTMLAttributes } from 'react'
import { Link, type LinkProps } from 'react-router'
import { cn } from '@/lib/cn'

type Padding = 'none' | 'sm' | 'md' | 'lg'

const paddings: Record<Padding, string> = {
  none: '',
  sm: 'p-4',
  md: 'p-5',
  lg: 'p-6',
}

const base = 'block rounded-[16px] border border-line bg-surface'
const interactiveClass = 'cursor-pointer transition-colors hover:bg-surface-2'

type CardProps = HTMLAttributes<HTMLDivElement> & {
  padding?: Padding
  /** Подсветка при наведении */
  interactive?: boolean
}

export function Card({ padding = 'md', interactive, className, ...rest }: CardProps) {
  return (
    <div
      className={cn(base, paddings[padding], interactive && interactiveClass, className)}
      {...rest}
    />
  )
}

type CardLinkProps = LinkProps & { padding?: Padding }

/** Карточка‑ссылка: вся карточка кликабельна. */
export function CardLink({ padding = 'md', className, ...rest }: CardLinkProps) {
  return <Link className={cn(base, paddings[padding], interactiveClass, className)} {...rest} />
}
