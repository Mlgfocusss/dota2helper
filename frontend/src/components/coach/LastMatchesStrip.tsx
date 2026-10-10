import { Link } from 'react-router'
import type { MatchRow } from '@/api/types'
import { Tooltip } from '@/components/ui/Tooltip'
import { useHeroes } from '@/features/useHeroes'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatRelativeTime } from '@/lib/format'
import { formatKda } from '@/lib/match'

/** Полоса последних матчей: квадратик на матч, слева старые, справа новые. */
export function LastMatchesStrip({ matches }: { matches: MatchRow[] }) {
  const { data: heroes } = useHeroes()
  // в ответе матчи идут от новых к старым — на полосе наоборот
  const ordered = [...matches].reverse()

  return (
    <ul className="flex flex-wrap gap-1" aria-label={t.coach.matches.last20}>
      {ordered.map((m) => {
        const hero = heroes?.byId.get(m.hero_id)
        const result = m.win ? t.coach.matches.victory : t.coach.matches.defeat
        return (
          <li key={m.match_id}>
            <Tooltip
              content={
                <span className="block text-left">
                  <span className="block font-semibold">
                    {hero?.localized_name ?? t.heroes.unknown}
                  </span>
                  <span className="block font-mono text-[12px] text-muted">
                    {formatKda(m)} · {formatRelativeTime(m.start_time)}
                  </span>
                </span>
              }
            >
              <Link
                to={`/coach/matches/${m.match_id}`}
                aria-label={`${result}: ${hero?.localized_name ?? t.heroes.unknown}, ${formatKda(m)}`}
                className={cn(
                  'block size-[14px] rounded-[4px] transition-transform hover:scale-125',
                  m.win ? 'bg-win' : 'bg-loss',
                )}
              />
            </Tooltip>
          </li>
        )
      })}
    </ul>
  )
}
