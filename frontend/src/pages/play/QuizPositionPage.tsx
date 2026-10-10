import { ArrowLeft, ChevronDown } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import type { Position, PositionResult } from '@/api/types'
import { HeroPortrait } from '@/components/dota/HeroPortrait'
import { PositionIcon } from '@/components/dota/PositionIcon'
import { quizIcon } from '@/components/quiz/icons'
import { QuizShell } from '@/components/quiz/QuizShell'
import { Button, ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Chip } from '@/components/ui/Chip'
import { ErrorState } from '@/components/ui/ErrorState'
import { Page } from '@/components/ui/Page'
import { Skeleton } from '@/components/ui/Skeleton'
import { toast } from '@/features/toast/store'
import { usePositionQuiz, usePositionResult } from '@/features/useGames'
import { useHeroes } from '@/features/useHeroes'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatPercent } from '@/lib/format'
import { POSITIONS, positionColor, positionRole } from '@/lib/positions'
import { optionIndexFromKey, setLastPosition } from '@/lib/quiz'

// Квиз «Какая твоя позиция?» (SPEC 5.8): 5 вопросов, подсчёт на сервере.
export default function QuizPositionPage() {
  const quiz = usePositionQuiz()
  const submit = usePositionResult()
  const [index, setIndex] = useState(0)
  const [direction, setDirection] = useState(1)
  const [answers, setAnswers] = useState<Record<string, string>>({})
  const [result, setResult] = useState<PositionResult | null>(null)

  const questions = quiz.data?.questions ?? []
  const question = questions[index]
  const isLast = index === questions.length - 1

  const finish = (all: Record<string, string>) =>
    submit.mutate(all, {
      onSuccess: (res) => {
        setLastPosition(res.primary)
        setResult(res)
      },
      onError: () => toast.error(t.quiz.submitError),
    })

  const choose = (optionId: string) => {
    if (!question || submit.isPending) return
    const next = { ...answers, [question.id]: optionId }
    setAnswers(next)
    if (isLast) {
      finish(next)
    } else {
      setDirection(1)
      setIndex(index + 1)
    }
  }
  const goBack = () => {
    if (index === 0 || submit.isPending) return
    setDirection(-1)
    setIndex(index - 1)
  }
  const restart = () => {
    setAnswers({})
    setIndex(0)
    setDirection(1)
    setResult(null)
  }

  // 1–4 — выбрать вариант, Enter — «Далее», если ответ уже есть, Backspace — «Назад» (SPEC 2.5)
  useEffect(() => {
    if (!question || result) return
    const onKey = (e: KeyboardEvent) => {
      if (e.metaKey || e.ctrlKey || e.altKey) return
      const opt = optionIndexFromKey(e.key, question.options.length)
      if (opt != null) {
        choose(question.options[opt].id)
      } else if (e.key === 'Enter' && answers[question.id] && !isLast) {
        setDirection(1)
        setIndex(index + 1)
      } else if (e.key === 'Backspace') {
        e.preventDefault()
        goBack()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  if (result) return <PositionResultView result={result} onRestart={restart} />

  return (
    <Page title={t.pages.quizPosition.title}>
      <h1 className="text-page-title mb-6 text-center max-sm:text-left">
        {t.pages.quizPosition.title}
      </h1>
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
            index > 0 && (
              <div className="mt-6 border-t border-line pt-4">
                <Button variant="outline" onClick={goBack} disabled={submit.isPending}>
                  <ArrowLeft size={18} aria-hidden />
                  {t.quiz.back}
                </Button>
              </div>
            )
          }
        >
          <h2 className="text-[22px] font-semibold leading-7">{question.text}</h2>
          <div role="radiogroup" aria-label={question.text} className="mt-5 space-y-3">
            {question.options.map((opt, i) => {
              const Icon = quizIcon(opt.icon)
              const selected = answers[question.id] === opt.id
              return (
                <button
                  key={opt.id}
                  type="button"
                  role="radio"
                  aria-checked={selected}
                  disabled={submit.isPending}
                  onClick={() => choose(opt.id)}
                  className={cn(
                    'flex w-full items-center gap-4 rounded-[14px] border p-4 text-left transition-colors hover:bg-surface-2',
                    selected ? 'border-accent-2 bg-surface-2' : 'border-line bg-surface',
                  )}
                >
                  <span className="flex size-10 shrink-0 items-center justify-center rounded-[10px] bg-surface-2 text-accent-2">
                    <Icon size={20} aria-hidden />
                  </span>
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

function PositionChip({ position, large }: { position: Position; large?: boolean }) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-2 rounded-full bg-surface-2 font-semibold',
        large ? 'px-5 py-2.5 text-[22px]' : 'px-4 py-2 text-[17px]',
      )}
      style={{ boxShadow: `inset 0 0 0 1.5px ${positionColor(position)}` }}
    >
      <PositionIcon position={position} size={large ? 26 : 20} />
      {positionRole(position)}
    </span>
  )
}

function PositionResultView({
  result,
  onRestart,
}: {
  result: PositionResult
  onRestart: () => void
}) {
  const heroes = useHeroes()
  const [open, setOpen] = useState(false)

  return (
    <Page title={t.pages.quizPosition.title}>
      <Card padding="none" className="mx-auto max-w-[680px] p-8 text-center max-sm:p-5">
        <p className="caps">{t.quiz.position.yourPosition}</p>
        <div className="mt-4">
          <PositionChip position={result.primary} large />
        </div>
        {result.secondary && (
          <>
            <p className="caps mt-8">{t.quiz.position.extra}</p>
            <div className="mt-3">
              <PositionChip position={result.secondary} />
            </div>
          </>
        )}

        <Button
          variant="surface"
          className="mt-8"
          aria-expanded={open}
          onClick={() => setOpen((v) => !v)}
        >
          {open ? t.quiz.position.hideDetails : t.quiz.position.showDetails}
          <ChevronDown
            size={18}
            className={cn('transition-transform', open && 'rotate-180')}
            aria-hidden
          />
        </Button>

        {open && (
          <div className="mt-6 space-y-6 text-left">
            <p className="text-[16px] leading-relaxed text-muted">{result.description}</p>

            <div>
              <h3 className="caps mb-3">{t.quiz.position.shares}</h3>
              <ul className="space-y-2.5">
                {POSITIONS.map((pos, i) => (
                  <li key={pos} className="flex items-center gap-3">
                    <PositionIcon position={pos} size={20} />
                    <span className="w-24 shrink-0 text-[14px] max-sm:w-20">
                      {positionRole(pos)}
                    </span>
                    <span className="h-2 flex-1 overflow-hidden rounded-full bg-surface-2">
                      <span
                        className="block h-full rounded-full"
                        style={{
                          width: `${Math.min(100, result.shares[i])}%`,
                          background: positionColor(pos),
                        }}
                      />
                    </span>
                    <span className="w-10 shrink-0 text-right font-mono text-[13px] tabular-nums text-muted">
                      {Math.round(result.shares[i])}%
                    </span>
                  </li>
                ))}
              </ul>
            </div>

            {result.heroes.length > 0 && (
              <div>
                <h3 className="caps mb-3">{t.quiz.position.topHeroes}</h3>
                <ul className="grid grid-cols-4 gap-2 max-sm:grid-cols-2">
                  {result.heroes.map((h) => {
                    const hero = heroes.data?.byId.get(h.hero_id)
                    return (
                      <li key={h.hero_id}>
                        <Link
                          to={`/tools/heroes/${h.hero_id}`}
                          className="group block rounded-[12px] border border-line bg-surface p-2 transition-colors hover:bg-surface-2"
                        >
                          <HeroPortrait hero={hero} width={160} height={90} radius={8} fluid />
                          <div className="mt-2 truncate text-[13px] font-medium">
                            {hero?.localized_name ?? t.heroes.unknown}
                          </div>
                          <div className="flex items-center gap-1.5 text-[12px] text-muted">
                            <span className="tabular-nums">{formatPercent(h.winrate)}</span>
                            {h.difficulty && (
                              <Chip size="sm" tone="outline">
                                {h.difficulty === 'easy'
                                  ? t.quiz.position.easy
                                  : t.quiz.position.hard}
                              </Chip>
                            )}
                          </div>
                        </Link>
                      </li>
                    )
                  })}
                </ul>
              </div>
            )}

            <ButtonLink
              to={`/play/quiz/hero?position=${result.primary}`}
              variant="accent2"
              fullWidth
            >
              {t.quiz.position.pickHero}
            </ButtonLink>
          </div>
        )}

        <div className="mt-8 flex flex-wrap justify-center gap-3 border-t border-line pt-6">
          <ButtonLink to="/play" variant="outline">
            <ArrowLeft size={18} aria-hidden />
            {t.quiz.toQuizzes}
          </ButtonLink>
          <Button variant="surface" onClick={onRestart}>
            {t.quiz.position.retry}
          </Button>
        </div>
      </Card>
    </Page>
  )
}
