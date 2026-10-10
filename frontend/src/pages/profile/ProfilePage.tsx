import {
  ArrowUpRight,
  BadgeCheck,
  ChevronRight,
  Ellipsis,
  ExternalLink,
  Eye,
  EyeOff,
  LogOut,
  Pencil,
  Search,
  Swords,
  UserRound,
  UserRoundPlus,
  Users,
} from 'lucide-react'
import { Link } from 'react-router'
import type { PartyCard, ProfileStats } from '@/api/types'
import { SteamLoginButton } from '@/components/auth/SteamLoginButton'
import { HeroIcon } from '@/components/dota/HeroPortrait'
import { PositionIcon } from '@/components/dota/PositionIcon'
import { SteamIcon } from '@/components/icons/SteamIcon'
import { PartyCardView } from '@/components/party/PartyCardView'
import { Accordion } from '@/components/ui/Accordion'
import { Avatar } from '@/components/ui/Avatar'
import { Button, ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Chip } from '@/components/ui/Chip'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Menu } from '@/components/ui/Menu'
import { Page } from '@/components/ui/Page'
import { Skeleton } from '@/components/ui/Skeleton'
import { toast } from '@/features/toast/store'
import { useAnalysisQuota } from '@/features/useAnalysis'
import { useHeroes } from '@/features/useHeroes'
import { useLogout, useMe, useAppConfig } from '@/features/useMe'
import { useBumpCard, usePartyMe, useProfileStats, useSetCardActive } from '@/features/useParty'
import { t } from '@/i18n/ru'
import { formatNumber } from '@/lib/format'
import { bumpRemainingMs, formatRemaining } from '@/lib/party'
import { positionFull } from '@/lib/positions'
import { steamId64FromAccountId, steamProfileUrl } from '@/lib/steam'

// Профиль (SPEC 4.5): аккаунты, моя анкета, рекорды игр, квизы и драфтер.
export default function ProfilePage() {
  const { data: me, isPending, isError, refetch, isRefetching } = useMe()
  const logout = useLogout()

  if (isPending) {
    return (
      <Page title={t.pages.profile.title}>
        <div className="flex items-center gap-4">
          <Skeleton radius={18} className="size-[72px]" />
          <div className="flex-1 space-y-2">
            <Skeleton className="h-7 w-1/2" />
            <Skeleton className="h-4 w-1/3" />
          </div>
        </div>
      </Page>
    )
  }

  if (isError) {
    return (
      <Page title={t.pages.profile.title}>
        <Card padding="none">
          <ErrorState onRetry={() => refetch()} retrying={isRefetching} />
        </Card>
      </Page>
    )
  }

  const user = me.user
  if (!user) {
    return (
      <Page title={t.pages.profile.title}>
        <Card padding="none" className="mt-4">
          <EmptyState
            icon={UserRound}
            title={t.profile.guestTitle}
            description={t.profile.guestText}
            action={<SteamLoginButton size="lg" />}
          />
        </Card>
      </Page>
    )
  }

  return (
    <Page title={t.pages.profile.title}>
      <div className="flex items-center gap-4">
        <Avatar src={user.avatar} name={user.persona_name} size={72} radius={18} />
        <div className="min-w-0">
          <h1 className="text-page-title truncate">{user.persona_name || t.profile.noName}</h1>
          <a
            href={steamProfileUrl(user.account_id)}
            target="_blank"
            rel="noreferrer"
            className="text-[14px] text-muted transition-colors hover:text-fg"
          >
            {t.profile.steamId(steamId64FromAccountId(user.account_id))}
          </a>
        </div>
      </div>

      <AccountsCard
        accountId={user.account_id}
        name={user.persona_name || t.profile.noName}
        onLogout={() => logout.mutate()}
      />
      <PartySection />
      <GamesSection />

      <Button
        variant="danger"
        fullWidth
        className="mt-6"
        loading={logout.isPending}
        onClick={() => logout.mutate()}
      >
        {t.auth.logout}
      </Button>
    </Page>
  )
}

function SectionTitle({ children }: { children: string }) {
  return <h2 className="caps mb-3 mt-8">{children}</h2>
}

function IconTile({ children }: { children: React.ReactNode }) {
  return (
    <span className="flex size-10 shrink-0 items-center justify-center rounded-[12px] bg-surface-2 text-accent-2">
      {children}
    </span>
  )
}

// ----- аккаунты ----------------------------------------------------------------

