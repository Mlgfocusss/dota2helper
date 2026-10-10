import { ArrowLeft, ChevronLeft, ChevronRight, Info, Trophy } from 'lucide-react'
import { useMemo } from 'react'
import { Link, useSearchParams } from 'react-router'
import type { LeaderboardRow } from '@/api/types'
import { Avatar } from '@/components/ui/Avatar'
import { IconButton } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Page } from '@/components/ui/Page'
import { Skeleton } from '@/components/ui/Skeleton'
import { useLeaderboard } from '@/features/useDrafter'
import { useMe } from '@/features/useMe'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { pluralize } from '@/lib/format'
import { currentMonth, FIRST_MONTH, monthLabel, normalizeMonth, shiftMonth } from '@/lib/months'

function Row({ row, mine, pinned }: { row: LeaderboardRow; mine: boolean; pinned?: boolean }) {
  return (
    <li
      className={cn(
        'flex h-[72px] items-center gap-4 rounded-[14px] border px-5',
        mine ? 'border-accent-2 bg-accent-2/10' : 'border-line bg-surface',
        pinned && 'shadow-2xl backdrop-blur',
      )}
      aria-label={t.drafter.leaderboard.place(row.rank)}
    >
      <span
        className={cn(
          'w-9 shrink-0 text-center font-mono text-[18px] tabular-nums',
          row.rank <= 3 ? 'font-bold text-fg' : 'text-dim',
        )}
      >
        {row.rank}
      </span>
      <Avatar src={row.avatar} name={row.name} size={40} />
      <div className="min-w-0 flex-1">
        <div className="truncate text-[16px] font-medium">
          {row.name || t.profile.noName}
          {mine && (
            <span className="ml-2 text-[12px] font-normal text-accent-2">
              {t.drafter.leaderboard.you}
            </span>
          )}
        </div>
        <div className="text-[13px] text-muted">{pluralize(row.drafts, t.plural.draft)}</div>
      </div>
      <span className="font-mono text-[22px] font-semibold tabular-nums">
        {row.score.toFixed(1)}
      </span>
    </li>
  )
}

// Топ драфтеров месяца (SPEC 4.2): топ‑100, своя строка подсвечена и закреплена, если не в топе.
export default function DrafterTopPage() {
  const [params, setParams] = useSearchParams()
  const month = useMemo(() => normalizeMonth(params.get('month')), [params])
  const isCurrent = month === currentMonth()
  const board = useLeaderboard(month)
  const { data: me } = useMe()
  const myId = me?.user?.id

  const go = (delta: number) =>
    setParams(
      (prev) => {
        const out = new URLSearchParams(prev)
        const next = shiftMonth(month, delta)
        if (next === currentMonth()) out.delete('month')
        else out.set('month', next)
        return out
      },
      { preventScrollReset: true },
    )

  const items = board.data?.items ?? []
  const meRow = board.data?.me ?? null
  const meInTop = meRow !== null && items.some((r) => r.user_id === meRow.user_id)

  return (
    <Page width={1000} title={t.drafter.leaderboard.title}>
      <div className="relative mb-2 flex items-center justify-center">
        <Link
          to="/tools/drafter"
          className="absolute left-0 flex items-center gap-1.5 text-[15px] text-muted transition-colors hover:text-fg"
        >
          <ArrowLeft size={18} aria-hidden />
          {t.common.back}
        </Link>
        <h1 className="text-page-title">{t.drafter.leaderboard.title}</h1>
      </div>

      <div className="mb-3 flex items-center justify-center gap-3">
        <IconButton
          aria-label={t.drafter.leaderboard.prevMonth}
          size="icon-sm"
          disabled={month <= FIRST_MONTH}
          onClick={() => go(-1)}
        >
          <ChevronLeft size={20} />
        </IconButton>
        <span className="caps w-[160px] text-center !text-[14px] !text-fg" aria-live="polite">
          {monthLabel(month)}
        </span>
        <IconButton
          aria-label={t.drafter.leaderboard.nextMonth}
          size="icon-sm"
          disabled={isCurrent}
          onClick={() => go(1)}
        >
          <ChevronRight size={20} />
        </IconButton>
      </div>

      <p className="mb-6 flex items-center justify-center gap-2 text-[14px] text-muted">
        <Info size={16} aria-hidden />
        {t.drafter.leaderboard.info}
      </p>

      {board.isPending ? (
        <ul className="space-y-2">
          {Array.from({ length: 6 }, (_, i) => (
            <Skeleton key={i} radius={14} className="h-[72px]" />
          ))}
        </ul>
      ) : board.isError ? (
        <Card padding="none">
          <ErrorState onRetry={() => board.refetch()} retrying={board.isRefetching} />
        </Card>
      ) : items.length === 0 ? (
        <Card padding="none">
          <EmptyState
            icon={Trophy}
            title={t.drafter.leaderboard.emptyTitle}
            description={t.drafter.leaderboard.emptyText}
          />
        </Card>
      ) : (
        <>
          <ul className="space-y-2">
            {items.map((row) => (
              <Row key={row.user_id} row={row} mine={row.user_id === myId} />
            ))}
          </ul>
          {meRow && !meInTop && (
            <ul className="sticky bottom-[calc(var(--nav-h)+var(--nav-offset)+12px+env(safe-area-inset-bottom))] mt-3">
              <Row row={meRow} mine pinned />
            </ul>
          )}
        </>
      )}
    </Page>
  )
}
