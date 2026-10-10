import { Heart, MessageCircle } from 'lucide-react'
import { LatestNewsCard } from '@/components/home/LatestNewsCard'
import { MetaCarousel } from '@/components/home/MetaCarousel'
import { PromoBanner } from '@/components/home/PromoBanner'
import { ButtonAnchor, ButtonLink } from '@/components/ui/Button'
import { Page } from '@/components/ui/Page'
import { useAppConfig } from '@/features/useMe'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { APP_NAME } from '@/lib/config'

// Главная (SPEC 5.1): мета патча, промо, последняя новость, отзыв и поддержка.
export default function HomePage() {
  const { data: config } = useAppConfig()
  const supportUrl = config?.support_url

  return (
    <Page>
      {/* название и аватар уже в шапке — заголовок только для скринридеров */}
      <h1 className="sr-only">{APP_NAME}</h1>
      <div className="space-y-4">
        <MetaCarousel />
        <PromoBanner />
        <LatestNewsCard />
        <div className={cn('grid gap-3', supportUrl ? 'grid-cols-2' : 'grid-cols-1')}>
          <ButtonLink to="/feedback" variant="outline" fullWidth>
            <MessageCircle size={18} aria-hidden />
            {t.home.feedback}
          </ButtonLink>
          {supportUrl && (
            <ButtonAnchor
              href={supportUrl}
              target="_blank"
              rel="noopener noreferrer"
              variant="outline"
              fullWidth
            >
              <Heart size={18} className="text-loss-strong" aria-hidden />
              {t.home.support}
            </ButtonAnchor>
          )}
        </div>
      </div>
    </Page>
  )
}