function AccountsCard({
  accountId,
  name,
  onLogout,
}: {
  accountId: number
  name: string
  onLogout: () => void
}) {
  const { data: config } = useAppConfig()
  const quota = useAnalysisQuota(Boolean(config?.features.deep_analysis))
  return (
    <>
      <SectionTitle>{t.profile.accounts}</SectionTitle>
      <Card padding="none" className="divide-y divide-line">
        <div className="flex items-center gap-3 px-5 py-3.5">
          <IconTile>
            <SteamIcon size={20} />
          </IconTile>
          <div className="min-w-0 flex-1">
            <div className="truncate font-medium">{name}</div>
            <div className="flex items-center gap-1 text-[13px] text-win">
              <BadgeCheck size={14} aria-hidden />
              {t.profile.steamLinked}
            </div>
          </div>
          <Menu
            label={t.profile.accountMenu}
            triggerClassName="flex size-10 items-center justify-center rounded-[10px] text-muted transition-colors hover:bg-surface-2 hover:text-fg"
            trigger={() => <Ellipsis size={20} aria-hidden />}
            items={[
              {
                label: t.profile.openInSteam,
                icon: ExternalLink,
                href: steamProfileUrl(accountId),
              },
              { label: t.auth.logout, icon: LogOut, onSelect: onLogout, tone: 'danger' },
            ]}
          />
        </div>

        <Link
          to="/coach"
          className="flex items-center gap-3 px-5 py-3.5 transition-colors hover:bg-surface-2"
        >
          <IconTile>
            <ArrowUpRight size={20} aria-hidden />
          </IconTile>
          <div className="min-w-0 flex-1">
            <div className="font-medium">{t.profile.coach}</div>
            <div className="text-[13px] text-muted">{t.profile.coachHint}</div>
          </div>
          <ChevronRight size={18} className="text-muted" aria-hidden />
        </Link>

        {config?.features.deep_analysis && (
          <div className="flex items-center gap-3 px-5 py-3.5">
            <IconTile>
              <Search size={20} aria-hidden />
            </IconTile>
            <div className="min-w-0 flex-1">
              <div className="font-medium">{t.profile.deepAnalysis}</div>
              <div className="text-[13px] text-muted">
                {quota.data
                  ? t.profile.deepLeft(quota.data.left, quota.data.per_day)
                  : t.profile.deepFree}
              </div>
            </div>
            <ButtonLink to="/coach/deep" variant="surface" size="sm">
              {t.profile.deepOpen}
            </ButtonLink>
          </div>
        )}
      </Card>
    </>
  )
}

// ----- моя анкета --------------------------------------------------------------

function PartySection() {
  const party = usePartyMe()
  return (
    <>
      <SectionTitle>{t.profile.myCard}</SectionTitle>
      {party.isPending ? (
        <Skeleton radius={16} className="h-[180px]" />
      ) : party.isError ? (
        <Card padding="none">
          <ErrorState onRetry={() => party.refetch()} retrying={party.isRefetching} />
        </Card>
      ) : party.data.card ? (
        <MyCard card={party.data.card} />
      ) : (
        <Card padding="none" className="flex flex-wrap items-center gap-4 p-5">
          <IconTile>
            <UserRoundPlus size={20} aria-hidden />
          </IconTile>
          <div className="min-w-0 flex-1 basis-[240px]">
            <h3 className="text-card-title">{t.profile.noCardTitle}</h3>
            <p className="text-[14px] text-muted">{t.profile.noCardText}</p>
          </div>
          <ButtonLink to="/profile/party-card" variant="accent">
            {t.profile.createCard}
          </ButtonLink>
        </Card>
      )}
    </>
  )
}

function MyCard({ card }: { card: PartyCard }) {
  const bump = useBumpCard()
  const setActive = useSetCardActive()
  const remaining = bumpRemainingMs(card)
  const canBump = remaining === 0 && card.is_active

  return (
    <PartyCardView
      card={card}
      className={card.is_active ? undefined : 'opacity-80'}
      footer={
        <div className="space-y-2">
          {!card.is_active && (
            <p className="rounded-[10px] bg-surface-2 px-3 py-2 text-[13px] text-muted">
              <span className="font-medium text-fg">{t.profile.hidden}.</span>{' '}
              {t.profile.hiddenText}
            </p>
          )}
          <div className="grid grid-cols-2 gap-2">
            <ButtonLink to="/profile/party-card" variant="surface">
              <Pencil size={16} aria-hidden />
              {t.profile.editCard}
            </ButtonLink>
            <Button
              variant="surface"
              disabled={!canBump}
              loading={bump.isPending}
              onClick={() =>
                bump.mutate(undefined, { onSuccess: () => toast.success(t.profile.bumped) })
              }
            >
              {canBump || !card.is_active
                ? t.profile.bump
                : t.profile.bumpIn(formatRemaining(remaining))}
            </Button>
            <Button
              variant="surface"
              loading={setActive.isPending}
              onClick={() =>
                setActive.mutate(!card.is_active, {
                  onSuccess: () =>
                    toast.info(card.is_active ? t.profile.hiddenToast : t.profile.shownToast),
                })
              }
            >
              {card.is_active ? <EyeOff size={16} aria-hidden /> : <Eye size={16} aria-hidden />}
              {card.is_active ? t.profile.hide : t.profile.show}
            </Button>
            <ButtonLink to="/play/party" variant="surface">
              <Users size={16} aria-hidden />
              {t.profile.findParty}
            </ButtonLink>
          </div>
        </div>
      }
    />
  )
}

