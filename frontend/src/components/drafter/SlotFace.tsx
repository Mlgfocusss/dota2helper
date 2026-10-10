import { Ban } from 'lucide-react'
import type { Hero, Position } from '@/api/types'
import { HeroPortrait } from '@/components/dota/HeroPortrait'
import { PositionIcon } from '@/components/dota/PositionIcon'
import { cn } from '@/lib/cn'
import { formatSigned } from '@/lib/format'

export const SLOT_SIZE = 144

type SlotFaceProps = {
  hero?: Hero | null
  position: Position
  /** Цвет верхней границы: команда Света, Тьмы или «моя» команда */
  tone?: 'radiant' | 'dire' | 'mine' | 'none'
  /** Оценка героя в слоте (SPEC 4.2): плашка справа вверху */
  score?: number | null
  active?: boolean
  /** Слот не реагирует на наведение (враги в тренировке, итог) */
  passive?: boolean
  className?: string
}

const TONE: Record<NonNullable<SlotFaceProps['tone']>, string> = {
  radiant: 'var(--radiant)',
  dire: 'var(--dire)',
  mine: 'var(--accent-2)',
  none: 'var(--border)',
}

/**
 * Внешний вид слота героя: портрет на весь слот, значок позиции и оценка.
 * Сам не кликабелен — обёртку‑кнопку (или меню) ставит родитель, а наведение
 * берётся от родителя с классом group.
 */
export function SlotFace({
  hero,
  position,
  tone = 'none',
  score,
  active,
  passive,
  className,
}: SlotFaceProps) {
  const filled = Boolean(hero)
  return (
    <span
      className={cn(
        'relative block aspect-square w-full overflow-hidden rounded-[14px] border bg-surface-2 transition-[transform,box-shadow,border-color] duration-150',
        active ? 'border-accent-2 ring-2 ring-accent-2' : 'border-line',
        !passive && 'group-hover:scale-[1.03] group-hover:border-accent-2',
        'group-focus-visible:ring-2 group-focus-visible:ring-accent-2',
        className,
      )}
      style={{
        maxWidth: SLOT_SIZE,
        borderTopColor: active ? undefined : TONE[tone],
        borderTopWidth: 3,
      }}
    >
      {filled ? (
        <>
          <HeroPortrait
            hero={hero}
            width={SLOT_SIZE}
            height={SLOT_SIZE}
            radius={0}
            fluid
            className="block size-full"
          />
          <span className="absolute bottom-1.5 right-1.5 rounded-[8px] bg-black/65 p-1 backdrop-blur-[2px]">
            <PositionIcon position={position} size={16} />
          </span>
          {score !== undefined && score !== null && (
            <span
              className={cn(
                'absolute right-1 top-1 rounded-[8px] px-1 py-0.5 font-mono text-[10px] font-semibold tabular-nums backdrop-blur-[2px] sm:right-1.5 sm:top-1.5 sm:px-1.5 sm:text-[12px]',
                score >= 0
                  ? 'bg-black/75 text-win ring-1 ring-win-strong/60'
                  : 'bg-black/75 text-loss ring-1 ring-loss-strong/60',
              )}
            >
              {formatSigned(score)}
            </span>
          )}
        </>
      ) : (
        <span className="flex size-full items-center justify-center opacity-70">
          <PositionIcon position={position} size={40} />
        </span>
      )}
    </span>
  )
}

type BanFaceProps = { hero?: Hero | null; className?: string }

/** Маленький слот бана: пустой — ⊘, заполненный — чёрно‑белая иконка героя. */
export function BanFace({ hero, className }: BanFaceProps) {
  return (
    <span
      className={cn(
        'flex aspect-square w-full max-w-[64px] items-center justify-center overflow-hidden rounded-[12px] border border-line bg-surface-2 text-dim transition-[transform,border-color] group-hover:scale-[1.05] group-hover:border-accent-2',
        className,
      )}
    >
      {hero ? (
        <HeroPortrait
          hero={hero}
          width={64}
          height={64}
          radius={0}
          fluid
          grayscale
          className="block size-full"
        />
      ) : (
        <Ban size={22} aria-hidden />
      )}
    </span>
  )
}
