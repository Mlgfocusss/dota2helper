import { useAppConfig } from '@/features/useMe'
import { t } from '@/i18n/ru'

/** Слот под собственный баннер (PROMO_IMAGE_URL, PROMO_LINK). Без картинки не рендерится. */
export function PromoBanner() {
  const { data } = useAppConfig()
  const promo = data?.promo
  if (!promo?.image_url) return null

  // 21:9 на десктопе (760×326), 16:9 на мобиле
  const image = (
    <img
      src={promo.image_url}
      alt={t.home.promoAlt}
      width={760}
      height={326}
      className="aspect-video w-full rounded-[16px] bg-surface object-cover md:aspect-[21/9]"
    />
  )
  if (!promo.link) return image
  return (
    <a
      href={promo.link}
      target="_blank"
      rel="noopener noreferrer"
      className="block rounded-[16px] transition-opacity hover:opacity-90"
    >
      {image}
    </a>
  )
}
