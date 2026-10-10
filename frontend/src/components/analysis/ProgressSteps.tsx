import { Check, LoaderCircle } from 'lucide-react'
import type { AnalysisStatus } from '@/api/types'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'

const STEPS = ['queued', 'parsing', 'analyzing'] as const

/** Стадии разбора: пройденные — с галочкой, текущая — со спиннером и подсказкой. */
export function ProgressSteps({ status }: { status: AnalysisStatus }) {
  const current = STEPS.indexOf(status as (typeof STEPS)[number])
  return (
    <ol className="space-y-3" aria-label={t.analysis.title}>
      {STEPS.map((step, i) => {
        const state = i < current ? 'done' : i === current ? 'active' : 'todo'
        return (
          <li
            key={step}
            aria-current={state === 'active' ? 'step' : undefined}
            className={cn(
              'flex items-start gap-3 rounded-[14px] border px-4 py-3.5 transition-colors',
              state === 'active' ? 'border-accent-2 bg-accent-2/10' : 'border-line bg-surface',
            )}
          >
            <span
              className={cn(
                'mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full',
                state === 'done' && 'bg-win-strong text-on-accent',
                state === 'active' && 'text-accent-2',
                state === 'todo' && 'border border-line text-dim',
              )}
            >
              {state === 'done' ? (
                <Check size={14} strokeWidth={3} aria-hidden />
              ) : state === 'active' ? (
                <LoaderCircle size={22} className="animate-spin" aria-hidden />
              ) : (
                <span className="text-[12px] font-semibold">{i + 1}</span>
              )}
            </span>
            <div className="min-w-0">
              <div className={cn('font-semibold', state === 'todo' && 'text-muted')}>
                {t.analysis.steps[step]}
              </div>
              {state === 'active' && (
                <div className="mt-0.5 text-[14px] text-muted">{t.analysis.stepHints[step]}</div>
              )}
            </div>
          </li>
        )
      })}
    </ol>
  )
}
