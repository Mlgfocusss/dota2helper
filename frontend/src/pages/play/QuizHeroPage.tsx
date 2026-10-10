import { ArrowLeft, ArrowRight, ChartNoAxesColumn, ChevronRight, Target } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import type { HeroPick, Position } from '@/api/types'
import { HeroPortrait } from '@/components/dota/HeroPortrait'
import { PositionIcon } from '@/components/dota/PositionIcon'
import { QuizShell } from '@/components/quiz/QuizShell'
import { Button, ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Chip } from '@/components/ui/Chip'
import { ErrorState } from '@/components/ui/ErrorState'
import { Page } from '@/components/ui/Page'
import { Skeleton } from '@/components/ui/Skeleton'
import { toast } from '@/features/toast/store'
import { useHeroQuiz, useHeroResult, useLastPosition } from '@/features/useGames'
import { useHeroes } from '@/features/useHeroes'
import { useMe } from '@/features/useMe'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatPercent } from '@/lib/format'
import { isPosition, POSITIONS, positionFull, positionRole } from '@/lib/positions'
import { getLastPosition, joinLabels, optionIndexFromKey, toggleChoice } from '@/lib/quiz'

// Подбор идеального героя (SPEC 5.9): позиция → 4 вопроса → топ-3 героя.
export default function QuizHeroPage() {
  const [params, setParams] = useSearchParams()
  const raw = Number(params.get('position'))
  const position: Position | null = isPosition(raw) ? raw : null

  if (position == null) {
    return <PositionStep onPick={(p) => setParams({ position: String(p) })} />
  }
  // key — при смене позиции квиз начинается заново
  return <Questions key={position} position={position} onChangePosition={() => setParams({})} />
}

function PositionStep({ onPick }: { onPick: (p: Position) => void }) {
  const { data: me } = useMe()
  const loggedIn = Boolean(me?.user)
  const lastServer = useLastPosition(loggedIn)
  const [manual, setManual] = useState(false)

  const last: Position | null = loggedIn ? (lastServer.data?.position ?? null) : getLastPosition()
  const waiting = loggedIn && lastServer.isPending

  return (
    <Page title={t.pages.quizHero.title}>
      <h1 className="text-page-title text-center max-sm:text-left">{t.pages.quizHero.title}</h1>
      <p className="mb-6 mt-1.5 text-center text-muted max-sm:text-left">{t.quiz.hero.subtitle}</p>

      <Card padding="none" className="mx-auto max-w-[680px] p-4">
        {!manual ? (
          <div className="space-y-2">
            {waiting && <Skeleton className="h-16" radius={14} />}
            {last && (
              <ActionRow
                icon={<ChartNoAxesColumn size={22} aria-hidden />}
                title={t.quiz.hero.lastPosition(positionRole(last))}
                onClick={() => onPick(last)}
              />
            )}
            <ActionRow
              icon={<Target size={22} aria-hidden />}
              title={t.quiz.hero.manual}
              onClick={() => setManual(true)}
            />
            {!last && !waiting && (
              <div className="px-2 pt-2 text-center">
                <Link
                  to="/play/quiz/position"
                  className="text-[15px] font-medium text-accent-2 underline-offset-4 hover:underline"
                >
                  {t.quiz.hero.takeQuiz}
                </Link>
              </div>
            )}
          </div>
        ) : (
          <div>
            <h2 className="caps px-2 pb-3 pt-1">{t.quiz.hero.pickPosition}</h2>
            <ul className="space-y-1">
              {POSITIONS.map((pos) => (
                <li key={pos}>
                  <button
                    type="button"
                    onClick={() => onPick(pos)}
                    className="flex w-full items-center gap-3 rounded-[12px] px-3 py-3 text-left transition-colors hover:bg-surface-2"
                  >
                    <PositionIcon position={pos} size={22} />
                    <span className="flex-1 text-[16px]">{positionFull(pos)}</span>
                    <ChevronRight size={18} className="text-muted" aria-hidden />
                  </button>
                </li>
              ))}
            </ul>
            <div className="mt-3 border-t border-line pt-3">
              <Button variant="outline" onClick={() => setManual(false)}>
                <ArrowLeft size={18} aria-hidden />
                {t.quiz.back}
              </Button>
            </div>
          </div>
        )}
      </Card>
    </Page>
  )
}

