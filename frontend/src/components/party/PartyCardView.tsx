import { Ellipsis, Flag, Headphones, MessageCircle, MessageSquare } from 'lucide-react'
import { useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import type { PartyCard } from '@/api/types'
import { HeroIcon } from '@/components/dota/HeroPortrait'
import { PositionIcon } from '@/components/dota/PositionIcon'
import { RankMedal } from '@/components/dota/RankMedal'
import { Avatar } from '@/components/ui/Avatar'
import { Button } from '@/components/ui/Button'
import { Chip } from '@/components/ui/Chip'
import { Menu } from '@/components/ui/Menu'
import { Tooltip } from '@/components/ui/Tooltip'
import { useHeroes } from '@/features/useHeroes'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatNumber, formatRelativeTime, plural } from '@/lib/format'
import { rankName } from '@/lib/ranks'

type PartyCardViewProps = {
  card: PartyCard
  /** В выдаче поиска: кнопка «Связаться» и меню жалобы. Без них — просто превью. */
  onContact?: () => void
  onReport?: () => void
  /** Дополнительный блок под анкетой (кнопки «Редактировать», «Поднять»…) */
  footer?: ReactNode
  className?: string
}

/** Анкета игрока: ранг, часы, позиции, режимы, цель, любимые герои и «О себе». */
export function PartyCardView({
  card,
  onContact,
  onReport,
  footer,
  className,
}: PartyCardViewProps) {
  const { data: heroes } = useHeroes()
  const name = card.persona_name || t.profile.noName
  const looking = t.party.looking[card.looking_for]

  return (
    <article
      aria-label={name}
      className={cn(
        'flex h-full flex-col rounded-[16px] border border-line bg-surface p-5 transition-colors hover:bg-surface-2/60',
        className,
      )}
    >
      <header className="flex items-start gap-3">
        <Avatar src={card.avatar} name={name} size={48} radius={14} />
        <div className="min-w-0 flex-1">
          <h3 className="truncate text-[17px] font-semibold leading-6">{name}</h3>
          <p className="truncate text-[13px] text-muted">
            {rankName(card.rank_medal)} ·{' '}
            {t.party.hours(formatNumber(card.hours), plural(card.hours, t.plural.hour))}
          </p>
        </div>
        <RankMedal rankTier={card.rank_medal * 10} size={36} showStars={false} />
        {onReport && (
          <Menu
            label={t.party.menu}
            triggerClassName="-mr-2 flex size-9 items-center justify-center rounded-[10px] text-muted transition-colors hover:bg-surface-2 hover:text-fg"
            trigger={() => <Ellipsis size={20} aria-hidden />}
            items={[{ label: t.party.report, icon: Flag, onSelect: onReport, tone: 'danger' }]}
          />
        )}
      </header>

      <div className="mt-4 flex flex-wrap gap-1.5">
        <Chip size="sm" tone={card.looking_for === 'players' ? 'accent' : 'default'}>
          {looking}
        </Chip>
        <Chip size="sm" tone="outline">
          {t.party.goalLabels[card.goal]}
        </Chip>
      </div>

      <div className="mt-4 flex items-center gap-3">
        <div className="flex gap-1.5" role="list" aria-label={t.party.positions}>
          {card.positions.map((p) => (
            <span
              key={p}
              role="listitem"
              className="flex size-9 items-center justify-center rounded-[10px] bg-surface-2"
            >
              <PositionIcon position={p} size={22} title={t.positions.full[p]} />
            </span>
          ))}
        </div>
        <div className="ml-auto flex items-center gap-2.5 text-muted">
          {card.mic && (
            <Tooltip content={t.party.micOn}>
              <Headphones size={18} aria-label={t.party.micOn} role="img" />
            </Tooltip>
          )}
          {card.discord && (
            <Tooltip content={t.party.discordOn}>
              <MessageCircle size={18} aria-label={t.party.discordOn} role="img" />
            </Tooltip>
          )}
        </div>
      </div>

      <div className="mt-3 flex flex-wrap gap-1.5">
        {card.modes.map((m) => (
          <Chip key={m} size="sm" tone="outline">
            {t.party.modeLabels[m]}
          </Chip>
        ))}
      </div>

      {card.fav_heroes.length > 0 && (
        <div className="mt-4 flex gap-2">
          {card.fav_heroes.map((id) => (
            <HeroIcon key={id} hero={heroes?.byId.get(id)} width={32} height={32} radius={8} />
          ))}
        </div>
      )}

      {card.about && <About text={card.about} />}

      <div className="mt-auto pt-4">
        <p className="mb-3 text-[12px] text-dim">
          {t.party.updated(formatRelativeTime(card.bumped_at))}
        </p>
        {onContact && (
          <Button variant="accent2" fullWidth onClick={onContact}>
            <MessageSquare size={18} aria-hidden />
            {t.party.contact}
          </Button>
        )}
        {footer}
      </div>
    </article>
  )
}

/** «О себе»: две строки, длинный текст разворачивается. */
function About({ text }: { text: string }) {
  const ref = useRef<HTMLParagraphElement>(null)
  const [expanded, setExpanded] = useState(false)
  const [clamped, setClamped] = useState(false)

  useLayoutEffect(() => {
    const el = ref.current
    if (el && !expanded) setClamped(el.scrollHeight > el.clientHeight + 1)
  }, [text, expanded])

  return (
    <div className="mt-4">
      <p
        ref={ref}
        className={cn(
          'whitespace-pre-line break-words text-[14px] text-muted',
          !expanded && 'line-clamp-2',
        )}
      >
        {text}
      </p>
      {(clamped || expanded) && (
        <button
          type="button"
          aria-expanded={expanded}
          onClick={() => setExpanded((v) => !v)}
          className="mt-1 text-[13px] text-accent-2 hover:underline"
        >
          {expanded ? t.party.aboutLess : t.party.aboutMore}
        </button>
      )}
    </div>
  )
}
