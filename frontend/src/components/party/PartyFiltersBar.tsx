import { ChevronDown, SlidersHorizontal } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Chip } from '@/components/ui/Chip'
import { Select } from '@/components/ui/Field'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { useIsDesktop } from '@/lib/hooks'
import {
  activeFilterCount,
  DEFAULT_FILTERS,
  PARTY_GOALS,
  PARTY_MODES,
  toggleIn,
  type PartyFilters,
} from '@/lib/party'
import { RANK_MEDALS, rankName } from '@/lib/ranks'
import type { PartyGoal } from '@/api/types'
import { PositionToggles } from './PositionToggles'

type PartyFiltersBarProps = {
  value: PartyFilters
  onChange: (value: PartyFilters) => void
}

function Group({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="min-w-0">
      <div className="caps mb-2">{label}</div>
      {children}
    </div>
  )
}

/** Фильтры поиска пати. На десктопе всегда открыты, на мобиле — раскрывающаяся панель. */
export function PartyFiltersBar({ value, onChange }: PartyFiltersBarProps) {
  const isDesktop = useIsDesktop()
  const [open, setOpen] = useState(false)
  const count = activeFilterCount(value)
  const set = (patch: Partial<PartyFilters>) => onChange({ ...value, ...patch })

  const panel = (
    <Card padding="none" className="p-4">
      <div className="flex flex-wrap items-start gap-x-5 gap-y-4">
        <Group label={t.party.rankFrom}>
          <Select
            aria-label={t.party.rankFrom}
            value={value.rankMin}
            className="h-11 w-[150px]"
            onChange={(e) => {
              const v = Number(e.target.value)
              set({ rankMin: v, rankMax: Math.max(v, value.rankMax) })
            }}
          >
            {RANK_MEDALS.map((m) => (
              <option key={m} value={m}>
                {rankName(m)}
              </option>
            ))}
          </Select>
        </Group>
        <Group label={t.party.rankTo}>
          <Select
            aria-label={t.party.rankTo}
            value={value.rankMax}
            className="h-11 w-[150px]"
            onChange={(e) => {
              const v = Number(e.target.value)
              set({ rankMax: v, rankMin: Math.min(v, value.rankMin) })
            }}
          >
            {RANK_MEDALS.map((m) => (
              <option key={m} value={m}>
                {rankName(m)}
              </option>
            ))}
          </Select>
        </Group>
        <Group label={t.party.positions}>
          <PositionToggles
            variant="icon"
            value={value.positions}
            onChange={(positions) => set({ positions })}
          />
        </Group>
        <Group label={t.party.modes}>
          <div className="flex min-h-11 flex-wrap items-center gap-2">
            {PARTY_MODES.map((m) => (
              <Chip
                key={m}
                selected={value.modes.includes(m)}
                onClick={() => set({ modes: toggleIn(value.modes, m, PARTY_MODES) })}
              >
                {t.party.modeLabels[m]}
              </Chip>
            ))}
          </div>
        </Group>
        <Group label={t.party.goals}>
          <Select
            aria-label={t.party.goals}
            value={value.goals[0] ?? ''}
            className="h-11 w-[210px]"
            onChange={(e) => set({ goals: e.target.value ? [e.target.value as PartyGoal] : [] })}
          >
            <option value="">{t.party.anyGoal}</option>
            {PARTY_GOALS.map((g) => (
              <option key={g} value={g}>
                {t.party.goalLabels[g]}
              </option>
            ))}
          </Select>
        </Group>
        <Group label={t.partyCard.contacts}>
          <div className="flex min-h-11 flex-wrap items-center gap-2">
            <Chip selected={value.mic} onClick={() => set({ mic: !value.mic })}>
              {t.party.mic}
            </Chip>
            <Chip selected={value.discord} onClick={() => set({ discord: !value.discord })}>
              {t.party.discord}
            </Chip>
          </div>
        </Group>
        {count > 0 && (
          <Button
            variant="ghost"
            size="sm"
            className="ml-auto"
            onClick={() => onChange(DEFAULT_FILTERS)}
          >
            {t.party.reset}
          </Button>
        )}
      </div>
    </Card>
  )

  if (isDesktop) return panel
  return (
    <div>
      <Button variant="surface" fullWidth aria-expanded={open} onClick={() => setOpen((v) => !v)}>
        <SlidersHorizontal size={18} aria-hidden />
        {t.party.filters}
        {count > 0 && (
          <span className="rounded-full bg-accent-2 px-2 text-[12px] text-on-accent">{count}</span>
        )}
        <ChevronDown
          size={18}
          aria-hidden
          className={cn('ml-auto transition-transform', open && 'rotate-180')}
        />
      </Button>
      {open && <div className="mt-3">{panel}</div>}
    </div>
  )
}
