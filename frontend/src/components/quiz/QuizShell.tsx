import { AnimatePresence, motion, useReducedMotion } from 'framer-motion'
import type { ReactNode } from 'react'
import { Card } from '@/components/ui/Card'
import { ProgressBar } from '@/components/ui/ProgressBar'
import { t } from '@/i18n/ru'

type QuizShellProps = {
  /** Номер текущего вопроса, с нуля */
  index: number
  total: number
  /** Направление последнего перехода: 1 — вперёд, -1 — назад (для слайда) */
  direction: number
  /** Вопрос и варианты; ключ — номер вопроса */
  children: ReactNode
  footer?: ReactNode
}

/** Карточка квиза: прогресс, «Вопрос N из M» и слайд между вопросами (SPEC 5.8). */
export function QuizShell({ index, total, direction, children, footer }: QuizShellProps) {
  const reduce = useReducedMotion()
  const shift = reduce ? 0 : 40

  return (
    <Card padding="none" className="mx-auto max-w-[680px] p-6 max-sm:p-4">
      <div className="flex items-center gap-4">
        <ProgressBar
          className="flex-1"
          value={(index + 1) / total}
          label={t.quiz.question(index + 1, total)}
        />
        <span className="shrink-0 text-[13px] text-muted tabular-nums">
          {t.quiz.question(index + 1, total)}
        </span>
      </div>
      <div className="relative mt-6 overflow-hidden">
        <AnimatePresence mode="wait" initial={false} custom={direction}>
          <motion.div
            key={index}
            custom={direction}
            initial={{ opacity: 0, x: direction * shift }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -direction * shift }}
            transition={{ duration: reduce ? 0 : 0.18, ease: 'easeOut' }}
          >
            {children}
          </motion.div>
        </AnimatePresence>
      </div>
      {footer}
    </Card>
  )
}
