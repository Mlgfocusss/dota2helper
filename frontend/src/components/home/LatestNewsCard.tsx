import { Link } from 'react-router'
import { Card } from '@/components/ui/Card'
import { SectionHeader } from '@/components/ui/SectionHeader'
import { Skeleton } from '@/components/ui/Skeleton'
import { useNews } from '@/features/useContent'
import { t } from '@/i18n/ru'
import { formatRelativeTime } from '@/lib/format'

/** Сколько новостей показываем на главной. */
const LATEST_COUNT = 4

/** «Последние обновления»: несколько свежих новостей Dota 2, у самой новой — зелёная точка. */
export function LatestNewsCard() {
  const { data, isPending, isError, refetch } = useNews()
  const latest = data?.slice(0, LATEST_COUNT)

  return (
    <Card padding="none" className="px-5 pb-3 pt-4">
      <SectionHeader
        label={t.home.latestTitle}
        right={
          <Link
            to="/news"
            className="text-[14px] font-medium text-muted transition-colors hover:text-fg"
          >
            {t.home.allNews}
          </Link>
        }
      />
      {isPending && <Skeleton className="mt-3 mb-2 h-6" />}
      {isError && (
        <p className="mt-2 py-2 text-[15px] text-muted">
          {t.states.errorTitle} ·{' '}
          <button
            type="button"
            onClick={() => refetch()}
            className="font-medium text-fg underline underline-offset-4"
          >
            {t.common.retry}
          </button>
        </p>
      )}
      {data && latest?.length === 0 && (
        <p className="mt-2 py-2 text-[15px] text-muted">{t.news.empty}</p>
      )}
      {latest && latest.length > 0 && (
        <ul className="mt-1.5">
          {latest.map((item, i) => (
            <li key={item.gid}>
              <Link
                to={`/news/${item.gid}`}
                className="-mx-2 flex items-center gap-3 rounded-[10px] px-2 py-2.5 transition-colors hover:bg-surface-2"
              >
                <span
                  aria-hidden
                  className={
                    i === 0
                      ? 'size-2 shrink-0 rounded-full bg-win-strong shadow-[0_0_0_4px_color-mix(in_oklab,var(--win-strong)_18%,transparent)]'
                      : 'size-2 shrink-0 rounded-full bg-line'
                  }
                />
                <span className="min-w-0 flex-1 truncate font-medium">{item.title}</span>
                <span className="shrink-0 font-mono text-[13px] text-muted">
                  {formatRelativeTime(item.date)} →
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}
