import type { DraftPair } from '@/api/types'
import { HeroIcon } from '@/components/dota/HeroPortrait'
import { Tooltip } from '@/components/ui/Tooltip'
import { useHeroes } from '@/features/useHeroes'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatSigned } from '@/lib/format'

type PairViewProps = {
  pair: DraftPair
  /** vs — матчап (иконка против иконки), with — пара союзников */
  kind: 'vs' | 'with'
}

/** Две иконки героев и значение справа: «⬛ vs ⬛  +4.2». */
export function PairView({ pair, kind }: PairViewProps) {
  const { data } = useHeroes()
  const a = data?.byId.get(pair.a)
  const b = data?.byId.get(pair.b)
  return (
    <div className="flex items-center gap-2">
      <Tooltip content={a?.localized_name ?? t.heroes.unknown}>
        <HeroIcon hero={a} width={36} height={36} radius={10} />
      </Tooltip>
      <span className="text-[12px] font-medium text-dim">
        {kind === 'vs' ? t.drafter.panel.vs : '+'}
      </span>
      <Tooltip content={b?.localized_name ?? t.heroes.unknown}>
        <HeroIcon hero={b} width={36} height={36} radius={10} />
      </Tooltip>
      <span
        className={cn(
          'ml-auto font-mono text-[18px] font-semibold tabular-nums',
          pair.value > 0 ? 'text-win-strong' : pair.value < 0 ? 'text-loss-strong' : 'text-muted',
        )}
      >
        {formatSigned(pair.value)}
      </span>
    </div>
  )
}

/** Блок панели: подпись‑капсом и содержимое. */
export function PanelBlock({
  title,
  children,
  className,
}: {
  title: string
  children: React.ReactNode
  className?: string
}) {
  return (
    <section className={cn('border-t border-line pt-4', className)}>
      <h3 className="caps mb-3">{title}</h3>
      {children}
    </section>
  )
}
