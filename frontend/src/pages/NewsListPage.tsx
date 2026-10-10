import { Newspaper } from 'lucide-react'
import { Card, CardLink } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Page, PageHeader } from '@/components/ui/Page'
import { Skeleton } from '@/components/ui/Skeleton'
import { useNews } from '@/features/useContent'
import { t } from '@/i18n/ru'
import { formatShortDate } from '@/lib/format'

// Новости (SPEC 5.1): дата, заголовок и начало текста; клик — полная новость.
export default function NewsListPage() {
  const { data, isPending, isError, refetch, isRefetching } = useNews()

  return (
    <Page title={t.news.title}>
      <PageHeader title={t.news.title} subtitle={t.news.subtitle} />

      {isPending && (
        <div className="space-y-3">
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-[124px]" radius={16} />
          ))}
        </div>
      )}
      {isError && (
        <Card padding="none">
          <ErrorState onRetry={() => refetch()} retrying={isRefetching} />
        </Card>
      )}
      {data?.length === 0 && (
        <Card padding="none">
          <EmptyState icon={Newspaper} title={t.news.empty} description={t.news.emptyText} />
        </Card>
      )}
      {data && data.length > 0 && (
        <ul className="space-y-3">
          {data.map((item) => (
            <li key={item.gid}>
              <CardLink to={`/news/${item.gid}`}>
                <time dateTime={item.date} className="font-mono text-[12px] text-muted">
                  {formatShortDate(item.date)}
                </time>
                <h2 className="text-card-title mt-1">{item.title}</h2>
                {item.excerpt && (
                  <p className="mt-1.5 line-clamp-2 text-[15px] text-muted">{item.excerpt}</p>
                )}
              </CardLink>
            </li>
          ))}
        </ul>
      )}
    </Page>
  )
}
