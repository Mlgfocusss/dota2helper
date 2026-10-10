import { ExternalLink, Newspaper } from 'lucide-react'
import { useParams } from 'react-router'
import { ApiError } from '@/api/client'
import { Comments } from '@/components/news/Comments'
import { PatchNotes } from '@/components/news/PatchNotes'
import { ButtonAnchor, ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Page } from '@/components/ui/Page'
import { Skeleton } from '@/components/ui/Skeleton'
import { useNewsItem } from '@/features/useContent'
import { t } from '@/i18n/ru'
import { formatShortDate } from '@/lib/format'

// Новость (SPEC 5.1): заголовок, дата, текст и ссылка на оригинал в Steam.
export default function NewsPage() {
  const { gid = '' } = useParams()
  const { data, isPending, isError, error, refetch, isRefetching } = useNewsItem(gid)
  const notFound = error instanceof ApiError && error.status === 404

  return (
    <Page title={data?.title ?? t.pages.newsItem.title}>
      {isPending && (
        <div aria-hidden>
          <Skeleton className="h-4 w-20" />
          <Skeleton className="mt-3 h-9 w-3/4" />
          <Skeleton className="mt-8 aspect-[2/1] w-full" radius={12} />
          <div className="mt-6 space-y-2.5">
            {[100, 96, 92, 70].map((w) => (
              <Skeleton key={w} className="h-4" style={{ width: `${w}%` }} />
            ))}
          </div>
        </div>
      )}
      {notFound && (
        <Card padding="none">
          <EmptyState
            icon={Newspaper}
            title={t.news.notFound}
            description={t.news.notFoundText}
            action={
              <ButtonLink to="/news" variant="outline">
                {t.news.toList}
              </ButtonLink>
            }
          />
        </Card>
      )}
      {isError && !notFound && (
        <Card padding="none">
          <ErrorState onRetry={() => refetch()} retrying={isRefetching} />
        </Card>
      )}
      {data && (
        <>
          <article>
            <time dateTime={data.date} className="font-mono text-[13px] text-muted">
              {formatShortDate(data.date)}
            </time>
            <h1 className="text-page-title mt-2">{data.title}</h1>
            {/* HTML прошёл белый список тегов на бэке (bluemonday) */}
            <div className="news-content mt-6" dangerouslySetInnerHTML={{ __html: data.html }} />
            {data.url && (
              <ButtonAnchor
                href={data.url}
                target="_blank"
                rel="noopener noreferrer"
                variant="outline"
                className="mt-8"
              >
                <ExternalLink size={18} aria-hidden />
                {t.news.openInSteam}
              </ButtonAnchor>
            )}
          </article>
          {data.patch && <PatchNotes version={data.patch} />}
          <Comments gid={data.gid} />
        </>
      )}
    </Page>
  )
}