function ActionRow({
  icon,
  title,
  onClick,
}: {
  icon: React.ReactNode
  title: string
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex w-full items-center gap-4 rounded-[14px] border border-line bg-surface p-4 text-left transition-colors hover:bg-surface-2"
    >
      <span className="flex size-11 shrink-0 items-center justify-center rounded-[12px] bg-surface-2 text-accent-2">
        {icon}
      </span>
      <span className="flex-1 text-[16px] font-medium">{title}</span>
      <ChevronRight size={18} className="text-muted" aria-hidden />
    </button>
  )
}

function Questions({
  position,
  onChangePosition,
}: {
  position: Position
  onChangePosition: () => void
}) {
  const quiz = useHeroQuiz(position)
  const submit = useHeroResult()
  const [index, setIndex] = useState(0)
  const [direction, setDirection] = useState(1)
  const [answers, setAnswers] = useState<Record<string, string[]>>({})
  const [picks, setPicks] = useState<HeroPick[] | null>(null)

  const questions = quiz.data?.questions ?? []
  const question = questions[index]
  const selected = question ? (answers[question.id] ?? []) : []
  const isLast = index === questions.length - 1

  const toggle = (id: string) => {
    if (!question) return
    setAnswers((prev) => ({ ...prev, [question.id]: toggleChoice(prev[question.id] ?? [], id) }))
  }
  const next = () => {
    if (selected.length === 0 || submit.isPending) return
    if (isLast) {
      submit.mutate(
        { position, answers },
        {
          onSuccess: (res) => setPicks(res.heroes),
          onError: () => toast.error(t.quiz.submitError),
        },
      )
    } else {
      setDirection(1)
      setIndex(index + 1)
    }
  }
  const back = () => {
    if (index === 0 || submit.isPending) return
    setDirection(-1)
    setIndex(index - 1)
  }
  const restart = () => {
    setAnswers({})
    setIndex(0)
    setDirection(1)
    setPicks(null)
  }

  // 1–N — выбрать вариант, Enter — «Далее», Backspace — «Назад»
  useEffect(() => {
    if (!question || picks) return
    const onKey = (e: KeyboardEvent) => {
      if (e.metaKey || e.ctrlKey || e.altKey) return
      const opt = optionIndexFromKey(e.key, question.options.length)
      if (opt != null) toggle(question.options[opt].id)
      else if (e.key === 'Enter') next()
      else if (e.key === 'Backspace') {
        e.preventDefault()
        back()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  if (picks) {
    return (
      <HeroResultView
        position={position}
        picks={picks}
        onRestart={restart}
        onChange={onChangePosition}
      />
    )
  }

  return (
    <Page title={t.pages.quizHero.title}>
      <h1 className="text-page-title text-center max-sm:text-left">{t.pages.quizHero.title}</h1>
      <p className="mb-6 mt-1.5 text-center text-muted max-sm:text-left">
        {t.quiz.hero.questionsSubtitle}
      </p>

      {quiz.isPending && <Skeleton className="mx-auto h-80 max-w-[680px]" radius={16} />}
      {quiz.isError && (
        <Card padding="none" className="mx-auto max-w-[680px]">
          <ErrorState
            description={t.quiz.loadError}
            onRetry={() => quiz.refetch()}
            retrying={quiz.isRefetching}
          />
        </Card>
      )}
      {question && (
        <QuizShell
          index={index}
          total={questions.length}
          direction={direction}
          footer={
            <div className="mt-6 flex items-center justify-between gap-3 border-t border-line pt-4">
              {index > 0 ? (
                <Button variant="outline" onClick={back} disabled={submit.isPending}>
                  <ArrowLeft size={18} aria-hidden />
                  {t.quiz.back}
                </Button>
              ) : (
                <span />
              )}
              <Button disabled={selected.length === 0} loading={submit.isPending} onClick={next}>
                {isLast ? t.quiz.showHeroes : t.quiz.next}
                {!isLast && <ArrowRight size={18} aria-hidden />}
              </Button>
            </div>
          }
        >
          <h2 className="text-[22px] font-semibold leading-7">{question.text}</h2>
          <p className="mt-1.5 text-[14px] text-muted">
            {selected.length === 0 ? t.quiz.hero.hintFirst : t.quiz.hero.hintSecond}
          </p>
          <div role="group" aria-label={question.text} className="mt-4 space-y-3">
            {question.options.map((opt, i) => {
              const isOn = selected.includes(opt.id)
              const locked = selected.length >= 2 && !isOn
              return (
                <button
                  key={opt.id}
                  type="button"
                  aria-pressed={isOn}
                  disabled={locked}
                  onClick={() => toggle(opt.id)}
                  className={cn(
                    'flex w-full items-center gap-4 rounded-[14px] border p-4 text-left transition-[background-color,border-color,opacity]',
                    isOn
                      ? 'border-accent-2 bg-surface-2'
                      : 'border-line bg-surface hover:bg-surface-2',
                    locked && 'cursor-not-allowed opacity-40 hover:bg-surface',
                  )}
                >
                  <span className="flex-1 text-[16px] leading-6">{opt.text}</span>
                  <kbd className="hidden shrink-0 font-mono text-[12px] text-dim md:block">
                    {i + 1}
                  </kbd>
                </button>
              )
            })}
          </div>
        </QuizShell>
      )}
    </Page>
  )
}

function HeroResultView({
  position,
  picks,
  onRestart,
  onChange,
}: {
  position: Position
  picks: HeroPick[]
  onRestart: () => void
  onChange: () => void
}) {
  const heroes = useHeroes()

  return (
    <Page title={t.pages.quizHero.title}>
      <h1 className="text-page-title">{t.quiz.hero.resultTitle(position)}</h1>
      <p className="mt-1.5 flex items-center gap-2 text-muted">
        <PositionIcon position={position} size={20} />
        {positionFull(position)}
      </p>

      {picks.length === 0 ? (
        <Card padding="none" className="mt-6 p-6 text-center text-muted">
          {t.quiz.hero.noHeroes}
        </Card>
      ) : (
        <ul className="mt-6 space-y-4">
          {picks.map((p) => {
            const hero = heroes.data?.byId.get(p.hero_id)
            const labels = p.matched.map((tag) => t.tags[tag]).filter(Boolean)
            return (
              <li key={p.hero_id}>
                <Card padding="none" className="flex gap-5 overflow-hidden p-4 max-sm:flex-col">
                  <div className="sm:w-[240px] sm:shrink-0">
                    <HeroPortrait hero={hero} width={480} height={270} radius={12} fluid />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-baseline gap-3">
                      <h2 className="text-[22px] font-semibold">
                        {hero?.localized_name ?? t.heroes.unknown}
                      </h2>
                      {p.winrate != null && (
                        <span className="text-[14px] text-muted">
                          <b className="font-semibold text-fg tabular-nums">
                            {formatPercent(p.winrate)}
                          </b>{' '}
                          {t.quiz.hero.winrate}
                        </span>
                      )}
                    </div>
                    <div className="mt-2 flex flex-wrap gap-1.5">
                      {p.complexity !== 2 && (
                        <Chip tone="accent" size="sm">
                          {p.complexity === 1 ? t.quiz.hero.easy : t.quiz.hero.hard}
                        </Chip>
                      )}
                      {p.matched.map((tag) =>
                        t.tags[tag] ? (
                          <Chip key={tag} size="sm">
                            {t.tags[tag]}
                          </Chip>
                        ) : null,
                      )}
                    </div>
                    <p className="mt-3 text-[15px] text-muted">
                      {labels.length > 0
                        ? t.quiz.hero.why(joinLabels(labels.slice(0, 3)))
                        : t.quiz.hero.whyNoTags}
                    </p>
                    <ButtonLink
                      to={`/tools/heroes/${p.hero_id}`}
                      variant="outline"
                      size="sm"
                      className="mt-4"
                    >
                      {t.quiz.hero.details}
                    </ButtonLink>
                  </div>
                </Card>
              </li>
            )
          })}
        </ul>
      )}

      <div className="mt-8 flex flex-wrap gap-3">
        <Button variant="surface" onClick={onRestart}>
          {t.quiz.hero.retry}
        </Button>
        <Button variant="surface" onClick={onChange}>
          {t.quiz.hero.manual}
        </Button>
        <ButtonLink to="/play" variant="outline">
          <ArrowLeft size={18} aria-hidden />
          {t.quiz.toQuizzes}
        </ButtonLink>
      </div>
    </Page>
  )
}
