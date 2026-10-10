import { Plus, Trash2, UserRound, X } from 'lucide-react'
import { useId, useRef, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router'
import { ApiError } from '@/api/client'
import type { PartyCardInput, PartyLooking } from '@/api/types'
import { SteamLoginButton } from '@/components/auth/SteamLoginButton'
import { HeroPicker } from '@/components/dota/HeroPicker'
import { HeroPortrait } from '@/components/dota/HeroPortrait'
import { PositionToggles } from '@/components/party/PositionToggles'
import { RankPicker } from '@/components/party/RankPicker'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Chip } from '@/components/ui/Chip'
import { EmptyState } from '@/components/ui/EmptyState'
import { ErrorState } from '@/components/ui/ErrorState'
import { Field, Input, Textarea } from '@/components/ui/Field'
import { Page, PageHeader } from '@/components/ui/Page'
import { Sheet } from '@/components/ui/Sheet'
import { Skeleton } from '@/components/ui/Skeleton'
import { Toggle } from '@/components/ui/Toggle'
import { toast } from '@/features/toast/store'
import { useHeroes } from '@/features/useHeroes'
import { useMe } from '@/features/useMe'
import { useDeleteCard, usePartyMe, useSaveCard } from '@/features/useParty'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import {
  ABOUT_MAX,
  emptyForm,
  FAVORITES_MAX,
  formFromCard,
  HOURS_MAX,
  PARTY_GOALS,
  PARTY_MODES,
  toggleIn,
  validateForm,
  type FormErrors,
} from '@/lib/party'

// Анкета пати (SPEC 4.6): создание и редактирование.
export default function PartyCardPage() {
  const { data: me, isPending: mePending } = useMe()
  const loggedIn = Boolean(me?.user)
  const card = usePartyMe(loggedIn)

  const header = <PageHeader title={t.partyCard.title} subtitle={t.partyCard.subtitle} />

  if (mePending || (loggedIn && card.isPending)) {
    return (
      <Page width={1000} title={t.partyCard.title}>
        {header}
        <div className="space-y-4">
          <Skeleton radius={16} className="h-[120px]" />
          <Skeleton radius={16} className="h-[180px]" />
          <Skeleton radius={16} className="h-[180px]" />
        </div>
      </Page>
    )
  }

  if (!loggedIn) {
    return (
      <Page width={1000} title={t.partyCard.title}>
        {header}
        <Card padding="none">
          <EmptyState
            icon={UserRound}
            title={t.partyCard.guestTitle}
            description={t.partyCard.guestText}
            action={<SteamLoginButton size="lg" />}
          />
        </Card>
      </Page>
    )
  }

  if (card.isError) {
    return (
      <Page width={1000} title={t.partyCard.title}>
        {header}
        <Card padding="none">
          <ErrorState onRetry={() => card.refetch()} retrying={card.isRefetching} />
        </Card>
      </Page>
    )
  }

  // данные есть всегда: загрузка и ошибка обработаны выше
  if (!card.data) return null
  const { card: existing, suggested_rank } = card.data
  return (
    <Page width={1000} title={t.partyCard.title}>
      {header}
      <CardForm
        initial={existing ? formFromCard(existing) : emptyForm(suggested_rank)}
        isNew={!existing}
        rankSuggested={!existing && suggested_rank !== null}
      />
    </Page>
  )
}

function Section({
  title,
  hint,
  id,
  children,
}: {
  title: string
  hint?: ReactNode
  id: string
  children: ReactNode
}) {
  const labelId = useId()
  return (
    <Card
      id={`field-${id}`}
      padding="none"
      className="scroll-mt-24 p-5"
      role="group"
      aria-labelledby={labelId}
    >
      <div className="mb-3 flex items-baseline gap-3">
        <h2 id={labelId} className="caps">
          {title}
        </h2>
        {hint && <span className="text-[13px] text-dim">{hint}</span>}
      </div>
      {children}
    </Card>
  )
}

function ErrorLine({ message }: { message?: string }) {
  if (!message) return null
  return (
    <p role="alert" className="mt-2 text-[13px] text-loss">
      {message}
    </p>
  )
}

