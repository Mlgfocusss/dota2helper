import { PositionIcon } from '@/components/dota/PositionIcon'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { toggleIn } from '@/lib/party'
import { POSITIONS, positionFull, positionShort, type Position } from '@/lib/positions'

type PositionTogglesProps = {
  value: Position[]
  onChange: (value: Position[]) => void
  /** tile — крупные кнопки с подписью (форма), icon — компактные (фильтры) */
  variant?: 'tile' | 'icon'
  invalid?: boolean
  'aria-label'?: string
  className?: string
}

/** Мультивыбор позиций 1–5. */
export function PositionToggles({
  value,
  onChange,
  variant = 'tile',
  invalid,
  className,
  ...aria
}: PositionTogglesProps) {
  return (
    <div
      role="group"
      aria-label={aria['aria-label'] ?? t.party.positions}
      className={cn(variant === 'tile' ? 'grid grid-cols-5 gap-2' : 'flex gap-1.5', className)}
    >
      {POSITIONS.map((p) => {
        const selected = value.includes(p)
        return (
          <button
            key={p}
            type="button"
            aria-pressed={selected}
            aria-label={positionFull(p)}
            title={positionFull(p)}
            onClick={() => onChange(toggleIn(value, p, POSITIONS))}
            className={cn(
              'flex items-center justify-center rounded-[12px] border transition-colors',
              variant === 'tile' ? 'h-[72px] flex-col gap-1.5' : 'size-11',
              selected
                ? 'border-accent-2 bg-accent-2/15'
                : cn('bg-bg hover:bg-surface-2', invalid ? 'border-loss' : 'border-line'),
            )}
          >
            <PositionIcon position={p} size={variant === 'tile' ? 26 : 22} />
            {variant === 'tile' && (
              <span className={cn('text-[12px]', selected ? 'text-fg' : 'text-muted')}>
                {positionShort(p)}
              </span>
            )}
          </button>
        )
      })}
    </div>
  )
}
