import { Layers, SearchX } from 'lucide-react'
import { useParams } from 'react-router'
import { ApiError } from '@/api/client'
import type { ItemRef, MatchDetail, MatchPlayerStat } from '@/api/types'
import { HeroPortrait } from '@/components/dota/HeroPortrait'
import { ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Page } from '@/components/ui/Page'
import { Skeleton } from '@/components/ui/Skeleton'
import { Tooltip } from '@/components/ui/Tooltip'
import { useCoachAccount, useMatchDetail } from '@/features/useCoach'
import { useHero } from '@/features/useHeroes'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatDuration, formatNumber, formatShortDate } from '@/lib/format'
import { formatKda, gameModeName, itemImageUrl } from '@/lib/match'

// Страница матча (SPEC 5.4): шапка, статистика игрока и две команды.
export default function MatchPage() {
  const { matchId = '' } = useParams()
  const id = Number(matchId)
  const { accountId } = useCoachAccount()
  const match = useMatchDetail(id, accountId)

  if (match.isPending) {
    return (
      <Page title={t.coach.match.title}>
        <Skeleton className="h-28" radius={16} />
        <Skeleton className="mt-4 h-40" radius={16} />
        <Skeleton className="mt-4 h-72" radius={16} />
      </Page>
    )
  }
  if (match.isError) {
    const notFound = match.error instanceof ApiError && match.error.status === 404
    return (
      <Page title={t.coach.match.title}>
        <Card padding="none">
          {notFound ? (
            <EmptyState
              icon={SearchX}
              title={t.coach.match.notFound}
              description={t.coach.match.notFoundText}
              action={
                <ButtonLink to="/coach" variant="outline">
                  {t.coach.match.toMatches}
                </ButtonLink>
              }
            />
          ) : (
            <ErrorState
              description={match.error.message}
              onRetry={() => match.refetch()}
              retrying={match.isRefetching}
            />
          )}
        </Card>
      </Page>
    )
  }
  return <MatchBody match={match.data} />
}

function MatchBody({ match }: { match: MatchDetail }) {
  const me = match.players.find((p) => p.is_me)
  const radiant = match.players.filter((p) => p.team === 'radiant')
  const dire = match.players.filter((p) => p.team === 'dire')

  return (
    <Page title={t.coach.match.title}>
      <MatchHeader match={match} me={me} />
      {me && <MyStats player={me} />}
      <TeamTable
        title={t.coach.match.radiant}
        players={radiant}
        score={match.radiant_score}
        won={match.radiant_win}
      />
      <TeamTable
        title={t.coach.match.dire}
        players={dire}
        score={match.dire_score}
        won={!match.radiant_win}
      />
      <ButtonLink
        to={`/coach/deep?match_id=${match.match_id}`}
        size="lg"
        fullWidth
        className="mt-6"
      >
        <Layers size={18} aria-hidden />
        {t.coach.match.deepButton}
      </ButtonLink>
    </Page>
  )
}

function MatchHeader({ match, me }: { match: MatchDetail; me?: MatchPlayerStat }) {
  const hero = useHero(me?.hero_id)
  const result =
    match.win == null ? null : match.win ? t.coach.matches.victory : t.coach.matches.defeat
  return (
    <Card
      padding="none"
      className="flex items-center gap-4 p-4 max-sm:flex-col max-sm:items-stretch"
    >
      {me && (
        <HeroPortrait
          hero={hero}
          width={160}
          height={90}
          radius={10}
          className="max-sm:w-full"
          fluid
        />
      )}
      <div className="min-w-0 flex-1">
        {result && (
          <div className={cn('text-[24px] font-bold', match.win ? 'text-win' : 'text-loss')}>
            {result}
          </div>
        )}
        <div className="text-[15px] text-muted">
          {gameModeName(match.game_mode)} · {formatDuration(match.duration)} ·{' '}
          {formatShortDate(match.start_time)}
        </div>
        <div className="mt-2 flex items-center gap-3 font-mono text-[22px] font-bold tabular-nums">
          <span className="text-win-strong">{match.radiant_score}</span>
          <span className="text-dim">:</span>
          <span className="text-loss-strong">{match.dire_score}</span>
          <span className="ml-1 font-sans text-[12px] font-normal text-muted">
            {t.coach.match.radiant} : {t.coach.match.dire}
          </span>
        </div>
      </div>
    </Card>
  )
}

