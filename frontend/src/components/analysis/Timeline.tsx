import { ChevronDown, Lightbulb, TriangleAlert } from 'lucide-react'
import { useState } from 'react'
import type { AnalysisResult } from '@/api/types'
import { Button } from '@/components/ui/Button'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'

type Episode = AnalysisResult['episodes'][number]

const DOT: Record<Episode['severity'], string> = {
  good: 'bg-win-strong',
  minor: 'bg-pos3',
  major: 'bg-loss-strong',
}

const TONE: Record<Episode['severity'], string> = {
  good: 'bg-win-strong/12 text-win',
  minor: 'bg-pos3/15 text-pos3',
  major: 'bg-loss-strong/12 text-loss',
}

/** Таймлайн эпизодов: метка времени слева, цвет по серьёзности, раскрытие по клику. */
export function Timeline({ episodes }: { episodes: Episode[] }) {
  // сразу раскрыт первый эпизод с ошибкой — самое важное видно без кликов
  const firstMajor = episodes.findIndex((e) => e.severity === 'major')
  const [open, setOpen] = useState<Set<number>>(new Set([firstMajor >= 0 ? firstMajor : 0]))
  const allOpen = open.size === episodes.length

  const toggle = (i: number) =>
    setOpen((prev) => {
      const next = new Set(prev)
      if (next.has(i)) next.delete(i)
      else next.add(i)
      return next
    })

  return (
    <section aria-labelledby="timeline-title">
      <div className="mb-3 flex items-center gap-3">
        <h2 id="timeline-title" className="caps">
          {t.analysis.result.timeline}
        </h2>
        <Button
          variant="ghost"
          size="sm"
          className="-my-1.5 ml-auto"
          onClick={() => setOpen(allOpen ? new Set() : new Set(episodes.map((_, i) => i)))}
        >
          {allOpen ? t.analysis.result.collapseAll : t.analysis.result.expandAll}
        </Button>
      </div>

      <ol className="relative space-y-3">
        {/* вертикальная линия времени */}
        <span aria-hidden className="absolute bottom-3 left-[41px] top-3 w-px bg-line" />
        {episodes.map((e, i) => {
          const expanded = open.has(i)
          return (
            <li key={`${e.time}-${i}`} className="relative flex gap-3">
              <span className="w-[34px] shrink-0 pt-4 text-right font-mono text-[13px] tabular-nums text-muted">
                {e.time}
              </span>
              <span
                aria-hidden
                className={cn(
                  'relative z-10 mt-[19px] size-3 shrink-0 rounded-full ring-4 ring-bg',
                  DOT[e.severity],
                )}
              />
              <div className="min-w-0 flex-1 rounded-[14px] border border-line bg-surface">
                <button
                  type="button"
                  aria-expanded={expanded}
                  onClick={() => toggle(i)}
                  className="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-surface-2"
                >
                  <span className="min-w-0 flex-1">
                    <span className="block font-semibold">{e.title}</span>
                    <span
                      className={cn(
                        'mt-1 inline-block rounded-full px-2 py-px text-[11px] font-semibold leading-4',
                        TONE[e.severity],
                      )}
                    >
                      {t.analysis.result.severity[e.severity]}
                    </span>
                  </span>
                  <ChevronDown
                    size={18}
                    aria-hidden
                    className={cn(
                      'shrink-0 text-muted transition-transform',
                      expanded && 'rotate-180',
                    )}
                  />
                </button>
                {expanded && (
                  <div className="space-y-3 border-t border-line px-4 py-3 text-[15px]">
                    <p className="text-muted">{e.what_happened}</p>
                    {e.mistake && (
                      <p className="flex gap-2">
                        <TriangleAlert
                          size={18}
                          aria-hidden
                          className="mt-0.5 shrink-0 text-loss"
                        />
                        <span>
                          <span className="font-semibold text-loss">
                            {t.analysis.result.mistake}:{' '}
                          </span>
                          {e.mistake}
                        </span>
                      </p>
                    )}
                    <p className="flex gap-2">
                      <Lightbulb size={18} aria-hidden className="mt-0.5 shrink-0 text-win" />
                      <span>
                        <span className="font-semibold text-win">{t.analysis.result.better}: </span>
                        {e.better}
                      </span>
                    </p>
                  </div>
                )}
              </div>
            </li>
          )
        })}
      </ol>
    </section>
  )
}