function CardForm({
  initial,
  isNew,
  rankSuggested,
}: {
  initial: PartyCardInput
  isNew: boolean
  rankSuggested: boolean
}) {
  const navigate = useNavigate()
  const save = useSaveCard()
  const remove = useDeleteCard()
  const { data: heroes } = useHeroes()
  const [form, setForm] = useState(initial)
  const [hoursText, setHoursText] = useState(String(initial.hours))
  const [errors, setErrors] = useState<FormErrors>({})
  const [pickerOpen, setPickerOpen] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const rankLabelId = useId()
  const formRef = useRef<HTMLFormElement>(null)

  const set = <K extends keyof PartyCardInput>(key: K, value: PartyCardInput[K]) => {
    setForm((f) => ({ ...f, [key]: value }))
    // правка поля снимает его ошибку
    setErrors((e) => (e[key] ? { ...e, [key]: undefined } : e))
  }

  const showErrors = (next: FormErrors) => {
    setErrors(next)
    const first = Object.keys(next)[0]
    if (first) {
      formRef.current
        ?.querySelector(
          `#field-${first === 'discord_tag' ? 'contacts' : first === 'telegram' ? 'contacts' : first}`,
        )
        ?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }

  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    const found = validateForm(form, t.partyCard.errors)
    if (Object.keys(found).length > 0) {
      showErrors(found)
      toast.error(t.partyCard.errors.fix)
      return
    }
    save.mutate(
      {
        ...form,
        discord_tag: form.discord ? form.discord_tag.trim() : '',
        telegram: form.telegram.trim(),
      },
      {
        onSuccess: () => {
          toast.success(t.partyCard.saved)
          void navigate('/profile')
        },
        onError: (err) => {
          if (err instanceof ApiError && err.code === 'validation_failed') {
            showErrors(err.fields as FormErrors)
            toast.error(t.partyCard.errors.fix)
          } else {
            toast.error(err instanceof ApiError ? err.message : t.states.unknown)
          }
        },
      },
    )
  }

  const onDelete = () =>
    remove.mutate(undefined, {
      onSuccess: () => {
        toast.success(t.partyCard.deleted)
        void navigate('/profile')
      },
    })

  const looking: PartyLooking[] = ['team', 'players']

  return (
    <form ref={formRef} onSubmit={submit} noValidate className="space-y-4">
      <Section id="looking_for" title={t.partyCard.lookingFor}>
        <div
          role="radiogroup"
          aria-label={t.partyCard.lookingFor}
          className="grid gap-3 sm:grid-cols-2"
        >
          {looking.map((value) => {
            const selected = form.looking_for === value
            const option = t.partyCard.lookingOptions[value]
            return (
              <button
                key={value}
                type="button"
                role="radio"
                aria-checked={selected}
                onClick={() => set('looking_for', value)}
                className={cn(
                  'rounded-[14px] border p-4 text-left transition-colors',
                  selected
                    ? 'border-accent-2 bg-accent-2/15'
                    : 'border-line bg-bg hover:bg-surface-2',
                )}
              >
                <div className="text-[16px] font-semibold">{option.title}</div>
                <div className="text-[14px] text-muted">{option.text}</div>
              </button>
            )
          })}
        </div>
      </Section>

      <Section id="goal" title={t.partyCard.goal}>
        <div role="radiogroup" aria-label={t.partyCard.goal} className="flex flex-wrap gap-2">
          {PARTY_GOALS.map((g) => (
            <Chip key={g} selected={form.goal === g} onClick={() => set('goal', g)}>
              {t.party.goalLabels[g]}
            </Chip>
          ))}
        </div>
      </Section>

      <Section
        id="rank_medal"
        title={t.partyCard.rank}
        hint={rankSuggested ? t.partyCard.rankSuggested : undefined}
      >
        <span id={rankLabelId} className="sr-only">
          {t.partyCard.rank}
        </span>
        <RankPicker
          value={form.rank_medal}
          onChange={(v) => set('rank_medal', v)}
          aria-labelledby={rankLabelId}
        />
      </Section>

      <Section id="hours" title={t.partyCard.hours}>
        <Field
          label={t.partyCard.hours}
          hideLabel
          htmlFor="party-hours"
          hint={t.partyCard.hoursHint}
          error={errors.hours}
        >
          <Input
            id="party-hours"
            inputMode="numeric"
            autoComplete="off"
            value={hoursText}
            aria-invalid={Boolean(errors.hours) || undefined}
            className="max-w-[200px] font-mono"
            onChange={(e) => {
              const text = e.target.value.replace(/\D/g, '').slice(0, 5)
              setHoursText(text)
              set('hours', text === '' ? -1 : Math.min(Number(text), HOURS_MAX + 1))
            }}
          />
        </Field>
      </Section>

      <Section id="positions" title={t.partyCard.positions} hint={t.partyCard.positionsHint}>
        <PositionToggles
          value={form.positions}
          onChange={(v) => set('positions', v)}
          invalid={Boolean(errors.positions)}
        />
        <ErrorLine message={errors.positions} />
      </Section>

      <Section id="modes" title={t.partyCard.modes}>
        <div className="flex flex-wrap gap-2" role="group" aria-label={t.partyCard.modes}>
          {PARTY_MODES.map((m) => (
            <Chip
              key={m}
              selected={form.modes.includes(m)}
              onClick={() => set('modes', toggleIn(form.modes, m, PARTY_MODES))}
            >
              {t.party.modeLabels[m]}
            </Chip>
          ))}
        </div>
        <ErrorLine message={errors.modes} />
      </Section>

      <Section id="contacts" title={t.partyCard.contacts}>
        <div className="space-y-4">
          <div className="flex items-center gap-4">
            <div className="min-w-0 flex-1">
              <div className="text-[15px] font-medium">{t.partyCard.mic}</div>
              <div className="text-[13px] text-muted">{t.partyCard.micHint}</div>
            </div>
            <Toggle label={t.partyCard.mic} checked={form.mic} onChange={(v) => set('mic', v)} />
          </div>

          <div className="border-t border-line pt-4">
            <div className="flex items-center gap-4">
              <div className="min-w-0 flex-1 text-[15px] font-medium">{t.partyCard.discord}</div>
              <Toggle
                label={t.partyCard.discord}
                checked={form.discord}
                onChange={(v) => set('discord', v)}
              />
            </div>
            {form.discord && (
              <Field
                label={t.partyCard.discordPlaceholder}
                htmlFor="party-discord"
                error={errors.discord_tag}
                className="mt-3"
              >
                <Input
                  id="party-discord"
                  value={form.discord_tag}
                  maxLength={40}
                  autoComplete="off"
                  placeholder={t.partyCard.discordPlaceholder}
                  aria-invalid={Boolean(errors.discord_tag) || undefined}
                  onChange={(e) => set('discord_tag', e.target.value)}
                />
              </Field>
            )}
          </div>

          <div className="border-t border-line pt-4">
            <Field label={t.partyCard.telegram} htmlFor="party-telegram" error={errors.telegram}>
              <Input
                id="party-telegram"
                value={form.telegram}
                maxLength={33}
                autoComplete="off"
                placeholder={t.partyCard.telegramPlaceholder}
                aria-invalid={Boolean(errors.telegram) || undefined}
                onChange={(e) => set('telegram', e.target.value)}
              />
            </Field>
          </div>
        </div>
      </Section>

      <Section id="fav_heroes" title={t.partyCard.favorites} hint={t.partyCard.favoritesHint}>
        <div className="flex gap-3">
          {Array.from({ length: FAVORITES_MAX }, (_, i) => {
            const id = form.fav_heroes[i]
            const hero = id === undefined ? undefined : heroes?.byId.get(id)
            if (id === undefined) {
              return (
                <button
                  key={`empty-${i}`}
                  type="button"
                  aria-label={t.partyCard.addHero}
                  onClick={() => setPickerOpen(true)}
                  className="flex size-[88px] items-center justify-center rounded-[14px] border border-dashed border-muted/50 text-muted transition-colors hover:border-accent-2 hover:text-fg"
                >
                  <Plus size={24} aria-hidden />
                </button>
              )
            }
            return (
              <div key={id} className="relative size-[88px]">
                <HeroPortrait hero={hero} width={88} height={88} radius={14} />
                <button
                  type="button"
                  aria-label={t.partyCard.removeHero(hero?.localized_name ?? t.heroes.unknown)}
                  onClick={() =>
                    set(
                      'fav_heroes',
                      form.fav_heroes.filter((x) => x !== id),
                    )
                  }
                  className="absolute -right-2 -top-2 flex size-6 items-center justify-center rounded-full border border-line bg-surface text-muted transition-colors hover:text-fg"
                >
                  <X size={14} aria-hidden />
                </button>
              </div>
            )
          })}
        </div>
        <ErrorLine message={errors.fav_heroes} />
      </Section>

      <Section id="about" title={t.partyCard.about}>
        <Field
          label={t.partyCard.about}
          hideLabel
          htmlFor="party-about"
          hint={t.partyCard.aboutHint}
          error={errors.about}
          counter={{ value: [...form.about].length, max: ABOUT_MAX }}
        >
          <Textarea
            id="party-about"
            value={form.about}
            placeholder={t.partyCard.aboutPlaceholder}
            className="min-h-[120px]"
            aria-invalid={Boolean(errors.about) || undefined}
            onChange={(e) => set('about', e.target.value)}
          />
        </Field>
      </Section>

      <div className="sticky bottom-[calc(var(--nav-h)+var(--nav-offset)+12px+env(safe-area-inset-bottom))] z-10 space-y-3">
        <Button
          type="submit"
          variant="accent"
          size="lg"
          fullWidth
          loading={save.isPending}
          className="shadow-2xl"
        >
          {t.partyCard.save}
        </Button>
      </div>

      {!isNew && (
        <Button type="button" variant="danger" fullWidth onClick={() => setConfirmDelete(true)}>
          <Trash2 size={18} aria-hidden />
          {t.partyCard.delete}
        </Button>
      )}

      <HeroPicker
        open={pickerOpen}
        onClose={() => setPickerOpen(false)}
        disabledIds={form.fav_heroes}
        onPick={(hero) => {
          set('fav_heroes', [...form.fav_heroes, hero.id].slice(0, FAVORITES_MAX))
          setPickerOpen(false)
        }}
      />

      <Sheet
        open={confirmDelete}
        onClose={() => setConfirmDelete(false)}
        title={t.partyCard.deleteTitle}
        width={440}
      >
        <p className="text-[15px] text-muted">{t.partyCard.deleteText}</p>
        <div className="mt-5 grid grid-cols-2 gap-3">
          <Button variant="surface" onClick={() => setConfirmDelete(false)}>
            {t.common.cancel}
          </Button>
          <Button variant="danger" loading={remove.isPending} onClick={onDelete}>
            {t.partyCard.deleteConfirm}
          </Button>
        </div>
      </Sheet>
    </form>
  )
}