// ----- игры, квизы и драфтер ----------------------------------------------------

const HL_MODES = ['popularity', 'kills', 'deaths'] as const

function GamesSection() {
  const stats = useProfileStats(true)
  return (
    <>
      <SectionTitle>{t.profile.games}</SectionTitle>
      <Card padding="none" className="divide-y divide-line px-5">
        <div className="flex items-center gap-3 py-3.5">
          <IconTile>
            <Swords size={20} aria-hidden />
          </IconTile>
          <div className="min-w-0 flex-1">
            <div className="font-medium">{t.profile.draftBattle}</div>
            <div className="text-[13px] text-muted">{t.profile.calibration}</div>
          </div>
          <Chip tone="outline" size="sm">
            {t.common.soon}
          </Chip>
        </div>

        {stats.isPending ? (
          <div className="space-y-3 py-4">
            <Skeleton className="h-6 w-1/2" />
            <Skeleton className="h-6 w-2/3" />
          </div>
        ) : stats.isError ? (
          <ErrorState
            title={t.profile.statsError}
            description=""
            onRetry={() => stats.refetch()}
            retrying={stats.isRefetching}
          />
        ) : (
          <>
            <Accordion
              title={
                <span className="flex flex-col">
                  <span>{t.profile.higherLower}</span>
                  <span className="text-[13px] font-normal text-muted">{t.profile.bestStreak}</span>
                </span>
              }
            >
              <HigherLowerCells records={stats.data.higher_lower} />
            </Accordion>
            <Accordion title={t.profile.quizzesAndDrafter}>
              <QuizzesAndDrafter stats={stats.data} />
            </Accordion>
          </>
        )}
      </Card>
    </>
  )
}

function HigherLowerCells({ records }: { records: ProfileStats['higher_lower'] }) {
  return (
    <div className="grid grid-cols-3 gap-2">
      {HL_MODES.map((mode) => (
        <div key={mode} className="rounded-[12px] bg-surface-2 px-3 py-3 text-center">
          <div className="font-mono text-[22px] font-semibold tabular-nums text-fg">
            {formatNumber(records[mode] ?? 0)}
          </div>
          <div className="text-[12px] text-muted">{t.profile.modes[mode]}</div>
        </div>
      ))}
    </div>
  )
}

function QuizzesAndDrafter({ stats }: { stats: ProfileStats }) {
  const { data: heroes } = useHeroes()
  const { quiz, drafter } = stats
  return (
    <dl className="space-y-3 text-[14px]">
      <Row label={t.profile.lastPosition}>
        {quiz.position ? (
          <span className="flex items-center gap-2 text-fg">
            <PositionIcon position={quiz.position} size={20} />
            {positionFull(quiz.position)}
          </span>
        ) : (
          t.profile.notTaken
        )}
      </Row>
      <Row label={t.profile.lastHeroes}>
        {quiz.hero ? (
          <span className="flex gap-1.5">
            {quiz.hero.hero_ids.slice(0, 4).map((id) => (
              <HeroIcon key={id} hero={heroes?.byId.get(id)} width={28} height={28} radius={8} />
            ))}
          </span>
        ) : (
          t.profile.notTaken
        )}
      </Row>
      <Row label={t.profile.drafterBest}>
        {drafter.best_score !== null ? (
          <span className="font-mono text-fg">{drafter.best_score.toFixed(1)}</span>
        ) : (
          t.profile.drafterNone
        )}
      </Row>
      {drafter.month_rank !== null && (
        <Row label={t.profile.drafterRank}>
          <span className="font-mono text-fg">#{drafter.month_rank}</span>
        </Row>
      )}
    </dl>
  )
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <dt>{label}</dt>
      <dd className="text-right">{children}</dd>
    </div>
  )
}
