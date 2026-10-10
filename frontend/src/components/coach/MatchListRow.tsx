import { Link } from 'react-router'
import type { MatchRow } from '@/api/types'
import { AnalysisStatusIcon } from '@/components/analysis/AnalysisStatusIcon'
import { HeroPortrait } from '@/components/dota/HeroPortrait'
import { useHero } from '@/features/useHeroes'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatDuration, formatRelativeTime } from '@/lib/format'
import { formatKda } from '@/lib/match'

/** Строка списка матчей: герой, результат, время и K/D/A. Ведёт на страницу матча. */
export function MatchListRow({ match }: { match: MatchRow }) {
  const hero = useHero(match.hero_id)
  return (
    <Link
      to={`/coach/matches/${match.match_id}`}
      className="-mx-3 flex items-center gap-4 rounded-[12px] px-3 py-3 transition-colors hover:bg-surface-2"
    >
      <div className="flex w-14 shrink-0 flex-col items-center gap-1.5">
        <HeroPortrait hero={hero} width={56} height={32} radius={6} />
        <span
          className={cn(
            'rounded-full px-2 py-px text-[11px] font-semibold leading-4',
            match.win ? 'bg-win-strong/12 text-win' : 'bg-loss-strong/12 text-loss',
          )}
        >
          {match.win ? t.coach.matches.victory : t.coach.matches.defeat}
        </span>
      </div>
      <div className="min-w-0 flex-1">
        <div className="truncate font-semibold">{hero?.localized_name ?? t.heroes.unknown}</div>
        <div className="mt-0.5 text-[14px] text-muted">
          {formatRelativeTime(match.start_time)} · {formatDuration(match.duration)}
        </div>
      </div>
      <div className="shrink-0 font-mono text-[15px] tabular-nums">{formatKda(match)}</div>
      {match.analysis && <AnalysisStatusIcon status={match.analysis.status} />}
    </Link>
  )
}
