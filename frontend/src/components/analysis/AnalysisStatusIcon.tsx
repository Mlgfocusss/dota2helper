import { CircleAlert, CircleCheck, LoaderCircle, Circle } from 'lucide-react'
import type { AnalysisStatus } from '@/api/types'
import { t } from '@/i18n/ru'
import { cn } from '@/lib/cn'

type Props = {
  status?: AnalysisStatus
  size?: number
  className?: string
}

/** Значок разбора на карточке матча: кружок — нет, спиннер — идёт, галочка — готов. */
export function AnalysisStatusIcon({ status, size = 18, className }: Props) {
  const label = status ? t.coach.matches.analysisStatus[status] : t.coach.matches.noAnalysis
  const common = { size, 'aria-label': label, role: 'img' as const }
  let icon
  switch (status) {
    case 'done':
      icon = <CircleCheck {...common} className="text-win-strong" />
      break
    case 'failed':
      icon = <CircleAlert {...common} className="text-loss" />
      break
    case 'queued':
    case 'parsing':
    case 'analyzing':
      icon = <LoaderCircle {...common} className="animate-spin text-accent-2" />
      break
    default:
      icon = <Circle {...common} className="text-dim" />
  }
  return (
    <span title={label} className={cn('inline-flex shrink-0', className)}>
      {icon}
    </span>
  )
}
