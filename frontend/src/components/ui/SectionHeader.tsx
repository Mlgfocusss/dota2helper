import type { ReactNode } from 'react'
import { cn } from '@/lib/cn'

type SectionHeaderProps = {
  /** Подпись‑капсом: «ТВОЙ ПРОФИЛЬ», «МЕТА ПАТЧА» */
  label: ReactNode
  /** Правая часть: ссылка, переключатель, кнопки */
  right?: ReactNode
  className?: string
  as?: 'h2' | 'h3'
}

export function SectionHeader({ label, right, className, as: Tag = 'h2' }: SectionHeaderProps) {
  return (
    <div className={cn('flex min-h-6 items-center gap-3', className)}>
      <Tag className="caps min-w-0 flex-1">{label}</Tag>
      {right}
    </div>
  )
}
