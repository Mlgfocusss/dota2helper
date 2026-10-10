import { RankMedal } from '@/components/dota/RankMedal'
import { cn } from '@/lib/cn'
import { RANK_MEDALS, rankName } from '@/lib/ranks'

type RankPickerProps = {
  value: number
  onChange: (medal: number) => void
  'aria-labelledby'?: string
}

/** Выбор ранга: 9 плиток в ряд (Калибровка…Титан), на мобиле — горизонтальная прокрутка. */
export function RankPicker({ value, onChange, ...aria }: RankPickerProps) {
  return (
    <div
      role="radiogroup"
      aria-labelledby={aria['aria-labelledby']}
      className="-mx-1 flex gap-2 overflow-x-auto px-1 pb-1 lg:grid lg:grid-cols-9 lg:overflow-visible"
    >
      {RANK_MEDALS.map((medal) => {
        const selected = value === medal
        return (
          <button
            key={medal}
            type="button"
            role="radio"
            aria-checked={selected}
            onClick={() => onChange(medal)}
            className={cn(
              'flex w-[92px] shrink-0 flex-col items-center gap-1.5 rounded-[12px] border px-1 py-3 transition-colors lg:w-auto',
              selected ? 'border-accent-2 bg-accent-2/15' : 'border-line bg-bg hover:bg-surface-2',
            )}
          >
            <RankMedal rankTier={medal * 10} size={40} showStars={false} />
            <span className={cn('text-[12px]', selected ? 'text-fg' : 'text-muted')}>
              {rankName(medal)}
            </span>
          </button>
        )
      })}
    </div>
  )
}
