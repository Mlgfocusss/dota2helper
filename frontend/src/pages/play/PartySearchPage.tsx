import { SearchX, UserRoundPlus, Users } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router'
import type { PartyCard } from '@/api/types'
import { SteamLoginButton } from '@/components/auth/SteamLoginButton'
import { ContactSheet } from '@/components/party/ContactSheet'
import { PartyCardView } from '@/components/party/PartyCardView'
import { PartyFiltersBar } from '@/components/party/PartyFiltersBar'
import { ReportSheet } from '@/components/party/ReportSheet'
import { Button, ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Page, PageHeader } from '@/components/ui/Page'
import { Segmented } from '@/components/ui/Segmented'
import { Skeleton } from '@/components/ui/Skeleton'
import { useMe } from '@/features/useMe'
import { usePartyMe, usePartySearch } from '@/features/useParty'
import { t } from '@/i18n/ru'
import {
  activeFilterCount,
  applyFilters,
  DEFAULT_FILTERS,
  parseFilters,
  parseTab,
  type PartyFilters,
  type SearchTab,
} from '@/lib/party'

const TAB_ITEMS = [
  { value: 'team' as const, label: t.party.tabs.team },
  { value: 'players' as const, label: t.party.tabs.players },
]

const GRID = 'grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3'

// Поиск пати (SPEC 4.4): две вкладки — «Найти команду» и «Найти игрока», фильтры в URL, подгрузка списка.
export default function PartySearchPage() {
  const { data: me, isPending: mePending, isError: meError, refetch: refetchMe } = useMe()
  const loggedIn = Boolean(me?.user)
  const header = (
    <PageHeader
      title={t.party.title}
      subtitle={t.party.subtitle}
      icon={<Users size={30} className="text-accent-2" aria-hidden />}
      actions={
        loggedIn && (
          <ButtonLink to="/profile/party-card" variant="surface" size="sm">
            {t.party.myCard}
          </ButtonLink>
        )
      }
    />
  )

  if (mePending) {
    return (
      <Page width={1200} title={t.party.title}>
        {header}
        <div className={GRID}>
          <CardSkeletons count={3} />
        </div>
      </Page>
    )
  }

  if (meError) {
    return (
      <Page width={1200} title={t.party.title}>
        {header}
        <Card padding="none">
          <ErrorState onRetry={() => refetchMe()} />
        </Card>
      </Page>
    )
  }

  if (!loggedIn) {
    return (
      <Page width={1200} title={t.party.title}>
        {header}
        <Card padding="none">
          <EmptyState
            icon={Users}
            title={t.party.guestTitle}
            description={t.party.guestText}
            action={<SteamLoginButton size="lg" />}
          />
        </Card>
      </Page>
    )
  }

  return (
    <Page width={1200} title={t.party.title}>
      {header}
      <SearchView />
    </Page>
  )
}

function CardSkeletons({ count }: { count: number }) {
  return Array.from({ length: count }, (_, i) => (
    <Skeleton key={i} radius={16} className="h-[300px]" />
  ))
}

function SearchView() {
  const [params, setParams] = useSearchParams()
  const tab = parseTab(params.get('tab'))
  const filters = useMemo(() => parseFilters(params), [params])
  const myCard = usePartyMe()
  const search = usePartySearch(tab, filters, true)
  const [contact, setContact] = useState<PartyCard | null>(null)
  const [report, setReport] = useState<PartyCard | null>(null)

  const update = useCallback(
    (mutate: (p: URLSearchParams) => URLSearchParams) =>
      setParams((prev) => mutate(prev), { replace: true, preventScrollReset: true }),
    [setParams],
  )
  const setTab = (next: SearchTab) =>
    update((p) => {
      const out = new URLSearchParams(p)
      out.set('tab', next)
      return out
    })
  const setFilters = (next: PartyFilters) => update((p) => applyFilters(p, next))

  const cards = search.data?.pages.flatMap((page) => page.items) ?? []
  const noCard = myCard.data && myCard.data.card === null

  // подгрузка: когда нижний край списка подходит к экрану, берём следующую страницу
  const sentinel = useRef<HTMLDivElement>(null)
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = search
  useEffect(() => {
    const el = sentinel.current
    if (!el || !hasNextPage) return
    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting && !isFetchingNextPage) void fetchNextPage()
      },
      { rootMargin: '400px' },
    )
    io.observe(el)
    return () => io.disconnect()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage, cards.length])

  return (
    <div className="space-y-4">
      {noCard && (
        <Card padding="none" className="flex flex-wrap items-center gap-4 p-5">
          <span className="flex size-12 shrink-0 items-center justify-center rounded-[14px] bg-surface-2 text-accent-2">
            <UserRoundPlus size={24} aria-hidden />
          </span>
          <div className="min-w-0 flex-1 basis-[240px]">
            <h2 className="text-card-title">{t.party.noCardTitle}</h2>
            <p className="text-[14px] text-muted">{t.party.noCardText}</p>
          </div>
          <ButtonLink to="/profile/party-card" variant="accent">
            {t.party.createCard}
          </ButtonLink>
        </Card>
      )}

      <div>
        <Segmented items={TAB_ITEMS} value={tab} onChange={setTab} aria-label={t.party.tabsLabel} />
        <p className="mt-2 text-[14px] text-muted">{t.party.tabHints[tab]}</p>
      </div>

      <PartyFiltersBar value={filters} onChange={setFilters} />

      {search.isPending ? (
        <div className={GRID}>
          <CardSkeletons count={6} />
        </div>
      ) : search.isError ? (
        <Card padding="none">
          <ErrorState onRetry={() => search.refetch()} retrying={search.isRefetching} />
        </Card>
      ) : cards.length === 0 ? (
        <Card padding="none">
          <EmptyState
            icon={SearchX}
            title={t.party.emptyTitle}
            description={t.party.emptyText}
            action={
              activeFilterCount(filters) > 0 && (
                <Button variant="surface" onClick={() => setFilters(DEFAULT_FILTERS)}>
                  {t.party.emptyReset}
                </Button>
              )
            }
          />
        </Card>
      ) : (
        <>
          <div className={GRID}>
            {cards.map((card) => (
              <PartyCardView
                key={card.user_id}
                card={card}
                onContact={() => setContact(card)}
                onReport={() => setReport(card)}
              />
            ))}
            {isFetchingNextPage && <CardSkeletons count={3} />}
          </div>
          {hasNextPage && (
            <div ref={sentinel} className="flex justify-center py-2">
              <Button
                variant="surface"
                loading={isFetchingNextPage}
                onClick={() => void fetchNextPage()}
              >
                {t.party.loadMore}
              </Button>
            </div>
          )}
        </>
      )}

      <ContactSheet card={contact} onClose={() => setContact(null)} />
      <ReportSheet card={report} onClose={() => setReport(null)} />
    </div>
  )
}