function MyStats({ player }: { player: MatchPlayerStat }) {
  const stats: [string, string][] = [
    [t.coach.match.kda, formatKda(player)],
    [t.coach.match.gpmXpm, `${player.gpm} / ${player.xpm}`],
    [t.coach.match.lastHits, `${player.last_hits} / ${player.denies}`],
    [t.coach.match.heroDamage, formatNumber(player.hero_damage)],
    [t.coach.match.towerDamage, formatNumber(player.tower_damage)],
    [t.coach.match.healing, formatNumber(player.healing)],
  ]
  return (
    <Card padding="none" className="mt-4 p-5">
      <h2 className="caps">{t.coach.match.yourStats}</h2>
      <dl className="mt-4 grid grid-cols-3 gap-x-4 gap-y-5 max-sm:grid-cols-2">
        {stats.map(([label, value]) => (
          <div key={label}>
            <dt className="text-[13px] text-muted">{label}</dt>
            <dd className="mt-0.5 font-mono text-[18px] font-semibold tabular-nums">{value}</dd>
          </div>
        ))}
      </dl>
      <div className="mt-5 flex items-center gap-2">
        <Items items={player.items} neutral={player.neutral} />
      </div>
    </Card>
  )
}

function TeamTable({
  title,
  players,
  score,
  won,
}: {
  title: string
  players: MatchPlayerStat[]
  score: number
  won: boolean
}) {
  return (
    <Card padding="none" className="mt-4 p-5">
      <div className="flex items-baseline gap-3">
        <h2 className="caps flex-1">{title}</h2>
        <span className={cn('text-[13px] font-semibold', won ? 'text-win' : 'text-loss')}>
          {won ? t.coach.matches.victory : t.coach.matches.defeat}
        </span>
        <span className="font-mono text-[15px] font-bold tabular-nums">{score}</span>
      </div>
      <ul className="mt-3 divide-y divide-line">
        {players.map((p, i) => (
          <li key={`${p.hero_id}-${i}`}>
            <PlayerRow player={p} />
          </li>
        ))}
      </ul>
    </Card>
  )
}

function PlayerRow({ player }: { player: MatchPlayerStat }) {
  const hero = useHero(player.hero_id)
  return (
    <div className={cn('-mx-2 rounded-[10px] px-2 py-3', player.is_me && 'bg-accent-2/10')}>
      <div className="flex items-center gap-3">
        <HeroPortrait hero={hero} width={56} height={32} radius={6} />
        <div className="min-w-0 flex-1">
          <div className="truncate text-[15px] font-semibold">
            {hero?.localized_name ?? t.heroes.unknown}
          </div>
          <div className="truncate text-[13px] text-muted">
            {player.name || t.coach.match.anonymous} · {t.coach.match.level} {player.level}
          </div>
        </div>
        <div className="text-right">
          <div className="font-mono text-[15px] tabular-nums">{formatKda(player)}</div>
          <div className="font-mono text-[12px] tabular-nums text-muted">
            {formatNumber(player.net_worth)}
          </div>
        </div>
      </div>
      <div className="mt-2 pl-[68px]">
        <Items items={player.items} neutral={player.neutral} small />
      </div>
    </div>
  )
}

function Items({
  items,
  neutral,
  small,
}: {
  items: (ItemRef | null)[]
  neutral: ItemRef | null
  small?: boolean
}) {
  const w = small ? 33 : 44
  const h = small ? 24 : 32
  return (
    <div className="flex flex-wrap items-center gap-1" aria-label={t.coach.match.items}>
      {items.map((item, i) => (
        <ItemSlot key={i} item={item} width={w} height={h} />
      ))}
      {neutral && (
        <>
          <span aria-hidden className="mx-1 h-4 w-px bg-line" />
          <ItemSlot item={neutral} width={w} height={h} round />
        </>
      )}
    </div>
  )
}

function ItemSlot({
  item,
  width,
  height,
  round,
}: {
  item: ItemRef | null
  width: number
  height: number
  round?: boolean
}) {
  const frame = 'overflow-hidden bg-surface-2'
  const radius = round ? 9999 : 4
  if (!item) {
    return <span className={frame} style={{ width, height, borderRadius: radius }} aria-hidden />
  }
  return (
    <Tooltip content={item.name}>
      <img
        src={itemImageUrl(item.key)}
        alt={item.name}
        width={width}
        height={height}
        loading="lazy"
        className={cn(frame, 'object-cover')}
        style={{ borderRadius: radius, width: round ? height : width, height }}
      />
    </Tooltip>
  )
}
