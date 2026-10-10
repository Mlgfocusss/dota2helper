import { Hammer } from 'lucide-react'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { Page, PageHeader } from '@/components/ui/Page'
import { t } from '@/i18n/ru'

type StubKey = Exclude<keyof typeof t.pages, 'uiKit'>

type StubPageProps = {
  page: StubKey
  /** Ширина колонки из SPEC 2.5 — заглушка сразу занимает место будущей страницы */
  width?: number
}

/** Временная страница раздела, который ещё не реализован. */
export function StubPage({ page, width = 760 }: StubPageProps) {
  const meta = t.pages[page]
  const subtitle = 'subtitle' in meta ? meta.subtitle : undefined
  return (
    <Page width={width} title={page === 'home' ? undefined : meta.title}>
      <PageHeader title={meta.title} subtitle={subtitle} />
      <Card padding="none">
        <EmptyState icon={Hammer} title={t.stub.title} description={t.stub.text(meta.soon)} />
      </Card>
    </Page>
  )
}
