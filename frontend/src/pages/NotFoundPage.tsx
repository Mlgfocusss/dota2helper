import { Compass } from 'lucide-react'
import { ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { Page } from '@/components/ui/Page'
import { t } from '@/i18n/ru'

export default function NotFoundPage() {
  return (
    <Page title={t.notFound.title}>
      <Card padding="none" className="mt-10">
        <EmptyState
          icon={Compass}
          title={t.notFound.title}
          description={t.notFound.text}
          action={<ButtonLink to="/">{t.common.toHome}</ButtonLink>}
        />
      </Card>
    </Page>
  )
}
