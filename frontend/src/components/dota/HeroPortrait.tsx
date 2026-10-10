import { useState } from 'react'
import type { Hero } from '@/api/types'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'

type HeroImageProps = {
  hero: Hero | null | undefined
  /** Размеры обязательны: место под картинку резервируется сразу, вёрстка не прыгает */
  width: number
  height: number
  /** Скругление в px */
  radius?: number
  /** Ч/б — для забаненных героев */
  grayscale?: boolean
  /** Тянуться по ширине контейнера (не больше width) с теми же пропорциями */
  fluid?: boolean
  className?: string
}

type Props = HeroImageProps & { variant: 'portrait' | 'icon' }

function HeroImage({
  hero,
  width,
  height,
  radius = 10,
  grayscale,
  fluid,
  className,
  variant,
}: Props) {
  const src = variant === 'portrait' ? hero?.img : hero?.icon
  const [failedSrc, setFailedSrc] = useState<string | null>(null)
  const name = hero?.localized_name ?? t.heroes.unknown
  const showImage = Boolean(src) && failedSrc !== src

  return (
    <span
      className={cn(
        'relative inline-block shrink-0 overflow-hidden bg-surface-2 align-middle',
        className,
      )}
      style={
        fluid
          ? {
              width: '100%',
              maxWidth: width,
              aspectRatio: `${width} / ${height}`,
              borderRadius: radius,
            }
          : { width, height, borderRadius: radius }
      }
    >
      {showImage ? (
        <img
          src={src}
          alt={name}
          width={width}
          height={height}
          loading="lazy"
          decoding="async"
          draggable={false}
          // портреты 256×144: в квадратных слотах держим лицо героя в кадре
          className={cn('size-full object-cover object-top', grayscale && 'grayscale')}
          onError={() => setFailedSrc(src ?? null)}
        />
      ) : (
        <span
          role="img"
          aria-label={name}
          className="flex size-full items-center justify-center font-semibold text-dim"
          style={{ fontSize: Math.max(10, Math.round(Math.min(width, height) * 0.4)) }}
        >
          {hero?.localized_name.charAt(0) ?? '?'}
        </span>
      )}
    </span>
  )
}

/** Портрет героя с CDN Valve (256×144), обрезается под нужный размер. */
export function HeroPortrait(props: HeroImageProps) {
  return <HeroImage {...props} variant="portrait" />
}

/** Маленькая квадратная иконка героя (миникарта, пары, тултипы). */
export function HeroIcon({ radius = 6, ...props }: HeroImageProps) {
  return <HeroImage {...props} radius={radius} variant="icon" />
}
