import { useMemo, useState } from 'react'
import type { PatchHero, PatchNote, PatchNotes as PatchNotesData, PatchSection } from '@/api/types'
import { HeroPortrait } from '@/components/dota/HeroPortrait'
import { Accordion } from '@/components/ui/Accordion'
import { Card } from '@/components/ui/Card'
import { ErrorState } from '@/components/ui/ErrorState'
import { Input } from '@/components/ui/Field'
import { SectionHeader } from '@/components/ui/SectionHeader'
import { Skeleton } from '@/components/ui/Skeleton'
import { Tabs, type TabItem } from '@/components/ui/Tabs'
import { usePatchNotes } from '@/features/useContent'
import { useHeroes } from '@/features/useHeroes'
import { t } from '@/i18n/ru'

type TabId = 'heroes' | 'items' | 'neutral' | 'general'

/** Герои с подробным списком раскрываем сразу, только если патч небольшой. */
const OPEN_ALL_UP_TO = 8

/** Окно патча на странице новости: что именно изменилось в обновлении. */
export function PatchNotes({ version }: { version: string }) {
  const { data, isPending, isError, error, refetch, isRefetching } = usePatchNotes(version)

  if (isError && 'status' in error && error.status === 404) return null
  return (
    <Card padding="none" className="mt-8 p-5">
      <SectionHeader label={`${t.news.patch.title} ${version}`} />
      {isPending && (
        <div className="mt-4 space-y-2.5" aria-hidden>
          <Skeleton className="h-9 w-2/3" />
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-12" radius={12} />
          ))}
        </div>
      )}
      {isError && (
        <ErrorState
          className="py-6"
          description={t.news.patch.error}
          onRetry={() => refetch()}
          retrying={isRefetching}
        />
      )}
      {data && <PatchBody patch={data} />}
    </Card>
  )
}

const hasNotes = (sections: PatchSection[]) => sections.some((s) => s.notes.length > 0)

function PatchBody({ patch }: { patch: PatchNotesData }) {
  const tabs = useMemo(() => {
    const all: (TabItem<TabId> & { show: boolean })[] = [
      {
        value: 'heroes',
        label: t.news.patch.heroes,
        count: patch.heroes.length,
        show: patch.heroes.length > 0,
      },
      { value: 'items', label: t.news.patch.items, show: hasNotes(patch.items) },
      { value: 'neutral', label: t.news.patch.neutral, show: hasNotes(patch.neutral_items) },
      { value: 'general', label: t.news.patch.general, show: hasNotes(patch.general) },
    ]
    return all.filter((tab) => tab.show)
  }, [patch])
  const [selected, setSelected] = useState<TabId | null>(null)
  const tab = tabs.find((x) => x.value === selected)?.value ?? tabs[0]?.value

  if (!tab) return <p className="py-6 text-center text-[15px] text-muted">{t.news.patch.empty}</p>
  return (
    <div className="mt-4">
      <div className="-mx-1 overflow-x-auto px-1">
        <Tabs items={tabs} value={tab} onChange={setSelected} aria-label={t.news.patch.title} />
      </div>
      <div className="mt-5">
        {tab === 'heroes' && <HeroesChanges heroes={patch.heroes} />}
        {tab === 'items' && <Sections sections={patch.items} />}
        {tab === 'neutral' && <Sections sections={patch.neutral_items} />}
        {tab === 'general' && <Sections sections={patch.general} />}
      </div>
    </div>
  )
}

function HeroesChanges({ heroes }: { heroes: PatchHero[] }) {
  const { data } = useHeroes()
  const [query, setQuery] = useState('')

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase()
    return heroes
      .map((h) => ({ change: h, hero: data?.byId.get(h.hero_id) }))
      .filter((r) => !q || (r.hero?.localized_name ?? '').toLowerCase().includes(q))
      .sort((a, b) => (a.hero?.localized_name ?? '').localeCompare(b.hero?.localized_name ?? ''))
  }, [heroes, data, query])

  return (
    <div>
      {heroes.length > OPEN_ALL_UP_TO && (
        <Input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t.news.patch.search}
          aria-label={t.news.patch.search}
          className="mb-2 h-11"
        />
      )}
      {rows.length === 0 ? (
        <p className="py-6 text-center text-[15px] text-muted">{t.news.patch.noHeroFound}</p>
      ) : (
        <ul className="divide-y divide-line">
          {rows.map(({ change, hero }) => (
            <li key={change.hero_id}>
              <Accordion
                defaultOpen={heroes.length <= OPEN_ALL_UP_TO}
                triggerClassName="py-3.5"
                title={
                  <span className="flex items-center gap-3">
                    <HeroPortrait hero={hero} width={64} height={36} radius={6} />
                    <span className="text-[16px] font-semibold">
                      {hero?.localized_name ?? t.heroes.unknown}
                    </span>
                  </span>
                }
              >
                <HeroBody change={change} />
              </Accordion>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function HeroBody({ change }: { change: PatchHero }) {
  return (
    <div className="space-y-4 text-fg">
      {change.notes.length > 0 && <NoteList notes={change.notes} />}
      {change.abilities.map((ability, i) => (
        <div key={`${ability.title}-${i}`}>
          <h4 className="mb-1.5 text-[14px] font-semibold text-accent">{ability.title}</h4>
          <NoteList notes={ability.notes} />
        </div>
      ))}
      {change.talents.length > 0 && (
        <div>
          <h4 className="mb-1.5 text-[14px] font-semibold text-accent">{t.news.patch.talents}</h4>
          <NoteList notes={change.talents} />
        </div>
      )}
    </div>
  )
}

/** Предметы и общие изменения: блок с заголовком; заголовок без заметок — подраздел. */
function Sections({ sections }: { sections: PatchSection[] }) {
  return (
    <div className="space-y-5">
      {sections.map((section, i) =>
        section.notes.length === 0 ? (
          <h3 key={`${section.title}-${i}`} className="caps pt-3">
            {section.title}
          </h3>
        ) : (
          <div key={`${section.title}-${i}`}>
            <h4 className="mb-1.5 text-[15px] font-semibold">{section.title}</h4>
            <NoteList notes={section.notes} />
          </div>
        ),
      )}
    </div>
  )
}

function NoteList({ notes }: { notes: PatchNote[] }) {
  return (
    <ul className="space-y-1.5 text-[15px] leading-relaxed text-muted">
      {notes.map((note, i) => (
        <li
          key={i}
          className="flex gap-2.5"
          style={{ paddingLeft: Math.max(0, note.indent - 1) * 18 }}
        >
          <span aria-hidden className="mt-[0.6em] size-1 shrink-0 rounded-full bg-dim" />
          <span className="min-w-0 [overflow-wrap:anywhere]">{note.text}</span>
        </li>
      ))}
    </ul>
  )
}
