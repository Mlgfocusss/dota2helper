import { Dumbbell, Target } from 'lucide-react'
import type { AnalysisResult } from '@/api/types'
import { ButtonLink } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'
import { formatSigned } from '@/lib/format'
import { MiniMap } from './MiniMap'
import { PlanChecklist } from './PlanChecklist'
import { Timeline } from './Timeline'

type ResultViewProps = {
  result: AnalysisResult
  analysisId: string
  matchId?: number
  isExample?: boolean
}

function ratingColor(rating: number): string {
  if (rating >= 7) return 'var(--win-strong)'
  if (rating <= 4) return 'var(--loss-strong)'
  return 'var(--accent-2)'
}

function RatingRing({ rating }: { rating: number }) {
  const r = 38
  const c = 2 * Math.PI * r
  return (
    <div
      className="relative size-[104px] shrink-0"
      role="img"
      aria-label={`${t.analysis.result.rating}: ${rating} ${t.analysis.result.outOf}`}
    >
      <svg viewBox="0 0 100 100" className="size-full -rotate-90">
        <circle cx="50" cy="50" r={r} fill="none" stroke="var(--surface-2)" strokeWidth="9" />
        <circle
          cx="50"
          cy="50"
          r={r}
          fill="none"
          stroke={ratingColor(rating)}
          strokeWidth="9"
          strokeLinecap="round"
          strokeDasharray={`${(c * Math.min(10, rating)) / 10} ${c}`}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="font-mono text-[30px] font-semibold leading-8 tabular-nums">{rating}</span>
        <span className="text-[11px] text-dim">{t.analysis.result.outOf}</span>
      </div>
    </div>
  )
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: 'win' | 'loss' }) {
  return (
    <div className="rounded-[12px] bg-surface-2 px-3.5 py-3">
      <div className="text-[12px] text-muted">{label}</div>
      <div
        className={cn(
          'mt-0.5 font-mono text-[18px] font-semibold tabular-nums',
          tone === 'win' && 'text-win-strong',
          tone === 'loss' && 'text-loss-strong',
        )}
      >
        {value}
      </div>
    </div>
  )
}

/** Результат разбора (SPEC 6): вердикт, линия, таймлайн с мини-картой, улучшения, план. */
export function ResultView({ result, analysisId, matchId, isExample }: ResultViewProps) {
  const { summary, lanes } = result
  return (
    <div className="space-y-6">
      {isExample && (
        <Card
          padding="none"
          className="flex flex-wrap items-center gap-4 border-accent-2/40 bg-accent-2/10 p-4"
        >
          <p className="min-w-0 flex-1 basis-[260px] text-[15px]">
            {t.analysis.result.exampleBanner}
          </p>
          <ButtonLink to="/coach/deep" variant="accent" size="sm">
            {t.analysis.result.exampleCta}
          </ButtonLink>
        </Card>
      )}

      <Card padding="none" className="p-5 sm:p-6">
        <div className="flex flex-wrap items-center gap-5">
          <RatingRing rating={summary.rating} />
          <div className="min-w-0 flex-1 basis-[280px]">
            <h2 className="caps mb-1.5">{t.analysis.result.verdict}</h2>
            <p className="text-[17px] leading-relaxed">{summary.verdict}</p>
          </div>
        </div>
        {summary.key_stats.length > 0 && (
          <dl className="mt-5 grid grid-cols-2 gap-2.5 sm:grid-cols-3">
            {summary.key_stats.map((s) => (
              <Stat key={s.label} label={s.label} value={s.value} />
            ))}
          </dl>
        )}
      </Card>

      <Card padding="none" className="p-5 sm:p-6">
        <h2 className="caps mb-3">{t.analysis.result.lane}</h2>
        {lanes.text && <p className="text-[15px] leading-relaxed text-muted">{lanes.text}</p>}
        <div className="mt-4 grid grid-cols-3 gap-2.5 max-sm:grid-cols-1">
          <Stat label={t.analysis.result.lh10} value={String(lanes.lh_10)} />
          <Stat label={t.analysis.result.dn10} value={String(lanes.dn_10)} />
          <Stat
            label={t.analysis.result.goldDiff10}
            value={lanes.gold_diff_10 === 0 ? '0' : formatSigned(lanes.gold_diff_10, 0)}
            tone={lanes.gold_diff_10 > 0 ? 'win' : lanes.gold_diff_10 < 0 ? 'loss' : undefined}
          />
        </div>
      </Card>

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_320px]">
        <Timeline episodes={result.episodes} />
        <aside className="xl:sticky xl:top-[88px]">
          <Card padding="none" className="p-4">
            <h2 className="caps mb-3">{t.analysis.result.map}</h2>
            <MiniMap points={result.death_points ?? []} />
          </Card>
        </aside>
      </div>

      <section aria-labelledby="improvements-title">
        <h2 id="improvements-title" className="caps mb-3">
          {t.analysis.result.improvements}
        </h2>
        <div className="grid gap-3 md:grid-cols-3">
          {result.improvements.map((im, i) => (
            <Card key={im.title} padding="none" className="flex flex-col p-5">
              <span className="mb-3 flex size-8 items-center justify-center rounded-[10px] bg-surface-2 font-mono text-[14px] font-semibold text-accent-2">
                {i + 1}
              </span>
              <h3 className="text-card-title">{im.title}</h3>
              <p className="mt-2 text-[14px] text-muted">
                <span className="mr-1 inline-flex align-text-bottom">
                  <Target size={14} aria-hidden />
                </span>
                {im.why}
              </p>
              <p className="mt-3 border-t border-line pt-3 text-[14px]">
                <span className="mr-1 inline-flex align-text-bottom text-win">
                  <Dumbbell size={14} aria-hidden />
                </span>
                <span className="font-semibold">{t.analysis.result.drill}: </span>
                {im.drill}
              </p>
            </Card>
          ))}
        </div>
      </section>

      <PlanChecklist analysisId={analysisId} items={result.next_games_plan} />

      {matchId !== undefined && (
        <div className="flex justify-center pt-2">
          <ButtonLink to={`/coach/matches/${matchId}`} variant="surface">
            {t.analysis.result.toMatch}
          </ButtonLink>
        </div>
      )}
    </div>
  )
}
