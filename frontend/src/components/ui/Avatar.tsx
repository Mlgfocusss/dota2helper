import { useState } from 'react'
import { cn } from '@/lib/cn'

type AvatarProps = {
  src?: string | null
  /** Ник: первая буква — заглушка, если картинки нет */
  name?: string | null
  size?: number
  /** Скругление в px или 'full' */
  radius?: number | 'full'
  className?: string
}

export function Avatar({ src, name, size = 36, radius = 'full', className }: AvatarProps) {
  const [failedSrc, setFailedSrc] = useState<string | null>(null)
  const showImage = Boolean(src) && failedSrc !== src
  const letter = (name?.trim().charAt(0) || '?').toUpperCase()

  return (
    <span
      className={cn(
        'inline-flex shrink-0 select-none items-center justify-center overflow-hidden bg-surface-2 font-semibold text-muted',
        className,
      )}
      style={{
        width: size,
        height: size,
        borderRadius: radius === 'full' ? 9999 : radius,
        fontSize: Math.round(size * 0.42),
      }}
    >
      {showImage ? (
        <img
          src={src ?? undefined}
          alt=""
          width={size}
          height={size}
          loading="lazy"
          className="size-full object-cover"
          onError={() => setFailedSrc(src ?? null)}
        />
      ) : (
        <span aria-hidden>{letter}</span>
      )}
    </span>
  )
}
