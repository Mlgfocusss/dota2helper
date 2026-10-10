import { Check } from 'lucide-react'
import { useState } from 'react'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'

const key = (analysisId: string) => `d2h_plan_${analysisId}`

function load(analysisId: string, total: number): boolean[] {
  try {
    const raw = JSON.parse(localStorage.getItem(key(analysisId)) ?? '[]') as unknown
    if (Array.isArray(raw)) return Array.from({ length: total }, (_, i) => raw[i] === true)
  } catch {
    // нет доступа к хранилищу или битые данные — начинаем с пустого плана
  }
  return Array.from({ length: total }, () => false)
}

function save(analysisId: string, done: boolean[]) {
  try {
    localStorage.setItem(key(analysisId), JSON.stringify(done))
  } catch {
    // отметки просто не сохранятся
  }
}

/** План на следующие игры: чек-лист, отметки лежат в localStorage по id разбора. */
export function PlanChecklist({ analysisId, items }: { analysisId: string; items: string[] }) {
  const [done, setDone] = useState(() => load(analysisId, items.length))
  const count = done.filter(Boolean).length

  const toggle = (i: number) =>
    setDone((prev) => {
      const next = prev.map((v, j) => (j === i ? !v : v))
      save(analysisId, next)
      return next
    })

  return (
    <section aria-labelledby="plan-title">
      <div className="mb-1 flex items-baseline gap-3">
        <h2 id="plan-title" className="caps">
          {t.analysis.result.plan}
        </h2>
        <span className="ml-auto font-mono text-[12px] text-muted" aria-live="polite">
          {t.analysis.result.planDone(count, items.length)}
        </span>
      </div>
      <p className="mb-3 text-[13px] text-dim">{t.analysis.result.planHint}</p>
      <ul className="space-y-2">
        {items.map((text, i) => (
          <li key={i}>
            <button
              type="button"
              role="checkbox"
              aria-checked={done[i]}
              onClick={() => toggle(i)}
              className="flex w-full items-start gap-3 rounded-[12px] border border-line bg-surface px-4 py-3 text-left transition-colors hover:bg-surface-2"
            >
              <span
                aria-hidden
                className={cn(
                  'mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-[6px] border transition-colors',
                  done[i] ? 'border-win-strong bg-win-strong text-on-accent' : 'border-muted/50',
                )}
              >
                {done[i] && <Check size={14} strokeWidth={3} />}
              </span>
              <span className={cn('text-[15px]', done[i] && 'text-muted line-through')}>
                {text}
              </span>
            </button>
          </li>
        ))}
      </ul>
    </section>
  )
}
